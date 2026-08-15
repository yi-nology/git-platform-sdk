package forgejo

import (
	"context"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*provider.MergeDiff, error) {
	diffBytes, _, err := p.client.GetPullRequestDiff(owner, repo, int64(number), forgejo.PullRequestDiffOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "GetCRDiff", err)
	}
	rawDiff := string(diffBytes)

	files, err := p.GetCRFiles(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	totalAdd, totalDel := 0, 0
	for _, f := range files {
		totalAdd += f.Additions
		totalDel += f.Deletions
	}
	return &provider.MergeDiff{Files: files, TotalAdd: totalAdd, TotalDel: totalDel, RawDiff: rawDiff}, nil
}

// GetCRFiles implements provider.DiffManager.
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*provider.ChangedFile, error) {
	changedFiles, _, err := p.client.ListPullRequestFiles(owner, repo, int64(number), forgejo.ListPullRequestFilesOptions{
		ListOptions: forgejo.ListOptions{PageSize: 100},
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "GetCRFiles", err)
	}
	result := make([]*provider.ChangedFile, 0, len(changedFiles))
	for _, f := range changedFiles {
		cf := &provider.ChangedFile{
			OldPath:   f.PreviousFilename,
			NewPath:   f.Filename,
			Additions: f.Additions,
			Deletions: f.Deletions,
			IsNew:     f.Status == "added",
			IsDeleted: f.Status == "removed",
			IsRenamed: f.Status == "renamed",
		}
		if cf.OldPath == "" {
			cf.OldPath = cf.NewPath
		}
		result = append(result, cf)
	}
	return result, nil
}

// CreateNote implements provider.DiffManager.
func (p *Provider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	comment, _, err := p.client.CreateIssueComment(owner, repo, int64(number), forgejo.CreateIssueCommentOption{Body: body})
	if err != nil {
		return "", provider.Wrap(provider.PlatformForgejo, "CreateNote", err)
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

// DeleteNote implements provider.DiffManager.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	id, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return provider.Wrap(provider.PlatformForgejo, "DeleteNote", err)
	}
	resp, err := p.client.DeleteIssueComment(owner, repo, id)
	if err != nil {
		// Forgejo returns 404 when the comment is already gone; tolerate it.
		if resp != nil && resp.StatusCode == 404 {
			return nil
		}
		return provider.Wrap(provider.PlatformForgejo, "DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
//
// Forgejo has no separate discussion endpoint; we approximate one by posting
// an issue comment (the same backing store Forgejo itself uses for PR
// discussions).
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts provider.DiscussionOptions) (string, error) {
	comment, _, err := p.client.CreateIssueComment(owner, repo, int64(number), forgejo.CreateIssueCommentOption{Body: opts.Body})
	if err != nil {
		return "", provider.Wrap(provider.PlatformForgejo, "CreateDiscussion", err)
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

var _ provider.DiffManager = (*Provider)(nil)
