package gitee

import (
	"context"

	gitee "gitee.com/openeuler/go-gitee/gitee"

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

// CreateBranchProtection implements provider.BranchProtectionManager. Gitee
// only supports enabling protection via PUT; this delegates to the PUT endpoint.
func (p *Provider) CreateBranchProtection(ctx context.Context, owner, repo string, opts provider.CreateBranchProtectionOptions) (*provider.BranchProtection, error) {
	_, resp, err := p.client.RepositoriesApi.PutV5ReposOwnerRepoBranchesBranchProtection(ctx, esc(owner), esc(repo), esc(opts.BranchName), gitee.BranchProtectionPutParam{
		AccessToken: p.token,
	})
	if err != nil {
		return nil, p.sdkErr("CreateBranchProtection", resp, err)
	}
	return &provider.BranchProtection{
		BranchName: opts.BranchName,
	}, nil
}

// UpdateBranchProtection implements provider.BranchProtectionManager. Gitee has
// no update endpoint; this re-enables protection via PUT.
func (p *Provider) UpdateBranchProtection(ctx context.Context, owner, repo, branch string, opts provider.UpdateBranchProtectionOptions) (*provider.BranchProtection, error) {
	_, resp, err := p.client.RepositoriesApi.PutV5ReposOwnerRepoBranchesBranchProtection(ctx, esc(owner), esc(repo), esc(branch), gitee.BranchProtectionPutParam{
		AccessToken: p.token,
	})
	if err != nil {
		return nil, p.sdkErr("UpdateBranchProtection", resp, err)
	}
	return &provider.BranchProtection{
		BranchName: branch,
	}, nil
}

// DeleteBranchProtection implements provider.BranchProtectionManager.
func (p *Provider) DeleteBranchProtection(ctx context.Context, owner, repo, branch string) error {
	resp, err := p.client.RepositoriesApi.DeleteV5ReposOwnerRepoBranchesBranchProtection(ctx, esc(owner), esc(repo), esc(branch), &gitee.DeleteV5ReposOwnerRepoBranchesBranchProtectionOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return p.sdkErr("DeleteBranchProtection", resp, err)
	}
	return nil
}

var _ provider.BranchProtectionManager = (*Provider)(nil)
