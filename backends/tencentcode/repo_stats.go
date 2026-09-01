package tencentcode

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListForks implements provider.RepoStatsManager.
//
// TencentCode does not expose a forks list endpoint; this method
// always returns ErrNotImplemented.
func (p *Provider) ListForks(_ context.Context, _, _ string) ([]*provider.PlatformRepo, error) {
	return nil, provider.Wrapf(provider.PlatformTencentCode, "ListForks", "not supported on this platform")
}

// ListStargazers implements provider.RepoStatsManager.
func (p *Provider) ListStargazers(ctx context.Context, owner, repo string) ([]*provider.CRUser, error) {
	stars, _, err := p.client.Projects.ListProjectStars(ctx, pid(owner, repo), nil)
	if err != nil {
		return nil, sdkError("ListStargazers", err)
	}
	result := make([]*provider.CRUser, 0, len(stars))
	for _, s := range stars {
		if s.User != nil {
			result = append(result, &provider.CRUser{
				ID:        int64(s.User.ID),
				Username:  s.User.Username,
				Name:      s.User.Name,
				AvatarURL: s.User.AvatarURL,
			})
		}
	}
	return result, nil
}

// ListContributors implements provider.RepoStatsManager.
//
// TencentCode does not expose a contributors list endpoint; this method
// always returns ErrNotImplemented.
func (p *Provider) ListContributors(_ context.Context, _, _ string) ([]*provider.Contributor, error) {
	return nil, provider.Wrapf(provider.PlatformTencentCode, "ListContributors", "not supported on this platform")
}

var _ provider.RepoStatsManager = (*Provider)(nil)
