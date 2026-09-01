package gitee

import (
	"context"
	"fmt"

	gitee "github.com/next-bin/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListForks implements provider.RepoStatsManager.
func (p *Provider) ListForks(ctx context.Context, owner, repo string) ([]*provider.PlatformRepo, error) {
	forks, _, err := p.client.Repositories.ListForks(ctx, esc(owner), esc(repo), nil)
	if err != nil {
		return nil, p.sdkErr("ListForks", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(forks))
	for _, f := range forks {
		result = append(result, convertProject(f))
	}
	return result, nil
}

// ListStargazers implements provider.RepoStatsManager.
//
// The new SDK does not expose a dedicated stargazers endpoint, so we use the
// client's generic NewRequest + Do pattern to call the API directly.
func (p *Provider) ListStargazers(ctx context.Context, owner, repo string) ([]*provider.CRUser, error) {
	u := fmt.Sprintf("repos/%s/%s/stargazers", esc(owner), esc(repo))
	req, err := p.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, p.sdkErr("ListStargazers", err)
	}
	var users []*gitee.UserBasic
	_, err = p.client.Do(req, &users)
	if err != nil {
		return nil, p.sdkErr("ListStargazers", err)
	}
	result := make([]*provider.CRUser, 0, len(users))
	for _, u := range users {
		result = append(result, &provider.CRUser{
			ID:        int64(deref(u.ID)),
			Username:  deref(u.Login),
			Name:      deref(u.Name),
			AvatarURL: deref(u.AvatarURL),
		})
	}
	return result, nil
}

// ListContributors implements provider.RepoStatsManager.
func (p *Provider) ListContributors(ctx context.Context, owner, repo string) ([]*provider.Contributor, error) {
	contributors, _, err := p.client.Repositories.ListContributors(ctx, esc(owner), esc(repo), nil)
	if err != nil {
		return nil, p.sdkErr("ListContributors", err)
	}
	result := make([]*provider.Contributor, 0, len(contributors))
	for _, c := range contributors {
		result = append(result, convertContributor(c))
	}
	return result, nil
}

var _ provider.RepoStatsManager = (*Provider)(nil)
