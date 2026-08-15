package github

import (
	"context"
	"strings"

	"github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListIssues implements provider.IssueManager.
func (p *Provider) ListIssues(ctx context.Context, opts provider.ListIssuesOptions) ([]*provider.Issue, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &github.IssueListByRepoOptions{ListOptions: github.ListOptions{Page: page, PerPage: perPage}}
	if opts.State != "" {
		listOpts.State = string(opts.State)
	}
	if opts.Assignee != "" {
		listOpts.Assignee = opts.Assignee
	}
	if opts.Labels != "" {
		listOpts.Labels = []string{opts.Labels}
	}
	issues, _, err := p.client.Issues.ListByRepo(ctx, opts.Owner, opts.Repo, listOpts)
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitHub, "ListIssues", err)
	}
	result := make([]*provider.Issue, 0, len(issues))
	for _, i := range issues {
		result = append(result, convertIssue(i))
	}
	return result, len(result), nil
}

// GetIssue implements provider.IssueManager.
func (p *Provider) GetIssue(ctx context.Context, owner, repo string, number int) (*provider.Issue, error) {
	issue, _, err := p.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "GetIssue", err)
	}
	return convertIssue(issue), nil
}

// CreateIssue implements provider.IssueManager.
func (p *Provider) CreateIssue(ctx context.Context, opts provider.CreateIssueOptions) (*provider.Issue, error) {
	req := &github.IssueRequest{Title: github.Ptr(opts.Title)}
	if opts.Body != "" {
		req.Body = github.Ptr(opts.Body)
	}
	if len(opts.Assignees) > 0 {
		req.Assignees = &opts.Assignees
	}
	if len(opts.Labels) > 0 {
		req.Labels = &opts.Labels
	}
	if opts.Milestone != 0 {
		req.Milestone = github.Ptr(opts.Milestone)
	}
	issue, _, err := p.client.Issues.Create(ctx, opts.Owner, opts.Repo, req)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateIssue", err)
	}
	return convertIssue(issue), nil
}

// UpdateIssue implements provider.IssueManager.
func (p *Provider) UpdateIssue(ctx context.Context, owner, repo string, number int, opts provider.UpdateIssueOptions) (*provider.Issue, error) {
	req := &github.IssueRequest{}
	if opts.Title != "" {
		req.Title = github.Ptr(opts.Title)
	}
	if opts.Body != "" {
		req.Body = github.Ptr(opts.Body)
	}
	if len(opts.Assignees) > 0 {
		req.Assignees = &opts.Assignees
	}
	if len(opts.Labels) > 0 {
		req.Labels = &opts.Labels
	}
	if opts.Milestone != 0 {
		req.Milestone = github.Ptr(opts.Milestone)
	}
	issue, _, err := p.client.Issues.Edit(ctx, owner, repo, number, req)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "UpdateIssue", err)
	}
	return convertIssue(issue), nil
}

// CloseIssue implements provider.IssueManager.
func (p *Provider) CloseIssue(ctx context.Context, owner, repo string, number int) (*provider.Issue, error) {
	issue, _, err := p.client.Issues.Edit(ctx, owner, repo, number, &github.IssueRequest{State: github.Ptr("closed")})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CloseIssue", err)
	}
	return convertIssue(issue), nil
}

// ReopenIssue implements provider.IssueManager.
func (p *Provider) ReopenIssue(ctx context.Context, owner, repo string, number int) (*provider.Issue, error) {
	issue, _, err := p.client.Issues.Edit(ctx, owner, repo, number, &github.IssueRequest{State: github.Ptr("open")})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ReopenIssue", err)
	}
	return convertIssue(issue), nil
}

// ListIssueComments implements provider.IssueManager.
func (p *Provider) ListIssueComments(ctx context.Context, owner, repo string, number int) ([]*provider.IssueComment, error) {
	comments, _, err := p.client.Issues.ListComments(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListIssueComments", err)
	}
	result := make([]*provider.IssueComment, 0, len(comments))
	for _, c := range comments {
		result = append(result, convertIssueComment(c))
	}
	return result, nil
}

