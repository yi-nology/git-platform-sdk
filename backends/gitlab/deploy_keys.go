package gitlab

import (
	"context"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListDeployKeys implements provider.DeploymentKeyManager.
func (p *Provider) ListDeployKeys(ctx context.Context, owner, repo string) ([]*provider.DeployKey, error) {
	keys, _, err := p.client.DeployKeys.ListProjectDeployKeys(pidOf(owner, repo), &gitlab.ListProjectDeployKeysOptions{}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListDeployKeys", err)
	}
	result := make([]*provider.DeployKey, 0, len(keys))
	for _, k := range keys {
		result = append(result, convertDeployKey(k))
	}
	return result, nil
}

// AddDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) AddDeployKey(ctx context.Context, owner, repo string, opts provider.AddDeployKeyOptions) (*provider.DeployKey, error) {
	// GitLab uses CanPush (true = read-write); provider uses ReadOnly (true =
	// read-only), so the mapping is ReadOnly = !CanPush.
	canPush := !opts.ReadOnly
	createOpts := &gitlab.AddDeployKeyOptions{
		Key:     gitlab.Ptr(opts.Key),
		Title:   gitlab.Ptr(opts.Title),
		CanPush: gitlab.Ptr(canPush),
	}
	key, _, err := p.client.DeployKeys.AddDeployKey(pidOf(owner, repo), createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "AddDeployKey", err)
	}
	return convertDeployKey(key), nil
}

// DeleteDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) DeleteDeployKey(ctx context.Context, owner, repo string, keyID int64) error {
	if _, err := p.client.DeployKeys.DeleteDeployKey(pidOf(owner, repo), keyID, gitlab.WithContext(ctx)); err != nil {
		return provider.Wrap(provider.PlatformGitLab, "DeleteDeployKey", err)
	}
	return nil
}

// convertDeployKey maps a gitlab.ProjectDeployKey to a provider.DeployKey.
// GitLab's CanPush is the inverse of provider's ReadOnly.
func convertDeployKey(k *gitlab.ProjectDeployKey) *provider.DeployKey {
	if k == nil {
		return nil
	}
	return &provider.DeployKey{
		ID:       k.ID,
		Title:    k.Title,
		Key:      k.Key,
		ReadOnly: !k.CanPush,
	}
}

var _ provider.DeploymentKeyManager = (*Provider)(nil)
