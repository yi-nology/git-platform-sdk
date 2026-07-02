package forgejo

import (
	"context"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	c, _, err := p.client.GetSingleCommit(owner, repo, sha)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "GetCommit", err)
	}
	return convertCommit(c), nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	listOpts := forgejo.ListCommitOptions{ListOptions: forgejo.ListOptions{Page: opts.Page, PageSize: opts.PerPage}}
	listOpts.Page, listOpts.PageSize = provider.NormalizePageOpts(listOpts.Page, listOpts.PageSize)
	commits, _, err := p.client.ListRepoCommits(owner, repo, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListCommits", err)
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
		return nil, provider.Wrap(provider.PlatformForgejo, "CompareCommits", err)
	}
	result := &provider.CompareResult{TotalCommits: cmp.TotalCommits}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, convertCommit(c))
	}
	return result, nil
}

// CreateCommitStatus implements provider.CommitManager.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	stateMap := map[string]forgejo.StatusState{
		"success": forgejo.StatusSuccess,
		"failed":  forgejo.StatusFailure,
		"pending": forgejo.StatusPending,
		"error":   forgejo.StatusError,
	}
	state := stateMap[opts.State]
	if state == "" {
		state = forgejo.StatusPending
	}
	_, _, err := p.client.CreateStatus(owner, repo, sha, forgejo.CreateStatusOption{
		State:       state,
		Context:     opts.Context,
		Description: opts.Description,
		TargetURL:   opts.TargetURL,
	})
	if err != nil {
		return provider.Wrap(provider.PlatformForgejo, "CreateCommitStatus", err)
	}
	return nil
}

var _ provider.CommitManager = (*Provider)(nil)