// CreateIssueComment implements provider.IssueManager.
func (p *Provider) CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) (*provider.IssueComment, error) {
	comment, _, err := p.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{Body: github.Ptr(body)})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateIssueComment", err)
	}
	return convertIssueComment(comment), nil
}

// ListIssueLabels implements provider.IssueManager: repository-level labels.
func (p *Provider) ListIssueLabels(ctx context.Context, owner, repo string) ([]*provider.IssueLabel, error) {
	labels, _, err := p.client.Issues.ListLabels(ctx, owner, repo, &github.ListOptions{PerPage: 100})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListIssueLabels", err)
	}
	result := make([]*provider.IssueLabel, 0, len(labels))
	for _, l := range labels {
		result = append(result, &provider.IssueLabel{ID: l.GetID(), Name: l.GetName(), Color: trimHash(l.GetColor())})
	}
	return result, nil
}

// AddIssueLabels implements provider.IssueManager.
func (p *Provider) AddIssueLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	if _, _, err := p.client.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels); err != nil {
		return provider.Wrap(provider.PlatformGitHub, "AddIssueLabels", err)
	}
	return nil
}

// RemoveIssueLabel implements provider.IssueManager.
func (p *Provider) RemoveIssueLabel(ctx context.Context, owner, repo string, number int, name string) error {
	if _, err := p.client.Issues.RemoveLabelForIssue(ctx, owner, repo, number, name); err != nil {
		return provider.Wrap(provider.PlatformGitHub, "RemoveIssueLabel", err)
	}
	return nil
}

// convertIssue maps a github.Issue to a provider.Issue.
func convertIssue(i *github.Issue) *provider.Issue {
	if i == nil {
		return nil
	}
	var milestone *provider.MilestoneRef
	if i.Milestone != nil {
		milestone = &provider.MilestoneRef{Number: i.Milestone.GetNumber(), Title: i.Milestone.GetTitle()}
	}
	issue := &provider.Issue{
		ID:        i.GetID(),
		Number:    i.GetNumber(),
		Title:     i.GetTitle(),
		Body:      i.GetBody(),
		Author:    convertUser(i.GetUser()),
		Milestone: milestone,
		WebURL:    i.GetHTMLURL(),
	}
	if s := i.GetState(); s != "" {
		issue.State = provider.IssueState(s)
	}
	for _, l := range i.Labels {
		issue.Labels = append(issue.Labels, l.GetName())
	}
	for _, a := range i.Assignees {
		issue.Assignees = append(issue.Assignees, a.GetLogin())
	}
	if t := i.GetCreatedAt(); !t.IsZero() {
		issue.CreatedAt = t.Time
	}
	if t := i.GetUpdatedAt(); !t.IsZero() {
		issue.UpdatedAt = t.Time
	}
	// GetClosedAt returns the Timestamp by value; a zero timestamp means the
	// issue was never closed.
	if t := i.GetClosedAt(); !t.IsZero() {
		issue.ClosedAt = &t.Time
	}
	return issue
}

// convertIssueComment maps a github.IssueComment to a provider.IssueComment.
func convertIssueComment(c *github.IssueComment) *provider.IssueComment {
	if c == nil {
		return nil
	}
	comment := &provider.IssueComment{
		ID:     c.GetID(),
		Body:   c.GetBody(),
		Author: convertUser(c.GetUser()),
	}
	if t := c.GetCreatedAt(); !t.IsZero() {
		comment.CreatedAt = t.Time
	}
	if t := c.GetUpdatedAt(); !t.IsZero() {
		comment.UpdatedAt = t.Time
	}
	return comment
}

// trimHash strips a leading '#' from a color so the SDK's canonical form is
// '#' free, mirroring convertLabel in labels.go.
func trimHash(s string) string { return strings.TrimPrefix(s, "#") }

var _ provider.IssueManager = (*Provider)(nil)
