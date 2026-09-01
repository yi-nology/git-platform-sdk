package gitea

import (
	"context"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListForks implements provider.RepoStatsManager.
func (p *Provider) ListForks(ctx context.Context, owner, repo string) ([]*provider.PlatformRepo, error) {
	forks, _, err := p.client.ListForks(owner, repo, gitea.ListForksOptions{
		ListOptions: gitea.ListOptions{Page: 1, PageSize: 50},
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

// ListStargazers implements provider.RepoStatsManager.
func (p *Provider) ListStargazers(ctx context.Context, owner, repo string) ([]*provider.CRUser, error) {
	stargazers, _, err := p.client.ListRepoStargazers(owner, repo, gitea.ListStargazersOptions{
		ListOptions: gitea.ListOptions{Page: 1, PageSize: 50},
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
