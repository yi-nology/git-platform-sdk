package gitee

import (
	"context"

	gitee "github.com/next-bin/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// SearchRepos implements provider.SearchManager.
func (p *Provider) SearchRepos(ctx context.Context, opts provider.SearchReposOptions) ([]*provider.SearchRepoResult, *int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := &gitee.SearchRepoOptions{
		Q:       gitee.String(opts.Query),
		Page:    gitee.Int(page),
		PerPage: gitee.Int(perPage),
	}
	if opts.Sort != "" {
		searchOpts.Sort = gitee.String(opts.Sort)
	}
	if opts.Order != "" {
		searchOpts.Order = gitee.String(opts.Order)
	}
	repos, _, err := p.client.Search.Repositories(ctx, searchOpts)
	if err != nil {
		return nil, nil, p.sdkErr("SearchRepos", err)
	}
	out := make([]*provider.SearchRepoResult, 0, len(repos))
	for _, r := range repos {
		out = append(out, &provider.SearchRepoResult{
			FullName:      deref(r.FullName),
			Description:   deref(r.Description),
			WebURL:        deref(r.HTMLURL),
			Stars:         deref(r.StargazersCount),
			Forks:         deref(r.ForksCount),
			DefaultBranch: deref(r.DefaultBranch),
			Private:       deref(r.Private),
		})
	}
	return out, nil, nil
}

// SearchIssues implements provider.SearchManager.
func (p *Provider) SearchIssues(ctx context.Context, opts provider.SearchIssuesOptions) ([]*provider.SearchIssueResult, *int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := &gitee.SearchIssueOptions{
		Q:       gitee.String(opts.Query),
		Page:    gitee.Int(page),
		PerPage: gitee.Int(perPage),
	}
	if opts.Repo != "" {
		searchOpts.Repo = gitee.String(opts.Repo)
	}
	if opts.State != "" {
		searchOpts.State = gitee.String(opts.State)
	}
	if opts.Sort != "" {
		searchOpts.Sort = gitee.String(opts.Sort)
	}
	if opts.Order != "" {
		searchOpts.Order = gitee.String(opts.Order)
	}
	issues, _, err := p.client.Search.Issues(ctx, searchOpts)
	if err != nil {
		return nil, nil, p.sdkErr("SearchIssues", err)
	}
	out := make([]*provider.SearchIssueResult, 0, len(issues))
	for _, issue := range issues {
		var labels []string
		if issue.Labels != nil {
			for _, l := range *issue.Labels {
				labels = append(labels, deref(l.Name))
			}
		}
		out = append(out, &provider.SearchIssueResult{
			Number:    deref(issue.Number),
			Title:     deref(issue.Title),
			Body:      deref(issue.Body),
			State:     provider.IssueState(deref(issue.State)),
			WebURL:    deref(issue.HTMLURL),
			Labels:    labels,
			Comments:  deref(issue.Comments),
			CreatedAt: parseGiteeTime(issue.CreatedAt),
		})
	}
	return out, nil, nil
}

// SearchUsers implements provider.SearchManager.
func (p *Provider) SearchUsers(ctx context.Context, opts provider.SearchUsersOptions) ([]*provider.SearchUserResult, *int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := &gitee.SearchUserOptions{
		Q:       gitee.String(opts.Query),
		Page:    gitee.Int(page),
		PerPage: gitee.Int(perPage),
	}
	if opts.Sort != "" {
		searchOpts.Sort = gitee.String(opts.Sort)
	}
	if opts.Order != "" {
		searchOpts.Order = gitee.String(opts.Order)
	}
	users, _, err := p.client.Search.Users(ctx, searchOpts)
	if err != nil {
		return nil, nil, p.sdkErr("SearchUsers", err)
	}
	out := make([]*provider.SearchUserResult, 0, len(users))
	for _, u := range users {
		out = append(out, &provider.SearchUserResult{
			Login:     deref(u.Login),
			Name:      deref(u.Name),
			AvatarURL: deref(u.AvatarURL),
			WebURL:    deref(u.HTMLURL),
		})
	}
	return out, nil, nil
}

var _ provider.SearchManager = (*Provider)(nil)
