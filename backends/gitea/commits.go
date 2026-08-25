package gitea

import (
	"context"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	c, _, err := p.client.GetSingleCommit(owner, repo, sha)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetCommit", err)
	}
	return convertCommit(c), nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	listOpts := gitea.ListCommitOptions{ListOptions: gitea.ListOptions{Page: opts.Page, PageSize: opts.PerPage}}
	listOpts.Page, listOpts.PageSize = provider.NormalizePageOpts(listOpts.Page, listOpts.PageSize)
	commits, _, err := p.client.ListRepoCommits(owner, repo, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListCommits", err)
	}
	result := make([]*provider.CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, convertCommit(c))
	}
	return result, nil
}

// CompareCommits implements provider.CommitManager.
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	cmp, _, err := p.client.CompareCommits(owner, repo, base, head)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CompareCommits", err)
	}
	result := &provider.CompareResult{TotalCommits: cmp.TotalCommits}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, convertCommit(c))
	}
	return result, nil
}

// CreateCommitStatus implements provider.CommitStatusManager.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	stateMap := map[string]gitea.StatusState{
		"success": gitea.StatusSuccess,
		"failed":  gitea.StatusFailure,
		"pending": gitea.StatusPending,
		"error":   gitea.StatusError,
	}
	state := stateMap[opts.State]
	if state == "" {
		state = gitea.StatusPending
	}
	_, _, err := p.client.CreateStatus(owner, repo, sha, gitea.CreateStatusOption{
		State:       state,
		Context:     opts.Context,
		Description: opts.Description,
		TargetURL:   opts.TargetURL,
	})
	if err != nil {
		return provider.Wrap(provider.PlatformGitea, "CreateCommitStatus", err)
	}
	return nil
}

var _ provider.CommitManager = (*Provider)(nil)
var _ provider.CommitStatusManager = (*Provider)(nil)
