package gitcode

import (
	"context"
	"strconv"
	"strings"

	"github.com/yi-nology/git-platform-sdk/provider"
	gitcode "github.com/yi-nology/gitcode_api"
)

// issueNumber parses the SDK's string issue number into gitcode's int form.
// op is the public operation the parse serves; failures surface under it.
func issueNumber(op, number string) (int, error) {
	n, err := strconv.Atoi(number)
	if err != nil {
		return 0, provider.Wrapf(provider.PlatformGitCode, op, "invalid issue number %q", number)
	}
	return n, nil
}

// milestoneNumber parses the SDK's string milestone identifier (gitcode
// milestone IDs) into gitcode's int64 form.
func milestoneNumber(op, milestone string) (int64, error) {
	m, err := strconv.ParseInt(milestone, 10, 64)
	if err != nil {
		return 0, provider.Wrapf(provider.PlatformGitCode, op, "invalid milestone number %q", milestone)
	}
	return m, nil
}

// ListIssues implements provider.IssueManager.
func (p *Provider) ListIssues(ctx context.Context, opts provider.ListIssuesOptions) ([]*provider.Issue, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := gitcode.ListIssuesOptions{
		ListOptions: gitcode.ListOptions{Page: page, PerPage: perPage},
	}
	if opts.State != "" {
		listOpts.State = gitcode.IssueState(opts.State)
	}
	if opts.Assignee != "" {
		listOpts.Assignee = opts.Assignee
	}
	if opts.Labels != "" {
		listOpts.Labels = opts.Labels
	}
	issues, err := p.client.ListIssues(ctx, opts.Owner, opts.Repo, listOpts)
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitCode, "ListIssues", err)
	}
	result := make([]*provider.Issue, 0, len(issues))
	for _, i := range issues {
		result = append(result, convertIssue(i))
	}
	return result, len(result), nil
}

// GetIssue implements provider.IssueManager.
func (p *Provider) GetIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	n, err := issueNumber("GetIssue", number)
	if err != nil {
		return nil, err
	}
	issue, err := p.client.GetIssue(ctx, owner, repo, n)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetIssue", err)
	}
	return convertIssue(issue), nil
}

// CreateIssue implements provider.IssueManager.
func (p *Provider) CreateIssue(ctx context.Context, opts provider.CreateIssueOptions) (*provider.Issue, error) {
	createOpts := gitcode.CreateIssueOptions{
		Title:     opts.Title,
		Body:      opts.Body,
		Assignees: opts.Assignees,
		Labels:    opts.Labels,
	}
	if opts.Milestone != "" {
		m, err := milestoneNumber("CreateIssue", opts.Milestone)
		if err != nil {
			return nil, err
		}
		createOpts.Milestone = m
	}
	issue, err := p.client.CreateIssue(ctx, opts.Owner, opts.Repo, createOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CreateIssue", err)
	}
	return convertIssue(issue), nil
}

// UpdateIssue implements provider.IssueManager.
func (p *Provider) UpdateIssue(ctx context.Context, owner, repo, number string, opts provider.UpdateIssueOptions) (*provider.Issue, error) {
	n, err := issueNumber("UpdateIssue", number)
	if err != nil {
		return nil, err
	}
	updateOpts := gitcode.UpdateIssueOptions{
		Title:     opts.Title,
		Body:      opts.Body,
		Assignees: opts.Assignees,
		Labels:    opts.Labels,
	}
	if opts.Milestone != "" {
		m, err := milestoneNumber("UpdateIssue", opts.Milestone)
		if err != nil {
			return nil, err
		}
		updateOpts.Milestone = m
	}
	if opts.State != "" {
		updateOpts.State = gitcode.IssueState(opts.State)
	}
	issue, err := p.client.UpdateIssue(ctx, owner, repo, n, updateOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateIssue", err)
	}
	return convertIssue(issue), nil
}

// CloseIssue implements provider.IssueManager.
func (p *Provider) CloseIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	n, err := issueNumber("CloseIssue", number)
	if err != nil {
		return nil, err
	}
	issue, err := p.client.CloseIssue(ctx, owner, repo, n)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CloseIssue", err)
	}
	return convertIssue(issue), nil
}

// ReopenIssue implements provider.IssueManager.
func (p *Provider) ReopenIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	n, err := issueNumber("ReopenIssue", number)
	if err != nil {
		return nil, err
	}
	issue, err := p.client.ReopenIssue(ctx, owner, repo, n)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ReopenIssue", err)
	}
	return convertIssue(issue), nil
}

