package gitea

import (
	"context"
	"strconv"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// prNumber parses the SDK's string change-request number into gitea's
// int64 index form. op is the public operation the parse serves; failures
// surface under it.
func prNumber(op, number string) (int64, error) {
	n, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return 0, provider.Wrapf(provider.PlatformGitea, op, "invalid pull request number %q", number)
	}
	return n, nil
}

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	pr, _, err := p.client.CreatePullRequest(opts.Owner, opts.Repo, gitea.CreatePullRequestOption{
		Head:  opts.SourceBranch,
		Base:  opts.TargetBranch,
		Title: opts.Title,
		Body:  opts.Description,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateCR", err)
	}
	return convertPR(pr), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo, number string) (*provider.ChangeRequest, error) {
	n, err := prNumber("GetCR", number)
	if err != nil {
		return nil, err
	}
	pr, _, err := p.client.GetPullRequest(owner, repo, n)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetCR", err)
	}
	return convertPR(pr), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	opts.Page, opts.PerPage = provider.NormalizePageOpts(opts.Page, opts.PerPage)
	prs, resp, err := p.client.ListRepoPullRequests(opts.Owner, opts.Repo, gitea.ListPullRequestsOptions{
		State:       gitea.StateType(opts.State),
		ListOptions: gitea.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
	})
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitea, "ListCRs", err)
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
func (p *Provider) MergeCR(ctx context.Context, owner, repo, number string, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	n, err := prNumber("MergeCR", number)
	if err != nil {
		return nil, err
	}
	style := gitea.MergeStyleMerge
	if opts.Squash {
		style = gitea.MergeStyleSquash
	}
	deleteBranch := opts.RemoveSourceBranch
	_, resp, err := p.client.MergePullRequest(owner, repo, n, gitea.MergePullRequestOption{
		Style:                  style,
		Title:                  opts.MergeCommitMessage,
		DeleteBranchAfterMerge: &deleteBranch,
	})
	if err != nil {
		// Gitea returns 405 when the PR is already merged; recover gracefully.
		if resp != nil && resp.StatusCode == 405 {
			cr, getErr := p.GetCR(ctx, owner, repo, number)
			if getErr == nil && cr.State == provider.CRStateMerged {
				return cr, nil
			}
		}
		return nil, provider.Wrap(provider.PlatformGitea, "MergeCR", err)
	}
	return p.GetCR(ctx, owner, repo, number)
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo, number string) (*provider.ChangeRequest, error) {
	n, err := prNumber("CloseCR", number)
	if err != nil {
		return nil, err
	}
	state := gitea.StateClosed
	_, _, err = p.client.EditPullRequest(owner, repo, n, gitea.EditPullRequestOption{State: &state})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CloseCR", err)
	}
	return p.GetCR(ctx, owner, repo, number)
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo, number string) (*provider.ChangeRequest, error) {
	n, err := prNumber("ReopenCR", number)
	if err != nil {
		return nil, err
	}
	state := gitea.StateOpen
	pr, _, err := p.client.EditPullRequest(owner, repo, n, gitea.EditPullRequestOption{State: &state})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ReopenCR", err)
	}
	return convertPR(pr), nil
}

// UpdateCR implements provider.ChangeRequestManager.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo, number string, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	n, err := prNumber("UpdateCR", number)
	if err != nil {
		return nil, err
	}
	editOpts := gitea.EditPullRequestOption{}
	if opts.Title != "" {
		editOpts.Title = opts.Title
	}
	if opts.Description != "" {
		editOpts.Body = &opts.Description
	}
	if opts.TargetBranch != "" {
		editOpts.Base = opts.TargetBranch
	}
	pr, _, err := p.client.EditPullRequest(owner, repo, n, editOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "UpdateCR", err)
	}
	return convertPR(pr), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo, number string, labels []string) error {
	n, err := prNumber("UpdateCRLabels", number)
	if err != nil {
		return err
	}
	labelIDs := make([]int64, 0, len(labels))
	for _, l := range labels {
		if id, err := strconv.ParseInt(l, 10, 64); err == nil {
			labelIDs = append(labelIDs, id)
		}
	}
	_, _, err = p.client.AddIssueLabels(owner, repo, n, gitea.IssueLabelsOption{Labels: labelIDs})
	if err != nil {
		return provider.Wrap(provider.PlatformGitea, "UpdateCRLabels", err)
	}
	return nil
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo, number string) ([]*provider.CRComment, error) {
	n, err := prNumber("ListCRComments", number)
	if err != nil {
		return nil, err
	}
	comments, _, err := p.client.ListIssueComments(owner, repo, n, gitea.ListIssueCommentOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListCRComments", err)
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
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo, number string) ([]*provider.CRCommit, error) {
	n, err := prNumber("ListCRCommits", number)
	if err != nil {
		return nil, err
	}
	commits, _, err := p.client.ListPullRequestCommits(owner, repo, n, gitea.ListPullRequestCommitsOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListCRCommits", err)
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
