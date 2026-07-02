package github

import (
	"context"

	"github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	newPR := &github.NewPullRequest{
		Title: github.String(opts.Title),
		Body:  github.String(opts.Description),
		Head:  github.String(opts.SourceBranch),
		Base:  github.String(opts.TargetBranch),
	}
	pr, _, err := p.client.PullRequests.Create(ctx, opts.Owner, opts.Repo, newPR)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateCR", err)
	}
	return convertPR(pr), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	pr, _, err := p.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "GetCR", err)
	}
	return convertPR(pr), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{Page: page, PerPage: perPage},
	}
	if opts.State != "" {
		listOpts.State = mapCRState(opts.State)
	}
	if opts.SourceBranch != "" {
		listOpts.Head = opts.Owner + ":" + opts.SourceBranch
	}
	if opts.TargetBranch != "" {
		listOpts.Base = opts.TargetBranch
	}
	prs, resp, err := p.client.PullRequests.List(ctx, opts.Owner, opts.Repo, listOpts)
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitHub, "ListCRs", err)
	}
	crs := make([]*provider.ChangeRequest, 0, len(prs))
	for _, pr := range prs {
		crs = append(crs, convertPR(pr))
	}
	var total int
	if resp != nil && resp.LastPage > 0 {
		total = resp.LastPage * listOpts.PerPage
	} else {
		total = len(crs)
	}
	return crs, total, nil
}

// MergeCR implements provider.ChangeRequestManager.
func (p *Provider) MergeCR(ctx context.Context, owner, repo string, number int, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	mergeOpts := &github.PullRequestOptions{}
	if opts.Squash {
		mergeOpts.MergeMethod = "squash"
	}
	_, _, err := p.client.PullRequests.Merge(ctx, owner, repo, number, opts.MergeCommitMessage, mergeOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "MergeCR", err)
	}
	return p.GetCR(ctx, owner, repo, number)
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	pr, _, err := p.client.PullRequests.Edit(ctx, owner, repo, number, &github.PullRequest{
		State: github.String("closed"),
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CloseCR", err)
	}
	return convertPR(pr), nil
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	result, _, err := p.client.PullRequests.Edit(ctx, owner, repo, number, &github.PullRequest{
		State: github.String("open"),
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ReopenCR", err)
	}
	return convertPR(result), nil
}

// UpdateCR implements provider.ChangeRequestManager.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo string, number int, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	pr := &github.PullRequest{}
	if opts.Title != "" {
		pr.Title = github.String(opts.Title)
	}
	if opts.Description != "" {
		pr.Body = github.String(opts.Description)
	}
	if opts.TargetBranch != "" {
		pr.Base = &github.PullRequestBranch{Ref: github.String(opts.TargetBranch)}
	}
	result, _, err := p.client.PullRequests.Edit(ctx, owner, repo, number, pr)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "UpdateCR", err)
	}
	return convertPR(result), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	_, _, err := p.client.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels)
	if err != nil {
		return provider.Wrap(provider.PlatformGitHub, "UpdateCRLabels", err)
	}
	return nil
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*provider.CRComment, error) {
	comments, _, err := p.client.PullRequests.ListComments(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListCRComments", err)
	}
	result := make([]*provider.CRComment, 0, len(comments))
	for _, c := range comments {
		cc := &provider.CRComment{
			ID:        c.GetID(),
			Body:      c.GetBody(),
			CreatedAt: tsOrZero(c.GetCreatedAt()),
			UpdatedAt: tsOrZero(c.GetUpdatedAt()),
		}
		if c.GetUser() != nil {
			cc.Author = convertUser(c.GetUser())
		}
		result = append(result, cc)
	}
	return result, nil
}

// ListCRCommits implements provider.ChangeRequestManager.
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*provider.CRCommit, error) {
	commits, _, err := p.client.PullRequests.ListCommits(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListCRCommits", err)
	}
	result := make([]*provider.CRCommit, 0, len(commits))
	for _, c := range commits {
		cc := &provider.CRCommit{
			SHA:       c.GetSHA(),
			Message:   c.GetCommit().GetMessage(),
			CreatedAt: c.GetCommit().GetAuthor().GetDate().Time,
		}
		if c.GetAuthor() != nil {
			cc.Author = convertUser(c.GetAuthor())
		}
		result = append(result, cc)
	}
	return result, nil
}

// compile-time guard
var _ provider.ChangeRequestManager = (*Provider)(nil)