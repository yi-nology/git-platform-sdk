package gitcode

import (
	"context"

	gitcode "github.com/yi-nology/go-gitcode"

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

// GetReleaseByTag implements provider.ReleaseManager. GitCode exposes a
// dedicated tag-addressed endpoint.
func (p *Provider) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*provider.ReleaseInfo, error) {
	r, err := p.client.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetReleaseByTag", err)
	}
	return convertRelease(r), nil
}

// UpdateRelease implements provider.ReleaseManager. The update endpoint is
// id-addressed, so the tag is resolved through the by-tag endpoint first
// (exact lookup, no list window). The SDK's UpdateReleaseOptions carries
// plain-string name/body fields that would clobber the release when sent
// empty, so the current values from the resolution fetch backfill every
// option the caller left nil; draft/prerelease are pointers that pass
// through untouched.
func (p *Provider) UpdateRelease(ctx context.Context, owner, repo, tag string, opts provider.UpdateReleaseOptions) (*provider.ReleaseInfo, error) {
	cur, err := p.client.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateRelease", err)
	}
	name, body := cur.Name, cur.Body
	if opts.Name != nil {
		name = *opts.Name
	}
	if opts.Body != nil {
		body = *opts.Body
	}
	r, err := p.client.UpdateRelease(ctx, owner, repo, cur.ID, gitcode.UpdateReleaseOptions{
		TagName:    cur.TagName,
		Name:       name,
		Body:       body,
		Draft:      opts.Draft,
		Prerelease: opts.Prerelease,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateRelease", err)
	}
	return convertRelease(r), nil
}

// DeleteRelease implements provider.ReleaseManager. The delete endpoint is
// id-addressed, so the tag is resolved through the by-tag endpoint first
// (exact lookup, no list window) and the release is deleted by ID. The
// SDK's tag-addressed DeleteRelease is deliberately NOT used: it falls
// back to deleting the tag itself on any release-delete error — transient
// 500/401/429 included — which would silently destroy the git tag; the
// release-object-only semantics every other platform honors rule that out.
func (p *Provider) DeleteRelease(ctx context.Context, owner, repo, tag string) error {
	cur, err := p.client.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteRelease", err)
	}
	if err := p.client.DeleteReleaseByID(ctx, owner, repo, cur.ID); err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteRelease", err)
	}
	return nil
}

// GetArchive implements provider.ReleaseManager.
//
// GitCode's archive endpoint carries the ref and extension as a single
// `ref.ext` path segment (GET /repos/{o}/{r}/archive/{archive}), so the
// provider's (ref, format) pair maps: format "tar.gz" → ref+".tar.gz";
// format "zip", empty, or anything else → ref+".zip" (default-zip
// semantics, mirroring the gitee backend's format mapping, adapted to
// GitCode's extension-in-path scheme — gitee's zipball/tarball keyword
// URLs do not apply here). Returns the raw archive bytes.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	ext := "zip"
	if format == "tar.gz" {
		ext = "tar.gz"
	}
	data, err := p.client.GetRepositoryArchive(ctx, owner, repo, ref+"."+ext)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetArchive", err)
	}
	return data, nil
}

var _ provider.ReleaseManager = (*Provider)(nil)
