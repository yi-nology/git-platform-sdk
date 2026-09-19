package gitea

import (
	"context"

	gitea "gitea.dev/sdk"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListCollaborators implements provider.CollaboratorManager. The provider
// surface carries no pagination parameters, so the full collaborator list
// is fetched by exhausting the endpoint's pagination (backendutil.AllPages).
func (p *Provider) ListCollaborators(ctx context.Context, owner, repo string) ([]*provider.Collaborator, error) {
	users, err := backendutil.AllPages(func(page int) ([]*gitea.User, error) {
		list, _, err := p.client.Repositories.ListCollaborators(ctx, owner, repo, gitea.ListCollaboratorsOptions{
			ListOptions: gitea.ListOptions{Page: page, PageSize: listPageSize},
		})
		return list, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListCollaborators", err)
	}
	result := make([]*provider.Collaborator, 0, len(users))
	for _, u := range users {
		collab := &provider.Collaborator{
			ID:       u.ID,
			Username: u.UserName,
		}
		// Fetch permission for each collaborator. Gitea's ListCollaborators
		// returns User objects without permission info.
		perm, _, permErr := p.client.Repositories.CollaboratorPermission(ctx, owner, repo, u.UserName)
		if permErr == nil && perm != nil {
			collab.Permission = string(perm.Permission)
		}
		result = append(result, collab)
	}
	return result, nil
}

// AddCollaborator implements provider.CollaboratorManager.
func (p *Provider) AddCollaborator(ctx context.Context, owner, repo, username string, opts provider.AddCollaboratorOptions) error {
	opt := gitea.AddCollaboratorOption{}
	if opts.Permission != "" {
		perm := gitea.AccessMode(opts.Permission)
		opt.Permission = &perm
	}
	if _, err := p.client.Repositories.AddCollaborator(ctx, owner, repo, username, opt); err != nil {
		return provider.Wrap(provider.PlatformGitea, "AddCollaborator", err)
	}
	return nil
}

// RemoveCollaborator implements provider.CollaboratorManager.
func (p *Provider) RemoveCollaborator(ctx context.Context, owner, repo, username string) error {
	if _, err := p.client.Repositories.DeleteCollaborator(ctx, owner, repo, username); err != nil {
		return provider.Wrap(provider.PlatformGitea, "RemoveCollaborator", err)
	}
	return nil
}

var _ provider.CollaboratorManager = (*Provider)(nil)
