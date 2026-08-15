package gitee

import (
	"context"
	"fmt"

	gitee "gitee.com/openeuler/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// ListBranches implements provider.BranchManager.
//
// Note: the SDK's GetV5ReposOwnerRepoBranchesOpts carries only AccessToken
// (no Page/PerPage), so the previous explicit page=1&per_page=20 query
// parameters are no longer sent; Gitee applies the same defaults server-side.
func (p *Provider) ListBranches(ctx context.Context, owner, repo string) ([]*provider.PlatformBranch, error) {
	branches, resp, err := p.client.RepositoriesApi.GetV5ReposOwnerRepoBranches(ctx, esc(owner), esc(repo), &gitee.GetV5ReposOwnerRepoBranchesOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return nil, p.sdkErr("ListBranches", resp, err)
	}
	result := make([]*provider.PlatformBranch, 0, len(branches))
	for i := range branches {
		result = append(result, convertBranch(branches[i]))
	}
	return result, nil
}

// CreateBranch implements provider.BranchManager.
func (p *Provider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*provider.PlatformBranch, error) {
	created, resp, err := p.client.RepositoriesApi.PostV5ReposOwnerRepoBranches(ctx, esc(owner), esc(repo), gitee.CreateBranchParam{
		AccessToken: p.token,
		Refs:        ref,
		BranchName:  branch,
	})
	if err != nil {
		return nil, p.sdkErr("CreateBranch", resp, err)
	}
	name := created.Name
	if name == "" {
		name = branch
	}
	return &provider.PlatformBranch{Name: name}, nil
}

// DeleteBranch implements provider.BranchManager.
//
// Routed through the raw transport client: go-gitee ships no
// DeleteV5ReposOwnerRepoBranches method (only branch-protection variants),
// so this endpoint is registered as SDK-missing and stays on the raw client.
func (p *Provider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	_, err := p.raw().DoJSON(ctx, &transport.Request{
		Method: "DELETE",
		Path:   fmt.Sprintf("/repos/%s/%s/branches/%s", esc(owner), esc(repo), esc(branch)),
	})
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteBranch", err)
	}
	return nil
}

var _ provider.BranchManager = (*Provider)(nil)
