package gitea

import (
	"context"

	gitea "gitea.dev/sdk"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranches implements provider.BranchManager. The provider surface
// carries no pagination parameters, so the full branch list is fetched by
// exhausting the endpoint's pagination (backendutil.AllPages).
func (p *Provider) ListBranches(ctx context.Context, owner, repo string) ([]*provider.PlatformBranch, error) {
	branches, err := backendutil.AllPages(func(page int) ([]*gitea.Branch, error) {
		list, _, err := p.client.Repositories.ListRepoBranches(ctx, owner, repo, gitea.ListRepoBranchesOptions{
			ListOptions: gitea.ListOptions{Page: page, PageSize: listPageSize},
		})
		return list, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListBranches", err)
	}
	result := make([]*provider.PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, convertBranch(b))
	}
	return result, nil
}

// CreateBranch implements provider.BranchManager.
func (p *Provider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*provider.PlatformBranch, error) {
	b, _, err := p.client.Repositories.CreateBranch(ctx, owner, repo, gitea.CreateBranchOption{
		BranchName:    branch,
		OldBranchName: ref,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateBranch", err)
	}
	return convertBranch(b), nil
}

// DeleteBranch implements provider.BranchManager.
func (p *Provider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	_, _, err := p.client.Repositories.DeleteRepoBranch(ctx, owner, repo, branch)
	if err != nil {
		return provider.Wrap(provider.PlatformGitea, "DeleteBranch", err)
	}
	return nil
}

var _ provider.BranchManager = (*Provider)(nil)
