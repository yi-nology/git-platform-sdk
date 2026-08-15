package tencentcode

import (
	"bytes"
	"context"

	gongfeng "github.com/studyzy/gongfeng-sdk-go"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListTags implements provider.ReleaseManager.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	pid := owner + "/" + repo
	tags, _, err := p.client.Tags.ListTags(ctx, pid, nil)
	if err != nil {
		return nil, sdkError("ListTags", err)
	}
	result := make([]*provider.TagInfo, 0, len(tags))
	for _, t := range tags {
		result = append(result, convertTag(t))
	}
	return result, nil
}

// ListReleases implements provider.ReleaseManager.
func (p *Provider) ListReleases(ctx context.Context, owner, repo string) ([]*provider.ReleaseInfo, error) {
	pid := owner + "/" + repo
	releases, _, err := p.client.Releases.ListReleases(ctx, pid, nil)
	if err != nil {
		return nil, sdkError("ListReleases", err)
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, convertRelease(r))
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	pid := owner + "/" + repo
	createOpts := &gongfeng.CreateReleaseOptions{
		TagName:     gongfeng.Ptr(opts.TagName),
		Description: gongfeng.Ptr(opts.Body),
	}
	release, _, err := p.client.Releases.CreateRelease(ctx, pid, createOpts)
	if err != nil {
		return nil, sdkError("CreateRelease", err)
	}
	return convertRelease(release), nil
}

// GetReleaseByTag implements provider.ReleaseManager. Gongfeng's release
// endpoints are tag-addressed natively.
func (p *Provider) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*provider.ReleaseInfo, error) {
	pid := owner + "/" + repo
	release, _, err := p.client.Releases.GetRelease(ctx, pid, tag)
	if err != nil {
		return nil, sdkError("GetReleaseByTag", err)
	}
	return convertRelease(release), nil
}

// UpdateRelease implements provider.ReleaseManager via Gongfeng's
// tag-addressed update (PUT). Registration: the platform's update surface
// accepts only a description, so UpdateReleaseOptions.Name, .Draft, and
// .Prerelease are ignored on this platform — only Body is carried. An
// update carrying nothing (Body nil, the only field forwarded) short-
// circuits to the GetReleaseByTag result instead of PUTting an empty
// update body.
func (p *Provider) UpdateRelease(ctx context.Context, owner, repo, tag string, opts provider.UpdateReleaseOptions) (*provider.ReleaseInfo, error) {
	if opts.Body == nil {
		return p.GetReleaseByTag(ctx, owner, repo, tag)
	}
	pid := owner + "/" + repo
	updateOpts := &gongfeng.UpdateReleaseOptions{
		Description: gongfeng.Ptr(*opts.Body),
	}
	release, _, err := p.client.Releases.UpdateRelease(ctx, pid, tag, updateOpts)
	if err != nil {
		return nil, sdkError("UpdateRelease", err)
	}
	return convertRelease(release), nil
}

// DeleteRelease implements provider.ReleaseManager via Gongfeng's
// tag-addressed delete.
func (p *Provider) DeleteRelease(ctx context.Context, owner, repo, tag string) error {
	pid := owner + "/" + repo
	_, err := p.client.Releases.DeleteRelease(ctx, pid, tag)
	if err != nil {
		return sdkError("DeleteRelease", err)
	}
	return nil
}

// GetArchive implements provider.ReleaseManager.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	pid := owner + "/" + repo
	var buf bytes.Buffer
	opts := &gongfeng.ArchiveOptions{}
	if ref != "" {
		opts.SHA = gongfeng.Ptr(ref)
	}
	_, err := p.client.Repositories.Archive(ctx, pid, &buf, opts)
	if err != nil {
		return nil, sdkError("GetArchive", err)
	}
	return buf.Bytes(), nil
}

var _ provider.ReleaseManager = (*Provider)(nil)
