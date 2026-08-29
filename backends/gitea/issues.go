package gitea

import (
	"context"
	"strconv"
	"strings"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// issueNumber parses the SDK's string issue number into gitea's int64 index
// form. op is the public operation the parse serves; failures surface under
// it.
func issueNumber(op, number string) (int64, error) {
	n, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return 0, provider.Wrapf(provider.PlatformGitea, op, "invalid issue number %q", number)
	}
	return n, nil
}

// milestoneNumber parses the SDK's string milestone identifier (gitea
// milestone IDs) into gitea's int64 form.
func milestoneNumber(op, milestone string) (int64, error) {
	m, err := strconv.ParseInt(milestone, 10, 64)
	if err != nil {
		return 0, provider.Wrapf(provider.PlatformGitea, op, "invalid milestone number %q", milestone)
	}
	return m, nil
}

// ListIssues implements provider.IssueManager. The gitea SDK accepts no
// context (same as its other services).
func (p *Provider) ListIssues(ctx context.Context, opts provider.ListIssuesOptions) ([]*provider.Issue, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := gitea.ListIssueOption{ListOptions: gitea.ListOptions{Page: page, PageSize: perPage}}
	// Unset, the endpoint returns PRs mixed in with the issues.
	listOpts.Type = gitea.IssueTypeIssue
	if opts.State != "" {
		listOpts.State = gitea.StateType(opts.State)
	}
	if opts.Labels != "" {
		listOpts.Labels = strings.Split(opts.Labels, ",")
	}
	if opts.Assignee != "" {
		listOpts.AssignedBy = opts.Assignee
	}
	issues, _, err := p.client.ListRepoIssues(opts.Owner, opts.Repo, listOpts)
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitea, "ListIssues", err)
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
	issue, _, err := p.client.GetIssue(owner, repo, n)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetIssue", err)
	}
	return convertIssue(issue), nil
}

// CreateIssue implements provider.IssueManager. Gitea's create endpoint
// takes label IDs, so names are resolved first (one list call per name;
// label counts are small).
func (p *Provider) CreateIssue(ctx context.Context, opts provider.CreateIssueOptions) (*provider.Issue, error) {
	createOpts := gitea.CreateIssueOption{
		Title:     opts.Title,
		Body:      opts.Body,
		Assignees: opts.Assignees,
	}
	if opts.Milestone != "" {
		m, err := milestoneNumber("CreateIssue", opts.Milestone)
		if err != nil {
			return nil, err
		}
		createOpts.Milestone = m
	}
	if len(opts.Labels) > 0 {
		ids, err := p.resolveLabelIDs("CreateIssue", opts.Owner, opts.Repo, opts.Labels)
		if err != nil {
			return nil, err
		}
		createOpts.Labels = ids
	}
	issue, _, err := p.client.CreateIssue(opts.Owner, opts.Repo, createOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateIssue", err)
	}
	return convertIssue(issue), nil
}

// UpdateIssue implements provider.IssueManager. Gitea's EditIssueOption
// always serializes Title (no omitempty), so a caller leaving fields empty
// would clear them; the current title is backfilled via one GET, and an
// all-empty update short-circuits to a plain GetIssue. opts.Labels replaces
// the issue's labels via the dedicated endpoint.
func (p *Provider) UpdateIssue(ctx context.Context, owner, repo, number string, opts provider.UpdateIssueOptions) (*provider.Issue, error) {
	n, err := issueNumber("UpdateIssue", number)
	if err != nil {
		return nil, err
	}
	if opts.Title == "" && opts.Body == "" && opts.State == "" && len(opts.Assignees) == 0 && len(opts.Labels) == 0 && opts.Milestone == "" {
		return p.GetIssue(ctx, owner, repo, number)
	}
	edit, err := p.buildEditIssueOption("UpdateIssue", owner, repo, n, opts)
	if err != nil {
		return nil, err
	}
	if len(opts.Labels) > 0 {
		ids, err := p.resolveLabelIDs("UpdateIssue", owner, repo, opts.Labels)
		if err != nil {
			return nil, err
		}
		if _, _, err := p.client.ReplaceIssueLabels(owner, repo, n, gitea.IssueLabelsOption{Labels: ids}); err != nil {
			return nil, provider.Wrap(provider.PlatformGitea, "UpdateIssue", err)
		}
	}
	issue, _, err := p.client.EditIssue(owner, repo, n, edit)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "UpdateIssue", err)
	}
	return convertIssue(issue), nil
}

