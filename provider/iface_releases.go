package provider

import "context"

// ReleaseManager handles tags, releases, and archives.
type ReleaseManager interface {
	ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error)
	ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error)
	CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error)
	GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error)
}
