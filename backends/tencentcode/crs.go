package tencentcode

import (
	"context"
	"strconv"

	gongfeng "github.com/studyzy/gongfeng-sdk-go"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// prNumber parses the SDK's string change-request number into gongfeng's
// int merge-request IID form. op is the public operation the parse serves;
// failures surface under it.
func prNumber(op, number string) (int, error) {
	n, err := strconv.Atoi(number)
	if err != nil {
		return 0, provider.Wrapf(provider.PlatformTencentCode, op, "invalid pull request number %q", number)
	}
	return n, nil
}

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	pid := opts.Owner + "/" + opts.Repo
	createOpts := &gongfeng.CreateMergeRequestOptions{
		SourceBranch: gongfeng.Ptr(opts.SourceBranch),
		TargetBranch: gongfeng.Ptr(opts.TargetBranch),
		Title:        gongfeng.Ptr(opts.Title),
	}
	if opts.Description != "" {
		createOpts.Description = gongfeng.Ptr(opts.Description)
	}
	mr, _, err := p.client.MergeRequests.CreateMergeRequest(ctx, pid, createOpts)
	if err != nil {
		return nil, sdkError("CreateCR", err)
	}
	return convertMR(mr), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo, number string) (*provider.ChangeRequest, error) {
	n, err := prNumber("GetCR", number)
	if err != nil {
		return nil, err
	}
	pid := owner + "/" + repo
	mr, _, err := p.client.MergeRequests.GetMergeRequest(ctx, pid, n)
	if err != nil {
		return nil, sdkError("GetCR", err)
	}
	return convertMR(mr), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	pid := opts.Owner + "/" + opts.Repo
	opts.Page, opts.PerPage = provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gongfeng.ListMergeRequestsOptions{
		ListOptions: gongfeng.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
	}
	if opts.State != "" {
		listOpts.State = gongfeng.Ptr(string(opts.State))
	}
	mrs, resp, err := p.client.MergeRequests.ListMergeRequests(ctx, pid, listOpts)
	if err != nil {
		return nil, 0, sdkError("ListCRs", err)
	}
	crs := make([]*provider.ChangeRequest, 0, len(mrs))
	for _, mr := range mrs {
		crs = append(crs, convertMR(mr))
	}
	return crs, extractTotalCount(resp, len(crs)), nil
}

// MergeCR implements provider.ChangeRequestManager.
func (p *Provider) MergeCR(ctx context.Context, owner, repo, number string, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	n, err := prNumber("MergeCR", number)
	if err != nil {
		return nil, err
	}
	pid := owner + "/" + repo
	// Pre-flight: check MR state.
	existingMR, _, err := p.client.MergeRequests.GetMergeRequest(ctx, pid, n)
	if err == nil {
		if mapState(existingMR.State) != provider.CRStateOpened {
			return nil, provider.Wrapf(provider.PlatformTencentCode, "MergeCR",
				"MR is not in 'opened' state (current: %s)", existingMR.State)
		}
	}
	acceptOpts := &gongfeng.AcceptMergeRequestOptions{}
	if opts.MergeCommitMessage != "" {
		acceptOpts.MergeCommitMessage = gongfeng.Ptr(opts.MergeCommitMessage)
	}
	mr, _, err := p.client.MergeRequests.AcceptMergeRequest(ctx, pid, n, acceptOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformTencentCode, "MergeCR", err)
	}
	return convertMR(mr), nil
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo, number string) (*provider.ChangeRequest, error) {
	n, err := prNumber("CloseCR", number)
	if err != nil {
		return nil, err
	}
	pid := owner + "/" + repo
	updateOpts := &gongfeng.UpdateMergeRequestOptions{
		StateEvent: gongfeng.Ptr("close"),
	}
	mr, _, err := p.client.MergeRequests.UpdateMergeRequest(ctx, pid, n, updateOpts)
	if err != nil {
		return nil, sdkError("CloseCR", err)
	}
	return convertMR(mr), nil
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo, number string) (*provider.ChangeRequest, error) {
	n, err := prNumber("ReopenCR", number)
	if err != nil {
		return nil, err
	}
	pid := owner + "/" + repo
	updateOpts := &gongfeng.UpdateMergeRequestOptions{
		StateEvent: gongfeng.Ptr("reopen"),
	}
	mr, _, err := p.client.MergeRequests.UpdateMergeRequest(ctx, pid, n, updateOpts)
	if err != nil {
		return nil, sdkError("ReopenCR", err)
	}
	return convertMR(mr), nil
}

// UpdateCR implements provider.ChangeRequestManager.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo, number string, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	n, err := prNumber("UpdateCR", number)
	if err != nil {
		return nil, err
	}
	pid := owner + "/" + repo
	updateOpts := &gongfeng.UpdateMergeRequestOptions{}
	if opts.Title != "" {
		updateOpts.Title = gongfeng.Ptr(opts.Title)
	}
	if opts.Description != "" {
		updateOpts.Description = gongfeng.Ptr(opts.Description)
	}
	if opts.TargetBranch != "" {
		updateOpts.TargetBranch = gongfeng.Ptr(opts.TargetBranch)
	}
	mr, _, err := p.client.MergeRequests.UpdateMergeRequest(ctx, pid, n, updateOpts)
	if err != nil {
		return nil, sdkError("UpdateCR", err)
	}
	return convertMR(mr), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
//
// Tencent Code's API no longer supports setting labels via UpdateMergeRequest.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo, number string, labels []string) error {
	return provider.Wrap(provider.PlatformTencentCode, "UpdateCRLabels", provider.ErrNotImplemented)
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo, number string) ([]*provider.CRComment, error) {
	n, err := prNumber("ListCRComments", number)
	if err != nil {
		return nil, err
	}
	pid := owner + "/" + repo
	notes, _, err := p.client.Notes.ListMergeRequestNotes(ctx, pid, n, nil)
	if err != nil {
		return nil, sdkError("ListCRComments", err)
	}
	result := make([]*provider.CRComment, 0, len(notes))
	for _, n := range notes {
		comment := &provider.CRComment{
			ID:        int64(n.ID),
			Body:      n.Body,
			CreatedAt: n.CreatedAt.Time,
			UpdatedAt: n.UpdatedAt.Time,
		}
		if n.Author != nil {
			comment.Author = convertUser(n.Author)
		}
		result = append(result, comment)
	}
	return result, nil
}

// ListCRCommits implements provider.ChangeRequestManager.
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo, number string) ([]*provider.CRCommit, error) {
	n, err := prNumber("ListCRCommits", number)
	if err != nil {
		return nil, err
	}
	pid := owner + "/" + repo
	commits, _, err := p.client.MergeRequests.ListMergeRequestCommits(ctx, pid, n, nil)
	if err != nil {
		return nil, sdkError("ListCRCommits", err)
	}
	result := make([]*provider.CRCommit, 0, len(commits))
	for _, c := range commits {
		result = append(result, &provider.CRCommit{
			SHA:     c.ID,
			Message: c.Message,
			Author:  &provider.CRUser{Name: c.AuthorName},
		})
	}
	return result, nil
}

var _ provider.ChangeRequestManager = (*Provider)(nil)