// ListIssueComments implements provider.IssueManager.
func (p *Provider) ListIssueComments(ctx context.Context, owner, repo, number string) ([]*provider.IssueComment, error) {
	n, err := issueNumber("ListIssueComments", number)
	if err != nil {
		return nil, err
	}
	comments, err := p.client.ListIssueComments(ctx, owner, repo, n)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListIssueComments", err)
	}
	result := make([]*provider.IssueComment, 0, len(comments))
	for _, c := range comments {
		result = append(result, convertIssueComment(c))
	}
	return result, nil
}

// CreateIssueComment implements provider.IssueManager.
func (p *Provider) CreateIssueComment(ctx context.Context, owner, repo, number, body string) (*provider.IssueComment, error) {
	n, err := issueNumber("CreateIssueComment", number)
	if err != nil {
		return nil, err
	}
	comment, err := p.client.CreateIssueComment(ctx, owner, repo, n, body)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CreateIssueComment", err)
	}
	return convertIssueComment(comment), nil
}

// ListIssueLabels implements provider.IssueManager.
func (p *Provider) ListIssueLabels(ctx context.Context, owner, repo string) ([]*provider.IssueLabel, error) {
	labels, err := p.client.ListIssueLabels(ctx, owner, repo)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListIssueLabels", err)
	}
	result := make([]*provider.IssueLabel, 0, len(labels))
	for _, l := range labels {
		result = append(result, &provider.IssueLabel{
			ID:   l.ID,
			Name: l.Name,
			// Same canonicalization as convertLabel in labels.go: GitCode
			// labels carry '#'-prefixed colors on the wire; the SDK type is
			// '#' free.
			Color: strings.TrimPrefix(l.Color, "#"),
		})
	}
	return result, nil
}

// AddIssueLabels implements provider.IssueManager.
func (p *Provider) AddIssueLabels(ctx context.Context, owner, repo, number string, labels []string) error {
	n, err := issueNumber("AddIssueLabels", number)
	if err != nil {
		return err
	}
	if err := p.client.AddIssueLabels(ctx, owner, repo, n, labels); err != nil {
		return provider.Wrap(provider.PlatformGitCode, "AddIssueLabels", err)
	}
	return nil
}

// RemoveIssueLabel implements provider.IssueManager.
func (p *Provider) RemoveIssueLabel(ctx context.Context, owner, repo, number, name string) error {
	n, err := issueNumber("RemoveIssueLabel", number)
	if err != nil {
		return err
	}
	if err := p.client.RemoveIssueLabel(ctx, owner, repo, n, name); err != nil {
		return provider.Wrap(provider.PlatformGitCode, "RemoveIssueLabel", err)
	}
	return nil
}

var _ provider.IssueManager = (*Provider)(nil)

// convertUser maps a gitcode.User to a provider.CRUser. Returns nil if u is nil.
func convertUser(u *gitcode.User) *provider.CRUser {
	if u == nil {
		return nil
	}
	id, _ := strconv.ParseInt(string(u.ID), 10, 64)
	return &provider.CRUser{
		ID:        id,
		Username:  u.Login,
		Name:      u.Name,
		AvatarURL: u.AvatarURL,
	}
}

// convertIssue maps a gitcode.Issue to a provider.Issue.
func convertIssue(i *gitcode.Issue) *provider.Issue {
	labels := make([]string, 0, len(i.Labels))
	for _, l := range i.Labels {
		labels = append(labels, l.Name)
	}
	assignees := make([]string, 0, len(i.Assignees))
	for _, a := range i.Assignees {
		assignees = append(assignees, a.Login)
	}
	author := convertUser(i.Author)
	if author == nil {
		author = convertUser(i.User)
	}
	var milestone *provider.MilestoneRef
	if i.Milestone != nil {
		milestone = &provider.MilestoneRef{Number: strconv.FormatInt(i.Milestone.ID, 10), Title: i.Milestone.Title}
	}
	return &provider.Issue{
		ID:        i.ID,
		Number:    strconv.Itoa(int(i.Number)),
		Title:     i.Title,
		Body:      i.Body,
		State:     provider.IssueState(i.State),
		Author:    author,
		Labels:    labels,
		Assignees: assignees,
		Milestone: milestone,
		WebURL:    i.HTMLURL,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
		ClosedAt:  i.ClosedAt,
	}
}

// convertIssueComment maps a gitcode.IssueComment to a provider.IssueComment.
func convertIssueComment(c *gitcode.IssueComment) *provider.IssueComment {
	author := convertUser(c.Author)
	if author == nil {
		author = convertUser(c.User)
	}
	return &provider.IssueComment{
		ID:        c.ID,
		Body:      c.Body,
		Author:    author,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}
