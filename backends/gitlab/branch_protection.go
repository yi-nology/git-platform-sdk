package gitlab

import (
	"context"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranchProtections implements provider.BranchProtectionManager.
func (p *Provider) ListBranchProtections(ctx context.Context, owner, repo string) ([]*provider.BranchProtection, error) {
	branches, _, err := p.client.ProtectedBranches.ListProtectedBranches(pidOf(owner, repo), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListBranchProtections", err)
	}
	result := make([]*provider.BranchProtection, 0, len(branches))
	for _, b := range branches {
		result = append(result, convertProtectedBranch(b))
	}
	return result, nil
}

// GetBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*provider.BranchProtection, error) {
	b, _, err := p.client.ProtectedBranches.GetProtectedBranch(pidOf(owner, repo), branch, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "GetBranchProtection", err)
	}
	return convertProtectedBranch(b), nil
}

// CreateBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) CreateBranchProtection(ctx context.Context, owner, repo string, opts provider.CreateBranchProtectionOptions) (*provider.BranchProtection, error) {
	protectOpts := &gitlab.ProtectRepositoryBranchesOptions{
		Name:           gitlab.Ptr(opts.BranchName),
		AllowForcePush: gitlab.Ptr(opts.AllowForcePushes),
	}
	b, _, err := p.client.ProtectedBranches.ProtectRepositoryBranches(pidOf(owner, repo), protectOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateBranchProtection", err)
	}
	return convertProtectedBranch(b), nil
}

// UpdateBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) UpdateBranchProtection(ctx context.Context, owner, repo, branch string, opts provider.UpdateBranchProtectionOptions) (*provider.BranchProtection, error) {
	updateOpts := &gitlab.UpdateProtectedBranchOptions{}
	if opts.AllowForcePushes != nil {
		updateOpts.AllowForcePush = opts.AllowForcePushes
	}
	b, _, err := p.client.ProtectedBranches.UpdateProtectedBranch(pidOf(owner, repo), branch, updateOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "UpdateBranchProtection", err)
	}
	return convertProtectedBranch(b), nil
}

// DeleteBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) DeleteBranchProtection(ctx context.Context, owner, repo, branch string) error {
	if _, err := p.client.ProtectedBranches.UnprotectRepositoryBranches(pidOf(owner, repo), branch, gitlab.WithContext(ctx)); err != nil {
		return provider.Wrap(provider.PlatformGitLab, "DeleteBranchProtection", err)
	}
	return nil
}

// convertProtectedBranch maps a gitlab.ProtectedBranch to a
// provider.BranchProtection.
func convertProtectedBranch(b *gitlab.ProtectedBranch) *provider.BranchProtection {
	return &provider.BranchProtection{
		BranchName:       b.Name,
		AllowForcePushes: b.AllowForcePush,
	}
}

var _ provider.BranchProtectionManager = (*Provider)(nil)
