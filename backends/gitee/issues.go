package gitee

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	gitee "gitee.com/openeuler/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// This file implements the full provider.IssueManager surface over Gitee's
// native REST endpoints, but the capability is deliberately NOT declared in
// Capabilities() (see gitee.go): its verification against live Gitee and the
// contract fixtures is pending the SDK migration. The string-typed issue
// numbers now match Gitee's alphanumeric identifiers (e.g. "IAINVA")
// natively. The implementation stays compile-guarded and spike-ready.

// giteeUser mirrors Gitee's user JSON shape.
type giteeUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// giteeMilestone mirrors Gitee's milestone JSON.
type giteeMilestone struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// giteeIssue mirrors Gitee's v5 issue JSON shape. Number is Gitee's
// alphanumeric issue identifier as a string.
type giteeIssue struct {
	Number    string          `json:"number"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	State     string          `json:"state"`
	User      *giteeUser      `json:"user"`
	Labels    []gitee.Label   `json:"labels"`
	Assignees []giteeUser     `json:"assignees"`
	Milestone *giteeMilestone `json:"milestone"`
	HTMLURL   string          `json:"html_url"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// giteeIssueComment mirrors Gitee's issue-comment JSON shape.
type giteeIssueComment struct {
	ID        int64      `json:"id"`
	Body      string     `json:"body"`
	User      *giteeUser `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ListIssues implements provider.IssueManager.
func (p *Provider) ListIssues(ctx context.Context, opts provider.ListIssuesOptions) ([]*provider.Issue, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	path := fmt.Sprintf("/repos/%s/%s/issues?page=%d&per_page=%d", esc(opts.Owner), esc(opts.Repo), page, perPage)
	if opts.State != "" {
		path += "&state=" + url.QueryEscape(string(opts.State))
	}
	if opts.Assignee != "" {
		path += "&assignee=" + url.QueryEscape(opts.Assignee)
	}
	if opts.Labels != "" {
		path += "&labels=" + url.QueryEscape(opts.Labels)
	}
	var issues []giteeIssue
	if err := p.doRequest(ctx, "GET", path, nil, &issues); err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitee, "ListIssues", err)
	}
	result := make([]*provider.Issue, 0, len(issues))
	for i := range issues {
		result = append(result, convertIssue(issues[i]))
	}
	return result, len(result), nil
}

// GetIssue implements provider.IssueManager. Gitee addresses issues by
// their alphanumeric string number directly.
func (p *Provider) GetIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	var issue giteeIssue
	path := fmt.Sprintf("/repos/%s/%s/issues/%s", esc(owner), esc(repo), esc(number))
	if err := p.doRequest(ctx, "GET", path, nil, &issue); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetIssue", err)
	}
	return convertIssue(issue), nil
}

// CreateIssue implements provider.IssueManager.
func (p *Provider) CreateIssue(ctx context.Context, opts provider.CreateIssueOptions) (*provider.Issue, error) {
	body := map[string]any{"title": opts.Title}
	if opts.Body != "" {
		body["body"] = opts.Body
	}
	if len(opts.Labels) > 0 {
		body["labels"] = strings.Join(opts.Labels, ",")
	}
	if len(opts.Assignees) > 0 {
		body["assignees"] = strings.Join(opts.Assignees, ",")
	}
	if opts.Milestone != "" {
		m, err := strconv.Atoi(opts.Milestone)
		if err != nil {
			return nil, provider.Wrapf(provider.PlatformGitee, "CreateIssue", "invalid milestone number %q", opts.Milestone)
		}
		body["milestone"] = m
	}
	var issue giteeIssue
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/issues", esc(opts.Owner), esc(opts.Repo)), body, &issue); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateIssue", err)
	}
	return convertIssue(issue), nil
}

// UpdateIssue implements provider.IssueManager.
func (p *Provider) UpdateIssue(ctx context.Context, owner, repo, number string, opts provider.UpdateIssueOptions) (*provider.Issue, error) {
	body := map[string]any{}
	if opts.Title != "" {
		body["title"] = opts.Title
	}
	if opts.Body != "" {
		body["body"] = opts.Body
	}
	if opts.State != "" {
		body["state"] = string(opts.State)
	}
	if len(opts.Assignees) > 0 {
		body["assignees"] = strings.Join(opts.Assignees, ",")
	}
	if len(opts.Labels) > 0 {
		body["labels"] = strings.Join(opts.Labels, ",")
	}
	if opts.Milestone != "" {
		m, err := strconv.Atoi(opts.Milestone)
		if err != nil {
			return nil, provider.Wrapf(provider.PlatformGitee, "UpdateIssue", "invalid milestone number %q", opts.Milestone)
		}
		body["milestone"] = m
	}
	var issue giteeIssue
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/issues/%s", esc(owner), esc(repo), esc(number)), body, &issue); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateIssue", err)
	}
	return convertIssue(issue), nil
}

// CloseIssue implements provider.IssueManager.
func (p *Provider) CloseIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	return p.patchIssueState(ctx, owner, repo, number, "closed", "CloseIssue")
}

// ReopenIssue implements provider.IssueManager.
func (p *Provider) ReopenIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	return p.patchIssueState(ctx, owner, repo, number, "open", "ReopenIssue")
}

// patchIssueState flips an issue's state via PATCH, forwarding the caller's
// context. op is the public operation this patch serves; failures surface
// under that op.
func (p *Provider) patchIssueState(ctx context.Context, owner, repo, number, state, op string) (*provider.Issue, error) {
	var issue giteeIssue
	path := fmt.Sprintf("/repos/%s/%s/issues/%s", esc(owner), esc(repo), esc(number))
	if err := p.doRequest(ctx, "PATCH", path, map[string]any{"state": state}, &issue); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, op, err)
	}
	return convertIssue(issue), nil
}

// ListIssueComments implements provider.IssueManager.
func (p *Provider) ListIssueComments(ctx context.Context, owner, repo, number string) ([]*provider.IssueComment, error) {
	var comments []giteeIssueComment
	path := fmt.Sprintf("/repos/%s/%s/issues/%s/comments", esc(owner), esc(repo), esc(number))
	if err := p.doRequest(ctx, "GET", path, nil, &comments); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListIssueComments", err)
	}
	result := make([]*provider.IssueComment, 0, len(comments))
	for _, c := range comments {
		result = append(result, convertIssueComment(c))
	}
	return result, nil
}

// CreateIssueComment implements provider.IssueManager.
func (p *Provider) CreateIssueComment(ctx context.Context, owner, repo, number, body string) (*provider.IssueComment, error) {
	var comment giteeIssueComment
	path := fmt.Sprintf("/repos/%s/%s/issues/%s/comments", esc(owner), esc(repo), esc(number))
	if err := p.doRequest(ctx, "POST", path, map[string]any{"body": body}, &comment); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateIssueComment", err)
	}
	return convertIssueComment(comment), nil
}

// ListIssueLabels implements provider.IssueManager: repository-level labels.
func (p *Provider) ListIssueLabels(ctx context.Context, owner, repo string) ([]*provider.IssueLabel, error) {
	var labels []gitee.Label
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/labels", esc(owner), esc(repo)), nil, &labels); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListIssueLabels", err)
	}
	result := make([]*provider.IssueLabel, 0, len(labels))
	for _, l := range labels {
		result = append(result, &provider.IssueLabel{ID: int64(l.Id), Name: l.Name, Color: strings.TrimPrefix(l.Color, "#")})
	}
	return result, nil
}

// AddIssueLabels implements provider.IssueManager.
func (p *Provider) AddIssueLabels(ctx context.Context, owner, repo, number string, labels []string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%s/labels", esc(owner), esc(repo), esc(number))
	body := map[string]any{"labels": strings.Join(labels, ",")}
	if err := p.doRequest(ctx, "POST", path, body, nil); err != nil {
		return provider.Wrap(provider.PlatformGitee, "AddIssueLabels", err)
	}
	return nil
}

// RemoveIssueLabel implements provider.IssueManager.
func (p *Provider) RemoveIssueLabel(ctx context.Context, owner, repo, number, name string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%s/labels/%s", esc(owner), esc(repo), esc(number), esc(name))
	if err := p.doRequest(ctx, "DELETE", path, nil, nil); err != nil {
		return provider.Wrap(provider.PlatformGitee, "RemoveIssueLabel", err)
	}
	return nil
}

// convertIssue maps a giteeIssue to a provider.Issue.
func convertIssue(i giteeIssue) *provider.Issue {
	issue := &provider.Issue{
		Number: i.Number,
		Title:  i.Title,
		Body:   i.Body,
		State:  provider.IssueState(i.State),
		Author: giteeCRUser(i.User),
		WebURL: i.HTMLURL,
	}
	for _, l := range i.Labels {
		issue.Labels = append(issue.Labels, l.Name)
	}
	for _, a := range i.Assignees {
		issue.Assignees = append(issue.Assignees, a.Login)
	}
	if i.Milestone != nil {
		issue.Milestone = &provider.MilestoneRef{Number: strconv.Itoa(i.Milestone.ID), Title: i.Milestone.Title}
	}
	issue.CreatedAt, issue.UpdatedAt = i.CreatedAt, i.UpdatedAt
	return issue
}

// convertIssueComment maps a giteeIssueComment to a provider.IssueComment.
func convertIssueComment(c giteeIssueComment) *provider.IssueComment {
	return &provider.IssueComment{
		ID:        c.ID,
		Body:      c.Body,
		Author:    giteeCRUser(c.User),
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// giteeCRUser maps a giteeUser to a provider.CRUser.
func giteeCRUser(u *giteeUser) *provider.CRUser {
	if u == nil {
		return nil
	}
	return &provider.CRUser{Username: u.Login, Name: u.Name}
}

var _ provider.IssueManager = (*Provider)(nil)
