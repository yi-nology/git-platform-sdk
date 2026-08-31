package github

import (
	"context"

	"github.com/google/go-github/v72/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListCollaborators implements provider.CollaboratorManager.
func (p *Provider) ListCollaborators(ctx context.Context, owner, repo string) ([]*provider.Collaborator, error) {
	users, _, err := p.client.Repositories.ListCollaborators(ctx, owner, repo, &github.ListCollaboratorsOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListCollaborators", err)
	}
	result := make([]*provider.Collaborator, 0, len(users))
	for _, u := range users {
		result = append(result, convertCollaborator(u))
	}
	return result, nil
}

// AddCollaborator implements provider.CollaboratorManager.
func (p *Provider) AddCollaborator(ctx context.Context, owner, repo, username string, opts provider.AddCollaboratorOptions) error {
	githubOpts := &github.RepositoryAddCollaboratorOptions{
		Permission: opts.Permission,
	}
	if _, _, err := p.client.Repositories.AddCollaborator(ctx, owner, repo, username, githubOpts); err != nil {
		return provider.Wrap(provider.PlatformGitHub, "AddCollaborator", err)
	}
	return nil
}

// RemoveCollaborator implements provider.CollaboratorManager.
func (p *Provider) RemoveCollaborator(ctx context.Context, owner, repo, username string) error {
	if _, err := p.client.Repositories.RemoveCollaborator(ctx, owner, repo, username); err != nil {
		return provider.Wrap(provider.PlatformGitHub, "RemoveCollaborator", err)
	}
	return nil
}

// convertCollaborator maps a github.User to a provider.Collaborator.
// GitHub's ListCollaborators returns User objects; the permission map is
// collapsed to the highest role found (admin > push > pull).
func convertCollaborator(u *github.User) *provider.Collaborator {
	if u == nil {
		return nil
	}
	c := &provider.Collaborator{
		ID:       u.GetID(),
		Username: u.GetLogin(),
	}
	perms := u.GetPermissions()
	if len(perms) > 0 {
		// GitHub returns a map like {"admin": true, "push": true}; pick the
		// highest-level permission that is set.
		for _, role := range []string{"admin", "maintain", "push", "triage", "pull"} {
			if perms[role] {
				c.Permission = role
				break
			}
		}
	}
	return c
}

var _ provider.CollaboratorManager = (*Provider)(nil)
