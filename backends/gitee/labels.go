package gitee

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// giteeLabel mirrors Gitee's v5 label JSON shape.
type giteeLabel struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
}

// ListLabels implements provider.LabelManager. Gitee addresses labels by
// name natively.
func (p *Provider) ListLabels(ctx context.Context, owner, repo string, opts provider.ListLabelsOptions) ([]*provider.Label, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	var labels []giteeLabel
	path := fmt.Sprintf("/repos/%s/%s/labels?page=%d&per_page=%d", esc(owner), esc(repo), page, perPage)
	if err := p.doRequest(ctx, "GET", path, nil, &labels); err != nil {
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
	body := map[string]any{"name": opts.Name, "color": opts.Color}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	var label giteeLabel
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/labels", esc(owner), esc(repo)), body, &label); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateLabel", err)
	}
	return convertLabel(label), nil
}

// UpdateLabel implements provider.LabelManager. Gitee addresses labels by
// name natively.
func (p *Provider) UpdateLabel(ctx context.Context, owner, repo, name string, opts provider.UpdateLabelOptions) (*provider.Label, error) {
	body := map[string]any{}
	if opts.NewName != nil {
		body["name"] = *opts.NewName
	}
	if opts.Color != nil {
		body["color"] = *opts.Color
	}
	if opts.Description != nil {
		body["description"] = *opts.Description
	}
	var label giteeLabel
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/labels/%s", esc(owner), esc(repo), esc(name)), body, &label); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateLabel", err)
	}
	return convertLabel(label), nil
}

// DeleteLabel implements provider.LabelManager.
func (p *Provider) DeleteLabel(ctx context.Context, owner, repo, name string) error {
	if err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/labels/%s", esc(owner), esc(repo), esc(name)), nil, nil); err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteLabel", err)
	}
	return nil
}

// convertLabel maps a giteeLabel to a provider.Label. Gitee colors are
// 6-digit hex without '#'; TrimPrefix keeps malformed payloads canonical too.
func convertLabel(l giteeLabel) *provider.Label {
	return &provider.Label{
		ID:          l.ID,
		Name:        l.Name,
		Color:       strings.TrimPrefix(l.Color, "#"),
		Description: l.Description,
	}
}

var _ provider.LabelManager = (*Provider)(nil)
