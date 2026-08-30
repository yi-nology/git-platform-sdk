package gitee

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	gitee "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// This file implements the provider.IssueManager surface over the go-gitee
// SDK. Gitee issue numbers are alphanumeric strings (e.g. "IAINVA"), carried
// natively by the string-typed interface since the M1 addressing change; the
// SDK's Issue model types Number as a string as well. CreateIssue keeps a
// registered raw detour (see the method) because the SDK's generated create
// call cannot encode its body correctly.

// ListIssues implements provider.IssueManager via the SDK. State, labels, and
// assignee filters travel as query params alongside the normalized
// pagination.
func (p *Provider) ListIssues(ctx context.Context, opts provider.ListIssuesOptions) ([]*provider.Issue, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := gitee.GetV5ReposOwnerRepoIssuesOpts{
		AccessToken: p.accessToken(),
		Page:        optional.NewInt32(toInt32(page)),
		PerPage:     optional.NewInt32(toInt32(perPage)),
	}
	if opts.State != "" {
		listOpts.State = optional.NewString(string(opts.State))
	}
	if opts.Assignee != "" {
		listOpts.Assignee = optional.NewString(opts.Assignee)
	}
	if opts.Labels != "" {
		listOpts.Labels = optional.NewString(opts.Labels)
	}
	issues, resp, err := p.client.IssuesApi.GetV5ReposOwnerRepoIssues(ctx, esc(opts.Owner), esc(opts.Repo), &listOpts)
	if err != nil {
		return nil, 0, p.sdkErr("ListIssues", resp, err)
	}
	result := make([]*provider.Issue, 0, len(issues))
	for i := range issues {
		result = append(result, convertIssue(issues[i]))
	}
	return result, len(result), nil
}

// GetIssue implements provider.IssueManager via the SDK. Gitee addresses
// issues by their alphanumeric string number directly — the SDK parameter is
// a string, so no conversion is needed.
func (p *Provider) GetIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	issue, resp, err := p.client.IssuesApi.GetV5ReposOwnerRepoIssuesNumber(ctx, esc(owner), esc(repo), esc(number), &gitee.GetV5ReposOwnerRepoIssuesNumberOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return nil, p.sdkErr("GetIssue", resp, err)
	}
	return convertIssue(issue), nil
}

// CreateIssue implements provider.IssueManager.
//
// Routed through the raw transport client rather than the SDK: the generated
// PostV5ReposOwnerIssues encodes its opts as a multipart body while sending an
// application/json Content-Type header (upstream client.go prepareRequest bug
// — same family as the labels patch and releases create detours), which the
// server cannot parse. The raw call uses the documented owner-scoped
// POST /repos/{owner}/issues endpoint with the repo path in the body, and
// Gitee's own parameter vocabulary (assignees join onto the single "assignee"
// param, labels are comma-joined, milestone carries the milestone serial
// number).
func (p *Provider) CreateIssue(ctx context.Context, opts provider.CreateIssueOptions) (*provider.Issue, error) {
	body := map[string]any{"repo": opts.Repo, "title": opts.Title}
	if opts.Body != "" {
		body["body"] = opts.Body
	}
	if len(opts.Labels) > 0 {
		body["labels"] = strings.Join(opts.Labels, ",")
	}
	if len(opts.Assignees) > 0 {
		body["assignee"] = strings.Join(opts.Assignees, ",")
	}
	if opts.Milestone != "" {
		m, err := strconv.Atoi(opts.Milestone)
		if err != nil {
			return nil, provider.Wrapf(provider.PlatformGitee, "CreateIssue", "invalid milestone number %q", opts.Milestone)
		}
		body["milestone"] = m
	}
	var issue gitee.Issue
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/issues", esc(opts.Owner)), body, &issue); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateIssue", err)
	}
	return convertIssue(issue), nil
}

