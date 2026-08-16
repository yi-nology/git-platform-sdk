package tencentcode

import (
	"context"
	"strconv"
	"strings"

	gongfeng "github.com/studyzy/gongfeng-sdk-go"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// This file implements provider.IssueManager over the gongfeng SDK's
// IssuesService and NotesService (工蜂 models issue comments as notes).
// Four registered limitations apply:
//
//   - assignees: 工蜂's issue write surface takes assignee_ids — a csv of
//     numeric user IDs — so the SDK's username-based Assignees fields on
//     CreateIssueOptions/UpdateIssueOptions cannot be mapped without a
//     Users-API resolution round and are ignored (GitLab-backend
//     precedent); ListIssuesOptions.Assignee is likewise not carried.
//   - last-label removal: gongfeng's UpdateIssueOptions.Labels is a csv
//     string behind omitempty, so removing an issue's only label yields an
//     empty csv that cannot travel on the PUT body — the update becomes a
//     no-op (the label stays).
//   - web URL: gongfeng's Issue model carries no web_url field, so
//     Issue.WebURL is always empty on this platform.
//   - closed-at: the model carries no closed_at field either, so
//     Issue.ClosedAt is always nil (the wire state still round-trips via
//     Issue.State).

// issueNumber parses the SDK's string issue number into gongfeng's int IID
// form. op is the public operation the parse serves; failures surface
// under it.
func issueNumber(op, number string) (int, error) {
	n, err := strconv.Atoi(number)
	if err != nil {
		return 0, provider.Wrapf(provider.PlatformTencentCode, op, "invalid issue number %q", number)
	}
	return n, nil
}

// ListIssues implements provider.IssueManager. State and the Labels csv
// pass through as filters (open→opened per 工蜂's vocabulary; inbound
// convertIssue maps back); the Assignee filter is not carried (registered
// above).
func (p *Provider) ListIssues(ctx context.Context, opts provider.ListIssuesOptions) ([]*provider.Issue, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gongfeng.ListIssuesOptions{
		ListOptions: gongfeng.ListOptions{Page: page, PerPage: perPage},
	}
	if opts.State != "" {
		s := string(opts.State)
		if s == string(provider.IssueStateOpen) {
			s = "opened" // 工蜂's issues API vocabulary; inbound convertIssue maps back
		}
		listOpts.State = gongfeng.Ptr(s)
	}
	if opts.Labels != "" {
		listOpts.Labels = gongfeng.Ptr(opts.Labels) // csv filter, passed through
	}
	issues, resp, err := p.client.Issues.ListIssues(ctx, pid(opts.Owner, opts.Repo), listOpts)
	if err != nil {
		return nil, 0, sdkError("ListIssues", err)
	}
	result := make([]*provider.Issue, 0, len(issues))
	for _, i := range issues {
		result = append(result, convertIssue(i))
	}
	return result, extractTotalCount(resp, len(result)), nil
}

// GetIssue implements provider.IssueManager. 工蜂 addresses issues by IID.
func (p *Provider) GetIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	n, err := issueNumber("GetIssue", number)
	if err != nil {
		return nil, err
	}
	issue, _, err := p.client.Issues.GetIssue(ctx, pid(owner, repo), n)
	if err != nil {
		return nil, sdkError("GetIssue", err)
	}
	return convertIssue(issue), nil
}

// CreateIssue implements provider.IssueManager. opts.Assignees is ignored:
// 工蜂's create surface takes assignee IDs, and resolving usernames to IDs
// requires the Users API (a future UserManager round).
func (p *Provider) CreateIssue(ctx context.Context, opts provider.CreateIssueOptions) (*provider.Issue, error) {
	createOpts := &gongfeng.CreateIssueOptions{Title: gongfeng.Ptr(opts.Title)}
	if opts.Body != "" {
		createOpts.Description = gongfeng.Ptr(opts.Body)
	}
	if len(opts.Labels) > 0 {
		createOpts.Labels = gongfeng.Ptr(strings.Join(opts.Labels, ","))
	}
	if opts.Milestone != "" {
		m, err := milestoneNumber("CreateIssue", opts.Milestone)
		if err != nil {
			return nil, err
		}
		createOpts.MilestoneID = gongfeng.Ptr(m)
	}
	issue, _, err := p.client.Issues.CreateIssue(ctx, pid(opts.Owner, opts.Repo), createOpts)
	if err != nil {
		return nil, sdkError("CreateIssue", err)
	}
	return convertIssue(issue), nil
}

