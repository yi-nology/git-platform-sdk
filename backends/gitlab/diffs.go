package gitlab

import (
	"context"
	"fmt"
	"strconv"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*provider.MergeDiff, error) {
	diffs, _, err := p.client.MergeRequests.ListMergeRequestDiffs(pidOf(owner, repo), int64(number), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "GetCRDiff", err)
	}
	diff := &provider.MergeDiff{}
	for _, c := range diffs {
		additions, deletions := provider.CountDiffLines(c.Diff)
		cf := &provider.ChangedFile{
			OldPath: c.OldPath, NewPath: c.NewPath, Diff: c.Diff,
			Additions: additions, Deletions: deletions,
			IsNew: c.NewFile, IsDeleted: c.DeletedFile, IsRenamed: c.RenamedFile,
		}
		diff.Files = append(diff.Files, cf)
	}
	diff.TotalAdd, diff.TotalDel = provider.SumDiffStats(diff.Files)
	diff.RawDiff = provider.BuildRawDiff(diff.Files)
	return diff, nil
}

// GetCRFiles implements provider.DiffManager.
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*provider.ChangedFile, error) {
	diff, err := p.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

// CreateNote implements provider.DiffManager.
func (p *Provider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	note, _, err := p.client.Notes.CreateMergeRequestNote(pidOf(owner, repo), int64(number),
		&gitlab.CreateMergeRequestNoteOptions{Body: gitlab.Ptr(body)}, gitlab.WithContext(ctx))
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitLab, "CreateNote", err)
	}
	return strconv.FormatInt(note.ID, 10), nil
}

// DeleteNote implements provider.DiffManager.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	nid, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return provider.Wrapf(provider.PlatformGitLab, "DeleteNote", "invalid note ID %q: %v", noteID, err)
	}
	_, err = p.client.Notes.DeleteMergeRequestNote(pidOf(owner, repo), int64(number), nid, gitlab.WithContext(ctx))
	if err != nil {
		return provider.Wrap(provider.PlatformGitLab, "DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts provider.DiscussionOptions) (string, error) {
	pid := pidOf(owner, repo)
	discOpts := &gitlab.CreateMergeRequestDiscussionOptions{Body: gitlab.Ptr(opts.Body)}
	if opts.FilePath != "" {
		position := &gitlab.PositionOptions{
			PositionType: gitlab.Ptr("text"),
			NewPath:      gitlab.Ptr(opts.FilePath),
		}
		if opts.BaseSHA != "" {
			position.BaseSHA = gitlab.Ptr(opts.BaseSHA)
		}
		if opts.StartSHA != "" {
			position.StartSHA = gitlab.Ptr(opts.StartSHA)
		}
		if opts.HeadSHA != "" {
			position.HeadSHA = gitlab.Ptr(opts.HeadSHA)
		}
		if opts.OldLine > 0 {
			position.OldPath = gitlab.Ptr(opts.FilePath)
			position.OldLine = gitlab.Ptr(int64(opts.OldLine))
		}
		if opts.NewLine > 0 {
			position.NewLine = gitlab.Ptr(int64(opts.NewLine))
		}
		if opts.StartNewLine > 0 && opts.NewLine > opts.StartNewLine {
			position.LineRange = &gitlab.LineRangeOptions{
				Start: &gitlab.LinePositionOptions{Type: gitlab.Ptr("new"), NewLine: gitlab.Ptr(int64(opts.StartNewLine))},
				End:   &gitlab.LinePositionOptions{Type: gitlab.Ptr("new"), NewLine: gitlab.Ptr(int64(opts.NewLine))},
			}
		}
		discOpts.Position = position
	}
	disc, _, err := p.client.Discussions.CreateMergeRequestDiscussion(pid, int64(number), discOpts, gitlab.WithContext(ctx))
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitLab, "CreateDiscussion", err)
	}
	return disc.ID, nil
}

// CreateReview implements provider.DiffManager.
//
// GitLab has no native "review" object. We approximate one by:
//  1. Posting the summary body as a Note (if non-empty).
//  2. Posting each inline comment as a Discussion.
//  3. Setting a commit status with the verdict (if CommitID is set).
//
// The returned ID is a synthetic identifier; the Comments slice reports the
// per-comment discussion IDs and any errors.
func (p *Provider) CreateReview(ctx context.Context, owner, repo string, number int, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	pid := pidOf(owner, repo)

	var baseSHA, startSHA, headSHA string
	mr, _, err := p.client.MergeRequests.GetMergeRequest(pid, int64(number), nil, gitlab.WithContext(ctx))
	if err == nil && mr != nil {
		headSHA = mrHeadSHA(mr)
		baseSHA = mr.DiffRefs.BaseSha
		startSHA = mr.DiffRefs.StartSha
	}

	if opts.Body != "" {
		if _, err := p.CreateNote(ctx, owner, repo, number, opts.Body); err != nil {
			return nil, fmt.Errorf("create summary note: %w", err)
		}
	}

	if opts.CommitID != "" {
		state := "success"
		if opts.Event == "REQUEST_CHANGES" {
			state = "failed"
		}
		_ = p.CreateCommitStatus(ctx, owner, repo, opts.CommitID, provider.CommitStatusOptions{
			State:       state,
			Context:     "review-service",
			Description: opts.Body,
		})
	}

	commentResults := make([]provider.ReviewCommentResult, 0, len(opts.Comments))
	for _, c := range opts.Comments {
		discOpts := provider.DiscussionOptions{
			Body:     c.Body,
			FilePath: c.Path,
			BaseSHA:  baseSHA,
			StartSHA: startSHA,
			HeadSHA:  headSHA,
		}
		if c.Side == "LEFT" && c.Line > 0 {
			discOpts.OldLine = c.Line
		} else if c.Line > 0 {
			discOpts.NewLine = c.Line
		}
		if c.StartLine > 0 && c.EndLine > c.StartLine {
			discOpts.StartNewLine = c.StartLine
			discOpts.NewLine = c.EndLine
		}

		discID, discErr := p.CreateDiscussion(ctx, owner, repo, number, discOpts)
		cr := provider.ReviewCommentResult{
			Path:       c.Path,
			Line:       c.Line,
			ExternalID: discID,
		}
		if discErr != nil {
			cr.Error = discErr.Error()
		}
		commentResults = append(commentResults, cr)
	}

	return &provider.ReviewResult{
		ID:       fmt.Sprintf("gl-review-%d-%d", number, time.Now().UnixNano()),
		Comments: commentResults,
	}, nil
}

var _ provider.DiffManager = (*Provider)(nil)