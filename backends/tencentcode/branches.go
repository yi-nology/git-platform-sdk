package tencentcode

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranches implements provider.BranchManager.
func (p *Provider) ListBranches(ctx context.Context, owner, repo string) ([]*provider.PlatformBranch, error) {
	encoded := encodeProjectPath(owner, repo)
	var branches []struct {
		Name string `json:"name"`
	}
	if err := p.doRequest(ctx, "GET", "/projects/"+encoded+"/repository/branches", nil, &branches); err != nil {
		return nil, err
	}
	result := make([]*provider.PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, &provider.PlatformBranch{Name: b.Name})
	}
	return result, nil
}

// CreateBranch implements provider.BranchManager.
func (p *Provider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*provider.PlatformBranch, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{"branch": branch, "ref": ref}
	var b struct {
		Name string `json:"name"`
	}
	if err := p.doRequest(ctx, "POST", "/projects/"+encoded+"/repository/branches", body, &b); err != nil {
		return nil, err
	}
	return &provider.PlatformBranch{Name: b.Name}, nil
}

// DeleteBranch implements provider.BranchManager.
func (p *Provider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	encoded := encodeProjectPath(owner, repo)
	return p.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/repository/branches/%s", encoded, branch), nil, nil)
}

var _ provider.BranchManager = (*Provider)(nil)