// UpdateIssue implements provider.IssueManager. Empty option fields stay
// absent from the PUT body, leaving the issue unchanged; state changes
// travel as 工蜂's state_event verbs. opts.Assignees is ignored (see
// CreateIssue).
func (p *Provider) UpdateIssue(ctx context.Context, owner, repo, number string, opts provider.UpdateIssueOptions) (*provider.Issue, error) {
	n, err := issueNumber("UpdateIssue", number)
	if err != nil {
		return nil, err
	}
	updateOpts := &gongfeng.UpdateIssueOptions{}
	if opts.Title != "" {
		updateOpts.Title = gongfeng.Ptr(opts.Title)
	}
	if opts.Body != "" {
		updateOpts.Description = gongfeng.Ptr(opts.Body)
	}
	if opts.State != "" {
		event := "close"
		if opts.State == provider.IssueStateOpen {
			event = "reopen"
		}
		updateOpts.StateEvent = gongfeng.Ptr(event) // IssueState → state_event verb
	}
	if len(opts.Labels) > 0 {
		updateOpts.Labels = gongfeng.Ptr(strings.Join(opts.Labels, ","))
	}
	if opts.Milestone != "" {
		m, err := milestoneNumber("UpdateIssue", opts.Milestone)
		if err != nil {
			return nil, err
		}
		updateOpts.MilestoneID = gongfeng.Ptr(m)
	}
	issue, _, err := p.client.Issues.UpdateIssue(ctx, pid(owner, repo), n, updateOpts)
	if err != nil {
		return nil, sdkError("UpdateIssue", err)
	}
	return convertIssue(issue), nil
}

// CloseIssue implements provider.IssueManager via the state_event API.
func (p *Provider) CloseIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	n, err := issueNumber("CloseIssue", number)
	if err != nil {
		return nil, err
	}
	issue, _, err := p.client.Issues.UpdateIssue(ctx, pid(owner, repo), n,
		&gongfeng.UpdateIssueOptions{StateEvent: gongfeng.Ptr("close")})
	if err != nil {
		return nil, sdkError("CloseIssue", err)
	}
	return convertIssue(issue), nil
}

// ReopenIssue implements provider.IssueManager via the state_event API.
func (p *Provider) ReopenIssue(ctx context.Context, owner, repo, number string) (*provider.Issue, error) {
	n, err := issueNumber("ReopenIssue", number)
	if err != nil {
		return nil, err
	}
	issue, _, err := p.client.Issues.UpdateIssue(ctx, pid(owner, repo), n,
		&gongfeng.UpdateIssueOptions{StateEvent: gongfeng.Ptr("reopen")})
	if err != nil {
		return nil, sdkError("ReopenIssue", err)
	}
	return convertIssue(issue), nil
}

// ListIssueComments implements provider.IssueManager. 工蜂 models issue
// comments as notes.
func (p *Provider) ListIssueComments(ctx context.Context, owner, repo, number string) ([]*provider.IssueComment, error) {
	n, err := issueNumber("ListIssueComments", number)
	if err != nil {
		return nil, err
	}
	notes, _, err := p.client.Notes.ListIssueNotes(ctx, pid(owner, repo), n, nil)
	if err != nil {
		return nil, sdkError("ListIssueComments", err)
	}
	result := make([]*provider.IssueComment, 0, len(notes))
	for _, note := range notes {
		result = append(result, convertIssueComment(note))
	}
	return result, nil
}

// CreateIssueComment implements provider.IssueManager via the notes API.
func (p *Provider) CreateIssueComment(ctx context.Context, owner, repo, number, body string) (*provider.IssueComment, error) {
	n, err := issueNumber("CreateIssueComment", number)
	if err != nil {
		return nil, err
	}
	note, _, err := p.client.Notes.CreateIssueNote(ctx, pid(owner, repo), n,
		&gongfeng.CreateIssueNoteOptions{Body: gongfeng.Ptr(body)})
	if err != nil {
		return nil, sdkError("CreateIssueComment", err)
	}
	return convertIssueComment(note), nil
}

// ListIssueLabels implements provider.IssueManager: repository-level
// labels (labels.go owns the model conversion; IDs stay zero because the
// gongfeng Label model has none).
func (p *Provider) ListIssueLabels(ctx context.Context, owner, repo string) ([]*provider.IssueLabel, error) {
	labels, _, err := p.client.Labels.ListLabels(ctx, pid(owner, repo),
		&gongfeng.ListLabelsOptions{ListOptions: gongfeng.ListOptions{PerPage: 100}})
	if err != nil {
		return nil, sdkError("ListIssueLabels", err)
	}
	result := make([]*provider.IssueLabel, 0, len(labels))
	for _, l := range labels {
		result = append(result, &provider.IssueLabel{Name: l.Name, Color: strings.TrimPrefix(l.Color, "#")})
	}
	return result, nil
}

