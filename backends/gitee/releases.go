package gitee

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// ListTags implements provider.ReleaseManager.
//
// Routed through the raw transport client rather than the SDK: the SDK's
// GetV5ReposOwnerRepoTags returns a single Tag whose Commit is a plain
// string, while the live wire is an array of objects with a nested
// commit{sha} (verified against the live v5 tags payload) — the generated
// call cannot decode the response at all.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	page, perPage := provider.NormalizePageOpts(1, 0)
	var tags []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/tags?page=%d&per_page=%d", esc(owner), esc(repo), page, perPage), nil, &tags); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListTags", err)
	}
	result := make([]*provider.TagInfo, 0, len(tags))
	for _, t := range tags {
		result = append(result, &provider.TagInfo{Name: t.Name, Commit: t.Commit.SHA})
	}
	return result, nil
}

// ListReleases implements provider.ReleaseManager.
//
// Routed through the raw transport client rather than the SDK: the SDK's
// Release model types the live payload's boolean "prerelease" and object
// "author" as plain strings (upstream swagger generation bug — verified
// against the live v5 releases payload), so GetV5ReposOwnerRepoReleases
// fails to decode every real response. The model also lacks the
// draft/published_at/html_url fields ReleaseInfo surfaces.
func (p *Provider) ListReleases(ctx context.Context, owner, repo string) ([]*provider.ReleaseInfo, error) {
	page, perPage := provider.NormalizePageOpts(1, 0)
	var releases []giteeRelease
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/releases?page=%d&per_page=%d", esc(owner), esc(repo), page, perPage), nil, &releases); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListReleases", err)
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for i := range releases {
		result = append(result, releases[i].toReleaseInfo())
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
//
// Routed through the raw transport client rather than the SDK: the generated
// PostV5ReposOwnerRepoReleases posts its parameters as a multipart body
// labeled application/json (upstream client.go bug) and offers no draft
// parameter, and its response decodes into the mis-typed Release model (see
// ListReleases).
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	body := map[string]any{
		"tag_name": opts.TagName,
		"name":     opts.Title,
	}
	if opts.Target != "" {
		body["target_commitish"] = opts.Target
	}
	if opts.Body != "" {
		body["body"] = opts.Body
	}
	if opts.Draft {
		body["draft"] = true
	}
	if opts.Prerelease {
		body["prerelease"] = true
	}
	var r giteeRelease
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/releases", esc(owner), esc(repo)), body, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateRelease", err)
	}
	return r.toReleaseInfo(), nil
}

// GetReleaseByTag implements provider.ReleaseManager.
//
// Routed through the raw transport client rather than the SDK: the
// generated GetV5ReposOwnerRepoReleasesTagsTag decodes into the mis-typed
// Release model (see ListReleases), and the tag-addressed endpoint is
// outside the SDK's release surface anyway.
func (p *Provider) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*provider.ReleaseInfo, error) {
	var r giteeRelease
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/releases/tags/%s", esc(owner), esc(repo), esc(tag)), nil, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetReleaseByTag", err)
	}
	return r.toReleaseInfo(), nil
}

// UpdateRelease implements provider.ReleaseManager.
//
// Routed through the raw transport client rather than the SDK: the
// generated PatchV5ReposOwnerRepoReleasesId posts its parameters as a
// multipart body labeled application/json (the upstream client.go bug
// behind CreateRelease's detour) and decodes into the mis-typed Release
// model. The update endpoint is id-addressed, so the tag is resolved
// through the by-tag endpoint first (exact lookup, no list window); the
// PATCH body carries only the fields the caller set, so nil options leave
// the release untouched.
func (p *Provider) UpdateRelease(ctx context.Context, owner, repo, tag string, opts provider.UpdateReleaseOptions) (*provider.ReleaseInfo, error) {
	id, err := p.resolveReleaseID(ctx, "UpdateRelease", owner, repo, tag)
	if err != nil {
		return nil, err
	}
	body := map[string]any{}
	if opts.Name != nil {
		body["name"] = *opts.Name
	}
	if opts.Body != nil {
		body["body"] = *opts.Body
	}
	if opts.Draft != nil {
		body["draft"] = *opts.Draft
	}
	if opts.Prerelease != nil {
		body["prerelease"] = *opts.Prerelease
	}
	var r giteeRelease
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/releases/%d", esc(owner), esc(repo), id), body, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateRelease", err)
	}
	return r.toReleaseInfo(), nil
}

// DeleteRelease implements provider.ReleaseManager.
//
// Routed through the raw transport client rather than the SDK, for the same
// multipart/mis-typed-model reasons as UpdateRelease. The delete endpoint
// is id-addressed, so the tag is resolved through the by-tag endpoint
// first. The release's tag is kept.
func (p *Provider) DeleteRelease(ctx context.Context, owner, repo, tag string) error {
	id, err := p.resolveReleaseID(ctx, "DeleteRelease", owner, repo, tag)
	if err != nil {
		return err
	}
	if err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/releases/%d", esc(owner), esc(repo), id), nil, nil); err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteRelease", err)
	}
	return nil
}

// resolveReleaseID finds the numeric ID of the release addressed by tag via
// the exact by-tag endpoint (no pagination window to bound, unlike the
// list-scanning label resolution). op is the public operation the
// resolution serves; failures surface under that op.
func (p *Provider) resolveReleaseID(ctx context.Context, op, owner, repo, tag string) (int64, error) {
	var r giteeRelease
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/releases/tags/%s", esc(owner), esc(repo), esc(tag)), nil, &r); err != nil {
		return 0, provider.Wrap(provider.PlatformGitee, op, err)
	}
	return r.ID, nil
}

// GetArchive implements provider.ReleaseManager.
//
// Routed through the raw transport client (registered detour): the
// go-gitee SDK exposes no archive-download endpoint, so the GitHub-shaped
// zipball/tarball URLs are fetched directly.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	archiveFormat := "zipball"
	if format == "tar.gz" {
		archiveFormat = "tarball"
	}
	// esc encodes '/' as %2F, so slash-bearing refs (e.g. "refs/tags/v1.0")
	// travel as a single encoded segment. This is deliberate: ref is one path
	// segment on the wire, unlike file paths (escPath) which preserve '/'.
	resp, err := p.raw().Do(ctx, &transport.Request{
		Method: "GET",
		Path:   fmt.Sprintf("/repos/%s/%s/%s/%s", esc(owner), esc(repo), archiveFormat, esc(ref)),
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetArchive", err)
	}
	return resp.Body, nil
}

var _ provider.ReleaseManager = (*Provider)(nil)
