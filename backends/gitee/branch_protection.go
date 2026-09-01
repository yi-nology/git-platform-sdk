package gitee

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranchProtections implements provider.BranchProtectionManager. Gitee has
// no list endpoint for branch protections; this returns nil.
func (p *Provider) ListBranchProtections(ctx context.Context, owner, repo string) ([]*provider.BranchProtection, error) {
	return nil, nil
}

// GetBranchProtection implements provider.BranchProtectionManager. Gitee has no
// get endpoint for branch protections; this returns nil.
func (p *Provider) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*provider.BranchProtection, error) {
	return nil, nil
}

// CreateBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) CreateBranchProtection(ctx context.Context, owner, repo string, opts provider.CreateBranchProtectionOptions) (*provider.BranchProtection, error) {
	_, _, err := p.client.Repositories.SetBranchProtection(ctx, esc(owner), esc(repo), esc(opts.BranchName))
	if err != nil {
		return nil, p.sdkErr("CreateBranchProtection", err)
	}
	return &provider.BranchProtection{
		BranchName: opts.BranchName,
	}, nil
}

// UpdateBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) UpdateBranchProtection(ctx context.Context, owner, repo, branch string, opts provider.UpdateBranchProtectionOptions) (*provider.BranchProtection, error) {
	_, _, err := p.client.Repositories.SetBranchProtection(ctx, esc(owner), esc(repo), esc(branch))
	if err != nil {
		return nil, p.sdkErr("UpdateBranchProtection", err)
	}
	return &provider.BranchProtection{
		BranchName: branch,
	}, nil
}

// DeleteBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) DeleteBranchProtection(ctx context.Context, owner, repo, branch string) error {
	_, err := p.client.Repositories.RemoveBranchProtection(ctx, esc(owner), esc(repo), esc(branch))
	if err != nil {
		return p.sdkErr("DeleteBranchProtection", err)
	}
	return nil
}

var _ provider.BranchProtectionManager = (*Provider)(nil)
