package gitcode

import (
	"context"
	"fmt"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListTags implements provider.ReleaseManager.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	tags, err := p.client.ListTags(ctx, owner, repo)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListTags", err)
	}
	result := make([]*provider.TagInfo, 0, len(tags))
	for _, t := range tags {
		// Tag.Commit is a value type in the SDK; no nil check needed.
		result = append(result, &provider.TagInfo{Name: t.Name, Commit: t.Commit.SHA})
	}
	return result, nil
}

// ListReleases implements provider.ReleaseManager.
func (p *Provider) ListReleases(ctx context.Context, owner, repo string) ([]*provider.ReleaseInfo, error) {
	releases, err := p.client.ListReleases(ctx, owner, repo)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListReleases", err)
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, convertRelease(r))
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	r, err := p.client.CreateRelease(ctx, owner, repo, gitcode.CreateReleaseOptions{
		TagName: opts.TagName, Target: opts.Target, Title: opts.Title,
		Body: opts.Body, Draft: opts.Draft, Prerelease: opts.Prerelease,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CreateRelease", err)
	}
	return convertRelease(r), nil
}

// GetArchive implements provider.ReleaseManager.
//
// GitCode's API does not expose a public archive endpoint.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	return nil, provider.Wrap(provider.PlatformGitCode, "GetArchive", fmt.Errorf("%w: GetArchive for GitCode", provider.ErrNotImplemented))
}

func convertRelease(r *gitcode.Release) *provider.ReleaseInfo {
	if r == nil {
		return nil
	}
	return &provider.ReleaseInfo{
		ID:          r.ID,
		TagName:     r.TagName,
		Title:       r.Name,
		Body:        r.Body,
		URL:         r.HTMLURL,
		Draft:       r.Draft,
		Prerelease:  r.Prerelease,
		CreatedAt:   r.CreatedAt,
		PublishedAt: r.PublishedAt,
	}
}

var _ provider.ReleaseManager = (*Provider)(nil)
