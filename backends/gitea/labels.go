package gitea

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListLabels implements provider.LabelManager.
func (p *Provider) ListLabels(ctx context.Context, owner, repo string, opts provider.ListLabelsOptions) ([]*provider.Label, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	labels, _, err := p.client.ListRepoLabels(owner, repo, gitea.ListLabelsOptions{
		ListOptions: gitea.ListOptions{Page: page, PageSize: perPage},
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListLabels", err)
	}
	result := make([]*provider.Label, 0, len(labels))
	for _, l := range labels {
		result = append(result, convertLabel(l))
	}
	return result, nil
}

// CreateLabel implements provider.LabelManager. Gitea accepts colors with or
// without a leading '#'; we send the '#'-prefixed form.
func (p *Provider) CreateLabel(ctx context.Context, owner, repo string, opts provider.CreateLabelOptions) (*provider.Label, error) {
	label, _, err := p.client.CreateLabel(owner, repo, gitea.CreateLabelOption{
		Name:        opts.Name,
		Color:       "#" + opts.Color,
		Description: opts.Description,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateLabel", err)
	}
	return convertLabel(label), nil
}

// UpdateLabel implements provider.LabelManager. Gitea addresses labels by
// numeric ID, so the label is resolved by name first.
func (p *Provider) UpdateLabel(ctx context.Context, owner, repo, name string, opts provider.UpdateLabelOptions) (*provider.Label, error) {
	id, err := p.resolveLabelID("UpdateLabel", owner, repo, name)
	if err != nil {
		return nil, err
	}
	edit := gitea.EditLabelOption{}
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
		return nil, provider.Wrap(provider.PlatformGitea, "UpdateLabel", err)
	}
	return convertLabel(label), nil
}

// DeleteLabel implements provider.LabelManager.
func (p *Provider) DeleteLabel(ctx context.Context, owner, repo, name string) error {
	id, err := p.resolveLabelID("DeleteLabel", owner, repo, name)
	if err != nil {
		return err
	}
	if _, err := p.client.DeleteLabel(owner, repo, id); err != nil {
		return provider.Wrap(provider.PlatformGitea, "DeleteLabel", err)
	}
	return nil
}

// resolveLabelID finds the numeric ID of the named label via the shared
// paginated resolver with a per-provider TTL cache. Gitea's update and
// delete endpoints address labels by ID while the SDK's surface addresses
// them by name. Exhausting the 50-page budget surfaces a scan-limit error
// (distinct from a definitive 404). op is the public operation the
// resolution serves; failures surface under that op rather than under this
// unexported helper's name. The gitea SDK's label methods accept no
// context, so this helper carries none either.
func (p *Provider) resolveLabelID(op, owner, repo, name string) (int64, error) {
	id, err := p.labelIDs.ResolveLabel(owner+"/"+repo, name, func(page, perPage int) ([]backendutil.LabelRef, error) {
		labels, _, err := p.client.ListRepoLabels(owner, repo, gitea.ListLabelsOptions{
			ListOptions: gitea.ListOptions{Page: page, PageSize: perPage},
		})
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
			return 0, provider.Wrap(provider.PlatformGitea, op, backendutil.NewScanLimitError(msg))
		}
		return 0, provider.Wrap(provider.PlatformGitea, op, err)
	}
	return id, nil
}

// convertLabel maps a gitea.Label to a provider.Label. Gitea colors may
// carry a leading '#'; it is stripped for the SDK's canonical form.
func convertLabel(l *gitea.Label) *provider.Label {
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
