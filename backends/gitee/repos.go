package gitee

import (
	"context"
	"fmt"
	"strconv"

	gitee "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// ListRepos implements provider.RepoManager.
//
// Routed through the raw transport client rather than the SDK: go-gitee's
// GetV5UserRepos/GetV5UsersUsernameRepos are generated with a single-Project
// return type and cannot decode the array this endpoint actually returns.
// The response is still decoded into the SDK's Project model so the
// conversion layer stays in one place.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	var path string
	if opts.Owner != "" {
		path = fmt.Sprintf("/users/%s/repos?page=%d&per_page=%d", esc(opts.Owner), page, perPage)
	} else {
		path = fmt.Sprintf("/user/repos?page=%d&per_page=%d", page, perPage)
	}
	var repos []gitee.Project
	if _, err := p.raw().DoJSON(ctx, &transport.Request{Method: "GET", Path: path, Result: &repos}); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListRepos", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(repos))
	for i := range repos {
		result = append(result, convertRepo(repos[i]))
	}
	return result, nil
}

// GetRepo implements provider.RepoManager.
func (p *Provider) GetRepo(ctx context.Context, owner, repo string) (*provider.PlatformRepo, error) {
	r, resp, err := p.client.RepositoriesApi.GetV5ReposOwnerRepo(ctx, esc(owner), esc(repo), &gitee.GetV5ReposOwnerRepoOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return nil, p.sdkErr("GetRepo", resp, err)
	}
	return convertRepo(r), nil
}

// DeleteRepo implements provider.RepoManager.
func (p *Provider) DeleteRepo(ctx context.Context, owner, repo string) error {
	resp, err := p.client.RepositoriesApi.DeleteV5ReposOwnerRepo(ctx, esc(owner), esc(repo), &gitee.DeleteV5ReposOwnerRepoOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return p.sdkErr("DeleteRepo", resp, err)
	}
	return nil
}

// UpdateRepo implements provider.RepoManager.
func (p *Provider) UpdateRepo(ctx context.Context, owner, repo string, opts provider.UpdateRepoOptions) (*provider.PlatformRepo, error) {
	// The SDK's RepoPatchParam models Gitee's string-typed boolean and leaves
	// empty fields out of the JSON body, matching the previous behaviour of
	// only sending explicitly set fields.
	body := gitee.RepoPatchParam{
		AccessToken:   p.token,
		Name:          opts.Name,
		Description:   opts.Description,
		DefaultBranch: opts.DefaultBranch,
	}
	if opts.Private != nil {
		body.Private = strconv.FormatBool(*opts.Private)
	}
	r, resp, err := p.client.RepositoriesApi.PatchV5ReposOwnerRepo(ctx, esc(owner), esc(repo), body)
	if err != nil {
		return nil, p.sdkErr("UpdateRepo", resp, err)
	}
	return convertRepo(r), nil
}

// ForkRepo implements provider.RepoManager.
func (p *Provider) ForkRepo(ctx context.Context, owner, repo string, opts provider.ForkRepoOptions) (*provider.PlatformRepo, error) {
	forkOpts := &gitee.PostV5ReposOwnerRepoForksOpts{
		AccessToken: p.accessToken(),
	}
	if opts.Organization != "" {
		forkOpts.Organization = optional.NewString(opts.Organization)
	}
	// The official fork endpoint accepts access_token/organization only, so
	// ForkRepoOptions.Name has no Gitee counterpart and is not forwarded.
	r, resp, err := p.client.RepositoriesApi.PostV5ReposOwnerRepoForks(ctx, esc(owner), esc(repo), forkOpts)
	if err != nil {
		return nil, p.sdkErr("ForkRepo", resp, err)
	}
	return convertRepo(r), nil
}

// CreateRepo implements provider.RepoManager.
//
// Routed through the raw transport client rather than the SDK: the SDK's
// RepositoryPostParam has no default_branch field, and this surface keeps
// forwarding it. The response is still decoded into the SDK's Project model
// so the conversion layer stays in one place.
func (p *Provider) CreateRepo(ctx context.Context, owner string, opts provider.CreateRepoOptions) (*provider.PlatformRepo, error) {
	body := map[string]any{
		"name":    opts.Name,
		"private": opts.Private,
	}
	if p.token != "" {
		body["access_token"] = p.token
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.AutoInit {
		body["auto_init"] = "true"
	}
	if opts.DefaultBranch != "" {
		body["default_branch"] = opts.DefaultBranch
	}

	var path string
	if owner != "" {
		path = fmt.Sprintf("/orgs/%s/repos", esc(owner))
	} else {
		path = "/user/repos"
	}

	var r gitee.Project
	if _, err := p.raw().DoJSON(ctx, &transport.Request{Method: "POST", Path: path, Body: body, Result: &r}); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateRepo", err)
	}
	return convertRepo(r), nil
}

var _ provider.RepoManager = (*Provider)(nil)
