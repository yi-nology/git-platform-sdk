package gitee

import (
	"context"
	"strconv"

	gitee "github.com/next-bin/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListMilestones implements provider.MilestoneManager via the SDK.
func (p *Provider) ListMilestones(ctx context.Context, owner, repo string, opts provider.ListMilestonesOptions) ([]provider.Milestone, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gitee.MilestoneListOptions{
		Page:    gitee.Int(page),
		PerPage: gitee.Int(perPage),
	}
	if opts.State != "" {
		listOpts.State = gitee.String(opts.State)
	}
	milestones, _, err := p.client.Milestones.List(ctx, esc(owner), esc(repo), listOpts)
	if err != nil {
		return nil, p.sdkErr("ListMilestones", err)
	}
	result := make([]provider.Milestone, 0, len(milestones))
	for _, m := range milestones {
		result = append(result, convertMilestone(m))
	}
	return result, nil
}

// GetMilestone implements provider.MilestoneManager via the SDK.
func (p *Provider) GetMilestone(ctx context.Context, owner, repo, number string) (*provider.Milestone, error) {
	n, err := milestoneSerial("GetMilestone", number)
	if err != nil {
		return nil, err
	}
	m, _, err := p.client.Milestones.Get(ctx, esc(owner), esc(repo), n)
	if err != nil {
		return nil, p.sdkErr("GetMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// CreateMilestone implements provider.MilestoneManager.
func (p *Provider) CreateMilestone(ctx context.Context, owner, repo string, opts provider.CreateMilestoneOptions) (*provider.Milestone, error) {
	createOpts := &gitee.CreateMilestoneOptions{
		Title: gitee.String(opts.Title),
	}
	if opts.Description != "" {
		createOpts.Description = gitee.String(opts.Description)
	}
	if opts.DueOn != nil {
		createOpts.DueOn = gitee.String(formatGiteeDueOn(*opts.DueOn))
	}
	m, _, err := p.client.Milestones.Create(ctx, esc(owner), esc(repo), createOpts)
	if err != nil {
		return nil, p.sdkErr("CreateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// UpdateMilestone implements provider.MilestoneManager.
func (p *Provider) UpdateMilestone(ctx context.Context, owner, repo, number string, opts provider.UpdateMilestoneOptions) (*provider.Milestone, error) {
	n, err := milestoneSerial("UpdateMilestone", number)
	if err != nil {
		return nil, err
	}
	updateOpts := &gitee.UpdateMilestoneOptions{}
	if opts.Title != nil {
		updateOpts.Title = gitee.String(*opts.Title)
	}
	if opts.Description != nil {
		updateOpts.Description = gitee.String(*opts.Description)
	}
	if opts.State != "" {
		updateOpts.State = gitee.String(string(opts.State))
	}
	if opts.DueOn != nil {
		updateOpts.DueOn = gitee.String(formatGiteeDueOn(*opts.DueOn))
	}
	m, _, err := p.client.Milestones.Edit(ctx, esc(owner), esc(repo), n, updateOpts)
	if err != nil {
		return nil, p.sdkErr("UpdateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// DeleteMilestone implements provider.MilestoneManager.
func (p *Provider) DeleteMilestone(ctx context.Context, owner, repo, number string) error {
	n, err := milestoneSerial("DeleteMilestone", number)
	if err != nil {
		return err
	}
	_, err = p.client.Milestones.Delete(ctx, esc(owner), esc(repo), n)
	if err != nil {
		return p.sdkErr("DeleteMilestone", err)
	}
	return nil
}

// milestoneSerial parses the string milestone identifier into an int.
func milestoneSerial(op, number string) (int, error) {
	n, err := strconv.Atoi(number)
	if err != nil {
		return 0, provider.Wrapf(provider.PlatformGitee, op, "invalid milestone number %q", number)
	}
	return n, nil
}

var _ provider.MilestoneManager = (*Provider)(nil)
