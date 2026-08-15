package gitcode

import (
	"context"
	"strconv"

	gitcode "github.com/yi-nology/gitcode_api"

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

// CreateCommitStatus implements provider.CommitManager.
//
// The GitCode platform API (/api/v5) does not expose a commit-status
// endpoint (equivalent to GitHub's POST /repos/{owner}/{repo}/statuses/{sha}).
// This is a platform-level limitation, not an SDK gap. If GitCode adds
// commit-status support in the future, this method should be implemented
// using the SDK's doRequest helper or raw HTTP.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	return provider.Wrap(provider.PlatformGitCode, "CreateCommitStatus", provider.ErrNotImplemented)
}

func convertCommit(c *gitcode.Commit) *provider.CommitInfo {
	if c == nil {
		return nil
	}
	ci := &provider.CommitInfo{SHA: c.SHA, Message: c.Message, CreatedAt: c.CreatedAt}
	if c.Author != nil {
		authorID, _ := strconv.ParseInt(string(c.Author.ID), 10, 64)
		ci.Author = &provider.CRUser{
			ID: authorID, Username: c.Author.Login, AvatarURL: c.Author.AvatarURL,
		}
	}
	return ci
}

var _ provider.CommitManager = (*Provider)(nil)
