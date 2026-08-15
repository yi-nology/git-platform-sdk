package gitlab

import (
	"context"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	c, _, err := p.client.Commits.GetCommit(pidOf(owner, repo), sha, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "GetCommit", err)
	}
	return convertCommit(c), nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gitlab.ListCommitsOptions{
		ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: int64(perPage)},
	}
	if opts.Branch != "" {
		listOpts.RefName = gitlab.Ptr(opts.Branch)
	}
	if opts.Since != "" {
		if t, err := time.Parse(time.RFC3339, opts.Since); err == nil {
			listOpts.Since = gitlab.Ptr(t)
		}
	}
	if opts.Until != "" {
		if t, err := time.Parse(time.RFC3339, opts.Until); err == nil {
			listOpts.Until = gitlab.Ptr(t)
		}
	}
	commits, _, err := p.client.Commits.ListCommits(pidOf(owner, repo), listOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListCommits", err)
	}
	result := make([]*provider.CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, convertCommit(c))
	}
	return result, nil
}

// CompareCommits implements provider.CommitManager.
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	cmp, _, err := p.client.Repositories.Compare(pidOf(owner, repo),
		&gitlab.CompareOptions{From: gitlab.Ptr(base), To: gitlab.Ptr(head)},
		gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CompareCommits", err)
	}
	result := &provider.CompareResult{}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, convertCommit(c))
		result.TotalCommits++
	}
	for _, d := range cmp.Diffs {
		add, del := provider.CountDiffLines(d.Diff)
		result.Files = append(result.Files, &provider.ChangedFile{
			OldPath: d.OldPath, NewPath: d.NewPath, Diff: d.Diff,
			Additions: add, Deletions: del,
			IsNew: d.NewFile, IsDeleted: d.DeletedFile, IsRenamed: d.RenamedFile,
		})
	}
	return result, nil
}

// CreateCommitStatus implements provider.CommitManager.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	statusOpts := &gitlab.SetCommitStatusOptions{
		State:       mapCommitState(opts.State),
		Context:     gitlab.Ptr(opts.Context),
		Description: gitlab.Ptr(opts.Description),
		TargetURL:   gitlab.Ptr(opts.TargetURL),
	}
	_, _, err := p.client.Commits.SetCommitStatus(pidOf(owner, repo), sha, statusOpts, gitlab.WithContext(ctx))
	if err != nil {
		return provider.Wrap(provider.PlatformGitLab, "CreateCommitStatus", err)
	}
	return nil
}

var _ provider.CommitManager = (*Provider)(nil)
