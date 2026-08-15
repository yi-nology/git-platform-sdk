package gitlab

import (
	"context"
	"strconv"

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

var _ provider.DiffManager = (*Provider)(nil)
