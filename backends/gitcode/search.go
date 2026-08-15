package gitcode

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/provider"
	gitcode "github.com/yi-nology/gitcode_api"
)

// SearchRepos implements provider.SearchManager.
func (p *Provider) SearchRepos(ctx context.Context, opts provider.SearchReposOptions) ([]*provider.SearchRepoResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := gitcode.SearchRepositoriesOptions{
		ListOptions: gitcode.ListOptions{Page: page, PerPage: perPage},
		Query:       opts.Query,
		Sort:        opts.Sort,
		Order:       opts.Order,
	}
	results, err := p.client.SearchRepositories(ctx, searchOpts)
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitCode, "SearchRepos", err)
	}
	out := make([]*provider.SearchRepoResult, 0, len(results))
	for _, r := range results {
		out = append(out, &provider.SearchRepoResult{
			FullName:      r.FullName,
			Description:   r.Description,
			WebURL:        r.WebURL,
			Stars:         r.StargazersCount,
			Forks:         r.ForksCount,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}
	return out, len(out), nil
}

// SearchIssues implements provider.SearchManager.
func (p *Provider) SearchIssues(ctx context.Context, opts provider.SearchIssuesOptions) ([]*provider.SearchIssueResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := gitcode.SearchIssuesOptions{
		ListOptions: gitcode.ListOptions{Page: page, PerPage: perPage},
		Query:       opts.Query,
		Sort:        opts.Sort,
		Order:       opts.Order,
		Repo:        opts.Repo,
		State:       opts.State,
	}
	results, err := p.client.SearchIssues(ctx, searchOpts)
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitCode, "SearchIssues", err)
	}
	out := make([]*provider.SearchIssueResult, 0, len(results))
	for _, r := range results {
		labels := make([]string, 0, len(r.Labels))
		for _, l := range r.Labels {
			labels = append(labels, l.Name)
		}
		out = append(out, &provider.SearchIssueResult{
			Number:    r.Number,
			Title:     r.Title,
			Body:      r.Body,
			State:     provider.IssueState(r.State),
			WebURL:    r.HTMLURL,
			Labels:    labels,
			Comments:  r.Comments,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, len(out), nil
}

// SearchUsers implements provider.SearchManager.
func (p *Provider) SearchUsers(ctx context.Context, opts provider.SearchUsersOptions) ([]*provider.SearchUserResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := gitcode.SearchUsersOptions{
		ListOptions: gitcode.ListOptions{Page: page, PerPage: perPage},
		Query:       opts.Query,
		Sort:        opts.Sort,
		Order:       opts.Order,
	}
	results, err := p.client.SearchUsers(ctx, searchOpts)
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitCode, "SearchUsers", err)
	}
	out := make([]*provider.SearchUserResult, 0, len(results))
	for _, r := range results {
		out = append(out, &provider.SearchUserResult{
			Login:     r.Login,
			Name:      r.Name,
			AvatarURL: r.AvatarURL,
			WebURL:    r.HTMLURL,
		})
	}
	return out, len(out), nil
}

var _ provider.SearchManager = (*Provider)(nil)
