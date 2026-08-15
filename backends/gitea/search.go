package gitea

import (
	"context"
	"strconv"
	"strings"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// This file implements the provider.SearchManager surface over the Gitea
// SDK. Mappings and registrations (design spec §4.6 ledger):
//   - SearchRepos rides the global keyword repo search (/repos/search);
//     Sort passes through in Gitea's own vocabulary (alpha/created/updated/
//     size/id) — values from other platforms' vocabularies are ignored
//     server-side (registered pass-through).
//   - SearchIssues rides the global keyword issue search
//     (/repos/issues/search, KeyWord param) restricted to real issues via
//     the "issues" type filter (the endpoint also matches pull requests).
//     SearchIssuesOptions.Repo ("owner/repo") is applied as a client-side
//     filter on each hit's repository metadata — the global endpoint
//     exposes no repo parameter (registered mapping). State forwards
//     natively.
//   - SearchUsers rides /users/search.
//   - The SDK returns no totals, so the page size is reported. The SDK
//     takes no per-call context (as with the rest of this backend).

// SearchRepos implements provider.SearchManager.
func (p *Provider) SearchRepos(ctx context.Context, opts provider.SearchReposOptions) ([]*provider.SearchRepoResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	repos, _, err := p.client.SearchRepos(gitea.SearchRepoOptions{
		Keyword:     opts.Query,
		Sort:        opts.Sort,
		Order:       opts.Order,
		ListOptions: gitea.ListOptions{Page: page, PageSize: perPage},
	})
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitea, "SearchRepos", err)
	}
	out := make([]*provider.SearchRepoResult, 0, len(repos))
	for _, r := range repos {
		out = append(out, &provider.SearchRepoResult{
			FullName:      r.FullName,
			Description:   r.Description,
			WebURL:        r.HTMLURL,
			Stars:         r.Stars,
			Forks:         r.Forks,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
		})
	}
	return out, len(out), nil
}

// SearchIssues implements provider.SearchManager.
func (p *Provider) SearchIssues(ctx context.Context, opts provider.SearchIssuesOptions) ([]*provider.SearchIssueResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	issues, _, err := p.client.ListIssues(gitea.ListIssueOption{
		KeyWord:     opts.Query,
		State:       gitea.StateType(opts.State),
		Type:        gitea.IssueTypeIssue,
		ListOptions: gitea.ListOptions{Page: page, PageSize: perPage},
	})
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitea, "SearchIssues", err)
	}
	out := make([]*provider.SearchIssueResult, 0, len(issues))
	for _, i := range issues {
		if opts.Repo != "" && i.Repository != nil && !strings.EqualFold(i.Repository.FullName, opts.Repo) {
			continue
		}
		labels := make([]string, 0, len(i.Labels))
		for _, l := range i.Labels {
			labels = append(labels, l.Name)
		}
		out = append(out, &provider.SearchIssueResult{
			Number:    strconv.FormatInt(i.Index, 10),
			Title:     i.Title,
			Body:      i.Body,
			State:     provider.IssueState(i.State),
			WebURL:    i.HTMLURL,
			Labels:    labels,
			Comments:  i.Comments,
			CreatedAt: i.Created,
		})
	}
	return out, len(out), nil
}

// SearchUsers implements provider.SearchManager.
func (p *Provider) SearchUsers(ctx context.Context, opts provider.SearchUsersOptions) ([]*provider.SearchUserResult, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	users, _, err := p.client.SearchUsers(gitea.SearchUsersOption{
		KeyWord:     opts.Query,
		ListOptions: gitea.ListOptions{Page: page, PageSize: perPage},
	})
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitea, "SearchUsers", err)
	}
	out := make([]*provider.SearchUserResult, 0, len(users))
	for _, u := range users {
		out = append(out, &provider.SearchUserResult{
			Login:     u.UserName,
			Name:      u.FullName,
			AvatarURL: u.AvatarURL,
			WebURL:    u.HTMLURL,
		})
	}
	return out, len(out), nil
}

var _ provider.SearchManager = (*Provider)(nil)
