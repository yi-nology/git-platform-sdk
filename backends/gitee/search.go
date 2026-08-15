package gitee

import (
	"context"

	gitee "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// This file implements the provider.SearchManager surface over the go-gitee
// SDK's search API (/v5/search/{repositories,issues,users}). All three
// endpoints are real; Repo/State on issue search forward as native query
// parameters, and the returned arrays are parsed directly (no generated
// wrapper defects observed — unlike gitee's create/patch endpoints, the
// search calls are plain GETs).

// SearchRepos implements provider.SearchManager.
func (p *Provider) SearchRepos(ctx context.Context, opts provider.SearchReposOptions) ([]*provider.SearchRepoResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := gitee.GetV5SearchRepositoriesOpts{
		AccessToken: p.accessToken(),
		Page:        optional.NewInt32(toInt32(page)),
		PerPage:     optional.NewInt32(toInt32(perPage)),
	}
	if opts.Sort != "" {
		searchOpts.Sort = optional.NewString(opts.Sort)
	}
	if opts.Order != "" {
		searchOpts.Order = optional.NewString(opts.Order)
	}
	repos, resp, err := p.client.SearchApi.GetV5SearchRepositories(ctx, opts.Query, &searchOpts)
	if err != nil {
		return nil, 0, p.sdkErr("SearchRepos", resp, err)
	}
	out := make([]*provider.SearchRepoResult, 0, len(repos))
	for i := range repos {
		r := repos[i]
		out = append(out, &provider.SearchRepoResult{
			FullName:      r.FullName,
			Description:   r.Description,
			WebURL:        r.HtmlUrl,
			Stars:         int(r.StargazersCount),
			Forks:         int(r.ForksCount),
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}
	return out, len(out), nil
}

// SearchIssues implements provider.SearchManager. Gitee issue numbers are
// alphanumeric strings on the wire and carry through as-is.
func (p *Provider) SearchIssues(ctx context.Context, opts provider.SearchIssuesOptions) ([]*provider.SearchIssueResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := gitee.GetV5SearchIssuesOpts{
		AccessToken: p.accessToken(),
		Page:        optional.NewInt32(toInt32(page)),
		PerPage:     optional.NewInt32(toInt32(perPage)),
	}
	if opts.Repo != "" {
		searchOpts.Repo = optional.NewString(opts.Repo)
	}
	if opts.State != "" {
		searchOpts.State = optional.NewString(opts.State)
	}
	if opts.Sort != "" {
		searchOpts.Sort = optional.NewString(opts.Sort)
	}
	if opts.Order != "" {
		searchOpts.Order = optional.NewString(opts.Order)
	}
	issues, resp, err := p.client.SearchApi.GetV5SearchIssues(ctx, opts.Query, &searchOpts)
	if err != nil {
		return nil, 0, p.sdkErr("SearchIssues", resp, err)
	}
	out := make([]*provider.SearchIssueResult, 0, len(issues))
	for i := range issues {
		issue := issues[i]
		labels := make([]string, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, l.Name)
		}
		out = append(out, &provider.SearchIssueResult{
			Number:    issue.Number,
			Title:     issue.Title,
			Body:      issue.Body,
			State:     provider.IssueState(issue.State),
			WebURL:    issue.HtmlUrl,
			Labels:    labels,
			Comments:  int(issue.Comments),
			CreatedAt: issue.CreatedAt,
		})
	}
	return out, len(out), nil
}

// SearchUsers implements provider.SearchManager.
func (p *Provider) SearchUsers(ctx context.Context, opts provider.SearchUsersOptions) ([]*provider.SearchUserResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := gitee.GetV5SearchUsersOpts{
		AccessToken: p.accessToken(),
		Page:        optional.NewInt32(toInt32(page)),
		PerPage:     optional.NewInt32(toInt32(perPage)),
	}
	if opts.Sort != "" {
		searchOpts.Sort = optional.NewString(opts.Sort)
	}
	if opts.Order != "" {
		searchOpts.Order = optional.NewString(opts.Order)
	}
	users, resp, err := p.client.SearchApi.GetV5SearchUsers(ctx, opts.Query, &searchOpts)
	if err != nil {
		return nil, 0, p.sdkErr("SearchUsers", resp, err)
	}
	out := make([]*provider.SearchUserResult, 0, len(users))
	for i := range users {
		u := users[i]
		out = append(out, &provider.SearchUserResult{
			Login:     u.Login,
			Name:      u.Name,
			AvatarURL: u.AvatarUrl,
			WebURL:    u.HtmlUrl,
		})
	}
	return out, len(out), nil
}

var _ provider.SearchManager = (*Provider)(nil)
