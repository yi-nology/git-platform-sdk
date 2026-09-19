package forgejo

import (
	"context"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranches implements provider.BranchManager. The provider surface
// carries no pagination parameters, so the full branch list is fetched by
// exhausting the endpoint's pagination (backendutil.AllPages).
//
// The forgejo SDK accepts no context parameter (registered platform
// limitation), so ctx is unused beyond signature conformance.
func (p *Provider) ListBranches(ctx context.Context, owner, repo string) ([]*provider.PlatformBranch, error) {
	branches, err := backendutil.AllPages(func(page int) ([]*forgejo.Branch, error) {
		list, _, err := p.client.ListRepoBranches(owner, repo, forgejo.ListRepoBranchesOptions{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: listPageSize},
		})
		return list, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListBranches", err)
	}
	result := make([]*provider.PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, convertBranch(b))
	}
	return result, nil
}

// CreateBranch implements provider.BranchManager.
func (p *Provider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*provider.PlatformBranch, error) {
	b, _, err := p.client.CreateBranch(owner, repo, forgejo.CreateBranchOption{
		BranchName:    branch,
		OldBranchName: ref,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "CreateBranch", err)
	}
	return convertBranch(b), nil
}

// DeleteBranch implements provider.BranchManager.
func (p *Provider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	_, _, err := p.client.DeleteRepoBranch(owner, repo, branch)
	if err != nil {
		return provider.Wrap(provider.PlatformForgejo, "DeleteBranch", err)
	}
	return nil
}

var _ provider.BranchManager = (*Provider)(nil)
