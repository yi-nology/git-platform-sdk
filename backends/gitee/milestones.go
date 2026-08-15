package gitee

import (
	"context"
	"fmt"
	"strconv"
	"time"

	gitee "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// This file implements provider.MilestoneManager over the go-gitee SDK's
// milestones surface. Gitee addresses milestones by their serial number
// (the "number" field of the milestone payload — the SDK model exposes no
// id), the same value MilestoneRef.Number and Milestone.Number carry on
// this platform and Gitee's issue write endpoints take.
//
// CreateMilestone and UpdateMilestone are routed through the raw transport
// client rather than the SDK: the generated Post/Patch calls encode their
// parameters as form values while sending an application/json Content-Type
// header (upstream client.go prepareRequest bug — FormDataContentType is
// only set when a file part exists; same family as the labels patch,
// issue create, and releases create detours), which the server cannot
// parse. The raw JSON bodies keep the documented wire shape.

// ListMilestones implements provider.MilestoneManager via the SDK (the
// generated GET carries its filters as query params, which the server
// reads fine). State filters by "open"/"closed" (Gitee also accepts
// "all"); Gitee defaults to open.
func (p *Provider) ListMilestones(ctx context.Context, owner, repo string, opts provider.ListMilestonesOptions) ([]provider.Milestone, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := gitee.GetV5ReposOwnerRepoMilestonesOpts{
		AccessToken: p.accessToken(),
		Page:        optional.NewInt32(toInt32(page)),
		PerPage:     optional.NewInt32(toInt32(perPage)),
	}
	if opts.State != "" {
		listOpts.State = optional.NewString(opts.State)
	}
	milestones, resp, err := p.client.MilestonesApi.GetV5ReposOwnerRepoMilestones(ctx, esc(owner), esc(repo), &listOpts)
	if err != nil {
		return nil, p.sdkErr("ListMilestones", resp, err)
	}
	result := make([]provider.Milestone, 0, len(milestones))
	for i := range milestones {
		result = append(result, convertMilestone(milestones[i]))
	}
	return result, nil
}

// GetMilestone implements provider.MilestoneManager via the SDK.
func (p *Provider) GetMilestone(ctx context.Context, owner, repo, number string) (*provider.Milestone, error) {
	n, err := milestoneSerial("GetMilestone", number)
	if err != nil {
		return nil, err
	}
	m, resp, err := p.client.MilestonesApi.GetV5ReposOwnerRepoMilestonesNumber(ctx, esc(owner), esc(repo), n, &gitee.GetV5ReposOwnerRepoMilestonesNumberOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return nil, p.sdkErr("GetMilestone", resp, err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// CreateMilestone implements provider.MilestoneManager through the raw
// transport client (SDK multipart bug — see the file comment).
func (p *Provider) CreateMilestone(ctx context.Context, owner, repo string, opts provider.CreateMilestoneOptions) (*provider.Milestone, error) {
	body := map[string]any{"title": opts.Title}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.DueOn != nil {
		body["due_on"] = formatGiteeDueOn(*opts.DueOn)
	}
	var m gitee.Milestone
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/milestones", esc(owner), esc(repo)), body, &m); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// UpdateMilestone implements provider.MilestoneManager through the raw
// transport client (SDK multipart bug — see the file comment). Fields the
// caller left nil stay out of the JSON body, leaving the milestone
// unchanged.
func (p *Provider) UpdateMilestone(ctx context.Context, owner, repo, number string, opts provider.UpdateMilestoneOptions) (*provider.Milestone, error) {
	n, err := milestoneSerial("UpdateMilestone", number)
	if err != nil {
		return nil, err
	}
	body := map[string]any{}
	if opts.Title != nil {
		body["title"] = *opts.Title
	}
	if opts.Description != nil {
		body["description"] = *opts.Description
	}
	if opts.State != "" {
		body["state"] = string(opts.State)
	}
	if opts.DueOn != nil {
		body["due_on"] = formatGiteeDueOn(*opts.DueOn)
	}
	var m gitee.Milestone
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/milestones/%d", esc(owner), esc(repo), n), body, &m); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// DeleteMilestone implements provider.MilestoneManager via the SDK (the
// generated DELETE carries no body).
func (p *Provider) DeleteMilestone(ctx context.Context, owner, repo, number string) error {
	n, err := milestoneSerial("DeleteMilestone", number)
	if err != nil {
		return err
	}
	resp, err := p.client.MilestonesApi.DeleteV5ReposOwnerRepoMilestonesNumber(ctx, esc(owner), esc(repo), n, &gitee.DeleteV5ReposOwnerRepoMilestonesNumberOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return p.sdkErr("DeleteMilestone", resp, err)
	}
	return nil
}

// milestoneSerial parses the SDK's string milestone identifier (Gitee's
// milestone serial number) into the SDK's int32 form. op is the public
// operation the parse serves; failures surface under it.
func milestoneSerial(op, number string) (int32, error) {
	n, err := strconv.ParseInt(number, 10, 32)
	if err != nil {
		return 0, provider.Wrapf(provider.PlatformGitee, op, "invalid milestone number %q", number)
	}
	return int32(n), nil // #nosec:G115 -- ParseInt already bounded to 32 bits
}

// giteeDueOnLayouts are the timestamp layouts Gitee's milestone payload
// uses for due_on, most structured first. Unparseable/empty values stay
// unset, matching time.Time decoding for absent fields.
var giteeDueOnLayouts = []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}

// parseGiteeDueOn parses the SDK model's string due_on into a time.Time,
// trying each layout in turn. The second return is false when the value is
// empty or matches no layout.
func parseGiteeDueOn(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range giteeDueOnLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// formatGiteeDueOn renders a due date for Gitee's write endpoints, which
// accept the same date/datetime strings the payload returns.
func formatGiteeDueOn(t time.Time) string { return t.Format(time.RFC3339) }

// convertMilestone maps the SDK gitee.Milestone to a provider.Milestone.
// Number carries Gitee's milestone serial number (the SDK model exposes no
// id), and the string due_on parses through the layouts above.
func convertMilestone(m gitee.Milestone) provider.Milestone {
	ms := provider.Milestone{
		Number:      strconv.Itoa(int(m.Number)),
		Title:       m.Title,
		Description: m.Description,
		State:       provider.MilestoneState(m.State),
	}
	if due, ok := parseGiteeDueOn(m.DueOn); ok {
		ms.DueOn = &due
	}
	return ms
}

var _ provider.MilestoneManager = (*Provider)(nil)
