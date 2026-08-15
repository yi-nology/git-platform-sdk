package gitlab

import (
	"context"
	"strconv"

	"github.com/yi-nology/git-platform-sdk/provider"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// This file implements the provider.SearchManager surface over the GitLab
// client-go SDK, via the typed SearchService scopes (projects/issues/users).
//
// Registrations (design spec §4.6 mapping ledger):
//   - SearchIssuesOptions.Repo routes to Search.IssuesByProject (the
//     project-scoped search endpoint) when set; otherwise the global
//     Search.Issues runs.
//   - GitLab's search API exposes no state, sort, or order parameters, so
//     SearchIssuesOptions.State/Sort/Order and the sort/order fields of the
//     repo/user options are not forwarded (registered ignore).
//   - The SDK returns no total for searches, so the page size is reported.
//   - Wire states "opened"/"reopened" normalize to open (as elsewhere).

// SearchRepos implements provider.SearchManager.
func (p *Provider) SearchRepos(ctx context.Context, opts provider.SearchReposOptions) ([]*provider.SearchRepoResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	projects, _, err := p.client.Search.Projects(opts.Query, &gitlab.SearchOptions{
		ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: int64(perPage)},
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitLab, "SearchRepos", err)
	}
	out := make([]*provider.SearchRepoResult, 0, len(projects))
	for _, pr := range projects {
		out = append(out, &provider.SearchRepoResult{
			FullName:      pr.PathWithNamespace,
			Description:   pr.Description,
			WebURL:        pr.WebURL,
			Stars:         int(pr.StarCount),
			Forks:         int(pr.ForksCount),
			DefaultBranch: pr.DefaultBranch,
			Private:       pr.Visibility != "public",
		})
	}
	return out, len(out), nil
}

// SearchIssues implements provider.SearchManager.
func (p *Provider) SearchIssues(ctx context.Context, opts provider.SearchIssuesOptions) ([]*provider.SearchIssueResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	searchOpts := &gitlab.SearchOptions{
		ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: int64(perPage)},
	}
	var issues []*gitlab.Issue
	var err error
	if opts.Repo != "" {
		issues, _, err = p.client.Search.IssuesByProject(opts.Repo, opts.Query, searchOpts, gitlab.WithContext(ctx))
	} else {
		issues, _, err = p.client.Search.Issues(opts.Query, searchOpts, gitlab.WithContext(ctx))
	}
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitLab, "SearchIssues", err)
	}
	out := make([]*provider.SearchIssueResult, 0, len(issues))
	for _, i := range issues {
		state := provider.IssueStateOpen
		if i.State == "closed" {
			state = provider.IssueStateClosed
		}
		out = append(out, &provider.SearchIssueResult{
			Number:    strconv.FormatInt(i.IID, 10),
			Title:     i.Title,
			Body:      i.Description,
			State:     state,
			WebURL:    i.WebURL,
			Labels:    append([]string(nil), i.Labels...),
			Comments:  int(i.UserNotesCount),
			CreatedAt: timeOrZero(i.CreatedAt),
		})
	}
	return out, len(out), nil
}

// SearchUsers implements provider.SearchManager.
func (p *Provider) SearchUsers(ctx context.Context, opts provider.SearchUsersOptions) ([]*provider.SearchUserResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	users, _, err := p.client.Search.Users(opts.Query, &gitlab.SearchOptions{
		ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: int64(perPage)},
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitLab, "SearchUsers", err)
	}
	out := make([]*provider.SearchUserResult, 0, len(users))
	for _, u := range users {
		out = append(out, &provider.SearchUserResult{
			Login:     u.Username,
			Name:      u.Name,
			AvatarURL: u.AvatarURL,
			WebURL:    u.WebURL,
		})
	}
	return out, len(out), nil
}

var _ provider.SearchManager = (*Provider)(nil)
