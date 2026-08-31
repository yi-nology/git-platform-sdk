package github

import (
	"context"

	"github.com/google/go-github/v72/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListDeployKeys implements provider.DeploymentKeyManager.
func (p *Provider) ListDeployKeys(ctx context.Context, owner, repo string) ([]*provider.DeployKey, error) {
	keys, _, err := p.client.Repositories.ListKeys(ctx, owner, repo, &github.ListOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListDeployKeys", err)
	}
	result := make([]*provider.DeployKey, 0, len(keys))
	for _, k := range keys {
		result = append(result, convertDeployKey(k))
	}
	return result, nil
}

// AddDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) AddDeployKey(ctx context.Context, owner, repo string, opts provider.AddDeployKeyOptions) (*provider.DeployKey, error) {
	key := &github.Key{
		Title:    github.Ptr(opts.Title),
		Key:      github.Ptr(opts.Key),
		ReadOnly: github.Ptr(opts.ReadOnly),
	}
	created, _, err := p.client.Repositories.CreateKey(ctx, owner, repo, key)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "AddDeployKey", err)
	}
	return convertDeployKey(created), nil
}

// DeleteDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) DeleteDeployKey(ctx context.Context, owner, repo string, keyID int64) error {
	if _, err := p.client.Repositories.DeleteKey(ctx, owner, repo, keyID); err != nil {
		return provider.Wrap(provider.PlatformGitHub, "DeleteDeployKey", err)
	}
	return nil
}

// convertDeployKey maps a github.Key to a provider.DeployKey.
func convertDeployKey(k *github.Key) *provider.DeployKey {
	if k == nil {
		return nil
	}
	return &provider.DeployKey{
		ID:       k.GetID(),
		Title:    k.GetTitle(),
		Key:      k.GetKey(),
		ReadOnly: k.GetReadOnly(),
	}
}

var _ provider.DeploymentKeyManager = (*Provider)(nil)
