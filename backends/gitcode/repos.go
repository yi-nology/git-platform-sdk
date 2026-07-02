package gitcode

import (
	"context"
	"fmt"
	"strconv"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListRepos implements provider.RepoManager.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	repos, err := p.client.ListRepositories(ctx, gitcode.ListRepositoriesOptions{
		ListOptions: gitcode.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
		Owner:       opts.Owner,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListRepos", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(repos))
	for _, r := range repos {
		owner := ""
		if r.Owner != nil {
			owner = r.Owner.Login
		}
		result = append(result, &provider.PlatformRepo{
			ID:            r.ID,
			FullName:      r.FullName,
			Name:          r.Name,
			Owner:         owner,
			Description:   r.Description,
			CloneURL:      r.CloneURL,
			SSHURL:        r.SSHURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
			Platform:      provider.PlatformGitCode,
		})
	}
	return result, nil
}

// GetRepo implements provider.RepoManager.
func (p *Provider) GetRepo(ctx context.Context, owner, repo string) (*provider.PlatformRepo, error) {
	r, err := p.client.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetRepo", err)
	}
	ownerName := ""
	if r.Owner != nil {
		ownerName = r.Owner.Login
	}
	return &provider.PlatformRepo{
		ID:            r.ID,
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         ownerName,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      provider.PlatformGitCode,
	}, nil
}

// ForkRepo implements provider.RepoManager.
func (p *Provider) ForkRepo(ctx context.Context, owner, repo string, opts provider.ForkRepoOptions) (*provider.PlatformRepo, error) {
	r, err := p.client.ForkRepository(ctx, owner, repo, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ForkRepo", err)
	}
	ownerName := ""
	if r.Owner != nil {
		ownerName = r.Owner.Login
	}
	return &provider.PlatformRepo{
		ID:            r.ID,
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         ownerName,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      provider.PlatformGitCode,
	}, nil
}

// DeleteRepo implements provider.RepoManager.
func (p *Provider) DeleteRepo(ctx context.Context, owner, repo string) error {
	err := p.client.DeleteRepository(ctx, owner, repo)
	if err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteRepo", err)
	}
	return nil
}

// UpdateRepo implements provider.RepoManager.
func (p *Provider) UpdateRepo(ctx context.Context, owner, repo string, opts provider.UpdateRepoOptions) (*provider.PlatformRepo, error) {
	r, err := p.client.UpdateRepository(ctx, owner, repo, gitcode.UpdateRepositoryOptions{
		Name:          opts.Name,
		Description:   opts.Description,
		DefaultBranch: opts.DefaultBranch,
		Private:       opts.Private,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateRepo", err)
	}
	ownerName := ""
	if r.Owner != nil {
		ownerName = r.Owner.Login
	}
	return &provider.PlatformRepo{
		ID:            r.ID,
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         ownerName,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      provider.PlatformGitCode,
	}, nil
}

// ensure fmt + strconv are referenced to silence unused-import warnings
// when the build excludes certain paths.
var _ = fmt.Sprintf
var _ = strconv.Atoi

var _ provider.RepoManager = (*Provider)(nil)