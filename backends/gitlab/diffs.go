package gitlab

import (
	"context"
	"strconv"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"

	gitlab "gitlab.com/gitlab-org/api/client-go/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo, number string) (*provider.MergeDiff, error) {
	n, err := backendutil.ParsePRNumber64(provider.PlatformGitLab, "GetCRDiff", number)
	if err != nil {
		return nil, err
	}
	diffs, _, err := p.client.MergeRequests.ListMergeRequestDiffs(pidOf(owner, repo), n, nil, gitlab.WithContext(ctx))
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
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo, number string) ([]*provider.ChangedFile, error) {
	diff, err := p.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

// CreateNote implements provider.DiffManager.
func (p *Provider) CreateNote(ctx context.Context, owner, repo, number, body string) (string, error) {
	n, err := backendutil.ParsePRNumber64(provider.PlatformGitLab, "CreateNote", number)
	if err != nil {
		return "", err
	}
	note, _, err := p.client.Notes.CreateMergeRequestNote(pidOf(owner, repo), n,
		&gitlab.CreateMergeRequestNoteOptions{Body: new(body)}, gitlab.WithContext(ctx))
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitLab, "CreateNote", err)
	}
	return strconv.FormatInt(note.ID, 10), nil
}

// DeleteNote implements provider.DiffManager.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo, number, noteID string) error {
	n, err := backendutil.ParsePRNumber64(provider.PlatformGitLab, "DeleteNote", number)
	if err != nil {
		return err
	}
	nid, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return provider.Wrapf(provider.PlatformGitLab, "DeleteNote", "invalid note ID %q: %v", noteID, err)
	}
	_, err = p.client.Notes.DeleteMergeRequestNote(pidOf(owner, repo), n, nid, gitlab.WithContext(ctx))
	if err != nil {
		return provider.Wrap(provider.PlatformGitLab, "DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo, number string, opts provider.DiscussionOptions) (string, error) {
	n, err := backendutil.ParsePRNumber64(provider.PlatformGitLab, "CreateDiscussion", number)
	if err != nil {
		return "", err
	}
	pid := pidOf(owner, repo)
	discOpts := &gitlab.CreateMergeRequestDiscussionOptions{Body: new(opts.Body)}
	if opts.FilePath != "" {
		position := &gitlab.PositionOptions{
			PositionType: new("text"),
			NewPath:      new(opts.FilePath),
		}
		if opts.BaseSHA != "" {
			position.BaseSHA = new(opts.BaseSHA)
		}
		if opts.StartSHA != "" {
			position.StartSHA = new(opts.StartSHA)
		}
		if opts.HeadSHA != "" {
			position.HeadSHA = new(opts.HeadSHA)
		}
		if opts.OldLine > 0 {
			position.OldPath = new(opts.FilePath)
			position.OldLine = new(int64(opts.OldLine))
		}
		if opts.NewLine > 0 {
			position.NewLine = new(int64(opts.NewLine))
		}
		if opts.StartNewLine > 0 && opts.NewLine > opts.StartNewLine {
			position.LineRange = &gitlab.LineRangeOptions{
				Start: &gitlab.LinePositionOptions{Type: new("new"), NewLine: new(int64(opts.StartNewLine))},
				End:   &gitlab.LinePositionOptions{Type: new("new"), NewLine: new(int64(opts.NewLine))},
			}
		}
		discOpts.Position = position
	}
	disc, _, err := p.client.Discussions.CreateMergeRequestDiscussion(pid, n, discOpts, gitlab.WithContext(ctx))
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitLab, "CreateDiscussion", err)
	}
	return disc.ID, nil
}

var _ provider.DiffManager = (*Provider)(nil)
