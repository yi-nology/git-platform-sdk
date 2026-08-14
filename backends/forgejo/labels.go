package forgejo

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListLabels implements provider.LabelManager.
func (p *Provider) ListLabels(ctx context.Context, owner, repo string, opts provider.ListLabelsOptions) ([]*provider.Label, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	labels, _, err := p.client.ListRepoLabels(owner, repo, forgejo.ListLabelsOptions{
		ListOptions: forgejo.ListOptions{Page: page, PageSize: perPage},
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListLabels", err)
	}
	result := make([]*provider.Label, 0, len(labels))
	for _, l := range labels {
		result = append(result, convertLabel(l))
	}
	return result, nil
}

// CreateLabel implements provider.LabelManager. Forgejo accepts colors with
// or without a leading '#'; we send the '#'-prefixed form.
func (p *Provider) CreateLabel(ctx context.Context, owner, repo string, opts provider.CreateLabelOptions) (*provider.Label, error) {
	label, _, err := p.client.CreateLabel(owner, repo, forgejo.CreateLabelOption{
		Name:        opts.Name,
		Color:       "#" + opts.Color,
		Description: opts.Description,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "CreateLabel", err)
	}
	return convertLabel(label), nil
}

// UpdateLabel implements provider.LabelManager. Forgejo addresses labels by
// numeric ID, so the label is resolved by name first.
func (p *Provider) UpdateLabel(ctx context.Context, owner, repo, name string, opts provider.UpdateLabelOptions) (*provider.Label, error) {
	id, err := p.resolveLabelID(owner, repo, name)
	if err != nil {
		return nil, err
	}
	edit := forgejo.EditLabelOption{}
	if opts.NewName != nil {
		edit.Name = opts.NewName
	}
	if opts.Color != nil {
		c := "#" + *opts.Color
		edit.Color = &c
	}
	if opts.Description != nil {
		edit.Description = opts.Description
	}
	label, _, err := p.client.EditLabel(owner, repo, id, edit)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "UpdateLabel", err)
	}
	return convertLabel(label), nil
}

// DeleteLabel implements provider.LabelManager.
func (p *Provider) DeleteLabel(ctx context.Context, owner, repo, name string) error {
	id, err := p.resolveLabelID(owner, repo, name)
	if err != nil {
		return err
	}
	if _, err := p.client.DeleteLabel(owner, repo, id); err != nil {
		return provider.Wrap(provider.PlatformForgejo, "DeleteLabel", err)
	}
	return nil
}

// resolveLabelID finds the numeric ID of the named label. Forgejo's update
// and delete endpoints address labels by ID while the SDK's surface
// addresses them by name. Only the first 100 labels are scanned; repositories
// with more labels than that are not supported by this resolution path yet.
// The forgejo SDK offers no per-call context, so no ctx parameter is taken
// (unlike the GitLab backend's equivalent helper).
func (p *Provider) resolveLabelID(owner, repo, name string) (int64, error) {
	labels, _, err := p.client.ListRepoLabels(owner, repo, forgejo.ListLabelsOptions{
		ListOptions: forgejo.ListOptions{PageSize: 100},
	})
	if err != nil {
		return 0, provider.Wrap(provider.PlatformForgejo, "resolveLabelID", err)
	}
	for _, l := range labels {
		if l.Name == name {
			return l.ID, nil
		}
	}
	return 0, provider.New(provider.PlatformForgejo, "resolveLabelID", http.StatusNotFound, fmt.Sprintf("label %q not found", name))
}

// convertLabel maps a forgejo.Label to a provider.Label. Forgejo colors may
// carry a leading '#'; it is stripped for the SDK's canonical form.
func convertLabel(l *forgejo.Label) *provider.Label {
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