// UpdateIssue implements provider.IssueManager via the SDK. The update
// endpoint is owner-scoped (PATCH /repos/{owner}/issues/{number}), so the
// repo path travels in the JSON body via the SDK's IssueUpdateParam; empty
// fields drop out of the wire body (omitempty), leaving the issue unchanged.
// Milestone carries Gitee's milestone serial number, the identifier round
// tripped by MilestoneRef below; assignees join onto the single "assignee"
// param.
func (p *Provider) UpdateIssue(ctx context.Context, owner, repo, number string, opts provider.UpdateIssueOptions) (*provider.Issue, error) {
	param := gitee.IssueUpdateParam{
		AccessToken: p.token,
		Repo:        repo,
	}
	if opts.Title != "" {
		param.Title = opts.Title
	}
	if opts.Body != "" {
		param.Body = opts.Body
	}
	if opts.State != "" {
		param.State = string(opts.State)
	}
	if len(opts.Assignees) > 0 {
		param.Assignee = strings.Join(opts.Assignees, ",")
	}
	if len(opts.Labels) > 0 {
		param.Labels = strings.Join(opts.Labels, ",")
	}
	if opts.Milestone != "" {
		m, err := strconv.Atoi(opts.Milestone)
		if err != nil {
			return nil, provider.Wrapf(provider.PlatformGitee, "UpdateIssue", "invalid milestone number %q", opts.Milestone)
		}
		param.Milestone = toInt32(m)
	}
	issue, resp, err := p.client.IssuesApi.PatchV5ReposOwnerIssuesNumber(ctx, esc(owner), esc(number), param)
	if err != nil {
		return nil, p.sdkErr("UpdateIssue", resp, err)
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

// patchIssueState flips an issue's state via the SDK's owner-scoped update
// (the repo path travels in the JSON body). op is the public operation this
// patch serves; failures surface under that op.
func (p *Provider) patchIssueState(ctx context.Context, owner, repo, number, state, op string) (*provider.Issue, error) {
	issue, resp, err := p.client.IssuesApi.PatchV5ReposOwnerIssuesNumber(ctx, esc(owner), esc(number), gitee.IssueUpdateParam{
		AccessToken: p.token,
		Repo:        repo,
		State:       state,
	})
	if err != nil {
		return nil, p.sdkErr(op, resp, err)
	}
	return convertIssue(issue), nil
}

// issueCommentPageSize is the per-page value for paginated issue-comment
// fetches — 100 is the documented maximum on the GitHub-shaped platforms;
// servers that cap lower are handled by the stop-on-empty loop.
const issueCommentPageSize = 100

// ListIssueComments implements provider.IssueManager via the SDK,
// exhausting Gitee's pagination (the loop advances until an empty page, so
// the result is the complete comment list).
//
// Registration: the SDK's Note model carries no updated_at field (the live
// wire does), so IssueComment.UpdatedAt stays zero on Gitee until the SDK
// model catches up; created_at parses from the wire's timestamp string.
func (p *Provider) ListIssueComments(ctx context.Context, owner, repo, number string) ([]*provider.IssueComment, error) {
	notes, err := backendutil.AllPages(func(page int) ([]gitee.Note, error) {
		batch, _, err := p.client.IssuesApi.GetV5ReposOwnerRepoIssuesNumberComments(ctx, esc(owner), esc(repo), esc(number), &gitee.GetV5ReposOwnerRepoIssuesNumberCommentsOpts{
			AccessToken: p.accessToken(),
			Page:        optional.NewInt32(toInt32(page)),
			PerPage:     optional.NewInt32(issueCommentPageSize),
		})
		return batch, err
	})
	if err != nil {
		return nil, p.sdkErr("ListIssueComments", nil, err)
	}
	result := make([]*provider.IssueComment, 0, len(notes))
	for _, n := range notes {
		result = append(result, convertIssueComment(n))
	}
	return result, nil
}

// CreateIssueComment implements provider.IssueManager via the SDK: the body
// param marshals as JSON ({"access_token": ..., "body": ...}).
func (p *Provider) CreateIssueComment(ctx context.Context, owner, repo, number, body string) (*provider.IssueComment, error) {
	note, resp, err := p.client.IssuesApi.PostV5ReposOwnerRepoIssuesNumberComments(ctx, esc(owner), esc(repo), esc(number), gitee.IssueCommentPostParam{
		AccessToken: p.token,
		Body:        body,
	})
	if err != nil {
		return nil, p.sdkErr("CreateIssueComment", resp, err)
	}
	return convertIssueComment(note), nil
}

// UpdateIssueComment implements provider.IssueManager. The edit endpoint
// addresses the comment directly, so number is unused. Gitee's wire IDs are
// int32, so an out-of-range comment ID is rejected up front instead of
// silently truncating to a different comment; the platform only lets the
// comment's author perform the edit.
func (p *Provider) UpdateIssueComment(ctx context.Context, owner, repo, number string, commentID int64, body string) (*provider.IssueComment, error) {
	if commentID < 0 || commentID > math.MaxInt32 {
		return nil, provider.Wrapf(provider.PlatformGitee, "UpdateIssueComment", "comment id %d out of gitee's int32 range", commentID)
	}
	note, resp, err := p.client.IssuesApi.PatchV5ReposOwnerRepoIssuesCommentsId(ctx, esc(owner), esc(repo), int32(commentID), gitee.IssueCommentPatchParam{
		AccessToken: p.token,
		Body:        body,
	})
	if err != nil {
		return nil, p.sdkErr("UpdateIssueComment", resp, err)
	}
	return convertIssueComment(note), nil
}

// ListIssueLabels implements provider.IssueManager: repository-level labels
// via the SDK. The interface takes no pagination, so the SDK's
// AccessToken-only opts lose nothing (unlike LabelManager.ListLabels, whose
// pagination contract forces its raw detour in labels.go).
func (p *Provider) ListIssueLabels(ctx context.Context, owner, repo string) ([]*provider.IssueLabel, error) {
	labels, resp, err := p.client.LabelsApi.GetV5ReposOwnerRepoLabels(ctx, esc(owner), esc(repo), &gitee.GetV5ReposOwnerRepoLabelsOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return nil, p.sdkErr("ListIssueLabels", resp, err)
	}
	result := make([]*provider.IssueLabel, 0, len(labels))
	for _, l := range labels {
		result = append(result, &provider.IssueLabel{ID: int64(l.Id), Name: l.Name, Color: strings.TrimPrefix(l.Color, "#")})
	}
	return result, nil
}

// AddIssueLabels implements provider.IssueManager via the SDK: the generated
// call posts the bare label-name array as its JSON body (an upstream patch on
// PullRequestLabelPostParam — the same wire shape as the PR-labels calls in
// crs.go). That patch drops the param's AccessToken from the wire; the call
// stays authenticated through the transport pipeline's Bearer header.
func (p *Provider) AddIssueLabels(ctx context.Context, owner, repo, number string, labels []string) error {
	_, resp, err := p.client.LabelsApi.PostV5ReposOwnerRepoIssuesNumberLabels(ctx, esc(owner), esc(repo), esc(number), gitee.PullRequestLabelPostParam{
		AccessToken: p.token,
		Body:        labels,
	})
	if err != nil {
		return p.sdkErr("AddIssueLabels", resp, err)
	}
	return nil
}

// RemoveIssueLabel implements provider.IssueManager via the SDK (the
// name-addressed delete passes the token as a query param).
func (p *Provider) RemoveIssueLabel(ctx context.Context, owner, repo, number, name string) error {
	resp, err := p.client.LabelsApi.DeleteV5ReposOwnerRepoIssuesNumberLabelsName(ctx, esc(owner), esc(repo), esc(number), esc(name), &gitee.DeleteV5ReposOwnerRepoIssuesNumberLabelsNameOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return p.sdkErr("RemoveIssueLabel", resp, err)
	}
	return nil
}

// convertIssue maps the SDK Issue model to a provider.Issue. Gitee issue
// numbers are alphanumeric strings, carried as-is. Registration: Gitee's
// issue payload carries a single assignee (负责人) plus collaborators, mapped
// onto Issue.Assignees in that order; MilestoneRef.Number carries Gitee's
// milestone serial number (the SDK Milestone model exposes no id), which is
// exactly the identifier Gitee's issue write endpoints take, so
// MilestoneRef round trips through CreateIssue/UpdateIssue. State passes
// through unfiltered: beyond open/closed, Gitee enterprise workspaces
// expose extra workflow states (progressing, rejected, ...) that surface
// as-is — IssueState is an open string vocabulary, not a closed enum.
func convertIssue(i gitee.Issue) *provider.Issue {
	issue := &provider.Issue{
		Number:    i.Number,
		Title:     i.Title,
		Body:      i.Body,
		State:     provider.IssueState(i.State),
		Author:    giteeCRUserBasic(i.User),
		WebURL:    i.HtmlUrl,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
	for _, l := range i.Labels {
		issue.Labels = append(issue.Labels, l.Name)
	}
	if i.Assignee != nil {
		issue.Assignees = append(issue.Assignees, i.Assignee.Login)
	}
	for ci := range i.Collaborators {
		issue.Assignees = append(issue.Assignees, i.Collaborators[ci].Login)
	}
	if i.Milestone != nil {
		issue.Milestone = &provider.MilestoneRef{Number: strconv.Itoa(int(i.Milestone.Number)), Title: i.Milestone.Title}
	}
	return issue
}

// convertIssueComment maps the SDK Note model to a provider.IssueComment
// (updated_at omission registered on ListIssueComments; created_at parses
// from the wire's timestamp string).
func convertIssueComment(n gitee.Note) *provider.IssueComment {
	return &provider.IssueComment{
		ID:        int64(n.Id),
		Body:      n.Body,
		Author:    giteeCRUser(n.User),
		CreatedAt: parseGiteeTime(n.CreatedAt),
	}
}

// giteeCRUserBasic maps the SDK UserBasic model (issues' author/assignee
// shape) to a provider.CRUser.
func giteeCRUserBasic(u *gitee.UserBasic) *provider.CRUser {
	if u == nil {
		return nil
	}
	return &provider.CRUser{ID: int64(u.Id), Username: u.Login, Name: u.Name}
}

// giteeCRUser maps the SDK User model (issue comments' author shape) to a
// provider.CRUser.
func giteeCRUser(u *gitee.User) *provider.CRUser {
	if u == nil {
		return nil
	}
	return &provider.CRUser{ID: int64(u.Id), Username: u.Login, Name: u.Name}
}

var _ provider.IssueManager = (*Provider)(nil)
