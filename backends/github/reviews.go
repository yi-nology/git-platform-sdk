package github

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListReviews implements provider.ReviewManager.
func (p *Provider) ListReviews(ctx context.Context, owner, repo, number string) ([]provider.Review, error) {
	n, err := prNumber("ListReviews", number)
	if err != nil {
		return nil, err
	}
	reviews, _, err := p.client.PullRequests.ListReviews(ctx, owner, repo, n, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListReviews", err)
	}
	result := make([]provider.Review, 0, len(reviews))
	for _, r := range reviews {
		result = append(result, convertReview(r))
	}
	return result, nil
}

// GetReview implements provider.ReviewManager.
func (p *Provider) GetReview(ctx context.Context, owner, repo, number string, reviewID int64) (*provider.Review, error) {
	n, err := prNumber("GetReview", number)
	if err != nil {
		return nil, err
	}
	review, _, err := p.client.PullRequests.GetReview(ctx, owner, repo, n, reviewID)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "GetReview", err)
	}
	r := convertReview(review)
	return &r, nil
}

// CreateReview implements provider.ReviewManager. It moved here from
// DiffManager; the change request number is now addressed as a string.
func (p *Provider) CreateReview(ctx context.Context, owner, repo, number string, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	n, err := prNumber("CreateReview", number)
	if err != nil {
		return nil, err
	}
	reviewRequest := &github.PullRequestReviewRequest{
		CommitID: github.Ptr(opts.CommitID),
		Body:     github.Ptr(opts.Body),
		Event:    github.Ptr(opts.Event),
	}
	for _, c := range opts.Comments {
		rc := &github.DraftReviewComment{
			Path: github.Ptr(c.Path),
			Body: github.Ptr(c.Body),
		}
		if c.StartLine > 0 && c.EndLine > c.StartLine {
			rc.StartLine = github.Ptr(c.StartLine)
			rc.Line = github.Ptr(c.EndLine)
			if c.Side != "" {
				rc.Side = github.Ptr(c.Side)
			} else {
				rc.Side = github.Ptr("RIGHT")
			}
			if c.StartLine != c.EndLine {
				rc.StartSide = github.Ptr("RIGHT")
			}
		} else if c.Line > 0 {
			rc.Line = github.Ptr(c.Line)
			if c.Side != "" {
				rc.Side = github.Ptr(c.Side)
			} else {
				rc.Side = github.Ptr("RIGHT")
			}
		}
		reviewRequest.Comments = append(reviewRequest.Comments, rc)
	}
	review, _, err := p.client.PullRequests.CreateReview(ctx, owner, repo, n, reviewRequest)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateReview", err)
	}
	result := &provider.ReviewResult{
		ID: strconv.FormatInt(review.GetID(), 10),
	}
	if review.GetHTMLURL() != "" {
		result.HTMLURL = review.GetHTMLURL()
	}
	if review.GetUser() != nil {
		result.User = convertUser(review.GetUser())
	}
	return result, nil
}

// RequestReviewers implements provider.ReviewManager.
func (p *Provider) RequestReviewers(ctx context.Context, owner, repo, number string, reviewers []string) error {
	n, err := prNumber("RequestReviewers", number)
	if err != nil {
		return err
	}
	req := github.ReviewersRequest{Reviewers: reviewers}
	if _, _, err := p.client.PullRequests.RequestReviewers(ctx, owner, repo, n, req); err != nil {
		return provider.Wrap(provider.PlatformGitHub, "RequestReviewers", err)
	}
	return nil
}

// DismissReview implements provider.ReviewManager.
func (p *Provider) DismissReview(ctx context.Context, owner, repo, number string, reviewID int64, message string) error {
	n, err := prNumber("DismissReview", number)
	if err != nil {
		return err
	}
	if _, _, err := p.client.PullRequests.DismissReview(ctx, owner, repo, n, reviewID, &github.PullRequestReviewDismissalRequest{
		Message: github.Ptr(message),
	}); err != nil {
		return provider.Wrap(provider.PlatformGitHub, "DismissReview", err)
	}
	return nil
}

// convertReview maps a github.PullRequestReview to a provider.Review,
// normalizing the UPPERCASE wire states to the SDK's lowercase ReviewState
// constants. Unknown states pass through lowercased rather than being
// silently dropped.
func convertReview(r *github.PullRequestReview) provider.Review {
	var review provider.Review
	if r == nil {
		return review
	}
	review = provider.Review{
		ID:   r.GetID(),
		User: r.GetUser().GetLogin(),
		Body: r.GetBody(),
	}
	switch r.GetState() {
	case "APPROVED":
		review.State = provider.ReviewStateApproved
	case "CHANGES_REQUESTED":
		review.State = provider.ReviewStateChangesRequested
	case "COMMENTED":
		review.State = provider.ReviewStateCommented
	case "PENDING":
		review.State = provider.ReviewStatePending
	default:
		if s := r.GetState(); s != "" {
			review.State = provider.ReviewState(strings.ToLower(s))
		}
	}
	if t := r.GetSubmittedAt(); !t.IsZero() {
		review.SubmittedAt = t.Time
	}
	return review
}

var _ provider.ReviewManager = (*Provider)(nil)
