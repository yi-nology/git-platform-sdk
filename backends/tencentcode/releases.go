package tencentcode

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListTags implements provider.ReleaseManager.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	var tags []struct {
		Name   string `json:"name"`
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/tags", encoded), nil, &tags); err != nil {
		return nil, err
	}
	result := make([]*provider.TagInfo, 0, len(tags))
	for _, tg := range tags {
		result = append(result, &provider.TagInfo{Name: tg.Name, Commit: tg.Commit.ID})
	}
	return result, nil
}

// ListReleases implements provider.ReleaseManager.
func (p *Provider) ListReleases(ctx context.Context, owner, repo string) ([]*provider.ReleaseInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	var releases []struct {
		ID          int    `json:"id"`
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/releases", encoded), nil, &releases); err != nil {
		return nil, err
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, &provider.ReleaseInfo{
			ID: int64(r.ID), TagName: r.TagName, Title: r.Name, Body: r.Description,
		})
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{
		"tag_name":    opts.TagName,
		"name":        opts.Title,
		"description": opts.Body,
	}
	if opts.Target != "" {
		body["target_commitish"] = opts.Target
	}
	var r struct {
		ID          int    `json:"id"`
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/releases", encoded), body, &r); err != nil {
		return nil, err
	}
	return &provider.ReleaseInfo{
		ID: int64(r.ID), TagName: r.TagName, Title: r.Name, Body: r.Description,
	}, nil
}

// GetArchive implements provider.ReleaseManager.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	encoded := encodeProjectPath(owner, repo)
	suffix := "tar.gz"
	if format == "zip" {
		suffix = "zip"
	}
	return p.doRawRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/archive.%s?sha=%s", encoded, suffix, ref))
}

var _ provider.ReleaseManager = (*Provider)(nil)