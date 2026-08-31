package provider

import "context"

// BranchProtectionManager provides branch protection rule CRUD. It is an
// optional capability interface: consumers should gate on
// Provider.Capabilities().BranchProtections (or type-assert) before use.
//
// Platform support: GitHub, GitCode, GitLab, Gitea, Forgejo, Gitee.
// TencentCode exposes branch protection only via its platform-specific
// TencentCodeExtras interface.
type BranchProtectionManager interface {
	// ListBranchProtections returns all branch protection rules for a repo.
	ListBranchProtections(ctx context.Context, owner, repo string) ([]*BranchProtection, error)
	// GetBranchProtection returns the protection rule for a specific branch.
	GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error)
	// CreateBranchProtection creates a new branch protection rule.
	CreateBranchProtection(ctx context.Context, owner, repo string, opts CreateBranchProtectionOptions) (*BranchProtection, error)
	// UpdateBranchProtection updates an existing branch protection rule.
	UpdateBranchProtection(ctx context.Context, owner, repo, branch string, opts UpdateBranchProtectionOptions) (*BranchProtection, error)
	// DeleteBranchProtection removes the protection rule for a branch.
	DeleteBranchProtection(ctx context.Context, owner, repo, branch string) error
}
