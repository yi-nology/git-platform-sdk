package gitea

import (
	"context"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListTags implements provider.ReleaseManager.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	tags, _, err := p.client.ListRepoTags(owner, repo, gitea.ListRepoTagsOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListTags", err)
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
	releases, _, err := p.client.ListReleases(owner, repo, gitea.ListReleasesOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListReleases", err)
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, convertRelease(r))
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	r, _, err := p.client.CreateRelease(owner, repo, gitea.CreateReleaseOption{
		TagName:      opts.TagName,
		Target:       opts.Target,
		Title:        opts.Title,
		Note:         opts.Body,
		IsDraft:      opts.Draft,
		IsPrerelease: opts.Prerelease,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateRelease", err)
	}
	return convertRelease(r), nil
}

// GetReleaseByTag implements provider.ReleaseManager. Gitea exposes a
// dedicated tag-addressed endpoint.
func (p *Provider) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*provider.ReleaseInfo, error) {
	r, _, err := p.client.GetReleaseByTag(owner, repo, tag)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetReleaseByTag", err)
	}
	return convertRelease(r), nil
}

// UpdateRelease implements provider.ReleaseManager. The edit endpoint is
// id-addressed, so the tag is resolved through the by-tag endpoint first
// (exact lookup, no list window). EditReleaseOption's name/note fields are
// plain strings that would clobber the release when sent empty, so the
// current values from the resolution fetch backfill every option the caller
// left nil; draft/prerelease are pointers that pass through untouched.
func (p *Provider) UpdateRelease(ctx context.Context, owner, repo, tag string, opts provider.UpdateReleaseOptions) (*provider.ReleaseInfo, error) {
	cur, _, err := p.client.GetReleaseByTag(owner, repo, tag)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "UpdateRelease", err)
	}
	title, note := cur.Title, cur.Note
	if opts.Name != nil {
		title = *opts.Name
	}
	if opts.Body != nil {
		note = *opts.Body
	}
	r, _, err := p.client.EditRelease(owner, repo, cur.ID, gitea.EditReleaseOption{
		TagName:      tag,
		Title:        title,
		Note:         note,
		IsDraft:      opts.Draft,
		IsPrerelease: opts.Prerelease,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "UpdateRelease", err)
	}
	return convertRelease(r), nil
}

// DeleteRelease implements provider.ReleaseManager via Gitea's
// tag-addressed delete (server >= 1.14; the SDK version-gates itself).
// The release's tag is kept.
func (p *Provider) DeleteRelease(ctx context.Context, owner, repo, tag string) error {
	if _, err := p.client.DeleteReleaseByTag(owner, repo, tag); err != nil {
		return provider.Wrap(provider.PlatformGitea, "DeleteRelease", err)
	}
	return nil
}

// GetArchive implements provider.ReleaseManager.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	if format == "zip" {
		data, _, err = p.client.GetArchive(owner, repo, ref, gitea.ZipArchive)
	} else {
		data, _, err = p.client.GetArchive(owner, repo, ref, gitea.TarGZArchive)
	}
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetArchive", err)
	}
	return data, nil
}

var _ provider.ReleaseManager = (*Provider)(nil)
