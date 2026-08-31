package forgejo

import (
	"context"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListDeployKeys implements provider.DeploymentKeyManager.
func (p *Provider) ListDeployKeys(ctx context.Context, owner, repo string) ([]*provider.DeployKey, error) {
	keys, _, err := p.client.ListDeployKeys(owner, repo, forgejo.ListDeployKeysOptions{})
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
