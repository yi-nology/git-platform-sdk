package provider

import "context"

// RepoManager handles repository CRUD operations.
type RepoManager interface {
	ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error)
	GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error)
	CreateRepo(ctx context.Context, owner string, opts CreateRepoOptions) (*PlatformRepo, error)
	DeleteRepo(ctx context.Context, owner, repo string) error
	UpdateRepo(ctx context.Context, owner, repo string, opts UpdateRepoOptions) (*PlatformRepo, error)
	ForkRepo(ctx context.Context, owner, repo string, opts ForkRepoOptions) (*PlatformRepo, error)
}
