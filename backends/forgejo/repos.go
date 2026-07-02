package forgejo

import (
	"context"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListRepos implements provider.RepoManager.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	opts.Page, opts.PerPage = provider.NormalizePageOpts(opts.Page, opts.PerPage)
	if opts.Owner != "" {
		results, _, err := p.client.SearchRepos(forgejo.SearchRepoOptions{
			ListOptions: forgejo.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
		})
		if err != nil {
			return nil, provider.Wrap(provider.PlatformForgejo, "ListRepos", err)
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
	repos, _, err := p.client.ListMyRepos(forgejo.ListReposOptions{
		ListOptions: forgejo.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListRepos", err)
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
		if resp != nil && resp.Response != nil {
			err = provider.Wrap(provider.PlatformForgejo, "GetRepo",
				provider.New(provider.PlatformForgejo, "GetRepo", resp.Response.StatusCode, err.Error()))
		} else {
			err = provider.Wrap(provider.PlatformForgejo, "GetRepo", err)
		}
		return nil, err
	}
	return convertRepo(r), nil
}

// ForkRepo implements provider.RepoManager.
func (p *Provider) ForkRepo(ctx context.Context, owner, repo string, opts provider.ForkRepoOptions) (*provider.PlatformRepo, error) {
	forkOpts := forgejo.CreateForkOption{}
	if opts.Organization != "" {
		forkOpts.Organization = &opts.Organization
	}
	if opts.Name != "" {
		forkOpts.Name = &opts.Name
	}
	r, _, err := p.client.CreateFork(owner, repo, forkOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ForkRepo", err)
	}
	return convertRepo(r), nil
}

// DeleteRepo implements provider.RepoManager.
func (p *Provider) DeleteRepo(ctx context.Context, owner, repo string) error {
	_, err := p.client.DeleteRepo(owner, repo)
	if err != nil {
		return provider.Wrap(provider.PlatformForgejo, "DeleteRepo", err)
	}
	return nil
}

// UpdateRepo implements provider.RepoManager.
func (p *Provider) UpdateRepo(ctx context.Context, owner, repo string, opts provider.UpdateRepoOptions) (*provider.PlatformRepo, error) {
	editOpts := forgejo.EditRepoOption{}
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
		return nil, provider.Wrap(provider.PlatformForgejo, "UpdateRepo", err)
	}
	return convertRepo(r), nil
}

var _ provider.RepoManager = (*Provider)(nil)
