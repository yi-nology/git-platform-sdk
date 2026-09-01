package gitee

import (
	"context"

	gitee "github.com/next-bin/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListLabels implements provider.LabelManager.
func (p *Provider) ListLabels(ctx context.Context, owner, repo string, opts provider.ListLabelsOptions) ([]*provider.Label, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gitee.ListOptions{
		Page:    page,
		PerPage: perPage,
	}
	labels, _, err := p.client.Labels.List(ctx, esc(owner), esc(repo), listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListLabels", err)
	}
	result := make([]*provider.Label, 0, len(labels))
	for _, l := range labels {
		result = append(result, convertLabel(l))
	}
	return result, nil
}

// CreateLabel implements provider.LabelManager.
func (p *Provider) CreateLabel(ctx context.Context, owner, repo string, opts provider.CreateLabelOptions) (*provider.Label, error) {
	createOpts := &gitee.CreateLabelOptions{
		Name:  gitee.String(opts.Name),
		Color: gitee.String(opts.Color),
	}
	label, _, err := p.client.Labels.Create(ctx, esc(owner), esc(repo), createOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateLabel", err)
	}
	return convertLabel(label), nil
}

// UpdateLabel implements provider.LabelManager. Gitee addresses labels by name.
func (p *Provider) UpdateLabel(ctx context.Context, owner, repo, name string, opts provider.UpdateLabelOptions) (*provider.Label, error) {
	updateOpts := &gitee.UpdateLabelOptions{}
	if opts.NewName != nil {
		updateOpts.Name = gitee.String(*opts.NewName)
	}
	if opts.Color != nil {
		updateOpts.Color = gitee.String(*opts.Color)
	}
	label, _, err := p.client.Labels.Edit(ctx, esc(owner), esc(repo), esc(name), updateOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateLabel", err)
	}
	return convertLabel(label), nil
}

// DeleteLabel implements provider.LabelManager.
func (p *Provider) DeleteLabel(ctx context.Context, owner, repo, name string) error {
	_, err := p.client.Labels.Delete(ctx, esc(owner), esc(repo), esc(name))
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteLabel", err)
	}
	return nil
}

var _ provider.LabelManager = (*Provider)(nil)
