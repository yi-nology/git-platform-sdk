package gitee

import (
	"context"

	gitee "github.com/next-bin/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"

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
		return nil, provider.Wrap(provider.PlatformGitee, "ListMilestones", err)
	}
	result := make([]provider.Milestone, 0, len(milestones))
	for _, m := range milestones {
		result = append(result, convertMilestone(m))
	}
	return result, nil
}

// GetMilestone implements provider.MilestoneManager via the SDK.
func (p *Provider) GetMilestone(ctx context.Context, owner, repo, number string) (*provider.Milestone, error) {
	n64, err := backendutil.ParseMilestoneNumber(provider.PlatformGitee, "GetMilestone", number)
	if err != nil {
		return nil, err
	}
	m, _, err := p.client.Milestones.Get(ctx, esc(owner), esc(repo), int(n64))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetMilestone", err)
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
		return nil, provider.Wrap(provider.PlatformGitee, "CreateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// UpdateMilestone implements provider.MilestoneManager.
func (p *Provider) UpdateMilestone(ctx context.Context, owner, repo, number string, opts provider.UpdateMilestoneOptions) (*provider.Milestone, error) {
	n64, err := backendutil.ParseMilestoneNumber(provider.PlatformGitee, "UpdateMilestone", number)
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
	m, _, err := p.client.Milestones.Edit(ctx, esc(owner), esc(repo), int(n64), updateOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateMilestone", err)
	}
	ms := convertMilestone(m)
	return &ms, nil
}

// DeleteMilestone implements provider.MilestoneManager.
func (p *Provider) DeleteMilestone(ctx context.Context, owner, repo, number string) error {
	n64, err := backendutil.ParseMilestoneNumber(provider.PlatformGitee, "DeleteMilestone", number)
	if err != nil {
		return err
	}
	_, err = p.client.Milestones.Delete(ctx, esc(owner), esc(repo), int(n64))
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteMilestone", err)
	}
	return nil
}

var _ provider.MilestoneManager = (*Provider)(nil)
