package gitlab

import (
	"context"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

func pidOf(owner, repo string) string { return owner + "/" + repo }

// ListRepos implements provider.RepoManager.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	page64, perPage64 := int64(page), int64(perPage)
	var projects []*gitlab.Project
	var err error
	if opts.Owner != "" {
		projects, _, err = p.client.Groups.ListGroupProjects(opts.Owner, &gitlab.ListGroupProjectsOptions{
			ListOptions: gitlab.ListOptions{Page: page64, PerPage: perPage64},
		}, gitlab.WithContext(ctx))
	} else {
		projects, _, err = p.client.Projects.ListProjects(&gitlab.ListProjectsOptions{
			ListOptions: gitlab.ListOptions{Page: page64, PerPage: perPage64},
		}, gitlab.WithContext(ctx))
	}
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListRepos", err)
	}
	repos := make([]*provider.PlatformRepo, 0, len(projects))
	for _, proj := range projects {
		repos = append(repos, convertProject(proj))
	}
	return repos, nil
}

// GetRepo implements provider.RepoManager.
func (p *Provider) GetRepo(ctx context.Context, owner, repo string) (*provider.PlatformRepo, error) {
	proj, _, err := p.client.Projects.GetProject(pidOf(owner, repo), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "GetRepo", err)
	}
	return convertProject(proj), nil
}

// CreateRepo implements provider.RepoManager.
func (p *Provider) CreateRepo(ctx context.Context, owner string, opts provider.CreateRepoOptions) (*provider.PlatformRepo, error) {
	createOpts := &gitlab.CreateProjectOptions{
		Name:                 gitlab.Ptr(opts.Name),
		Description:          gitlab.Ptr(opts.Description),
		Visibility:           gitlab.Ptr(gitlab.VisibilityValue("private")),
		InitializeWithReadme: gitlab.Ptr(opts.AutoInit),
	}
	if !opts.Private {
		createOpts.Visibility = gitlab.Ptr(gitlab.VisibilityValue("public"))
	}
	if opts.DefaultBranch != "" {
		createOpts.DefaultBranch = gitlab.Ptr(opts.DefaultBranch)
	}
	if owner != "" {
		// Create under a namespace (group) by looking up the namespace ID from the owner path.
		namespaces, _, nsErr := p.client.Namespaces.SearchNamespace(owner, gitlab.WithContext(ctx))
		if nsErr == nil {
			for _, ns := range namespaces {
				if ns.Path == owner || ns.FullPath == owner {
					createOpts.NamespaceID = gitlab.Ptr(ns.ID)
					break
				}
			}
		}
	}
	proj, _, err := p.client.Projects.CreateProject(createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateRepo", err)
	}
	return convertProject(proj), nil
}

// ForkRepo implements provider.RepoManager.
func (p *Provider) ForkRepo(ctx context.Context, owner, repo string, opts provider.ForkRepoOptions) (*provider.PlatformRepo, error) {
	forkOpts := &gitlab.ForkProjectOptions{}
	if opts.Organization != "" {
		//nolint:staticcheck // Namespace is deprecated in newer client-go but still functional
		forkOpts.Namespace = gitlab.Ptr(opts.Organization)
	}
	if opts.Name != "" {
		forkOpts.Name = gitlab.Ptr(opts.Name)
	}
	proj, _, err := p.client.Projects.ForkProject(pidOf(owner, repo), forkOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ForkRepo", err)
	}
	return convertProject(proj), nil
}

// DeleteRepo implements provider.RepoManager.
func (p *Provider) DeleteRepo(ctx context.Context, owner, repo string) error {
	_, err := p.client.Projects.DeleteProject(pidOf(owner, repo), nil, gitlab.WithContext(ctx))
	if err != nil {
		return provider.Wrap(provider.PlatformGitLab, "DeleteRepo", err)
	}
	return nil
}

// UpdateRepo implements provider.RepoManager.
func (p *Provider) UpdateRepo(ctx context.Context, owner, repo string, opts provider.UpdateRepoOptions) (*provider.PlatformRepo, error) {
	updateOpts := &gitlab.EditProjectOptions{}
	if opts.Name != "" {
		updateOpts.Name = gitlab.Ptr(opts.Name)
	}
	if opts.Description != "" {
		updateOpts.Description = gitlab.Ptr(opts.Description)
	}
	if opts.DefaultBranch != "" {
		updateOpts.DefaultBranch = gitlab.Ptr(opts.DefaultBranch)
	}
	if opts.Private != nil {
		vis := "public"
		if *opts.Private {
			vis = "private"
		}
		updateOpts.Visibility = gitlab.Ptr(gitlab.VisibilityValue(vis))
	}
	proj, _, err := p.client.Projects.EditProject(pidOf(owner, repo), updateOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "UpdateRepo", err)
	}
	return convertProject(proj), nil
}

var _ provider.RepoManager = (*Provider)(nil)
