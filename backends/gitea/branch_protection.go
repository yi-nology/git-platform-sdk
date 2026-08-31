package gitea

import (
	"context"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranchProtections implements provider.BranchProtectionManager.
func (p *Provider) ListBranchProtections(ctx context.Context, owner, repo string) ([]*provider.BranchProtection, error) {
	bps, _, err := p.client.ListBranchProtections(owner, repo, gitea.ListBranchProtectionsOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListBranchProtections", err)
	}
	result := make([]*provider.BranchProtection, 0, len(bps))
	for _, bp := range bps {
		result = append(result, convertBranchProtection(bp))
	}
	return result, nil
}

// GetBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*provider.BranchProtection, error) {
	bp, _, err := p.client.GetBranchProtection(owner, repo, branch)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetBranchProtection", err)
	}
	return convertBranchProtection(bp), nil
}

// CreateBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) CreateBranchProtection(ctx context.Context, owner, repo string, opts provider.CreateBranchProtectionOptions) (*provider.BranchProtection, error) {
	bp, _, err := p.client.CreateBranchProtection(owner, repo, gitea.CreateBranchProtectionOption{
		BranchName:        opts.BranchName,
		RequiredApprovals: int64(opts.RequiredApprovingReviews),
		EnableStatusCheck: opts.RequiredStatusChecks,
		EnablePush:        opts.AllowForcePushes,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateBranchProtection", err)
	}
	return convertBranchProtection(bp), nil
}

// UpdateBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) UpdateBranchProtection(ctx context.Context, owner, repo, branch string, opts provider.UpdateBranchProtectionOptions) (*provider.BranchProtection, error) {
	editOpts := gitea.EditBranchProtectionOption{}
	if opts.RequiredApprovingReviews != nil {
		v := int64(*opts.RequiredApprovingReviews)
		editOpts.RequiredApprovals = &v
	}
	if opts.RequiredStatusChecks != nil {
		editOpts.EnableStatusCheck = opts.RequiredStatusChecks
	}
	if opts.AllowForcePushes != nil {
		editOpts.EnablePush = opts.AllowForcePushes
	}
	if opts.AllowDeletions != nil {
		editOpts.BlockOnRejectedReviews = nil // not directly mapped
	}
	bp, _, err := p.client.EditBranchProtection(owner, repo, branch, editOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "UpdateBranchProtection", err)
	}
	return convertBranchProtection(bp), nil
}

// DeleteBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) DeleteBranchProtection(ctx context.Context, owner, repo, branch string) error {
	if _, err := p.client.DeleteBranchProtection(owner, repo, branch); err != nil {
		return provider.Wrap(provider.PlatformGitea, "DeleteBranchProtection", err)
	}
	return nil
}

// convertBranchProtection maps a gitea.BranchProtection to a
// provider.BranchProtection.
func convertBranchProtection(bp *gitea.BranchProtection) *provider.BranchProtection {
	return &provider.BranchProtection{
		BranchName:               bp.BranchName,
		RequiredApprovingReviews: int(bp.RequiredApprovals),
		RequiredStatusChecks:     bp.EnableStatusCheck,
		AllowForcePushes:         bp.EnablePush,
		AllowDeletions:           !bp.BlockOnRejectedReviews,
	}
}

var _ provider.BranchProtectionManager = (*Provider)(nil)
