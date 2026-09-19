package gitlab

import (
	"context"

	gitlab "gitlab.com/gitlab-org/api/client-go/v3"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListForks implements provider.RepoStatsManager.
func (p *Provider) ListForks(ctx context.Context, owner, repo string) ([]*provider.PlatformRepo, error) {
	forks, err := backendutil.AllPages(func(page int) ([]*gitlab.Project, error) {
		list, _, err := p.client.Projects.ListProjectForks(pidOf(owner, repo), &gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: 100}}, gitlab.WithContext(ctx))
		return list, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListForks", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(forks))
	for _, proj := range forks {
		result = append(result, convertProject(proj))
	}
	return result, nil
}

// ListStargazers implements provider.RepoStatsManager.
//
// GitLab does not expose a stargazers list endpoint equivalent to
// GitHub's; this method always returns ErrNotImplemented.
func (p *Provider) ListStargazers(_ context.Context, _, _ string) ([]*provider.CRUser, error) {
	return nil, provider.Wrapf(provider.PlatformGitLab, "ListStargazers", "not supported on this platform")
}

// ListContributors implements provider.RepoStatsManager.
func (p *Provider) ListContributors(ctx context.Context, owner, repo string) ([]*provider.Contributor, error) {
	contributors, err := backendutil.AllPages(func(page int) ([]*gitlab.Contributor, error) {
		list, _, err := p.client.Repositories.Contributors(pidOf(owner, repo), &gitlab.ListContributorsOptions{ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: 100}}, gitlab.WithContext(ctx))
		return list, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListContributors", err)
	}
	result := make([]*provider.Contributor, 0, len(contributors))
	for _, c := range contributors {
		result = append(result, &provider.Contributor{
			Username:      c.Name,
			Contributions: int(c.Commits),
		})
	}
	return result, nil
}

var _ provider.RepoStatsManager = (*Provider)(nil)
