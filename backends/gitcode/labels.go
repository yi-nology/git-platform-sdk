package gitcode

import (
	"context"
	"strings"

	"github.com/yi-nology/git-platform-sdk/provider"
	gitcode "github.com/yi-nology/gitcode_api"
)

// ListLabels implements provider.LabelManager. GitCode's list endpoint does
// not paginate, so opts are accepted but ignored.
func (p *Provider) ListLabels(ctx context.Context, owner, repo string, opts provider.ListLabelsOptions) ([]*provider.Label, error) {
	labels, err := p.client.ListIssueLabels(ctx, owner, repo)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListLabels", err)
	}
	result := make([]*provider.Label, 0, len(labels))
	for _, l := range labels {
		result = append(result, convertLabel(l))
	}
	return result, nil
}

// CreateLabel implements provider.LabelManager. GitCode's label API has no
// description field; opts.Description is ignored.
func (p *Provider) CreateLabel(ctx context.Context, owner, repo string, opts provider.CreateLabelOptions) (*provider.Label, error) {
	label, err := p.client.CreateIssueLabel(ctx, owner, repo, opts.Name, opts.Color)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CreateLabel", err)
	}
	return convertLabel(label), nil
}

// UpdateLabel implements provider.LabelManager. GitCode addresses labels by
// name natively. Its label API has no description field; opts.Description
// is ignored.
func (p *Provider) UpdateLabel(ctx context.Context, owner, repo, name string, opts provider.UpdateLabelOptions) (*provider.Label, error) {
	updateOpts := gitcode.UpdateLabelOptions{}
	if opts.NewName != nil {
		updateOpts.Name = *opts.NewName
	}
	if opts.Color != nil {
		// GitCode's label API uses '#'-prefixed colors on the wire (docs:
		// create "eg: #fff", update responses show "#ED4014") — the same
		// form gitcode_api's create path sends. The public SDK contract
		// stays '#' free; only the wire form carries it. TrimPrefix keeps a
		// caller-supplied '#' from doubling.
		updateOpts.Color = "#" + strings.TrimPrefix(*opts.Color, "#")
	}
	label, err := p.client.UpdateIssueLabel(ctx, owner, repo, name, updateOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateLabel", err)
	}
	return convertLabel(label), nil
}

// DeleteLabel implements provider.LabelManager.
func (p *Provider) DeleteLabel(ctx context.Context, owner, repo, name string) error {
	if err := p.client.DeleteIssueLabel(ctx, owner, repo, name); err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteLabel", err)
	}
	return nil
}

// convertLabel maps a gitcode.Label to a provider.Label. GitCode labels have
// no description, so Description is always empty.
func convertLabel(l *gitcode.Label) *provider.Label {
	if l == nil {
		return nil
	}
	return &provider.Label{
		ID:    l.ID,
		Name:  l.Name,
		Color: strings.TrimPrefix(l.Color, "#"),
	}
}

var _ provider.LabelManager = (*Provider)(nil)
