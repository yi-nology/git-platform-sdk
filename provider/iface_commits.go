package provider

import "context"

// CommitManager handles commit operations.
type CommitManager interface {
	GetCommit(ctx context.Context, owner, repo, sha string) (*CommitInfo, error)
	ListCommits(ctx context.Context, owner, repo string, opts ListCommitsOptions) ([]*CommitInfo, error)
	CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error)
	CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error
}
