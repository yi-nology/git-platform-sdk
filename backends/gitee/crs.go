package gitee

import (
	"context"
	"strings"

	gitee "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	pr, resp, err := p.client.PullRequestsApi.PostV5ReposOwnerRepoPulls(ctx, esc(opts.Owner), esc(opts.Repo), gitee.CreatePullRequestParam{
		AccessToken:       p.token,
		Title:             opts.Title,
		Head:              opts.SourceBranch,
		Base:              opts.TargetBranch,
		Body:              opts.Description,
		Labels:            strings.Join(opts.Labels, ","),
		PruneSourceBranch: opts.RemoveSourceBranch,
	})
	if err != nil {
		return nil, p.sdkErr("CreateCR", resp, err)
	}
	return convertPullRequest(pr), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	pr, resp, err := p.client.PullRequestsApi.GetV5ReposOwnerRepoPullsNumber(ctx, esc(owner), esc(repo), toInt32(number), &gitee.GetV5ReposOwnerRepoPullsNumberOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return nil, p.sdkErr("GetCR", resp, err)
	}
	return convertPullRequest(pr), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gitee.GetV5ReposOwnerRepoPullsOpts{
		AccessToken: p.accessToken(),
		State:       optional.NewString(mapCRStateForGitee(opts.State)),
		Page:        optional.NewInt32(toInt32(page)),
		PerPage:     optional.NewInt32(toInt32(perPage)),
	}
	if opts.SourceBranch != "" {
		listOpts.Head = optional.NewString(opts.SourceBranch)
	}
	if opts.TargetBranch != "" {
		listOpts.Base = optional.NewString(opts.TargetBranch)
	}
	prs, resp, err := p.client.PullRequestsApi.GetV5ReposOwnerRepoPulls(ctx, esc(opts.Owner), esc(opts.Repo), listOpts)
	if err != nil {
		return nil, 0, p.sdkErr("ListCRs", resp, err)
	}
	result := make([]*provider.ChangeRequest, 0, len(prs))
	for i := range prs {
		result = append(result, convertPullRequest(prs[i]))
	}
	return result, provider.ParseTotalCountHeader(resp.Header, len(result)), nil
}

// MergeCR implements provider.ChangeRequestManager.
//
// The SDK's PutV5ReposOwnerRepoPullsNumberMerge returns no decoded body (the
// live endpoint responds with the merged PR, but the generated method drops
// it), so the merged change request is re-fetched via GetV5ReposOwnerRepoPullsNumber
// right after a successful merge.
func (p *Provider) MergeCR(ctx context.Context, owner, repo string, number int, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	body := gitee.PullRequestMergePutParam{
		AccessToken:       p.token,
		PruneSourceBranch: opts.RemoveSourceBranch,
		Description:       opts.MergeCommitMessage,
	}
	if opts.Squash {
		body.MergeMethod = "squash"
	}
	if resp, err := p.client.PullRequestsApi.PutV5ReposOwnerRepoPullsNumberMerge(ctx, esc(owner), esc(repo), toInt32(number), body); err != nil {
		return nil, p.sdkErr("MergeCR", resp, err)
	}
	return p.GetCR(ctx, owner, repo, number)
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	return p.patchCR(ctx, owner, repo, number, gitee.PullRequestUpdateParam{
		AccessToken: p.token,
		State:       "closed",
	}, "CloseCR")
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	return p.patchCR(ctx, owner, repo, number, gitee.PullRequestUpdateParam{
		AccessToken: p.token,
		State:       "open",
	}, "ReopenCR")
}

// UpdateCR implements provider.ChangeRequestManager.
//
// Registration: UpdateCROptions.TargetBranch has no Gitee counterpart — the
// SDK's PullRequestUpdateParam (mirroring Gitee's PATCH /pulls/{number})
// carries no base/target-branch field, so retargeting a PR is not supported
// by the API and the option is not forwarded.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo string, number int, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	return p.patchCR(ctx, owner, repo, number, gitee.PullRequestUpdateParam{
		AccessToken: p.token,
		Title:       opts.Title,
		Body:        opts.Description,
	}, "UpdateCR")
}

// patchCR applies a PATCH /pulls/{number} update via the SDK and returns the
// updated change request.
func (p *Provider) patchCR(ctx context.Context, owner, repo string, number int, body gitee.PullRequestUpdateParam, op string) (*provider.ChangeRequest, error) {
	pr, resp, err := p.client.PullRequestsApi.PatchV5ReposOwnerRepoPullsNumber(ctx, esc(owner), esc(repo), toInt32(number), body)
	if err != nil {
		return nil, p.sdkErr(op, resp, err)
	}
	return convertPullRequest(pr), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	_, resp, err := p.client.PullRequestsApi.PutV5ReposOwnerRepoPullsNumberLabels(ctx, esc(owner), esc(repo), toInt32(number), gitee.PullRequestLabelPostParam{
		AccessToken: p.token,
		Body:        labels,
	})
	if err != nil {
		return p.sdkErr("UpdateCRLabels", resp, err)
	}
	return nil
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*provider.CRComment, error) {
	page, perPage := provider.NormalizePageOpts(1, 0)
	comments, resp, err := p.client.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberComments(ctx, esc(owner), esc(repo), toInt32(number), &gitee.GetV5ReposOwnerRepoPullsNumberCommentsOpts{
		AccessToken: p.accessToken(),
		Page:        optional.NewInt32(toInt32(page)),
		PerPage:     optional.NewInt32(toInt32(perPage)),
	})
	if err != nil {
		return nil, p.sdkErr("ListCRComments", resp, err)
	}
	result := make([]*provider.CRComment, 0, len(comments))
	for i := range comments {
		result = append(result, convertPRComment(comments[i]))
	}
	return result, nil
}

// ListCRCommits implements provider.ChangeRequestManager.
//
// Note: the SDK's GetV5ReposOwnerRepoPullsNumberCommitsOpts carries only
// AccessToken (no Page/PerPage), so the previous explicit page=1&per_page=20
// query parameters are no longer sent; Gitee applies the same defaults
// server-side.
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*provider.CRCommit, error) {
	commits, resp, err := p.client.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberCommits(ctx, esc(owner), esc(repo), toInt32(number), &gitee.GetV5ReposOwnerRepoPullsNumberCommitsOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return nil, p.sdkErr("ListCRCommits", resp, err)
	}
	result := make([]*provider.CRCommit, 0, len(commits))
	for i := range commits {
		result = append(result, convertPRCommit(commits[i]))
	}
	return result, nil
}

var _ provider.ChangeRequestManager = (*Provider)(nil)

// mapCRStateForGitee maps the SDK CRState to gitee's pull-list vocabulary
// (open/closed/merged/all). Empty defaults to open, matching gitee's API.
func mapCRStateForGitee(s provider.CRState) string {
	switch s {
	case provider.CRStateClosed:
		return "closed"
	case provider.CRStateMerged:
		return "merged"
	default: // CRStateOpened or ""
		return "open"
	}
}
