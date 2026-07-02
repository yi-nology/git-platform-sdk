package forgejo

import (
	"context"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListTags implements provider.ReleaseManager.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	tags, _, err := p.client.ListRepoTags(owner, repo, forgejo.ListRepoTagsOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListTags", err)
	}
	result := make([]*provider.TagInfo, 0, len(tags))
	for _, t := range tags {
		ti := &provider.TagInfo{Name: t.Name}
		if t.Commit != nil {
			ti.Commit = t.Commit.SHA
		}
		result = append(result, ti)
	}
	return result, nil
}

// ListReleases implements provider.ReleaseManager.
func (p *Provider) ListReleases(ctx context.Context, owner, repo string) ([]*provider.ReleaseInfo, error) {
	releases, _, err := p.client.ListReleases(owner, repo, forgejo.ListReleasesOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListReleases", err)
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, convertRelease(r))
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	r, _, err := p.client.CreateRelease(owner, repo, forgejo.CreateReleaseOption{
		TagName:      opts.TagName,
		Target:       opts.Target,
		Title:        opts.Title,
		Note:         opts.Body,
		IsDraft:      opts.Draft,
		IsPrerelease: opts.Prerelease,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "CreateRelease", err)
	}
	return convertRelease(r), nil
}

// GetArchive implements provider.ReleaseManager.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	if format == "zip" {
		data, _, err = p.client.GetArchive(owner, repo, ref, forgejo.ZipArchive)
	} else {
		data, _, err = p.client.GetArchive(owner, repo, ref, forgejo.TarGZArchive)
	}
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "GetArchive", err)
	}
	return data, nil
}

var _ provider.ReleaseManager = (*Provider)(nil)
