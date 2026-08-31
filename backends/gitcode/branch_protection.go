package gitcode

import (
	"context"

	gitcode "github.com/yi-nology/go-gitcode"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranchProtections implements provider.BranchProtectionManager.
func (p *Provider) ListBranchProtections(ctx context.Context, owner, repo string) ([]*provider.BranchProtection, error) {
	rules, err := p.client.ListBranchProtections(ctx, owner, repo)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListBranchProtections", err)
	}
	result := make([]*provider.BranchProtection, 0, len(rules))
	for _, r := range rules {
		result = append(result, convertBranchProtectionRule(r))
	}
	return result, nil
}

// GetBranchProtection implements provider.BranchProtectionManager. GitCode
// exposes branch protections as a list; this method filters by branch name.
func (p *Provider) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*provider.BranchProtection, error) {
	rules, err := p.client.ListBranchProtections(ctx, owner, repo)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetBranchProtection", err)
	}
	for _, r := range rules {
		if r.Name == branch {
			return convertBranchProtectionRule(r), nil
		}
	}
	return nil, provider.Wrapf(provider.PlatformGitCode, "GetBranchProtection", "branch protection %q not found", branch)
}

// CreateBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) CreateBranchProtection(ctx context.Context, owner, repo string, opts provider.CreateBranchProtectionOptions) (*provider.BranchProtection, error) {
	rule, err := p.client.CreateBranchProtection(ctx, owner, repo, gitcode.CreateBranchProtectionOptions{
		Name:                     opts.BranchName,
		RequiredStatusChecks:     opts.RequiredStatusChecks,
		RequiredApprovingReviews: opts.RequiredApprovingReviews,
		AllowForcePushes:         opts.AllowForcePushes,
		AllowDeletions:           opts.AllowDeletions,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CreateBranchProtection", err)
	}
	return convertBranchProtectionRule(rule), nil
}

// UpdateBranchProtection implements provider.BranchProtectionManager. GitCode's
// update endpoint only supports pusher/merger fields, so the existing
// protection is deleted and re-created with merged settings.
func (p *Provider) UpdateBranchProtection(ctx context.Context, owner, repo, branch string, opts provider.UpdateBranchProtectionOptions) (*provider.BranchProtection, error) {
	current, err := p.GetBranchProtection(ctx, owner, repo, branch)
	if err != nil {
		return nil, err
	}
	// Merge current values with caller-supplied overrides.
	createOpts := gitcode.CreateBranchProtectionOptions{
		Name:                     branch,
		RequiredStatusChecks:     current.RequiredStatusChecks,
		RequiredApprovingReviews: current.RequiredApprovingReviews,
		AllowForcePushes:         current.AllowForcePushes,
		AllowDeletions:           current.AllowDeletions,
	}
	if opts.RequiredStatusChecks != nil {
		createOpts.RequiredStatusChecks = *opts.RequiredStatusChecks
	}
	if opts.RequiredApprovingReviews != nil {
		createOpts.RequiredApprovingReviews = *opts.RequiredApprovingReviews
	}
	if opts.AllowForcePushes != nil {
		createOpts.AllowForcePushes = *opts.AllowForcePushes
	}
	if opts.AllowDeletions != nil {
		createOpts.AllowDeletions = *opts.AllowDeletions
	}
	// Delete then re-create (GitCode's PUT /setting/new replaces the rule).
	if err := p.client.DeleteBranchProtection(ctx, owner, repo, branch); err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateBranchProtection", err)
	}
	rule, err := p.client.CreateBranchProtection(ctx, owner, repo, createOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateBranchProtection", err)
	}
	return convertBranchProtectionRule(rule), nil
}

// DeleteBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) DeleteBranchProtection(ctx context.Context, owner, repo, branch string) error {
	if err := p.client.DeleteBranchProtection(ctx, owner, repo, branch); err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteBranchProtection", err)
	}
	return nil
}

// convertBranchProtectionRule maps a gitcode.BranchProtectionRule to a
// provider.BranchProtection.
func convertBranchProtectionRule(r *gitcode.BranchProtectionRule) *provider.BranchProtection {
	return &provider.BranchProtection{
		BranchName:               r.Name,
		RequiredApprovingReviews: r.RequiredApprovingReviews,
		RequiredStatusChecks:     r.RequiredStatusChecks,
		AllowForcePushes:         r.AllowForcePushes,
		AllowDeletions:           r.AllowDeletions,
	}
}

var _ provider.BranchProtectionManager = (*Provider)(nil)
