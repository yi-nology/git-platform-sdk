package gitee

import (
	"context"

	gitee "github.com/next-bin/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	commit, _, err := p.client.Repositories.GetCommit(ctx, esc(owner), esc(repo), esc(sha))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetCommit", err)
	}
	return convertRepoCommitWithFiles(commit), nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gitee.CommitListOptions{
		Page:    gitee.Int(page),
		PerPage: gitee.Int(perPage),
	}
	if opts.Branch != "" {
		listOpts.SHA = gitee.String(opts.Branch)
	}
	if opts.Since != "" {
		listOpts.Since = gitee.String(opts.Since)
	}
	if opts.Until != "" {
		listOpts.Until = gitee.String(opts.Until)
	}
	commits, _, err := p.client.Repositories.ListCommits(ctx, esc(owner), esc(repo), listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListCommits", err)
	}
	result := make([]*provider.CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, convertRepoCommit(c))
	}
	return result, nil
}

// CompareCommits implements provider.CommitManager.
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	cmp, _, err := p.client.Repositories.CompareCommits(ctx, esc(owner), esc(repo), esc(base), esc(head), nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CompareCommits", err)
	}
	result := &provider.CompareResult{}
	if cmp.Commits != nil {
		result.TotalCommits = len(*cmp.Commits)
		for _, c := range *cmp.Commits {
			result.Commits = append(result.Commits, convertRepoCommit(c))
		}
	}
	if cmp.Files != nil {
		for _, f := range *cmp.Files {
			result.Files = append(result.Files, convertDiffFile(f))
		}
	}
	return result, nil
}

var _ provider.CommitManager = (*Provider)(nil)
