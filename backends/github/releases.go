package github

import (
	"context"
	"io"

	"github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListTags implements provider.ReleaseManager.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	tags, _, err := p.client.Repositories.ListTags(ctx, owner, repo, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListTags", err)
	}
	result := make([]*provider.TagInfo, 0, len(tags))
	for _, t := range tags {
		result = append(result, &provider.TagInfo{Name: t.GetName(), Commit: t.GetCommit().GetSHA()})
	}
	return result, nil
}

// ListReleases implements provider.ReleaseManager.
func (p *Provider) ListReleases(ctx context.Context, owner, repo string) ([]*provider.ReleaseInfo, error) {
	releases, _, err := p.client.Repositories.ListReleases(ctx, owner, repo, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListReleases", err)
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, convertRelease(r))
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	r, _, err := p.client.Repositories.CreateRelease(ctx, owner, repo, &github.RepositoryRelease{
		TagName:         github.String(opts.TagName),
		TargetCommitish: github.String(opts.Target),
		Name:            github.String(opts.Title),
		Body:            github.String(opts.Body),
		Draft:           github.Bool(opts.Draft),
		Prerelease:      github.Bool(opts.Prerelease),
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateRelease", err)
	}
	return convertRelease(r), nil
}

// GetArchive implements provider.ReleaseManager.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	url, _, err := p.client.Repositories.GetArchiveLink(ctx, owner, repo, github.ArchiveFormat(format), &github.RepositoryContentGetOptions{Ref: ref}, 1)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "GetArchive", err)
	}
	resp, err := p.client.Client().Get(url.String())
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "GetArchive", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "GetArchive", err)
	}
	return data, nil
}

// compile-time guard
var _ provider.ReleaseManager = (*Provider)(nil)