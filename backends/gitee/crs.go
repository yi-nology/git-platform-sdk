package gitee

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	body := map[string]any{
		"source_branch": opts.SourceBranch,
		"target_branch": opts.TargetBranch,
		"title":         opts.Title,
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if len(opts.Labels) > 0 {
		body["labels"] = strings.Join(opts.Labels, ",")
	}
	var pr giteePR
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls", esc(opts.Owner), esc(opts.Repo)), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateCR", err)
	}
	return pr.toChangeRequest(), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	var pr giteePR
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", esc(owner), esc(repo), number), nil, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetCR", err)
	}
	return pr.toChangeRequest(), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	state := string(opts.State)
	if state == "" {
		state = "open"
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls?page=%d&per_page=%d&state=%s", esc(opts.Owner), esc(opts.Repo), page, perPage, state)
	if opts.SourceBranch != "" {
		path += "&source_branch=" + url.QueryEscape(opts.SourceBranch)
	}
	if opts.TargetBranch != "" {
		path += "&target_branch=" + url.QueryEscape(opts.TargetBranch)
	}
	var prs []giteePR
	headers, err := p.doRequestWithHeaders(ctx, "GET", path, nil, &prs)
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitee, "ListCRs", err)
	}
	result := make([]*provider.ChangeRequest, 0, len(prs))
	for i := range prs {
		result = append(result, prs[i].toChangeRequest())
	}
	return result, provider.ParseTotalCountHeader(headers, len(result)), nil
}

// MergeCR implements provider.ChangeRequestManager.
func (p *Provider) MergeCR(ctx context.Context, owner, repo string, number int, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	body := map[string]any{}
	if opts.MergeCommitMessage != "" {
		body["merge_message"] = opts.MergeCommitMessage
	}
	if opts.Squash {
		body["squash"] = true
	}
	var pr giteePR
	if err := p.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", esc(owner), esc(repo), number), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "MergeCR", err)
	}
	return pr.toChangeRequest(), nil
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	body := map[string]any{"state": "closed"}
	var pr giteePR
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", esc(owner), esc(repo), number), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CloseCR", err)
	}
	return pr.toChangeRequest(), nil
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	body := map[string]any{"state": "open"}
	var pr giteePR
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", esc(owner), esc(repo), number), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ReopenCR", err)
	}
	return pr.toChangeRequest(), nil
}

// UpdateCR implements provider.ChangeRequestManager.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo string, number int, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	body := map[string]any{}
	if opts.Title != "" {
		body["title"] = opts.Title
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.TargetBranch != "" {
		body["target_branch"] = opts.TargetBranch
	}
	var pr giteePR
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", esc(owner), esc(repo), number), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateCR", err)
	}
	return pr.toChangeRequest(), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	body := map[string]any{"labels": strings.Join(labels, ",")}
	err := p.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/pulls/%d", esc(owner), esc(repo), number), body, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "UpdateCRLabels", err)
	}
	return nil
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*provider.CRComment, error) {
	page, perPage := provider.NormalizePageOpts(1, 0)
	var comments []giteeComment
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments?page=%d&per_page=%d", esc(owner), esc(repo), number, page, perPage), nil, &comments); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListCRComments", err)
	}
	result := make([]*provider.CRComment, 0, len(comments))
	for _, c := range comments {
		result = append(result, c.toCRComment())
	}
	return result, nil
}

// ListCRCommits implements provider.ChangeRequestManager.
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*provider.CRCommit, error) {
	page, perPage := provider.NormalizePageOpts(1, 0)
	var commits []giteeCommit
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/commits?page=%d&per_page=%d", esc(owner), esc(repo), number, page, perPage), nil, &commits); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListCRCommits", err)
	}
	result := make([]*provider.CRCommit, 0, len(commits))
	for _, c := range commits {
		result = append(result, c.toCRCommit())
	}
	return result, nil
}

var _ provider.ChangeRequestManager = (*Provider)(nil)
