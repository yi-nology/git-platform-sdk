package github

import (
 "context"

 "github.com/yi-nology/git-platform-sdk/provider"
)

// ListForks implements provider.RepoStatsManager.
func (p *Provider) ListForks(ctx context.Context, owner, repo string) ([]*provider.PlatformRepo, error) {
 forks, _, err := p.client.Repositories.ListForks(ctx, owner, repo, nil)
 if err != nil {
  return nil, provider.Wrap(provider.PlatformGitHub, "ListForks", err)
 }
 result := make([]*provider.PlatformRepo, 0, len(forks))
 for _, r := range forks {
  result = append(result, convertRepo(r))
 }
 return result, nil
}

// ListStargazers implements provider.RepoStatsManager.
func (p *Provider) ListStargazers(ctx context.Context, owner, repo string) ([]*provider.CRUser, error) {
 stargazers, _, err := p.client.Activity.ListStargazers(ctx, owner, repo, nil)
 if err != nil {
  return nil, provider.Wrap(provider.PlatformGitHub, "ListStargazers", err)
 }
 result := make([]*provider.CRUser, 0, len(stargazers))
 for _, s := range stargazers {
  if s.User != nil {
   result = append(result, convertUser(s.User))
  }
 }
 return result, nil
}

// ListContributors implements provider.RepoStatsManager.
func (p *Provider) ListContributors(ctx context.Context, owner, repo string) ([]*provider.Contributor, error) {
 contributors, _, err := p.client.Repositories.ListContributors(ctx, owner, repo, nil)
 if err != nil {
  return nil, provider.Wrap(provider.PlatformGitHub, "ListContributors", err)
 }
 result := make([]*provider.Contributor, 0, len(contributors))
 for _, c := range contributors {
  username := ""
  if c.Login != nil {
   username = *c.Login
  }
  contributions := 0
  if c.Contributions != nil {
   contributions = *c.Contributions
  }
  result = append(result, &provider.Contributor{
   Username:      username,
   Contributions: contributions,
  })
 }
 return result, nil
}

var _ provider.RepoStatsManager = (*Provider)(nil)
