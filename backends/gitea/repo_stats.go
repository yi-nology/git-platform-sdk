package gitea

import (
	"context"

	gitea "gitea.dev/sdk"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListForks implements provider.RepoStatsManager. The provider surface
// carries no pagination parameters, so the full fork list is fetched by
// exhausting the endpoint's pagination (backendutil.AllPages).
func (p *Provider) ListForks(ctx context.Context, owner, repo string) ([]*provider.PlatformRepo, error) {
	forks, err := backendutil.AllPages(func(page int) ([]*gitea.Repository, error) {
		list, _, err := p.client.Repositories.ListForks(ctx, owner, repo, gitea.ListForksOptions{
			ListOptions: gitea.ListOptions{Page: page, PageSize: listPageSize},
		})
		return list, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListForks", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(forks))
	for _, r := range forks {
		result = append(result, convertRepo(r))
	}
	return result, nil
}

// ListStargazers implements provider.RepoStatsManager. The provider surface
// carries no pagination parameters, so the full stargazer list is fetched
// by exhausting the endpoint's pagination (backendutil.AllPages).
func (p *Provider) ListStargazers(ctx context.Context, owner, repo string) ([]*provider.CRUser, error) {
	stargazers, err := backendutil.AllPages(func(page int) ([]*gitea.User, error) {
		list, _, err := p.client.Repositories.ListRepoStargazers(ctx, owner, repo, gitea.ListStargazersOptions{
			ListOptions: gitea.ListOptions{Page: page, PageSize: listPageSize},
		})
		return list, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListStargazers", err)
	}
	result := make([]*provider.CRUser, 0, len(stargazers))
	for _, u := range stargazers {
		if u != nil {
			result = append(result, &provider.CRUser{
				ID:        u.ID,
				Username:  u.UserName,
				Name:      u.FullName,
				AvatarURL: u.AvatarURL,
			})
		}
	}
	return result, nil
}

// ListContributors implements provider.RepoStatsManager.
//
// Gitea does not expose a contributors list endpoint; this method
// always returns ErrNotImplemented.
func (p *Provider) ListContributors(_ context.Context, _, _ string) ([]*provider.Contributor, error) {
	return nil, provider.Wrapf(provider.PlatformGitea, "ListContributors", "not supported on this platform")
}

var _ provider.RepoStatsManager = (*Provider)(nil)
