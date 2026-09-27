package gitlab

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	gitlab "gitlab.com/gitlab-org/api/client-go/v3"

	"github.com/yi-nology/go-git-platform/backends/internal/backendutil"
	"github.com/yi-nology/go-git-platform/provider"
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
	iid, err := backendutil.ParsePRNumber64(provider.PlatformGitLab, "ListReviews", number)
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
	iid, err := backendutil.ParsePRNumber64(provider.PlatformGitLab, "GetReview", number)
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
//
// v0.65.0 行内评论：opts.Comments 非空时改走 discussions 路径——此前实现丢弃
// Comments 只发纯文本 note，调用方（argus poster）的行内评论链路在 GitLab
// 后端从未生效。position 用 MR diff_refs 三 SHA + new_path/new_line（模型给
// 的是新文件行号）；单条 discussion 失败（行号不在 diff hunk 内等 422）按条
// 跳过不整单失败；总结 note 照发保留评审事件语义。
func (p *Provider) CreateReview(ctx context.Context, owner, repo, number string, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	iid, err := backendutil.ParsePRNumber64(provider.PlatformGitLab, "CreateReview", number)
	if err != nil {
		return nil, err
	}
	if len(opts.Comments) > 0 {
		if res, err := p.createReviewWithComments(ctx, owner, repo, iid, opts); err == nil {
			return res, nil
		}
		// 行内路径失败（拉不到 diff_refs 等）→ 降级纯评论 note，评审事件不丢
	}
	return p.createNoteReview(ctx, owner, repo, iid, opts)
}

// createReviewWithComments 逐条建 position discussion + 总结 note。
func (p *Provider) createReviewWithComments(ctx context.Context, owner, repo string, iid int64, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	mr, _, err := p.client.MergeRequests.GetMergeRequest(pidOf(owner, repo), iid, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateReview", err)
	}
	refs := mr.DiffRefs
	if refs.HeadSha == "" || refs.BaseSha == "" {
		p.logger.Warn("gitlab.CreateReview: diff_refs 缺失，行内评论降级纯评论",
			"base", refs.BaseSha, "head", refs.HeadSha, "start", refs.StartSha)
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateReview",
			errors.New("mr diff_refs 缺失，无法定位行内评论"))
	}
	p.logger.Info("gitlab.CreateReview: inline comments path", "comments", len(opts.Comments))
	posType := "text"
	for _, c := range opts.Comments {
		if c.Path == "" || c.Line <= 0 || c.Body == "" {
			continue
		}
		path, line, body := c.Path, int64(c.Line), c.Body
		opt := &gitlab.CreateMergeRequestDiscussionOptions{
			Body: gitlab.Ptr(body),
			Position: &gitlab.PositionOptions{
				BaseSHA:      gitlab.Ptr(refs.BaseSha),
				HeadSHA:      gitlab.Ptr(refs.HeadSha),
				StartSHA:     gitlab.Ptr(refs.StartSha),
				PositionType: gitlab.Ptr(posType),
				NewPath:      gitlab.Ptr(path),
				NewLine:      gitlab.Ptr(line),
			},
		}
		// 单条失败 best-effort 跳过：行号不在 hunk / 文件二进制等，不该拖垮整轮评审；
		// 但必须留痕——否则调用方行内全丢还以为发出去了（v0.65.1 实测教训）
		if _, _, err := p.client.Discussions.CreateMergeRequestDiscussion(pidOf(owner, repo), iid, opt, gitlab.WithContext(ctx)); err != nil {
			p.logger.Warn("gitlab.CreateReview: inline discussion skipped",
				"path", c.Path, "line", c.Line, "error", err.Error())
		}
	}
	return p.createNoteReview(ctx, owner, repo, iid, opts)
}

// createNoteReview 纯评论评审（无行内时的原路径）。
func (p *Provider) createNoteReview(ctx context.Context, owner, repo string, iid int64, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	note, _, err := p.client.Notes.CreateMergeRequestNote(pidOf(owner, repo), iid,
		&gitlab.CreateMergeRequestNoteOptions{Body: new(opts.Body)}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateReview", err)
	}
	return convertNoteReviewResult(note), nil
}

// RequestReviewers implements provider.ReviewManager. Reviewer usernames
// are resolved to user IDs via the Users API (cached) and written through
// UpdateMergeRequest's reviewer_ids.
func (p *Provider) RequestReviewers(ctx context.Context, owner, repo, number string, reviewers []string) error {
	iid, err := backendutil.ParsePRNumber64(provider.PlatformGitLab, "RequestReviewers", number)
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
	iid, err := backendutil.ParsePRNumber64(provider.PlatformGitLab, "DismissReview", number)
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
