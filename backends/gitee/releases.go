package gitee

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// ListTags implements provider.ReleaseManager.
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

// GetArchive implements provider.ReleaseManager.
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
