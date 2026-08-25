package gitee

import (
	"context"
	"fmt"
	"net/url"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// GetCommit implements provider.CommitManager.
//
// Routed through the raw transport client rather than the SDK: the SDK's
// RepoCommit model types the live payload's author/committer/stats objects as
// plain strings (upstream swagger generation bug), so
// GetV5ReposOwnerRepoCommitsSha fails to decode every real response. The
// response is still decoded with a wire-accurate local type (giteeCommitDetail
// in types.go).
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	var c giteeCommitDetail
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/commits/%s", esc(owner), esc(repo), esc(sha)), nil, &c); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetCommit", err)
	}
	return c.toCommitInfo(), nil
}

// ListCommits implements provider.CommitManager.
//
// Routed through the raw transport client rather than the SDK for the same
// reason as GetCommit (RepoCommit model bug). Branch/since/until map to the
// endpoint's sha/since/until query parameters, matching Gitee's REST contract.
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
//
// Routed through the raw transport client rather than the SDK: the SDK's
// Compare model types the live payload's commits/files arrays as plain
// strings (upstream swagger generation bug), so
// GetV5ReposOwnerRepoCompareBaseHead fails to decode every real response. The
// file objects decode into giteeCompareFile (types.go), which mirrors the
// compare endpoint's wire shape (numeric stats, string patch).
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	var cmp struct {
		Commits []giteeCommitDetail `json:"commits"`
		Files   []giteeCompareFile  `json:"files"`
	}
	if _, err := p.raw().DoJSON(ctx, &transport.Request{
		Method: "GET",
		Path:   fmt.Sprintf("/repos/%s/%s/compare/%s...%s", esc(owner), esc(repo), esc(base), esc(head)),
		Result: &cmp,
	}); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CompareCommits", err)
	}
	result := &provider.CompareResult{TotalCommits: len(cmp.Commits)}
	for i := range cmp.Commits {
		result.Commits = append(result.Commits, cmp.Commits[i].toCommitInfo())
	}
	for i := range cmp.Files {
		result.Files = append(result.Files, cmp.Files[i].toChangedFile())
	}
	return result, nil
}

var _ provider.CommitManager = (*Provider)(nil)
