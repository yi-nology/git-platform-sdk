package gitee

import (
 "context"
 "fmt"
 "strconv"

 gitee "gitee.com/openeuler/go-gitee/gitee"
 "github.com/antihax/optional"

 "github.com/yi-nology/git-platform-sdk/provider"
 "github.com/yi-nology/git-platform-sdk/transport"
)

// ListForks implements provider.RepoStatsManager.
//
// Routed through the raw transport client rather than the SDK: go-gitee's
// GetV5ReposOwnerRepoForks is generated with a single-Project return type
// and cannot decode the array this endpoint actually returns.
func (p *Provider) ListForks(ctx context.Context, owner, repo string) ([]*provider.PlatformRepo, error) {
 path := fmt.Sprintf("/repos/%s/%s/forks?access_token=%s", esc(owner), esc(repo), p.token)
 var forks []gitee.Project
 if _, err := p.raw().DoJSON(ctx, &transport.Request{Method: "GET", Path: path, Result: &forks}); err != nil {
  return nil, provider.Wrap(provider.PlatformGitee, "ListForks", err)
 }
 result := make([]*provider.PlatformRepo, 0, len(forks))
 for i := range forks {
  result = append(result, convertRepo(forks[i]))
 }
 return result, nil
}

// ListStargazers implements provider.RepoStatsManager.
func (p *Provider) ListStargazers(ctx context.Context, owner, repo string) ([]*provider.CRUser, error) {
 stargazers, resp, err := p.client.ActivityApi.GetV5ReposOwnerRepoStargazers(ctx, esc(owner), esc(repo), &gitee.GetV5ReposOwnerRepoStargazersOpts{
  AccessToken: p.accessToken(),
  Page:        optional.NewInt32(1),
  PerPage:     optional.NewInt32(50),
 })
 if err != nil {
  return nil, p.sdkErr("ListStargazers", resp, err)
 }
 result := make([]*provider.CRUser, 0, len(stargazers))
 for _, u := range stargazers {
  result = append(result, &provider.CRUser{
   ID:        int64(u.Id),
   Username:  u.Login,
   Name:      u.Name,
   AvatarURL: u.AvatarUrl,
  })
 }
 return result, nil
}

// ListContributors implements provider.RepoStatsManager.
//
// Routed through the raw transport client rather than the SDK: go-gitee's
// GetV5ReposOwnerRepoContributors is generated with a single-Contributor
// return type and cannot decode the array this endpoint actually returns.
func (p *Provider) ListContributors(ctx context.Context, owner, repo string) ([]*provider.Contributor, error) {
 path := fmt.Sprintf("/repos/%s/%s/contributors?access_token=%s", esc(owner), esc(repo), p.token)
 var giteeContributors []struct {
  Name          string `json:"name"`
  Contributions string `json:"contributions"`
 }
 if _, err := p.raw().DoJSON(ctx, &transport.Request{Method: "GET", Path: path, Result: &giteeContributors}); err != nil {
  return nil, provider.Wrap(provider.PlatformGitee, "ListContributors", err)
 }
 result := make([]*provider.Contributor, 0, len(giteeContributors))
 for _, c := range giteeContributors {
  contributions := 0
  if n, err := strconv.Atoi(c.Contributions); err == nil {
   contributions = n
  }
  result = append(result, &provider.Contributor{
   Username:      c.Name,
   Contributions: contributions,
  })
 }
 return result, nil
}

var _ provider.RepoStatsManager = (*Provider)(nil)
