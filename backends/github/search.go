package github

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/provider"
)

func (p *Provider) SearchRepos(_ context.Context, _ provider.SearchReposOptions) ([]*provider.SearchRepoResult, int, error) {
	return nil, 0, provider.Wrap(provider.PlatformGitHub, "SearchRepos", provider.ErrNotImplemented)
}
func (p *Provider) SearchIssues(_ context.Context, _ provider.SearchIssuesOptions) ([]*provider.SearchIssueResult, int, error) {
	return nil, 0, provider.Wrap(provider.PlatformGitHub, "SearchIssues", provider.ErrNotImplemented)
}
func (p *Provider) SearchUsers(_ context.Context, _ provider.SearchUsersOptions) ([]*provider.SearchUserResult, int, error) {
	return nil, 0, provider.Wrap(provider.PlatformGitHub, "SearchUsers", provider.ErrNotImplemented)
}

var _ provider.SearchManager = (*Provider)(nil)
