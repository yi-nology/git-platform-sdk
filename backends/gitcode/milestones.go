package gitcode

import (
	"context"
	"strconv"
	"time"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// This file implements provider.MilestoneManager over gitcode_api's
// milestones surface. GitCode addresses milestones by their ID (the wire's
// "id" — the SDK's Get/Update/Delete path parameter, whatever its
// "number" spelling) — the same value MilestoneRef.Number and
// Milestone.Number carry on this platform. Wire states are
// "open"/"closed", already the SDK's vocabulary.

// ListMilestones implements provider.MilestoneManager. State filters by
// "open"/"closed" (GitCode also accepts "all").
func (p *Provider) ListMilestones(ctx context.Context, owner, repo string, opts provider.ListMilestonesOptions) ([]provider.Milestone, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := gitcode.ListMilestonesOptions{
		ListOptions: gitcode.ListOptions{Page: page, PerPage: perPage},
		State:       opts.State,
	}
	milestones, err := p.client.ListMilestonesWithOptions(ctx, owner, repo, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListMilestones", err)
	}
	result := make([]provider.Milestone, 0, len(milestones))
	for _, m := range milestones {
		result = append(result, convertMilestone(m))
	}
	return result, nil
}

// GetMilestone implements provider.MilestoneManager.
func (p *Provider) GetMilestone(ctx context.Context, owner, repo, number string) (*provider.Milestone, error) {
	id, err := issueNumber("GetMilestone", number)
	if err != nil {
		return nil, err
	}
	m, err := p.client.GetMilestone(ctx, owner, repo, id)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// CreateMilestone implements provider.MilestoneManager.
func (p *Provider) CreateMilestone(ctx context.Context, owner, repo string, opts provider.CreateMilestoneOptions) (*provider.Milestone, error) {
	createOpts := gitcode.CreateMilestoneOptions{
		Title:       opts.Title,
		Description: opts.Description,
	}
	if opts.DueOn != nil {
		createOpts.DueOn = opts.DueOn.Format(time.RFC3339)
	}
	m, err := p.client.CreateMilestoneWithOptions(ctx, owner, repo, createOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CreateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// UpdateMilestone implements provider.MilestoneManager. Fields the caller
// left nil stay zero-valued; the SDK options marshal description/state/
// due_on with omitempty so they drop off the wire (Title always marshals —
// an update that does not rename sends the empty string, which GitCode's
// GitHub-shaped API treats as "leave unchanged").
func (p *Provider) UpdateMilestone(ctx context.Context, owner, repo, number string, opts provider.UpdateMilestoneOptions) (*provider.Milestone, error) {
	id, err := issueNumber("UpdateMilestone", number)
	if err != nil {
		return nil, err
	}
	updateOpts := gitcode.UpdateMilestoneOptions{}
	if opts.Title != nil {
		updateOpts.Title = *opts.Title
	}
	if opts.Description != nil {
		updateOpts.Description = *opts.Description
	}
	if opts.State != "" {
		updateOpts.State = string(opts.State)
	}
	if opts.DueOn != nil {
		updateOpts.DueOn = opts.DueOn.Format(time.RFC3339)
	}
	m, err := p.client.UpdateMilestone(ctx, owner, repo, id, updateOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// DeleteMilestone implements provider.MilestoneManager.
func (p *Provider) DeleteMilestone(ctx context.Context, owner, repo, number string) error {
	id, err := issueNumber("DeleteMilestone", number)
	if err != nil {
		return err
	}
	if err := p.client.DeleteMilestone(ctx, owner, repo, id); err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteMilestone", err)
	}
	return nil
}

// convertMilestone maps a gitcode.Milestone to a provider.Milestone.
// Number carries the GitCode milestone ID (the identifier the write
// endpoints take).
func convertMilestone(m *gitcode.Milestone) provider.Milestone {
	var ms provider.Milestone
	if m == nil {
		return ms
	}
	ms = provider.Milestone{
		Number:      strconv.FormatInt(m.ID, 10),
		Title:       m.Title,
		Description: m.Description,
		State:       provider.MilestoneState(m.State),
		DueOn:       m.DueDate,
	}
	return ms
}

var _ provider.MilestoneManager = (*Provider)(nil)
