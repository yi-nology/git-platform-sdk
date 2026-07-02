package gitlab

import (
	"context"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListTags implements provider.ReleaseManager.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	tags, _, err := p.client.Tags.ListTags(pidOf(owner, repo), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListTags", err)
	}
	result := make([]*provider.TagInfo, 0, len(tags))
	for _, t := range tags {
		ti := &provider.TagInfo{Name: t.Name}
		if t.Commit != nil {
			ti.Commit = t.Commit.ID
		}
		result = append(result, ti)
	}
	return result, nil
}

// ListReleases implements provider.ReleaseManager.
func (p *Provider) ListReleases(ctx context.Context, owner, repo string) ([]*provider.ReleaseInfo, error) {
	releases, _, err := p.client.Releases.ListReleases(pidOf(owner, repo), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListReleases", err)
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		ri := &provider.ReleaseInfo{
			TagName:   r.TagName,
			Title:     r.Name,
			Body:      r.Description,
			URL:       r.Links.Self,
			CreatedAt: timeOrZero(r.CreatedAt),
		}
		if r.ReleasedAt != nil {
			ri.PublishedAt = *r.ReleasedAt
		}
		result = append(result, ri)
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	r, _, err := p.client.Releases.CreateRelease(pidOf(owner, repo), &gitlab.CreateReleaseOptions{
		TagName:     gitlab.Ptr(opts.TagName),
		Ref:         gitlab.Ptr(opts.Target),
		Name:        gitlab.Ptr(opts.Title),
		Description: gitlab.Ptr(opts.Body),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateRelease", err)
	}
	ri := &provider.ReleaseInfo{
		TagName:   r.TagName,
		Title:     r.Name,
		Body:      r.Description,
		URL:       r.Links.Self,
		CreatedAt: timeOrZero(r.CreatedAt),
	}
	if r.ReleasedAt != nil {
		ri.PublishedAt = *r.ReleasedAt
	}
	return ri, nil
}

// GetArchive implements provider.ReleaseManager.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	fmtVal := "tar.gz"
	if format == "zip" {
		fmtVal = "zip"
	}
	data, _, err := p.client.Repositories.Archive(pidOf(owner, repo),
		&gitlab.ArchiveOptions{Format: gitlab.Ptr(fmtVal), SHA: gitlab.Ptr(ref)},
		gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "GetArchive", err)
	}
	return data, nil
}

var _ provider.ReleaseManager = (*Provider)(nil)