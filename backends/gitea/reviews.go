package gitea

import (
	"context"
	"strconv"
	"strings"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListReviews implements provider.ReviewManager.
func (p *Provider) ListReviews(ctx context.Context, owner, repo, number string) ([]provider.Review, error) {
	index, err := issueNumber("ListReviews", number)
	if err != nil {
		return nil, err
	}
	reviews, _, err := p.client.ListPullReviews(owner, repo, index, gitea.ListPullReviewsOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListReviews", err)
	}
	result := make([]provider.Review, 0, len(reviews))
	for _, r := range reviews {
		result = append(result, convertReview(r))
	}
	return result, nil
}

// GetReview implements provider.ReviewManager.
func (p *Provider) GetReview(ctx context.Context, owner, repo, number string, reviewID int64) (*provider.Review, error) {
	index, err := issueNumber("GetReview", number)
	if err != nil {
		return nil, err
	}
	review, _, err := p.client.GetPullReview(owner, repo, index, reviewID)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetReview", err)
	}
	r := convertReview(review)
	return &r, nil
}

// CreateReview implements provider.ReviewManager. It moved here from
// DiffManager; the change request number is now addressed as a string.
//
// Gitea's CreatePullReviewOptions carries the verdict under the event JSON
// key (the SDK's State field), and the server finalizes the review on
// create, so a single POST is enough — SubmitPullReview only exists for
// pending-draft workflows and would needlessly double the round trips (and
// its pending create path rejects empty-body, comment-less reviews that the
// direct submit accepts). The event mapping follows CreateReviewOptions's
// GitHub-style values: APPROVE and REQUEST_CHANGES map to the gitea states
// of the same names; anything else (COMMENT included) becomes COMMENT.
func (p *Provider) CreateReview(ctx context.Context, owner, repo, number string, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	index, err := issueNumber("CreateReview", number)
	if err != nil {
		return nil, err
	}
	reviewOpts := gitea.CreatePullReviewOptions{
		CommitID: opts.CommitID,
		Body:     opts.Body,
	}
	switch opts.Event {
	case "APPROVE":
		reviewOpts.State = gitea.ReviewStateApproved
	case "REQUEST_CHANGES":
		reviewOpts.State = gitea.ReviewStateRequestChanges
	default:
		reviewOpts.State = gitea.ReviewStateComment
	}

	for _, c := range opts.Comments {
		rc := gitea.CreatePullReviewComment{Path: c.Path, Body: c.Body}
		if c.Side == "LEFT" {
			rc.OldLineNum = int64(c.Line)
		} else {
			rc.NewLineNum = int64(c.Line)
		}
		if c.StartLine > 0 && c.EndLine > c.StartLine {
			if c.Side == "LEFT" {
				rc.OldLineNum = int64(c.StartLine)
			} else {
				rc.NewLineNum = int64(c.StartLine)
			}
		}
		reviewOpts.Comments = append(reviewOpts.Comments, rc)
	}

	review, _, err := p.client.CreatePullReview(owner, repo, index, reviewOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateReview", err)
	}
	return convertReviewResult(review), nil
}

// RequestReviewers implements provider.ReviewManager via
// CreateReviewRequests, which posts the reviewer logins under the same
// "reviewers" wire key as GitHub.
func (p *Provider) RequestReviewers(ctx context.Context, owner, repo, number string, reviewers []string) error {
	index, err := issueNumber("RequestReviewers", number)
	if err != nil {
		return err
	}
	if _, err := p.client.CreateReviewRequests(owner, repo, index, gitea.PullReviewRequestOptions{
		Reviewers: reviewers,
	}); err != nil {
		return provider.Wrap(provider.PlatformGitea, "RequestReviewers", err)
	}
	return nil
}

// DismissReview implements provider.ReviewManager via DismissPullReview
// (POST .../reviews/{id}/dismissals with the dismissal message).
func (p *Provider) DismissReview(ctx context.Context, owner, repo, number string, reviewID int64, message string) error {
	index, err := issueNumber("DismissReview", number)
	if err != nil {
		return err
	}
	if _, err := p.client.DismissPullReview(owner, repo, index, reviewID, gitea.DismissPullReviewOptions{
		Message: message,
	}); err != nil {
		return provider.Wrap(provider.PlatformGitea, "DismissReview", err)
	}
	return nil
}

// convertReview maps a gitea PullReview to a provider.Review, normalizing
// the UPPERCASE wire states to the SDK's lowercase ReviewState constants.
// Gitea's REQUEST_REVIEW state has no provider constant (it is a review
// request rather than a submitted verdict) and passes through lowercased
// rather than being silently dropped, matching the GitHub backend.
func convertReview(r *gitea.PullReview) provider.Review {
	var review provider.Review
	if r == nil {
		return review
	}
	review = provider.Review{
		ID:   r.ID,
		Body: r.Body,
	}
	if r.Reviewer != nil {
		review.User = r.Reviewer.UserName
	}
	switch r.State {
	case gitea.ReviewStateApproved:
		review.State = provider.ReviewStateApproved
	case gitea.ReviewStateRequestChanges:
		review.State = provider.ReviewStateChangesRequested
	case gitea.ReviewStateComment:
		review.State = provider.ReviewStateCommented
	case gitea.ReviewStatePending:
		review.State = provider.ReviewStatePending
	default:
		if s := string(r.State); s != "" {
			review.State = provider.ReviewState(strings.ToLower(s))
		}
	}
	if !r.Submitted.IsZero() {
		review.SubmittedAt = r.Submitted
	}
	return review
}

// convertReviewResult maps a gitea PullReview to a provider.ReviewResult.
func convertReviewResult(r *gitea.PullReview) *provider.ReviewResult {
	if r == nil {
		return nil
	}
	result := &provider.ReviewResult{
		ID:      strconv.FormatInt(r.ID, 10),
		HTMLURL: r.HTMLURL,
	}
	if r.Reviewer != nil {
		result.User = &provider.CRUser{
			ID:        r.Reviewer.ID,
			Username:  r.Reviewer.UserName,
			AvatarURL: r.Reviewer.AvatarURL,
		}
	}
	return result
}

var _ provider.ReviewManager = (*Provider)(nil)
