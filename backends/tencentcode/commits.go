package tencentcode

import (
	"context"

	gongfeng "github.com/studyzy/gongfeng-sdk-go"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	pid := owner + "/" + repo
	c, _, err := p.client.Commits.GetCommit(ctx, pid, sha)
	if err != nil {
		return nil, sdkError("GetCommit", err)
	}
	return convertCommit(c), nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	pid := owner + "/" + repo
	opts.Page, opts.PerPage = provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gongfeng.ListCommitsOptions{
		ListOptions: gongfeng.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
	}
	if opts.Branch != "" {
		listOpts.RefName = gongfeng.Ptr(opts.Branch)
	}
	if opts.Since != "" {
		listOpts.Since = gongfeng.Ptr(opts.Since)
	}
	if opts.Until != "" {
		listOpts.Until = gongfeng.Ptr(opts.Until)
	}
	commits, _, err := p.client.Commits.ListCommits(ctx, pid, listOpts)
	if err != nil {
		return nil, sdkError("ListCommits", err)
	}
	result := make([]*provider.CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, convertCommit(c))
	}
	return result, nil
}

// CompareCommits implements provider.CommitManager.
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	pid := owner + "/" + repo
	opts := &gongfeng.CompareOptions{
		From: gongfeng.Ptr(base),
		To:   gongfeng.Ptr(head),
	}
	cmp, _, err := p.client.Repositories.Compare(ctx, pid, opts)
	if err != nil {
		return nil, sdkError("CompareCommits", err)
	}
	result := &provider.CompareResult{TotalCommits: len(cmp.Commits)}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, convertCommit(c))
	}
	for _, d := range cmp.Diffs {
		result.Files = append(result.Files, convertDiff(d))
	}
	return result, nil
}

// CreateCommitStatus implements provider.CommitStatusManager.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	pid := owner + "/" + repo
	statusOpts := &gongfeng.CreateCommitStatusOptions{
		State:       gongfeng.Ptr(opts.State),
		Context:     gongfeng.Ptr(opts.Context),
		Description: gongfeng.Ptr(opts.Description),
	}
	if opts.TargetURL != "" {
		statusOpts.TargetURL = gongfeng.Ptr(opts.TargetURL)
	}
	_, _, err := p.client.CommitStatuses.CreateCommitStatus(ctx, pid, sha, statusOpts)
	if err != nil {
		return sdkError("CreateCommitStatus", err)
	}
	return nil
}

var _ provider.CommitManager = (*Provider)(nil)
var _ provider.CommitStatusManager = (*Provider)(nil)
