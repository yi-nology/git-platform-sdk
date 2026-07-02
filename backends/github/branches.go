package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListBranches implements provider.BranchManager.
func (p *Provider) ListBranches(ctx context.Context, owner, repo string) ([]*provider.PlatformBranch, error) {
	branches, _, err := p.client.Repositories.ListBranches(ctx, owner, repo, &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListBranches", err)
	}
	result := make([]*provider.PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, convertBranch(b))
	}
	return result, nil
}

// CreateBranch implements provider.BranchManager.
func (p *Provider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*provider.PlatformBranch, error) {
	sha := ref
	if !isCommitSHA(ref) {
		commits, err := p.ListCommits(ctx, owner, repo, provider.ListCommitsOptions{Branch: ref, PerPage: 1})
		if err != nil {
			return nil, fmt.Errorf("github: resolve ref %q: %w", ref, err)
		}
		if len(commits) == 0 {
			return nil, fmt.Errorf("github: no commits found on ref %q", ref)
		}
		sha = commits[0].SHA
	}
	_, _, err := p.client.Git.CreateRef(ctx, owner, repo, &github.Reference{
		Ref: github.String("refs/heads/" + branch),
		Object: &github.GitObject{
			SHA: github.String(sha),
		},
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateBranch", err)
	}
	return &provider.PlatformBranch{Name: branch}, nil
}

// DeleteBranch implements provider.BranchManager.
func (p *Provider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	_, err := p.client.Git.DeleteRef(ctx, owner, repo, "heads/"+branch)
	if err != nil {
		return provider.Wrap(provider.PlatformGitHub, "DeleteBranch", err)
	}
	return nil
}

// isCommitSHA reports whether s looks like a 40-char hex SHA. It is also
// used by the createBranch flow to disambiguate branch refs from commit refs.
func isCommitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// compile-time guard
var _ provider.BranchManager = (*Provider)(nil)