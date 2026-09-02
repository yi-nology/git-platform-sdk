package gitcode

import (
	"context"

	gitcode "github.com/yi-nology/go-gitcode"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListForks implements provider.RepoStatsManager.
func (p *Provider) ListForks(ctx context.Context, owner, repo string) ([]*provider.PlatformRepo, error) {
	forks, err := p.client.ListForks(ctx, owner, repo, gitcode.ListOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListForks", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(forks))
	for _, r := range forks {
		result = append(result, convertGitcodeRepo(r))
	}
	return result, nil
}

// ListStargazers implements provider.RepoStatsManager.
func (p *Provider) ListStargazers(ctx context.Context, owner, repo string) ([]*provider.CRUser, error) {
	stargazers, err := p.client.ListStargazers(ctx, owner, repo, gitcode.ListOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListStargazers", err)
	}
	result := make([]*provider.CRUser, 0, len(stargazers))
	for _, u := range stargazers {
		id, _ := parseGitCodeID(u.ID)
		result = append(result, &provider.CRUser{
			ID:        id,
			Username:  u.Login,
			Name:      u.Name,
			AvatarURL: u.AvatarURL,
		})
	}
	return result, nil
}

// ListContributors implements provider.RepoStatsManager.
func (p *Provider) ListContributors(ctx context.Context, owner, repo string) ([]*provider.Contributor, error) {
	contributors, err := p.client.ListContributors(ctx, owner, repo)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListContributors", err)
	}
	result := make([]*provider.Contributor, 0, len(contributors))
	for _, c := range contributors {
		result = append(result, &provider.Contributor{
			Username:      c.Login,
			Contributions: c.Contributions,
		})
	}
	return result, nil
}

// convertGitcodeRepo maps a gitcode.Repository to a provider.PlatformRepo.
func convertGitcodeRepo(r *gitcode.Repository) *provider.PlatformRepo {
	if r == nil {
		return nil
	}
	owner := ""
	if r.Owner != nil {
		owner = r.Owner.Login
	}
	return &provider.PlatformRepo{
		ID:            r.ID,
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         owner,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      provider.PlatformGitCode,
	}
}

var _ provider.RepoStatsManager = (*Provider)(nil)
