package provider

import "context"

// CommitStatusManager reports CI statuses on commits. It is an optional
// capability interface: consumers should gate on
// Provider.Capabilities().CommitStatuses (or type-assert) before use.
//
// It is deliberately separate from CommitManager: commit statuses are a CI
// reporting concern that not every platform exposes (Gitee's public REST
// API has no commit-status endpoint), so absence is expressed by not
// declaring the capability instead of stubbing the method.
type CommitStatusManager interface {
	CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error
}
