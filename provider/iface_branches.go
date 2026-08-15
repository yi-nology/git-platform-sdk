package provider

import "context"

// BranchManager handles branch operations.
type BranchManager interface {
	ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error)
	CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error)
	DeleteBranch(ctx context.Context, owner, repo, branch string) error
}
