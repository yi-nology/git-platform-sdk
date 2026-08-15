package forgejo

import (
	"context"
	"strconv"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	pr, _, err := p.client.CreatePullRequest(opts.Owner, opts.Repo, forgejo.CreatePullRequestOption{
		Head:  opts.SourceBranch,
		Base:  opts.TargetBranch,
		Title: opts.Title,
		Body:  opts.Description,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "CreateCR", err)
	}
	return convertPR(pr), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	pr, _, err := p.client.GetPullRequest(owner, repo, int64(number))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "GetCR", err)
	}
	return convertPR(pr), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	opts.Page, opts.PerPage = provider.NormalizePageOpts(opts.Page, opts.PerPage)
	prs, resp, err := p.client.ListRepoPullRequests(opts.Owner, opts.Repo, forgejo.ListPullRequestsOptions{
		State:       forgejo.StateType(opts.State),
		ListOptions: forgejo.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
	})
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformForgejo, "ListCRs", err)
	}
	crs := make([]*provider.ChangeRequest, 0, len(prs))
	for _, pr := range prs {
		crs = append(crs, convertPR(pr))
	}
	total := parseTotalCount(resp)
	if total < len(crs) {
		total = len(crs)
	}
	return crs, total, nil
}

// MergeCR implements provider.ChangeRequestManager.
func (p *Provider) MergeCR(ctx context.Context, owner, repo string, number int, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	style := forgejo.MergeStyleMerge
	if opts.Squash {
		style = forgejo.MergeStyleSquash
	}
	deleteBranch := opts.RemoveSourceBranch
	_, resp, err := p.client.MergePullRequest(owner, repo, int64(number), forgejo.MergePullRequestOption{
		Style:                  style,
		Title:                  opts.MergeCommitMessage,
		DeleteBranchAfterMerge: deleteBranch,
	})
	if err != nil {
		// Forgejo returns 405 when the PR is already merged; recover gracefully.
		if resp != nil && resp.StatusCode == 405 {
			cr, getErr := p.GetCR(ctx, owner, repo, number)
			if getErr == nil && cr.State == provider.CRStateMerged {
				return cr, nil
			}
		}
		return nil, provider.Wrap(provider.PlatformForgejo, "MergeCR", err)
	}
	return p.GetCR(ctx, owner, repo, number)
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	state := forgejo.StateClosed
	_, _, err := p.client.EditPullRequest(owner, repo, int64(number), forgejo.EditPullRequestOption{State: &state})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "CloseCR", err)
	}
	return p.GetCR(ctx, owner, repo, number)
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	state := forgejo.StateOpen
	pr, _, err := p.client.EditPullRequest(owner, repo, int64(number), forgejo.EditPullRequestOption{State: &state})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ReopenCR", err)
	}
	return convertPR(pr), nil
}

// UpdateCR implements provider.ChangeRequestManager.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo string, number int, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	editOpts := forgejo.EditPullRequestOption{}
	if opts.Title != "" {
		editOpts.Title = opts.Title
	}
	if opts.Description != "" {
		editOpts.Body = &opts.Description
	}
	if opts.TargetBranch != "" {
		editOpts.Base = opts.TargetBranch
	}
	pr, _, err := p.client.EditPullRequest(owner, repo, int64(number), editOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "UpdateCR", err)
	}
	return convertPR(pr), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	labelIDs := make([]int64, 0, len(labels))
	for _, l := range labels {
		if id, err := strconv.ParseInt(l, 10, 64); err == nil {
			labelIDs = append(labelIDs, id)
		}
	}
	_, _, err := p.client.AddIssueLabels(owner, repo, int64(number), forgejo.IssueLabelsOption{Labels: labelIDs})
	if err != nil {
		return provider.Wrap(provider.PlatformForgejo, "UpdateCRLabels", err)
	}
	return nil
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*provider.CRComment, error) {
	comments, _, err := p.client.ListIssueComments(owner, repo, int64(number), forgejo.ListIssueCommentOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListCRComments", err)
	}
	result := make([]*provider.CRComment, 0, len(comments))
	for _, c := range comments {
		cc := &provider.CRComment{ID: c.ID, Body: c.Body, CreatedAt: c.Created, UpdatedAt: c.Updated}
		if c.Poster != nil {
			cc.Author = &provider.CRUser{ID: c.Poster.ID, Username: c.Poster.UserName, AvatarURL: c.Poster.AvatarURL}
		}
		result = append(result, cc)
	}
	return result, nil
}

// ListCRCommits implements provider.ChangeRequestManager.
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*provider.CRCommit, error) {
	commits, _, err := p.client.ListPullRequestCommits(owner, repo, int64(number), forgejo.ListPullRequestCommitsOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListCRCommits", err)
	}
	result := make([]*provider.CRCommit, 0, len(commits))
	for _, c := range commits {
		sha := ""
		if c.CommitMeta != nil {
			sha = c.CommitMeta.SHA
		}
		cc := &provider.CRCommit{SHA: sha}
		if c.RepoCommit != nil {
			cc.Message = c.RepoCommit.Message
			if c.RepoCommit.Author != nil {
				cc.Author = &provider.CRUser{Name: c.RepoCommit.Author.Name}
			}
		}
		result = append(result, cc)
	}
	return result, nil
}

var _ provider.ChangeRequestManager = (*Provider)(nil)