// buildEditIssueOption translates provider update options into gitea's
// EditIssueOption. Gitea always serializes Title (no omitempty), so when the
// caller leaves the title empty the current one is backfilled via one GET to
// avoid clearing it. op is the public operation this build serves; failures
// surface under that op. n is the parsed issue number.
func (p *Provider) buildEditIssueOption(op, owner, repo string, n int64, opts provider.UpdateIssueOptions) (gitea.EditIssueOption, error) {
	edit := gitea.EditIssueOption{}
	if opts.Title != "" {
		edit.Title = opts.Title
	} else {
		current, _, err := p.client.GetIssue(owner, repo, n)
		if err != nil {
			return edit, provider.Wrap(provider.PlatformGitea, op, err)
		}
		edit.Title = current.Title
	}
	if len(opts.Assignees) > 0 {
		edit.Assignees = opts.Assignees
	}
	if opts.Body != "" {
		body := opts.Body
		edit.Body = &body
	}
	if opts.State != "" {
		s := gitea.StateType(opts.State)
		edit.State = &s
	}
	if opts.Milestone != "" {
		m, err := milestoneNumber(op, opts.Milestone)
		if err != nil {
			return edit, err
		}
		edit.Milestone = &m
	}
	return edit, nil
}

// CloseIssue implements provider.IssueManager.
func (p *Provider) CloseIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	n, err := issueNumber("CloseIssue", number)
	if err != nil {
		return nil, err
	}
	return p.setIssueState(owner, repo, n, gitea.StateClosed, "CloseIssue")
}

// ReopenIssue implements provider.IssueManager.
func (p *Provider) ReopenIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	n, err := issueNumber("ReopenIssue", number)
	if err != nil {
		return nil, err
	}
	return p.setIssueState(owner, repo, n, gitea.StateOpen, "ReopenIssue")
}

// setIssueState closes/reopens an issue. EditIssue always serializes Title,
// so the current title is fetched first to avoid clearing it. n is the
// parsed issue number.
func (p *Provider) setIssueState(owner, repo string, n int64, state gitea.StateType, op string) (*provider.Issue, error) {
	current, _, err := p.client.GetIssue(owner, repo, n)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, op, err)
	}
	issue, _, err := p.client.EditIssue(owner, repo, n, gitea.EditIssueOption{Title: current.Title, State: &state})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, op, err)
	}
	return convertIssue(issue), nil
}

// ListIssueComments implements provider.IssueManager.
func (p *Provider) ListIssueComments(ctx context.Context, owner, repo, number string) ([]*provider.IssueComment, error) {
	n, err := issueNumber("ListIssueComments", number)
	if err != nil {
		return nil, err
	}
	comments, _, err := p.client.ListIssueComments(owner, repo, n, gitea.ListIssueCommentOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListIssueComments", err)
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
	comment, _, err := p.client.CreateIssueComment(owner, repo, n, gitea.CreateIssueCommentOption{Body: body})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateIssueComment", err)
	}
	return convertIssueComment(comment), nil
}

// UpdateIssueComment implements provider.IssueManager. The edit endpoint
// addresses the comment directly, so number is unused; the SDK accepts no
// context (see the gitea standing limitation). The platform only lets the
// comment's author perform the edit.
func (p *Provider) UpdateIssueComment(ctx context.Context, owner, repo, number string, commentID int64, body string) (*provider.IssueComment, error) {
	comment, _, err := p.client.EditIssueComment(owner, repo, commentID, gitea.EditIssueCommentOption{Body: body})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "UpdateIssueComment", err)
	}
	return convertIssueComment(comment), nil
}

