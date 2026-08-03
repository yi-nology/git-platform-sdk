package gitea

import (
	"context"
	"strings"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListRepos implements provider.RepoManager.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	opts.Page, opts.PerPage = provider.NormalizePageOpts(opts.Page, opts.PerPage)
	if opts.Owner != "" {
		results, _, err := p.client.SearchRepos(gitea.SearchRepoOptions{
			ListOptions: gitea.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
		})
		if err != nil {
			return nil, provider.Wrap(provider.PlatformGitea, "ListRepos", err)
		}
		filtered := make([]*provider.PlatformRepo, 0)
		for _, r := range results {
			pr := convertRepo(r)
			if strings.EqualFold(pr.Owner, opts.Owner) {
				filtered = append(filtered, pr)
			}
		}
		return filtered, nil
	}
	repos, _, err := p.client.ListMyRepos(gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListRepos", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(repos))
	for _, r := range repos {
		result = append(result, convertRepo(r))
	}
	return result, nil
}

// GetRepo implements provider.RepoManager.
func (p *Provider) GetRepo(ctx context.Context, owner, repo string) (*provider.PlatformRepo, error) {
	r, resp, err := p.client.GetRepo(owner, repo)
	if err != nil {
		// Gitea's error type does not carry the HTTP status; preserve it
		// from the *Response so provider.IsNotFound works.
		if resp != nil && resp.Response != nil {
			err = provider.Wrap(provider.PlatformGitea, "GetRepo",
				provider.New(provider.PlatformGitea, "GetRepo", resp.Response.StatusCode, err.Error()))
		} else {
			err = provider.Wrap(provider.PlatformGitea, "GetRepo", err)
		}
		return nil, err
	}
	return convertRepo(r), nil
}

// CreateRepo implements provider.RepoManager.
func (p *Provider) CreateRepo(ctx context.Context, owner string, opts provider.CreateRepoOptions) (*provider.PlatformRepo, error) {
	createOpts := gitea.CreateRepoOption{
		Name:        opts.Name,
		Description: opts.Description,
		Private:     opts.Private,
		AutoInit:    opts.AutoInit,
	}
	if opts.DefaultBranch != "" {
		createOpts.DefaultBranch = opts.DefaultBranch
	}
	var r *gitea.Repository
	var err error
	if owner != "" {
		r, _, err = p.client.CreateOrgRepo(owner, createOpts)
	} else {
		r, _, err = p.client.CreateRepo(createOpts)
	}
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateRepo", err)
	}
	return convertRepo(r), nil
}

// ForkRepo implements provider.RepoManager.
func (p *Provider) ForkRepo(ctx context.Context, owner, repo string, opts provider.ForkRepoOptions) (*provider.PlatformRepo, error) {
	forkOpts := gitea.CreateForkOption{}
	if opts.Organization != "" {
		forkOpts.Organization = &opts.Organization
	}
	if opts.Name != "" {
		forkOpts.Name = &opts.Name
	}
	r, _, err := p.client.CreateFork(owner, repo, forkOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ForkRepo", err)
	}
	return convertRepo(r), nil
}

// DeleteRepo implements provider.RepoManager.
func (p *Provider) DeleteRepo(ctx context.Context, owner, repo string) error {
	_, err := p.client.DeleteRepo(owner, repo)
	if err != nil {
		return provider.Wrap(provider.PlatformGitea, "DeleteRepo", err)
	}
	return nil
}

// UpdateRepo implements provider.RepoManager.
func (p *Provider) UpdateRepo(ctx context.Context, owner, repo string, opts provider.UpdateRepoOptions) (*provider.PlatformRepo, error) {
	editOpts := gitea.EditRepoOption{}
	if opts.Name != "" {
		editOpts.Name = &opts.Name
	}
	if opts.Description != "" {
		editOpts.Description = &opts.Description
	}
	if opts.DefaultBranch != "" {
		editOpts.DefaultBranch = &opts.DefaultBranch
	}
	if opts.Private != nil {
		editOpts.Private = opts.Private
	}
	r, _, err := p.client.EditRepo(owner, repo, editOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "UpdateRepo", err)
	}
	return convertRepo(r), nil
}

var _ provider.RepoManager = (*Provider)(nil)
