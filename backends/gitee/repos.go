package gitee

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListRepos implements provider.RepoManager.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	var path string
	if opts.Owner != "" {
		path = fmt.Sprintf("/users/%s/repos?page=%d&per_page=%d", opts.Owner, page, perPage)
	} else {
		path = fmt.Sprintf("/user/repos?page=%d&per_page=%d", page, perPage)
	}
	var repos []giteeRepo
	if err := p.doRequest(ctx, "GET", path, nil, &repos); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListRepos", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(repos))
	for i := range repos {
		result = append(result, repos[i].toPlatformRepo())
	}
	return result, nil
}

// GetRepo implements provider.RepoManager.
func (p *Provider) GetRepo(ctx context.Context, owner, repo string) (*provider.PlatformRepo, error) {
	var r giteeRepo
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s", owner, repo), nil, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetRepo", err)
	}
	return r.toPlatformRepo(), nil
}

// DeleteRepo implements provider.RepoManager.
func (p *Provider) DeleteRepo(ctx context.Context, owner, repo string) error {
	err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s", owner, repo), nil, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteRepo", err)
	}
	return nil
}

// UpdateRepo implements provider.RepoManager.
func (p *Provider) UpdateRepo(ctx context.Context, owner, repo string, opts provider.UpdateRepoOptions) (*provider.PlatformRepo, error) {
	body := map[string]any{}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.DefaultBranch != "" {
		body["default_branch"] = opts.DefaultBranch
	}
	if opts.Private != nil {
		body["private"] = *opts.Private
	}
	var r giteeRepo
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s", owner, repo), body, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateRepo", err)
	}
	return r.toPlatformRepo(), nil
}

// ForkRepo implements provider.RepoManager.
func (p *Provider) ForkRepo(ctx context.Context, owner, repo string, opts provider.ForkRepoOptions) (*provider.PlatformRepo, error) {
	body := map[string]any{}
	if opts.Organization != "" {
		body["organization"] = opts.Organization
	}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	var r giteeRepo
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/forks", owner, repo), body, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ForkRepo", err)
	}
	return r.toPlatformRepo(), nil
}

// CreateRepo implements provider.RepoManager.
func (p *Provider) CreateRepo(ctx context.Context, owner string, opts provider.CreateRepoOptions) (*provider.PlatformRepo, error) {
	body := map[string]any{
		"name": opts.Name,
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	body["private"] = opts.Private
	if opts.AutoInit {
		body["auto_init"] = "true"
	}
	if opts.DefaultBranch != "" {
		body["default_branch"] = opts.DefaultBranch
	}

	var path string
	if owner != "" {
		path = fmt.Sprintf("/orgs/%s/repos", owner)
	} else {
		path = "/user/repos"
	}

	var r giteeRepo
	if err := p.doRequest(ctx, "POST", path, body, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateRepo", err)
	}
	return r.toPlatformRepo(), nil
}

var _ provider.RepoManager = (*Provider)(nil)
