package gitcode

import (
	"context"

	gitcode "github.com/yi-nology/go-gitcode"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListDeployKeys implements provider.DeploymentKeyManager.
func (p *Provider) ListDeployKeys(ctx context.Context, owner, repo string) ([]*provider.DeployKey, error) {
	keys, err := p.client.ListDeployKeys(ctx, owner, repo, gitcode.ListOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListDeployKeys", err)
	}
	result := make([]*provider.DeployKey, 0, len(keys))
	for _, k := range keys {
		result = append(result, convertDeployKey(k))
	}
	return result, nil
}

// AddDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) AddDeployKey(ctx context.Context, owner, repo string, opts provider.AddDeployKeyOptions) (*provider.DeployKey, error) {
	readOnly := opts.ReadOnly
	key, err := p.client.CreateDeployKey(ctx, owner, repo, gitcode.CreateDeployKeyOptions{
		Title:    opts.Title,
		Key:      opts.Key,
		ReadOnly: &readOnly,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "AddDeployKey", err)
	}
	return convertDeployKey(key), nil
}

// DeleteDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) DeleteDeployKey(ctx context.Context, owner, repo string, keyID int64) error {
	if err := p.client.DeleteDeployKey(ctx, owner, repo, keyID); err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteDeployKey", err)
	}
	return nil
}

// convertDeployKey maps a gitcode.DeployKey to a provider.DeployKey.
func convertDeployKey(k *gitcode.DeployKey) *provider.DeployKey {
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