// ListIssueLabels implements provider.IssueManager: repository-level labels.
func (p *Provider) ListIssueLabels(ctx context.Context, owner, repo string) ([]*provider.IssueLabel, error) {
	labels, _, err := p.client.ListRepoLabels(owner, repo, gitea.ListLabelsOptions{ListOptions: gitea.ListOptions{PageSize: 100}})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListIssueLabels", err)
	}
	result := make([]*provider.IssueLabel, 0, len(labels))
	for _, l := range labels {
		result = append(result, &provider.IssueLabel{ID: l.ID, Name: l.Name, Color: strings.TrimPrefix(l.Color, "#")})
	}
	return result, nil
}

// AddIssueLabels implements provider.IssueManager.
func (p *Provider) AddIssueLabels(ctx context.Context, owner, repo, number string, labels []string) error {
	n, err := issueNumber("AddIssueLabels", number)
	if err != nil {
		return err
	}
	ids, err := p.resolveLabelIDs("AddIssueLabels", owner, repo, labels)
	if err != nil {
		return err
	}
	if _, _, err := p.client.AddIssueLabels(owner, repo, n, gitea.IssueLabelsOption{Labels: ids}); err != nil {
		return provider.Wrap(provider.PlatformGitea, "AddIssueLabels", err)
	}
	return nil
}

// RemoveIssueLabel implements provider.IssueManager.
func (p *Provider) RemoveIssueLabel(ctx context.Context, owner, repo, number, name string) error {
	n, err := issueNumber("RemoveIssueLabel", number)
	if err != nil {
		return err
	}
	id, err := p.resolveLabelID("RemoveIssueLabel", owner, repo, name)
	if err != nil {
		return err
	}
	if _, err := p.client.DeleteIssueLabel(owner, repo, n, id); err != nil {
		return provider.Wrap(provider.PlatformGitea, "RemoveIssueLabel", err)
	}
	return nil
}

// resolveLabelIDs resolves label names to numeric IDs, one list call per
// name (labels.go owns the single-name resolver).
func (p *Provider) resolveLabelIDs(op, owner, repo string, names []string) ([]int64, error) {
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		id, err := p.resolveLabelID(op, owner, repo, name)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// convertIssue maps a gitea.Issue to a provider.Issue.
func convertIssue(i *gitea.Issue) *provider.Issue {
	if i == nil {
		return nil
	}
	issue := &provider.Issue{
		ID:     i.ID,
		Number: strconv.FormatInt(i.Index, 10),
		Title:  i.Title,
		Body:   i.Body,
		State:  provider.IssueState(i.State),
		Author: convertUser(i.Poster),
		WebURL: i.HTMLURL,
	}
	for _, l := range i.Labels {
		issue.Labels = append(issue.Labels, l.Name)
	}
	for _, a := range i.Assignees {
		issue.Assignees = append(issue.Assignees, a.UserName)
	}
	if i.Milestone != nil {
		issue.Milestone = &provider.MilestoneRef{Number: strconv.FormatInt(i.Milestone.ID, 10), Title: i.Milestone.Title}
	}
	issue.CreatedAt, issue.UpdatedAt = i.Created, i.Updated
	if i.Closed != nil {
		issue.ClosedAt = i.Closed
	}
	return issue
}

// convertIssueComment maps a gitea.Comment to a provider.IssueComment.
func convertIssueComment(c *gitea.Comment) *provider.IssueComment {
	if c == nil {
		return nil
	}
	return &provider.IssueComment{
		ID:        c.ID,
		Body:      c.Body,
		Author:    convertUser(c.Poster),
		CreatedAt: c.Created,
		UpdatedAt: c.Updated,
	}
}

// convertUser maps a gitea.User to a provider.CRUser. Returns nil if u is nil.
func convertUser(u *gitea.User) *provider.CRUser {
	if u == nil {
		return nil
	}
	return &provider.CRUser{ID: u.ID, Username: u.UserName, AvatarURL: u.AvatarURL}
}

var _ provider.IssueManager = (*Provider)(nil)
