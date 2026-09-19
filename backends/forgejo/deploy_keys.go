package forgejo

import (
	"context"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListDeployKeys implements provider.DeploymentKeyManager. The provider
// surface carries no pagination parameters, so the full key list is
// fetched by exhausting the endpoint's pagination (backendutil.AllPages).
//
// The forgejo SDK accepts no context parameter (registered platform
// limitation), so ctx is unused beyond signature conformance.
func (p *Provider) ListDeployKeys(ctx context.Context, owner, repo string) ([]*provider.DeployKey, error) {
	keys, err := backendutil.AllPages(func(page int) ([]*forgejo.DeployKey, error) {
		list, _, err := p.client.ListDeployKeys(owner, repo, forgejo.ListDeployKeysOptions{
			ListOptions: forgejo.ListOptions{Page: page, PageSize: listPageSize},
		})
		return list, err
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListDeployKeys", err)
	}
	result := make([]*provider.DeployKey, 0, len(keys))
	for _, k := range keys {
		result = append(result, convertDeployKey(k))
	}
	return result, nil
}

// AddDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) AddDeployKey(ctx context.Context, owner, repo string, opts provider.AddDeployKeyOptions) (*provider.DeployKey, error) {
	key, _, err := p.client.CreateDeployKey(owner, repo, forgejo.CreateKeyOption{
		Title:    opts.Title,
		Key:      opts.Key,
		ReadOnly: opts.ReadOnly,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "AddDeployKey", err)
	}
	return convertDeployKey(key), nil
}

// DeleteDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) DeleteDeployKey(ctx context.Context, owner, repo string, keyID int64) error {
	if _, err := p.client.DeleteDeployKey(owner, repo, keyID); err != nil {
		return provider.Wrap(provider.PlatformForgejo, "DeleteDeployKey", err)
	}
	return nil
}

// convertDeployKey maps a forgejo.DeployKey to a provider.DeployKey.
func convertDeployKey(k *forgejo.DeployKey) *provider.DeployKey {
	if k == nil {
		return nil
	}
	return &provider.DeployKey{
		ID:       k.ID,
		Title:    k.Title,
		Key:      k.Key,
		ReadOnly: k.ReadOnly,
	}
}

var _ provider.DeploymentKeyManager = (*Provider)(nil)
