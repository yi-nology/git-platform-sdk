package gitee

import (
	"context"

	gitee "github.com/next-bin/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListRepos implements provider.RepoManager.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gitee.RepositoryListOptions{
		Page:    gitee.Int(page),
		PerPage: gitee.Int(perPage),
	}
	var repos []*gitee.Project
	var err error
	if opts.Owner != "" {
		userOpts := &gitee.RepositoryListByUserOptions{
			Page:    gitee.Int(page),
			PerPage: gitee.Int(perPage),
		}
		repos, _, err = p.client.Repositories.ListByUser(ctx, opts.Owner, userOpts)
	} else {
		repos, _, err = p.client.Repositories.List(ctx, listOpts)
	}
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListRepos", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(repos))
	for _, r := range repos {
		result = append(result, convertProject(r))
	}
	return result, nil
}

// GetRepo implements provider.RepoManager.
func (p *Provider) GetRepo(ctx context.Context, owner, repo string) (*provider.PlatformRepo, error) {
	r, _, err := p.client.Repositories.Get(ctx, esc(owner), esc(repo))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetRepo", err)
	}
	return convertProject(r), nil
}

// DeleteRepo implements provider.RepoManager.
func (p *Provider) DeleteRepo(ctx context.Context, owner, repo string) error {
	_, err := p.client.Repositories.Delete(ctx, esc(owner), esc(repo))
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteRepo", err)
	}
	return nil
}

// UpdateRepo implements provider.RepoManager.
func (p *Provider) UpdateRepo(ctx context.Context, owner, repo string, opts provider.UpdateRepoOptions) (*provider.PlatformRepo, error) {
	updateOpts := &gitee.UpdateRepositoryOptions{
		Name:          gitee.String(opts.Name),
		Description:   gitee.String(opts.Description),
		DefaultBranch: gitee.String(opts.DefaultBranch),
	}
	if opts.Private != nil {
		updateOpts.Private = gitee.Bool(*opts.Private)
	}
	r, _, err := p.client.Repositories.Edit(ctx, esc(owner), esc(repo), updateOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateRepo", err)
	}
	return convertProject(r), nil
}

// ForkRepo implements provider.RepoManager.
func (p *Provider) ForkRepo(ctx context.Context, owner, repo string, opts provider.ForkRepoOptions) (*provider.PlatformRepo, error) {
	forkOpts := &gitee.CreateForkOptions{}
	if opts.Organization != "" {
		forkOpts.Organization = gitee.String(opts.Organization)
	}
	r, _, err := p.client.Repositories.CreateFork(ctx, esc(owner), esc(repo), forkOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ForkRepo", err)
	}
	return convertProject(r), nil
}

// CreateRepo implements provider.RepoManager.
func (p *Provider) CreateRepo(ctx context.Context, owner string, opts provider.CreateRepoOptions) (*provider.PlatformRepo, error) {
	createOpts := &gitee.CreateRepositoryOptions{
		Name:    gitee.String(opts.Name),
		Private: gitee.Bool(opts.Private),
	}
	if opts.Description != "" {
		createOpts.Description = gitee.String(opts.Description)
	}
	if opts.AutoInit {
		createOpts.AutoInit = gitee.Bool(true)
	}

	var r *gitee.Project
	var err error
	if owner != "" {
		orgOpts := &gitee.CreateOrgRepoOptions{
			Name:        gitee.String(opts.Name),
			Description: gitee.String(opts.Description),
			AutoInit:    gitee.Bool(opts.AutoInit),
		}
		if opts.Private {
			orgOpts.Private = gitee.Bool(true)
		}
		r, _, err = p.client.Repositories.CreateInOrg(ctx, esc(owner), orgOpts)
	} else {
		r, _, err = p.client.Repositories.Create(ctx, createOpts)
	}
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateRepo", err)
	}
	return convertProject(r), nil
}

var _ provider.RepoManager = (*Provider)(nil)
