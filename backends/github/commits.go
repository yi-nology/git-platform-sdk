package github

import (
	"context"
	"time"

	"github.com/google/go-github/v72/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	c, _, err := p.client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "GetCommit", err)
	}
	return convertCommit(c), nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &github.CommitsListOptions{
		ListOptions: github.ListOptions{Page: page, PerPage: perPage},
	}
	if opts.Branch != "" {
		listOpts.SHA = opts.Branch
	}
	if opts.Since != "" {
		if t, err := time.Parse(time.RFC3339, opts.Since); err == nil {
			listOpts.Since = t
		}
	}
	if opts.Until != "" {
		if t, err := time.Parse(time.RFC3339, opts.Until); err == nil {
			listOpts.Until = t
		}
	}
	commits, _, err := p.client.Repositories.ListCommits(ctx, owner, repo, listOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListCommits", err)
	}
	result := make([]*provider.CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, convertCommit(c))
	}
	return result, nil
}

// CompareCommits implements provider.CommitManager.
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	cmp, _, err := p.client.Repositories.CompareCommits(ctx, owner, repo, base, head, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CompareCommits", err)
	}
	result := &provider.CompareResult{
		TotalCommits: cmp.GetTotalCommits(),
		AheadBy:      cmp.GetAheadBy(),
		BehindBy:     cmp.GetBehindBy(),
	}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, convertCommit(c))
	}
	for _, f := range cmp.Files {
		result.Files = append(result.Files, &provider.ChangedFile{
			OldPath:   f.GetPreviousFilename(),
			NewPath:   f.GetFilename(),
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
			IsNew:     f.GetStatus() == "added",
			IsDeleted: f.GetStatus() == "removed",
			IsRenamed: f.GetStatus() == "renamed",
		})
	}
	return result, nil
}

// CreateCommitStatus implements provider.CommitStatusManager.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	status := &github.RepoStatus{
		State:       github.Ptr(opts.State),
		Context:     github.Ptr(opts.Context),
		Description: github.Ptr(opts.Description),
		TargetURL:   github.Ptr(opts.TargetURL),
	}
	_, _, err := p.client.Repositories.CreateStatus(ctx, owner, repo, sha, status)
	if err != nil {
		return provider.Wrap(provider.PlatformGitHub, "CreateCommitStatus", err)
	}
	return nil
}

// compile-time guard
var _ provider.CommitManager = (*Provider)(nil)
var _ provider.CommitStatusManager = (*Provider)(nil)
