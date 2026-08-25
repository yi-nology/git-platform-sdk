package gitlab

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListLabels implements provider.LabelManager.
func (p *Provider) ListLabels(ctx context.Context, owner, repo string, opts provider.ListLabelsOptions) ([]*provider.Label, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	labels, _, err := p.client.Labels.ListLabels(pidOf(owner, repo),
		&gitlab.ListLabelsOptions{ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: int64(perPage)}},
		gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListLabels", err)
	}
	result := make([]*provider.Label, 0, len(labels))
	for _, l := range labels {
		result = append(result, convertLabel(l))
	}
	return result, nil
}

// CreateLabel implements provider.LabelManager. GitLab requires colors with
// a leading '#'.
func (p *Provider) CreateLabel(ctx context.Context, owner, repo string, opts provider.CreateLabelOptions) (*provider.Label, error) {
	createOpts := &gitlab.CreateLabelOptions{
		Name:  gitlab.Ptr(opts.Name),
		Color: gitlab.Ptr("#" + opts.Color),
	}
	if opts.Description != "" {
		createOpts.Description = gitlab.Ptr(opts.Description)
	}
	label, _, err := p.client.Labels.CreateLabel(pidOf(owner, repo), createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateLabel", err)
	}
	return convertLabel(label), nil
}

// UpdateLabel implements provider.LabelManager. GitLab addresses labels by
// numeric ID, so the label is resolved by name first.
func (p *Provider) UpdateLabel(ctx context.Context, owner, repo, name string, opts provider.UpdateLabelOptions) (*provider.Label, error) {
	id, err := p.resolveLabelID(ctx, "UpdateLabel", owner, repo, name)
	if err != nil {
		return nil, err
	}
	updateOpts := &gitlab.UpdateLabelOptions{}
	if opts.NewName != nil {
		updateOpts.NewName = opts.NewName
	}
	if opts.Color != nil {
		updateOpts.Color = gitlab.Ptr("#" + *opts.Color)
	}
	if opts.Description != nil {
		updateOpts.Description = opts.Description
	}
	label, _, err := p.client.Labels.UpdateLabel(pidOf(owner, repo), id, updateOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "UpdateLabel", err)
	}
	return convertLabel(label), nil
}

// DeleteLabel implements provider.LabelManager.
func (p *Provider) DeleteLabel(ctx context.Context, owner, repo, name string) error {
	id, err := p.resolveLabelID(ctx, "DeleteLabel", owner, repo, name)
	if err != nil {
		return err
	}
	if _, err := p.client.Labels.DeleteLabel(pidOf(owner, repo), id, nil, gitlab.WithContext(ctx)); err != nil {
		return provider.Wrap(provider.PlatformGitLab, "DeleteLabel", err)
	}
	return nil
}

// resolveLabelID finds the numeric ID of the named label via the shared
// paginated resolver with a per-provider TTL cache. GitLab's update and
// delete endpoints address labels by ID while the SDK's surface addresses
// them by name. Exhausting the 50-page budget surfaces a scan-limit error
// (distinct from a definitive 404). op is the public operation the
// resolution serves; failures surface under that op.
func (p *Provider) resolveLabelID(ctx context.Context, op, owner, repo, name string) (int64, error) {
	id, err := p.labelIDs.ResolveLabel(owner+"/"+repo, name, func(page, perPage int) ([]backendutil.LabelRef, error) {
		labels, _, err := p.client.Labels.ListLabels(pidOf(owner, repo),
			&gitlab.ListLabelsOptions{ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: int64(perPage)}},
			gitlab.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		refs := make([]backendutil.LabelRef, len(labels))
		for i, l := range labels {
			refs[i] = backendutil.LabelRef{ID: l.ID, Name: l.Name}
		}
		return refs, nil
	}, 50, 100)
	if err != nil {
		if errors.Is(err, backendutil.ErrLabelScanLimit) {
			msg := fmt.Sprintf("label %q not found within 50 pages (scan limit)", name)
			return 0, provider.Wrap(provider.PlatformGitLab, op, backendutil.NewScanLimitError(msg))
		}
		return 0, provider.Wrap(provider.PlatformGitLab, op, err)
	}
	return id, nil
}

// convertLabel maps a gitlab.Label to a provider.Label. GitLab colors carry
// a leading '#', which is stripped for the SDK's canonical form.
func convertLabel(l *gitlab.Label) *provider.Label {
	if l == nil {
		return nil
	}
	return &provider.Label{
		ID:          l.ID,
		Name:        l.Name,
		Color:       strings.TrimPrefix(l.Color, "#"),
		Description: l.Description,
	}
}

var _ provider.LabelManager = (*Provider)(nil)
