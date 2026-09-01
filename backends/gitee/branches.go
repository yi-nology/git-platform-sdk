package gitee

import (
	"context"
	"fmt"
	"net/http"

	gitee "github.com/next-bin/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranches implements provider.BranchManager.
func (p *Provider) ListBranches(ctx context.Context, owner, repo string) ([]*provider.PlatformBranch, error) {
	branches, _, err := p.client.Repositories.ListBranches(ctx, esc(owner), esc(repo), nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListBranches", err)
	}
	result := make([]*provider.PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, convertBranch(b))
	}
	return result, nil
}

// CreateBranch implements provider.BranchManager.
func (p *Provider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*provider.PlatformBranch, error) {
	opts := &gitee.CreateBranchOptions{
		Refs:       gitee.String(ref),
		BranchName: gitee.String(branch),
	}
	created, _, err := p.client.Repositories.CreateBranch(ctx, esc(owner), esc(repo), opts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateBranch", err)
	}
	name := deref(created.Name)
	if name == "" {
		name = branch
	}
	return &provider.PlatformBranch{Name: name}, nil
}

// DeleteBranch implements provider.BranchManager.
//
// The new SDK does not expose a DeleteBranch method, so we construct the
// request manually through the client's NewRequest + Do methods.
func (p *Provider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	u := fmt.Sprintf("repos/%s/%s/branches/%s", esc(owner), esc(repo), esc(branch))
	req, err := p.client.NewRequest(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteBranch", err)
	}
	_, err = p.client.Do(req, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteBranch", err)
	}
	return nil
}

var _ provider.BranchManager = (*Provider)(nil)
