package tencentcode

import (
	"context"

	gongfeng "github.com/studyzy/gongfeng-sdk-go"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	pid := owner + "/" + repo
	c, _, err := p.client.Commits.GetCommit(ctx, pid, sha)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformTencentCode, "GetCommit", err)
	}
	return convertCommit(c), nil
}

// ListCommits implements provider.CommitManager.
//
// Dual-mode pagination: opts.Page == 0 fetches every page via AllPages
// (工蜂's page-size ceiling is 100), so the caller gets the complete
// history; opts.Page > 0 returns exactly that single page and the caller
// drives pagination itself (opts.PerPage is used as given, falling back to
// the platform maximum when unset).
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	pid := owner + "/" + repo
	buildOpts := func(page, perPage int) *gongfeng.ListCommitsOptions {
		listOpts := &gongfeng.ListCommitsOptions{
			ListOptions: gongfeng.ListOptions{Page: page, PerPage: perPage},
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
		return listOpts
	}
	var commits []*gongfeng.Commit
	if opts.Page > 0 {
		// Caller-driven pagination: serve the requested page only.
		perPage := opts.PerPage
		if perPage <= 0 || perPage > provider.MaxPerPage {
			perPage = provider.MaxPerPage
		}
		var err error
		if commits, _, err = p.client.Commits.ListCommits(ctx, pid, buildOpts(opts.Page, perPage)); err != nil {
			return nil, provider.Wrap(provider.PlatformTencentCode, "ListCommits", err)
		}
	} else {
		var err error
		if commits, err = backendutil.AllPages(func(page int) ([]*gongfeng.Commit, error) {
			list, _, err := p.client.Commits.ListCommits(ctx, pid, buildOpts(page, provider.MaxPerPage))
			return list, err
		}); err != nil {
			return nil, provider.Wrap(provider.PlatformTencentCode, "ListCommits", err)
		}
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
		return nil, provider.Wrap(provider.PlatformTencentCode, "CompareCommits", err)
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
		return provider.Wrap(provider.PlatformTencentCode, "CreateCommitStatus", err)
	}
	return nil
}

// ListCommitStatuses implements provider.CommitStatusManager.
func (p *Provider) ListCommitStatuses(ctx context.Context, owner, repo, sha string) ([]provider.CommitStatus, error) {
	pid := owner + "/" + repo
	statuses, err := backendutil.AllPages(func(page int) ([]*gongfeng.CommitStatus, error) {
		list, _, err := p.client.CommitStatuses.ListCommitStatuses(ctx, pid, sha,
			&gongfeng.ListCommitStatusesOptions{ListOptions: gongfeng.ListOptions{Page: page, PerPage: 100}})
		return list, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformTencentCode, "ListCommitStatuses", err)
	}
	return convertCommitStatuses(statuses), nil
}

// convertCommitStatuses maps gongfeng CommitStatus entries onto the unified
// vocabulary. 工蜂 is GitLab-shaped: the wire "status" key carries the state
// verb and "name" the per-check context.
func convertCommitStatuses(statuses []*gongfeng.CommitStatus) []provider.CommitStatus {
	result := make([]provider.CommitStatus, 0, len(statuses))
	for _, s := range statuses {
		if s == nil {
			continue
		}
		result = append(result, provider.CommitStatus{
			State:       provider.NormalizeCommitStatusState(s.Status),
			Context:     s.Name,
			Description: s.Description,
			TargetURL:   s.TargetURL,
		})
	}
	return result
}

var _ provider.CommitManager = (*Provider)(nil)
var _ provider.CommitStatusManager = (*Provider)(nil)
