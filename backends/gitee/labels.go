package gitee

import (
	"context"
	"fmt"
	"strings"

	gitee "gitee.com/openeuler/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListLabels implements provider.LabelManager. Gitee addresses labels by
// name natively.
//
// Routed through the raw transport client rather than the SDK: the SDK's
// GetV5ReposOwnerRepoLabelsOpts exposes only AccessToken — no Page/PerPage —
// so the generated call would silently drop the pagination Gitee's list
// endpoint accepts and the provider contract asserts on the wire. The
// response still decodes into the SDK's gitee.Label model (a subset of the
// live wire shape; extra wire fields are ignored).
func (p *Provider) ListLabels(ctx context.Context, owner, repo string, opts provider.ListLabelsOptions) ([]*provider.Label, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	var labels []gitee.Label
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

// CreateLabel implements provider.LabelManager via the SDK
// (PostV5ReposOwnerRepoLabels posts a JSON body). Registration: Gitee's live
// label object carries no description (id/color/name/repository_id/url
// only), so CreateLabelOptions.Description is a no-op on this platform and
// provider.Label.Description stays empty.
func (p *Provider) CreateLabel(ctx context.Context, owner, repo string, opts provider.CreateLabelOptions) (*provider.Label, error) {
	label, resp, err := p.client.LabelsApi.PostV5ReposOwnerRepoLabels(ctx, esc(owner), esc(repo), gitee.LabelPostParam{
		AccessToken: p.token,
		Name:        opts.Name,
		Color:       opts.Color,
	})
	if err != nil {
		return nil, p.sdkErr("CreateLabel", resp, err)
	}
	return convertLabel(label), nil
}

// UpdateLabel implements provider.LabelManager. Gitee addresses labels by
// name natively.
//
// Routed through the raw transport client rather than the SDK: the generated
// PatchV5ReposOwnerRepoLabelsOriginalName encodes its opts as a multipart
// body while sending an application/json Content-Type header (upstream
// client.go prepareRequest bug — FormDataContentType is only set when a file
// part exists), which the server cannot parse. The raw JSON body keeps the
// documented wire shape. UpdateLabelOptions.Description is a no-op on the
// live API (Gitee labels have no description), matching CreateLabel.
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
	var label gitee.Label
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/labels/%s", esc(owner), esc(repo), esc(name)), body, &label); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateLabel", err)
	}
	return convertLabel(label), nil
}

// DeleteLabel implements provider.LabelManager via the SDK.
func (p *Provider) DeleteLabel(ctx context.Context, owner, repo, name string) error {
	resp, err := p.client.LabelsApi.DeleteV5ReposOwnerRepoLabelsName(ctx, esc(owner), esc(repo), esc(name), &gitee.DeleteV5ReposOwnerRepoLabelsNameOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return p.sdkErr("DeleteLabel", resp, err)
	}
	return nil
}

// convertLabel maps the SDK gitee.Label to a provider.Label. Gitee colors
// are 6-digit hex without '#'; TrimPrefix keeps malformed payloads canonical
// too. The SDK model has no description field — neither does the live wire.
func convertLabel(l gitee.Label) *provider.Label {
	return &provider.Label{
		ID:    int64(l.Id),
		Name:  l.Name,
		Color: strings.TrimPrefix(l.Color, "#"),
	}
}

var _ provider.LabelManager = (*Provider)(nil)
