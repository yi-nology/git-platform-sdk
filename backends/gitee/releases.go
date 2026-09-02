package gitee

import (
	"context"
	"io"

	gitee "github.com/next-bin/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListTags implements provider.ReleaseManager.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	tags, err := backendutil.AllPages(func(page int) ([]*gitee.Tag, error) {
		opts := &gitee.ListTagsOptions{
			Page:    gitee.Int(page),
			PerPage: gitee.Int(backendutil.IssueCommentPageSize),
		}
		batch, _, err := p.client.Repositories.ListTags(ctx, esc(owner), esc(repo), opts)
		return batch, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListTags", err)
	}
	result := make([]*provider.TagInfo, 0, len(tags))
	for _, t := range tags {
		result = append(result, convertTag(t))
	}
	return result, nil
}

// ListReleases implements provider.ReleaseManager.
func (p *Provider) ListReleases(ctx context.Context, owner, repo string) ([]*provider.ReleaseInfo, error) {
	releases, err := backendutil.AllPages(func(page int) ([]*gitee.Release, error) {
		opts := &gitee.ListReleasesOptions{
			Page:    gitee.Int(page),
			PerPage: gitee.Int(backendutil.IssueCommentPageSize),
		}
		batch, _, err := p.client.Repositories.ListReleases(ctx, esc(owner), esc(repo), opts)
		return batch, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListReleases", err)
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, convertRelease(r))
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	createOpts := &gitee.CreateReleaseOptions{
		TagName: gitee.String(opts.TagName),
		Name:    gitee.String(opts.Title),
	}
	if opts.Target != "" {
		createOpts.TargetCommitish = gitee.String(opts.Target)
	}
	if opts.Body != "" {
		createOpts.Body = gitee.String(opts.Body)
	}
	if opts.Prerelease {
		createOpts.Prerelease = gitee.Bool(true)
	}
	r, _, err := p.client.Repositories.CreateRelease(ctx, esc(owner), esc(repo), createOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateRelease", err)
	}
	return convertRelease(r), nil
}

// GetReleaseByTag implements provider.ReleaseManager.
func (p *Provider) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*provider.ReleaseInfo, error) {
	r, _, err := p.client.Repositories.GetReleaseByTag(ctx, esc(owner), esc(repo), esc(tag))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetReleaseByTag", err)
	}
	return convertRelease(r), nil
}

// UpdateRelease implements provider.ReleaseManager.
func (p *Provider) UpdateRelease(ctx context.Context, owner, repo, tag string, opts provider.UpdateReleaseOptions) (*provider.ReleaseInfo, error) {
	// Resolve the release ID by tag first.
	r, _, err := p.client.Repositories.GetReleaseByTag(ctx, esc(owner), esc(repo), esc(tag))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateRelease", err)
	}
	updateOpts := &gitee.UpdateReleaseOptions{}
	if opts.Name != nil {
		updateOpts.Name = gitee.String(*opts.Name)
	}
	if opts.Body != nil {
		updateOpts.Body = gitee.String(*opts.Body)
	}
	if opts.Prerelease != nil {
		updateOpts.Prerelease = gitee.Bool(*opts.Prerelease)
	}
	updated, _, err := p.client.Repositories.UpdateRelease(ctx, esc(owner), esc(repo), int64(deref(r.ID)), updateOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateRelease", err)
	}
	return convertRelease(updated), nil
}

// DeleteRelease implements provider.ReleaseManager.
func (p *Provider) DeleteRelease(ctx context.Context, owner, repo, tag string) error {
	// Resolve the release ID by tag first.
	r, _, err := p.client.Repositories.GetReleaseByTag(ctx, esc(owner), esc(repo), esc(tag))
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteRelease", err)
	}
	_, err = p.client.Repositories.DeleteRelease(ctx, esc(owner), esc(repo), int64(deref(r.ID)))
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteRelease", err)
	}
	return nil
}

// GetArchive implements provider.ReleaseManager.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	opts := &gitee.DownloadArchiveOptions{
		Ref: gitee.String(ref),
	}
	var resp *gitee.Response
	var err error
	if format == "tar.gz" {
		resp, err = p.client.Repositories.DownloadTarball(ctx, esc(owner), esc(repo), opts)
	} else {
		resp, err = p.client.Repositories.DownloadZipball(ctx, esc(owner), esc(repo), opts)
	}
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetArchive", err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf := make([]byte, 0, 1024*1024)
	tmp := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return nil, provider.Wrap(provider.PlatformGitee, "GetArchive", readErr)
		}
	}
	return buf, nil
}

var _ provider.ReleaseManager = (*Provider)(nil)
