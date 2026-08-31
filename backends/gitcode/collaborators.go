package gitcode

import (
	"context"

	gitcode "github.com/yi-nology/go-gitcode"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListCollaborators implements provider.CollaboratorManager.
func (p *Provider) ListCollaborators(ctx context.Context, owner, repo string) ([]*provider.Collaborator, error) {
	collaborators, err := p.client.ListCollaborators(ctx, owner, repo, gitcode.ListOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListCollaborators", err)
	}
	result := make([]*provider.Collaborator, 0, len(collaborators))
	for _, c := range collaborators {
		result = append(result, convertCollaborator(c))
	}
	return result, nil
}

// AddCollaborator implements provider.CollaboratorManager.
func (p *Provider) AddCollaborator(ctx context.Context, owner, repo, username string, opts provider.AddCollaboratorOptions) error {
	gitcodeOpts := &gitcode.AddCollaboratorOptions{
		Permission: opts.Permission,
	}
	if err := p.client.AddCollaborator(ctx, owner, repo, username, gitcodeOpts); err != nil {
		return provider.Wrap(provider.PlatformGitCode, "AddCollaborator", err)
	}
	return nil
}

// RemoveCollaborator implements provider.CollaboratorManager.
func (p *Provider) RemoveCollaborator(ctx context.Context, owner, repo, username string) error {
	if err := p.client.RemoveCollaborator(ctx, owner, repo, username); err != nil {
		return provider.Wrap(provider.PlatformGitCode, "RemoveCollaborator", err)
	}
	return nil
}

// convertCollaborator maps a gitcode.Collaborator to a provider.Collaborator.
func convertCollaborator(c *gitcode.Collaborator) *provider.Collaborator {
	if c == nil {
		return nil
	}
	return &provider.Collaborator{
		ID:         c.ID,
		Username:   c.Login,
		Permission: c.Permission,
	}
}

var _ provider.CollaboratorManager = (*Provider)(nil)
