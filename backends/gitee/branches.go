package gitee

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranches implements provider.BranchManager.
func (p *Provider) ListBranches(ctx context.Context, owner, repo string) ([]*provider.PlatformBranch, error) {
	page, perPage := provider.NormalizePageOpts(1, 0)
	var branches []struct {
		Name string `json:"name"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/branches?page=%d&per_page=%d", esc(owner), esc(repo), page, perPage), nil, &branches); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListBranches", err)
	}
	result := make([]*provider.PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, &provider.PlatformBranch{Name: b.Name})
	}
	return result, nil
}

// CreateBranch implements provider.BranchManager.
func (p *Provider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*provider.PlatformBranch, error) {
	body := map[string]any{"branch_name": branch, "refs": ref}
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/branches", esc(owner), esc(repo)), body, nil); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateBranch", err)
	}
	return &provider.PlatformBranch{Name: branch}, nil
}

// DeleteBranch implements provider.BranchManager.
func (p *Provider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/branches/%s", esc(owner), esc(repo), esc(branch)), nil, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteBranch", err)
	}
	return nil
}

var _ provider.BranchManager = (*Provider)(nil)
