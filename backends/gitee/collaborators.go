package gitee

import (
	"context"

	gitee "gitee.com/openeuler/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListCollaborators implements provider.CollaboratorManager. Gitee's
// collaborator endpoint returns ProjectMember objects; permission is not
// included in the list response and would require a separate per-user call.
func (p *Provider) ListCollaborators(ctx context.Context, owner, repo string) ([]*provider.Collaborator, error) {
	members, resp, err := p.client.RepositoriesApi.GetV5ReposOwnerRepoCollaborators(ctx, esc(owner), esc(repo), &gitee.GetV5ReposOwnerRepoCollaboratorsOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return nil, p.sdkErr("ListCollaborators", resp, err)
	}
	result := make([]*provider.Collaborator, 0, len(members))
	for _, m := range members {
		result = append(result, &provider.Collaborator{
			ID:       int64(m.Id),
			Username: m.Login,
		})
	}
	return result, nil
}

// AddCollaborator implements provider.CollaboratorManager.
func (p *Provider) AddCollaborator(ctx context.Context, owner, repo, username string, opts provider.AddCollaboratorOptions) error {
	body := gitee.ProjectMemberPutParam{
		AccessToken: p.token,
		Permission:  opts.Permission,
	}
	_, resp, err := p.client.RepositoriesApi.PutV5ReposOwnerRepoCollaboratorsUsername(ctx, esc(owner), esc(repo), esc(username), body)
	if err != nil {
		return p.sdkErr("AddCollaborator", resp, err)
	}
	return nil
}

// RemoveCollaborator implements provider.CollaboratorManager.
func (p *Provider) RemoveCollaborator(ctx context.Context, owner, repo, username string) error {
	resp, err := p.client.RepositoriesApi.DeleteV5ReposOwnerRepoCollaboratorsUsername(ctx, esc(owner), esc(repo), esc(username), &gitee.DeleteV5ReposOwnerRepoCollaboratorsUsernameOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return p.sdkErr("RemoveCollaborator", resp, err)
	}
	return nil
}

var _ provider.CollaboratorManager = (*Provider)(nil)
