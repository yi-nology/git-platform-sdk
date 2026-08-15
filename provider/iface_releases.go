package provider

import "context"

// ReleaseManager handles tags, releases, and archives. Releases are
// addressed by tag name across every method: tag names are stable and
// human-addressable, while the underlying numeric release IDs are not.
type ReleaseManager interface {
	ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error)
	ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error)
	CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error)
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*ReleaseInfo, error)
	UpdateRelease(ctx context.Context, owner, repo, tag string, opts UpdateReleaseOptions) (*ReleaseInfo, error)
	DeleteRelease(ctx context.Context, owner, repo, tag string) error
	GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error)
}
