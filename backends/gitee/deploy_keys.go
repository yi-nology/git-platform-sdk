package gitee

import (
	"context"

	gitee "github.com/next-bin/go-gitee/gitee"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListDeployKeys implements provider.DeploymentKeyManager.
func (p *Provider) ListDeployKeys(ctx context.Context, owner, repo string) ([]*provider.DeployKey, error) {
	keys, _, err := p.client.Repositories.ListKeys(ctx, esc(owner), esc(repo), &gitee.ListOptions{})
	if err != nil {
		return nil, p.sdkErr("ListDeployKeys", err)
	}
	result := make([]*provider.DeployKey, 0, len(keys))
	for _, k := range keys {
		result = append(result, convertSSHKey(k))
	}
	return result, nil
}

// AddDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) AddDeployKey(ctx context.Context, owner, repo string, opts provider.AddDeployKeyOptions) (*provider.DeployKey, error) {
	key, _, err := p.client.Repositories.CreateKey(ctx, esc(owner), esc(repo), &gitee.CreateKeyOptions{
		Title: gitee.String(opts.Title),
		Key:   gitee.String(opts.Key),
	})
	if err != nil {
		return nil, p.sdkErr("AddDeployKey", err)
	}
	return convertSSHKey(key), nil
}

// DeleteDeployKey implements provider.DeploymentKeyManager.
func (p *Provider) DeleteDeployKey(ctx context.Context, owner, repo string, keyID int64) error {
	_, err := p.client.Repositories.DeleteKey(ctx, esc(owner), esc(repo), keyID)
	return p.sdkErr("DeleteDeployKey", err)
}

func convertSSHKey(k *gitee.SSHKey) *provider.DeployKey {
	if k == nil {
		return nil
	}
	return &provider.DeployKey{
		ID:    int64(deref(k.ID)),
		Title: deref(k.Title),
		Key:   deref(k.Key),
	}
}

var _ provider.DeploymentKeyManager = (*Provider)(nil)