// AddIssueLabels implements provider.IssueManager. 工蜂's update surface
// takes the full label set only (no add_labels), so the current set is
// fetched, unioned with the adds, and rewritten.
func (p *Provider) AddIssueLabels(ctx context.Context, owner, repo, number string, labels []string) error {
	n, err := issueNumber("AddIssueLabels", number)
	if err != nil {
		return err
	}
	issue, _, err := p.client.Issues.GetIssue(ctx, pid(owner, repo), n)
	if err != nil {
		return sdkError("AddIssueLabels", err)
	}
	merged := unionLabels(issue.Labels, labels)
	if _, _, err := p.client.Issues.UpdateIssue(ctx, pid(owner, repo), n, labelUpdate(merged)); err != nil {
		return sdkError("AddIssueLabels", err)
	}
	return nil
}

// RemoveIssueLabel implements provider.IssueManager. 工蜂's update surface
// takes the full label set only (no remove_labels), so the current set is
// fetched, the name filtered out, and the remainder rewritten. Registered
// limitation: removing the issue's only label yields an empty csv that
// omitempty drops from the PUT body, so the update is a no-op and the
// label stays.
func (p *Provider) RemoveIssueLabel(ctx context.Context, owner, repo, number, name string) error {
	n, err := issueNumber("RemoveIssueLabel", number)
	if err != nil {
		return err
	}
	issue, _, err := p.client.Issues.GetIssue(ctx, pid(owner, repo), n)
	if err != nil {
		return sdkError("RemoveIssueLabel", err)
	}
	remaining := make([]string, 0, len(issue.Labels))
	for _, l := range issue.Labels {
		if l != name {
			remaining = append(remaining, l)
		}
	}
	if _, _, err := p.client.Issues.UpdateIssue(ctx, pid(owner, repo), n, labelUpdate(remaining)); err != nil {
		return sdkError("RemoveIssueLabel", err)
	}
	return nil
}

// labelUpdate builds the issue update carrying names as the full label
// set. An empty set cannot travel (Labels is a csv string behind
// omitempty), so the field stays nil — the registered last-label-removal
// no-op.
func labelUpdate(names []string) *gongfeng.UpdateIssueOptions {
	updateOpts := &gongfeng.UpdateIssueOptions{}
	if csv := strings.Join(names, ","); csv != "" {
		updateOpts.Labels = gongfeng.Ptr(csv)
	}
	return updateOpts
}

// unionLabels merges existing label names with adds, preserving order and
// dropping duplicates.
func unionLabels(existing, adds []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(adds))
	merged := make([]string, 0, len(existing)+len(adds))
	for _, l := range append(append([]string(nil), existing...), adds...) {
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		merged = append(merged, l)
	}
	return merged
}

// convertIssue maps a gongfeng.Issue to a provider.Issue. Number carries
// the IID (the identifier the write endpoints take); 工蜂 reports state
// "opened" and the SDK normalizes to "open"; MilestoneRef.Number carries
// the milestone ID (registered — the same identifier issue writes take);
// WebURL stays empty (the model has no field; registered).
func convertIssue(i *gongfeng.Issue) *provider.Issue {
	if i == nil {
		return nil
	}
	issue := &provider.Issue{
		ID:     int64(i.ID),
		Number: strconv.Itoa(i.IID),
		Title:  i.Title,
		Body:   i.Description,
		Labels: append([]string(nil), i.Labels...),
		Author: convertUser(i.Author),
	}
	switch i.State {
	case "opened", "reopened":
		issue.State = provider.IssueStateOpen
	case "closed":
		issue.State = provider.IssueStateClosed
	}
	if i.Milestone != nil {
		issue.Milestone = &provider.MilestoneRef{Number: strconv.Itoa(i.Milestone.ID), Title: i.Milestone.Title}
	}
	for _, a := range i.Assignees {
		issue.Assignees = append(issue.Assignees, a.Username)
	}
	issue.CreatedAt, issue.UpdatedAt = i.CreatedAt.Time, i.UpdatedAt.Time
	return issue
}

// convertIssueComment maps a gongfeng.Note to a provider.IssueComment.
func convertIssueComment(n *gongfeng.Note) *provider.IssueComment {
	if n == nil {
		return nil
	}
	return &provider.IssueComment{
		ID:        int64(n.ID),
		Body:      n.Body,
		Author:    convertUser(n.Author),
		CreatedAt: n.CreatedAt.Time,
		UpdatedAt: n.UpdatedAt.Time,
	}
}

var _ provider.IssueManager = (*Provider)(nil)
