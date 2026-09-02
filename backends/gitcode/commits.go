package gitcode

import (
	"context"

	gitcode "github.com/yi-nology/go-gitcode"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	c, err := p.client.GetCommit(ctx, owner, repo, sha)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetCommit", err)
	}
	return convertCommit(c), nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	commits, err := p.client.ListCommits(ctx, owner, repo, gitcode.ListCommitsOptions{
		ListOptions: gitcode.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
		Branch:      opts.Branch,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListCommits", err)
	}
	result := make([]*provider.CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, convertCommit(c))
	}
	return result, nil
}

// CompareCommits implements provider.CommitManager.
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	cmp, err := p.client.CompareCommits(ctx, owner, repo, base, head)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CompareCommits", err)
	}
	result := &provider.CompareResult{
		TotalCommits: cmp.TotalCommits,
		AheadBy:      cmp.AheadBy,
		BehindBy:     cmp.BehindBy,
	}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, convertCommit(c))
	}
	for _, f := range cmp.Files {
		result.Files = append(result.Files, &provider.ChangedFile{
			OldPath:   f.PreviousFilename,
			NewPath:   f.Filename,
			Additions: f.Additions,
			Deletions: f.Deletions,
			IsNew:     f.Status == "added",
			IsDeleted: f.Status == "removed",
			IsRenamed: f.Status == "renamed",
		})
	}
	return result, nil
}

// CreateCommitStatus implements provider.CommitStatusManager.
//
// provider.CommitStatusOptions map 1:1 onto the SDK's
// CreateCommitStatusOptions (POST /repos/{o}/{r}/statuses/{sha}). State
// passes through verbatim; GitCode expects the GitHub-shaped
// pending/success/error/failure verbs.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	_, err := p.client.CreateCommitStatus(ctx, owner, repo, sha, gitcode.CreateCommitStatusOptions{
		State:       opts.State,
		TargetURL:   opts.TargetURL,
		Description: opts.Description,
		Context:     opts.Context,
	})
	if err != nil {
		return provider.Wrap(provider.PlatformGitCode, "CreateCommitStatus", err)
	}
	return nil
}

func convertCommit(c *gitcode.Commit) *provider.CommitInfo {
	if c == nil {
		return nil
	}
	ci := &provider.CommitInfo{SHA: c.SHA, Message: c.Message, CreatedAt: c.CreatedAt}
	if c.Author != nil {
		authorID, _ := parseGitCodeID(c.Author.ID)
		ci.Author = &provider.CRUser{
			ID: authorID, Username: c.Author.Login, AvatarURL: c.Author.AvatarURL,
		}
	}
	return ci
}

var _ provider.CommitManager = (*Provider)(nil)
var _ provider.CommitStatusManager = (*Provider)(nil)
