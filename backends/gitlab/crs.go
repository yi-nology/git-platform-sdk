package gitlab

import (
	"context"
	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	createOpts := &gitlab.CreateMergeRequestOptions{
		SourceBranch:       gitlab.Ptr(opts.SourceBranch),
		TargetBranch:       gitlab.Ptr(opts.TargetBranch),
		Title:              gitlab.Ptr(opts.Title),
		Description:        gitlab.Ptr(opts.Description),
		RemoveSourceBranch: gitlab.Ptr(opts.RemoveSourceBranch),
	}
	if len(opts.Labels) > 0 {
		labels := gitlab.LabelOptions(opts.Labels)
		createOpts.Labels = &labels
	}
	mr, _, err := p.client.MergeRequests.CreateMergeRequest(pidOf(opts.Owner, opts.Repo), createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateCR", err)
	}
	return convertMR(mr), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	mr, _, err := p.client.MergeRequests.GetMergeRequest(pidOf(owner, repo), int64(number), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "GetCR", err)
	}
	return convertMR(mr), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	listOpts := &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: int64(perPage)},
	}
	if opts.State != "" {
		listOpts.State = gitlab.Ptr(string(opts.State))
	}
	if opts.SourceBranch != "" {
		listOpts.SourceBranch = gitlab.Ptr(opts.SourceBranch)
	}
	if opts.TargetBranch != "" {
		listOpts.TargetBranch = gitlab.Ptr(opts.TargetBranch)
	}
	mrs, resp, err := p.client.MergeRequests.ListProjectMergeRequests(pidOf(opts.Owner, opts.Repo), listOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitLab, "ListCRs", err)
	}
	total := len(mrs)
	if resp != nil && resp.TotalItems > 0 {
		total = int(resp.TotalItems)
	}
	crs := make([]*provider.ChangeRequest, 0, len(mrs))
	for _, mr := range mrs {
		crs = append(crs, convertBasicMR(mr))
	}
	return crs, total, nil
}

// MergeCR implements provider.ChangeRequestManager.
func (p *Provider) MergeCR(ctx context.Context, owner, repo string, number int, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	pid := pidOf(owner, repo)
	// Pre-flight: reject MRs in a non-mergeable state. This produces a
	// friendlier error than waiting for the API to fail.
	existing, _, err := p.client.MergeRequests.GetMergeRequest(pid, int64(number), nil, gitlab.WithContext(ctx))
	if err == nil && existing != nil {
		if existing.DetailedMergeStatus != "" && existing.DetailedMergeStatus != "mergeable" && existing.DetailedMergeStatus != "checking" {
			return nil, provider.Wrapf(provider.PlatformGitLab, "MergeCR",
				"MR cannot be merged (status: %s). It may have conflicts or an active pipeline", existing.DetailedMergeStatus)
		}
		if existing.State != "opened" {
			return nil, provider.Wrapf(provider.PlatformGitLab, "MergeCR",
				"MR is not in 'opened' state (current: %s)", existing.State)
		}
	}
	acceptOpts := &gitlab.AcceptMergeRequestOptions{}
	if opts.MergeCommitMessage != "" {
		acceptOpts.MergeCommitMessage = gitlab.Ptr(opts.MergeCommitMessage)
	}
	if opts.Squash {
		acceptOpts.Squash = gitlab.Ptr(true)
	}
	if opts.RemoveSourceBranch {
		acceptOpts.ShouldRemoveSourceBranch = gitlab.Ptr(true)
	}
	mr, _, err := p.client.MergeRequests.AcceptMergeRequest(pid, int64(number), acceptOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "MergeCR", fmt.Errorf("merge failed: %w", err))
	}
	return convertMR(mr), nil
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	mr, _, err := p.client.MergeRequests.UpdateMergeRequest(pidOf(owner, repo), int64(number),
		&gitlab.UpdateMergeRequestOptions{StateEvent: gitlab.Ptr("close")}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CloseCR", err)
	}
	return convertMR(mr), nil
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	mr, _, err := p.client.MergeRequests.UpdateMergeRequest(pidOf(owner, repo), int64(number),
		&gitlab.UpdateMergeRequestOptions{StateEvent: gitlab.Ptr("reopen")}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ReopenCR", err)
	}
	return convertMR(mr), nil
}

// UpdateCR implements provider.ChangeRequestManager.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo string, number int, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	updateOpts := &gitlab.UpdateMergeRequestOptions{}
	if opts.Title != "" {
		updateOpts.Title = gitlab.Ptr(opts.Title)
	}
	if opts.Description != "" {
		updateOpts.Description = gitlab.Ptr(opts.Description)
	}
	if opts.TargetBranch != "" {
		updateOpts.TargetBranch = gitlab.Ptr(opts.TargetBranch)
	}
	mr, _, err := p.client.MergeRequests.UpdateMergeRequest(pidOf(owner, repo), int64(number), updateOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "UpdateCR", err)
	}
	return convertMR(mr), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	l := gitlab.LabelOptions(labels)
	_, _, err := p.client.MergeRequests.UpdateMergeRequest(pidOf(owner, repo), int64(number),
		&gitlab.UpdateMergeRequestOptions{Labels: &l}, gitlab.WithContext(ctx))
	if err != nil {
		return provider.Wrap(provider.PlatformGitLab, "UpdateCRLabels", err)
	}
	return nil
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*provider.CRComment, error) {
	notes, _, err := p.client.Notes.ListMergeRequestNotes(pidOf(owner, repo), int64(number), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListCRComments", err)
	}
	result := make([]*provider.CRComment, 0, len(notes))
	for _, n := range notes {
		c := &provider.CRComment{
			ID:   n.ID,
			Body: n.Body,
			Author: &provider.CRUser{
				ID: n.Author.ID, Username: n.Author.Username,
				Name: n.Author.Name, AvatarURL: n.Author.AvatarURL,
			},
			CreatedAt: timeOrZero(n.CreatedAt),
			UpdatedAt: timeOrZero(n.UpdatedAt),
		}
		result = append(result, c)
	}
	return result, nil
}

// ListCRCommits implements provider.ChangeRequestManager.
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*provider.CRCommit, error) {
	commits, _, err := p.client.MergeRequests.GetMergeRequestCommits(pidOf(owner, repo), int64(number), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListCRCommits", err)
	}
	result := make([]*provider.CRCommit, 0, len(commits))
	for _, c := range commits {
		cc := &provider.CRCommit{SHA: c.ShortID, Message: c.Title, CreatedAt: timeOrZero(c.CreatedAt)}
		if c.AuthorName != "" {
			cc.Author = &provider.CRUser{Name: c.AuthorName}
		}
		result = append(result, cc)
	}
	return result, nil
}

var _ provider.ChangeRequestManager = (*Provider)(nil)
