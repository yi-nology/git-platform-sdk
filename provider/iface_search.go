package provider

import "context"

// SearchManager provides cross-platform search for repositories, issues, and users.
//
// The *int return is the server-side total when the platform reports one,
// and nil when it does not — callers must not treat a total as guaranteed.
type SearchManager interface {
	SearchRepos(ctx context.Context, opts SearchReposOptions) ([]*SearchRepoResult, *int, error)
	SearchIssues(ctx context.Context, opts SearchIssuesOptions) ([]*SearchIssueResult, *int, error)
	SearchUsers(ctx context.Context, opts SearchUsersOptions) ([]*SearchUserResult, *int, error)
}
