package gitee

import (
	"context"
	"fmt"
	"net/url"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	var c giteeCommitDetail
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/commits/%s", esc(owner), esc(repo), esc(sha)), nil, &c); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetCommit", err)
	}
	return c.toCommitInfo(), nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	path := fmt.Sprintf("/repos/%s/%s/commits?page=%d&per_page=%d", esc(owner), esc(repo), page, perPage)
	if opts.Branch != "" {
		path += "&sha=" + url.QueryEscape(opts.Branch)
	}
	if opts.Since != "" {
		path += "&since=" + url.QueryEscape(opts.Since)
	}
	if opts.Until != "" {
		path += "&until=" + url.QueryEscape(opts.Until)
	}
	var commits []giteeCommitDetail
	if err := p.doRequest(ctx, "GET", path, nil, &commits); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListCommits", err)
	}
	result := make([]*provider.CommitInfo, 0, len(commits))
	for i := range commits {
		result = append(result, commits[i].toCommitInfo())
	}
	return result, nil
}

// CompareCommits implements provider.CommitManager.
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	var cmp struct {
		Commits []giteeCommitDetail `json:"commits"`
		Files   []giteePRFile       `json:"files"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/compare/%s...%s", esc(owner), esc(repo), esc(base), esc(head)), nil, &cmp); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CompareCommits", err)
	}
	result := &provider.CompareResult{TotalCommits: len(cmp.Commits)}
	for i := range cmp.Commits {
		result.Commits = append(result.Commits, cmp.Commits[i].toCommitInfo())
	}
	for _, f := range cmp.Files {
		result.Files = append(result.Files, f.toChangedFile())
	}
	return result, nil
}

// CreateCommitStatus implements provider.CommitManager.
//
// Gitee does not expose a commit-status endpoint in the public REST API.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	return provider.Wrap(provider.PlatformGitee, "CreateCommitStatus", provider.ErrNotImplemented)
}

var _ provider.CommitManager = (*Provider)(nil)
