package forgejo

import (
	"context"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListCollaborators implements provider.CollaboratorManager.
func (p *Provider) ListCollaborators(ctx context.Context, owner, repo string) ([]*provider.Collaborator, error) {
	users, _, err := p.client.ListCollaborators(owner, repo, forgejo.ListCollaboratorsOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListCollaborators", err)
	}
	result := make([]*provider.Collaborator, 0, len(users))
	for _, u := range users {
		collab := &provider.Collaborator{
			ID:       u.ID,
			Username: u.UserName,
		}
		// Fetch permission for each collaborator. Forgejo's ListCollaborators
		// returns User objects without permission info.
		perm, _, permErr := p.client.CollaboratorPermission(owner, repo, u.UserName)
		if permErr == nil && perm != nil {
			collab.Permission = string(perm.Permission)
		}
		result = append(result, collab)
	}
	return result, nil
}

// AddCollaborator implements provider.CollaboratorManager.
func (p *Provider) AddCollaborator(ctx context.Context, owner, repo, username string, opts provider.AddCollaboratorOptions) error {
	opt := forgejo.AddCollaboratorOption{}
	if opts.Permission != "" {
		perm := forgejo.AccessMode(opts.Permission)
		opt.Permission = &perm
	}
	if _, err := p.client.AddCollaborator(owner, repo, username, opt); err != nil {
		return provider.Wrap(provider.PlatformForgejo, "AddCollaborator", err)
	}
	return nil
}

// RemoveCollaborator implements provider.CollaboratorManager.
func (p *Provider) RemoveCollaborator(ctx context.Context, owner, repo, username string) error {
	if _, err := p.client.DeleteCollaborator(owner, repo, username); err != nil {
		return provider.Wrap(provider.PlatformForgejo, "RemoveCollaborator", err)
	}
	return nil
}

var _ provider.CollaboratorManager = (*Provider)(nil)
