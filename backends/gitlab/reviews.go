package gitlab

import (
	"context"
	"net/http"
	"strconv"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListReviews implements provider.ReviewManager.
//
// Registered mapping (divergence ledger): GitLab has no per-review list — approvals
// are a single state on the merge request. ListReviews maps to
// MergeRequestApprovalsService.GetApprovalState(pid, mrIID) and synthesizes
// one summary review per approver: Review{ID: MR IID, User: approver
// username, State: approved}, taken from rules[].approved_by. There is no
// per-approval ID on the wire, so every synthesized review shares the MR IID
// as its ID, and an approver listed under several rules yields one review.
func (p *Provider) ListReviews(ctx context.Context, owner, repo, number string) ([]provider.Review, error) {
	iid, err := prNumber("ListReviews", number)
	if err != nil {
		return nil, err
	}
	state, _, err := p.client.MergeRequestApprovals.GetApprovalState(pidOf(owner, repo), iid, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListReviews", err)
	}
	return approvalStateReviews(state, iid), nil
}

// GetReview implements provider.ReviewManager.
//
// Registered mapping (divergence ledger): the same GetApprovalState call as
// ListReviews. GitLab approvals expose no per-review IDs, so reviewID cannot
// be matched; this returns the first synthesized approver review as an
// approximation. When nobody has approved yet the call reports NotFound.
func (p *Provider) GetReview(ctx context.Context, owner, repo, number string, reviewID int64) (*provider.Review, error) {
	iid, err := prNumber("GetReview", number)
	if err != nil {
		return nil, err
	}
	state, _, err := p.client.MergeRequestApprovals.GetApprovalState(pidOf(owner, repo), iid, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "GetReview", err)
	}
	reviews := approvalStateReviews(state, iid)
	if len(reviews) == 0 {
		return nil, provider.New(provider.PlatformGitLab, "GetReview", http.StatusNotFound, "no approvals")
	}
	return &reviews[0], nil
}

// CreateReview implements provider.ReviewManager.
//
// Registered mapping (divergence ledger): GitLab has no native review object, so a
// review is expressed as a comment-style review — a merge-request note via
// Notes.CreateMergeRequestNote(pid, iid, {Body}), the same shape the
// pre-ReviewManager DiffManager.CreateReview used for its summary. Inline
// comments (opts.Comments) and verdicts (opts.Event, opts.CommitID) are not
// mapped: a note is neither an approval nor a commit report, so the created
// review is always in the commented state.
func (p *Provider) CreateReview(ctx context.Context, owner, repo, number string, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	iid, err := prNumber("CreateReview", number)
	if err != nil {
		return nil, err
	}
	note, _, err := p.client.Notes.CreateMergeRequestNote(pidOf(owner, repo), iid,
		&gitlab.CreateMergeRequestNoteOptions{Body: gitlab.Ptr(opts.Body)}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateReview", err)
	}
	return convertNoteReviewResult(note), nil
}

// RequestReviewers implements provider.ReviewManager. Reviewer usernames
// are resolved to user IDs via the Users API (cached) and written through
// UpdateMergeRequest's reviewer_ids.
func (p *Provider) RequestReviewers(ctx context.Context, owner, repo, number string, reviewers []string) error {
	iid, err := prNumber("RequestReviewers", number)
	if err != nil {
		return err
	}
	if len(reviewers) == 0 {
		return nil
	}
	ids, err := p.resolveUserIDs(ctx, "RequestReviewers", reviewers)
	if err != nil {
		return err
	}
	if _, _, err := p.client.MergeRequests.UpdateMergeRequest(pidOf(owner, repo), iid,
		&gitlab.UpdateMergeRequestOptions{ReviewerIDs: &ids}, gitlab.WithContext(ctx)); err != nil {
		return provider.Wrap(provider.PlatformGitLab, "RequestReviewers", err)
	}
	return nil
}

// DismissReview implements provider.ReviewManager.
//
// Registered mapping (divergence ledger): UnapproveMergeRequest(pid, mrIID). GitLab
// approvals hang off the merge request as a whole (per user), not off
// individual review objects, so reviewID is not addressable and the
// dismissal message has no GitLab equivalent (ignored).
func (p *Provider) DismissReview(ctx context.Context, owner, repo, number string, reviewID int64, message string) error {
	iid, err := prNumber("DismissReview", number)
	if err != nil {
		return err
	}
	if _, err := p.client.MergeRequestApprovals.UnapproveMergeRequest(pidOf(owner, repo), iid, gitlab.WithContext(ctx)); err != nil {
		return provider.Wrap(provider.PlatformGitLab, "DismissReview", err)
	}
	return nil
}

// approvalStateReviews synthesizes the summary reviews of a GitLab approval
// state: one approved review per distinct approver listed in
// rules[].approved_by, each keyed by the MR IID because GitLab approvals
// carry no per-approval IDs.
func approvalStateReviews(state *gitlab.MergeRequestApprovalState, mrIID int64) []provider.Review {
	reviews := make([]provider.Review, 0)
	if state == nil {
		return reviews
	}
	seen := make(map[string]bool)
	for _, rule := range state.Rules {
		if rule == nil {
			continue
		}
		for _, approver := range rule.ApprovedBy {
			if approver == nil || approver.Username == "" || seen[approver.Username] {
				continue
			}
			seen[approver.Username] = true
			reviews = append(reviews, provider.Review{
				ID:    mrIID,
				User:  approver.Username,
				State: provider.ReviewStateApproved,
			})
		}
	}
	return reviews
}

// convertNoteReviewResult maps the merge-request note backing a comment-style
// review to a provider.ReviewResult (ID = note ID).
func convertNoteReviewResult(note *gitlab.Note) *provider.ReviewResult {
	if note == nil {
		return nil
	}
	return &provider.ReviewResult{
		ID:   strconv.FormatInt(note.ID, 10),
		Body: note.Body,
		User: convertNoteAuthor(note.Author),
	}
}

var _ provider.ReviewManager = (*Provider)(nil)
