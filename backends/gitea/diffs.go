package gitea

import (
	"context"
	"strconv"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo, number string) (*provider.MergeDiff, error) {
	n, err := backendutil.ParsePRNumber64(provider.PlatformGitea, "GetCRDiff", number)
	if err != nil {
		return nil, err
	}
	diffBytes, _, err := p.client.GetPullRequestDiff(owner, repo, n, gitea.PullRequestDiffOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetCRDiff", err)
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
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo, number string) ([]*provider.ChangedFile, error) {
	n, err := backendutil.ParsePRNumber64(provider.PlatformGitea, "GetCRFiles", number)
	if err != nil {
		return nil, err
	}
	changedFiles, _, err := p.client.ListPullRequestFiles(owner, repo, n, gitea.ListPullRequestFilesOptions{
		ListOptions: gitea.ListOptions{PageSize: 100},
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetCRFiles", err)
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
func (p *Provider) CreateNote(ctx context.Context, owner, repo, number, body string) (string, error) {
	n, err := backendutil.ParsePRNumber64(provider.PlatformGitea, "CreateNote", number)
	if err != nil {
		return "", err
	}
	comment, _, err := p.client.CreateIssueComment(owner, repo, n, gitea.CreateIssueCommentOption{Body: body})
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitea, "CreateNote", err)
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

// DeleteNote implements provider.DiffManager. The note itself is addressed
// by its numeric platform ID; the change-request number is only validated,
// as Gitea's delete-comment endpoint does not take it.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo, number, noteID string) error {
	if _, err := backendutil.ParsePRNumber64(provider.PlatformGitea, "DeleteNote", number); err != nil {
		return err
	}
	id, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return provider.Wrap(provider.PlatformGitea, "DeleteNote", err)
	}
	resp, err := p.client.DeleteIssueComment(owner, repo, id)
	if err != nil {
		// Gitea returns 404 when the comment is already gone; tolerate it.
		if resp != nil && resp.StatusCode == 404 {
			return nil
		}
		return provider.Wrap(provider.PlatformGitea, "DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
//
// Gitea has no separate discussion endpoint; we approximate one by posting
// an issue comment (the same backing store Gitea itself uses for PR
// discussions).
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo, number string, opts provider.DiscussionOptions) (string, error) {
	n, err := backendutil.ParsePRNumber64(provider.PlatformGitea, "CreateDiscussion", number)
	if err != nil {
		return "", err
	}
	comment, _, err := p.client.CreateIssueComment(owner, repo, n, gitea.CreateIssueCommentOption{Body: opts.Body})
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitea, "CreateDiscussion", err)
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

var _ provider.DiffManager = (*Provider)(nil)
