package github

import (
	"context"
	"strconv"

	"github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// This file implements provider.MilestoneManager over go-github's
// IssuesService milestone surface. GitHub addresses milestones by their
// per-repo number — the same value MilestoneRef.Number and Milestone.Number
// carry.

// ListMilestones implements provider.MilestoneManager. State filters by
// "open"/"closed" (GitHub also accepts "all"); GitHub defaults to open.
func (p *Provider) ListMilestones(ctx context.Context, owner, repo string, opts provider.ListMilestonesOptions) ([]provider.Milestone, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &github.MilestoneListOptions{
		State:       opts.State,
		ListOptions: github.ListOptions{Page: page, PerPage: perPage},
	}
	milestones, _, err := p.client.Issues.ListMilestones(ctx, owner, repo, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListMilestones", err)
	}
	result := make([]provider.Milestone, 0, len(milestones))
	for _, m := range milestones {
		result = append(result, convertMilestone(m))
	}
	return result, nil
}

// GetMilestone implements provider.MilestoneManager.
func (p *Provider) GetMilestone(ctx context.Context, owner, repo, number string) (*provider.Milestone, error) {
	n, err := issueNumber("GetMilestone", number)
	if err != nil {
		return nil, err
	}
	m, _, err := p.client.Issues.GetMilestone(ctx, owner, repo, n)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "GetMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// CreateMilestone implements provider.MilestoneManager.
func (p *Provider) CreateMilestone(ctx context.Context, owner, repo string, opts provider.CreateMilestoneOptions) (*provider.Milestone, error) {
	createOpts := &github.Milestone{
		Title:       github.Ptr(opts.Title),
		Description: github.Ptr(opts.Description),
	}
	if opts.DueOn != nil {
		createOpts.DueOn = &github.Timestamp{Time: *opts.DueOn}
	}
	m, _, err := p.client.Issues.CreateMilestone(ctx, owner, repo, createOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// UpdateMilestone implements provider.MilestoneManager. Nil fields in opts
// stay absent from the PATCH body, leaving the milestone unchanged.
func (p *Provider) UpdateMilestone(ctx context.Context, owner, repo, number string, opts provider.UpdateMilestoneOptions) (*provider.Milestone, error) {
	n, err := issueNumber("UpdateMilestone", number)
	if err != nil {
		return nil, err
	}
	editOpts := &github.Milestone{}
	if opts.Title != nil {
		editOpts.Title = opts.Title
	}
	if opts.Description != nil {
		editOpts.Description = opts.Description
	}
	if opts.State != "" {
		editOpts.State = github.Ptr(string(opts.State))
	}
	if opts.DueOn != nil {
		editOpts.DueOn = &github.Timestamp{Time: *opts.DueOn}
	}
	m, _, err := p.client.Issues.EditMilestone(ctx, owner, repo, n, editOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "UpdateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// DeleteMilestone implements provider.MilestoneManager.
func (p *Provider) DeleteMilestone(ctx context.Context, owner, repo, number string) error {
	n, err := issueNumber("DeleteMilestone", number)
	if err != nil {
		return err
	}
	if _, err := p.client.Issues.DeleteMilestone(ctx, owner, repo, n); err != nil {
		return provider.Wrap(provider.PlatformGitHub, "DeleteMilestone", err)
	}
	return nil
}

// convertMilestone maps a github.Milestone to a provider.Milestone.
// GitHub's wire states ("open"/"closed") are already the SDK's vocabulary;
// anything else passes through unchanged.
func convertMilestone(m *github.Milestone) provider.Milestone {
	var ms provider.Milestone
	if m == nil {
		return ms
	}
	ms = provider.Milestone{
		Number:      strconv.Itoa(m.GetNumber()),
		Title:       m.GetTitle(),
		Description: m.GetDescription(),
		State:       provider.MilestoneState(m.GetState()),
	}
	if m.DueOn != nil {
		due := m.DueOn.Time
		ms.DueOn = &due
	}
	return ms
}

var _ provider.MilestoneManager = (*Provider)(nil)
