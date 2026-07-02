package tencentcode

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/yi-nology/git-platform-sdk/pkg/encoding"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// TencentCodeExtras exposes Tencent 工蜂-specific capabilities that are not
// part of the cross-platform Provider interface (native code reviews, MR review
// workflow, commit-level comments/diff, repository tree/blob and branch
// protection). Obtain it via type assertion:
//
//	p, _ := provider.NewProvider(provider.Config{Platform: provider.PlatformTencentCode, Token: t})
//	if tc, ok := p.(provider.TencentCodeExtras); ok {
//	    tc.SubmitCodeReview(ctx, owner, repo, reviewID, opts)
//	}
type TencentCodeExtras interface {
	// --- Native code review (/review) ---
	CreateCodeReview(ctx context.Context, owner, repo string, opts CreateCodeReviewOptions) (*CodeReview, error)
	ListCodeReviews(ctx context.Context, owner, repo string, opts ListCodeReviewsOptions) ([]*CodeReview, error)
	GetCodeReview(ctx context.Context, owner, repo string, reviewID int) (*CodeReview, error)
	UpdateCodeReview(ctx context.Context, owner, repo string, reviewID int, opts UpdateCodeReviewOptions) (*CodeReview, error)
	InviteCodeReviewer(ctx context.Context, owner, repo string, reviewID int, opts InviteReviewerOptions) error
	RemoveCodeReviewer(ctx context.Context, owner, repo string, reviewID int, reviewerID string) error
	SubmitCodeReview(ctx context.Context, owner, repo string, reviewID int, opts SubmitReviewOptions) error
	ReopenCodeReview(ctx context.Context, owner, repo string, reviewID int) error
	GetCodeReviewChangedFiles(ctx context.Context, owner, repo string, reviewID int) ([]*provider.ChangedFile, error)

	// --- MR review workflow (/merge_request/{iid}/review*) ---
	GetMRReview(ctx context.Context, owner, repo string, mrNumber int) (*MRReview, error)
	InviteMRReviewer(ctx context.Context, owner, repo string, mrNumber int, opts InviteReviewerOptions) error
	RemoveMRReviewer(ctx context.Context, owner, repo string, mrNumber int, reviewerID int) error
	CancelMRReview(ctx context.Context, owner, repo string, mrNumber int) error
	SubmitMRReview(ctx context.Context, owner, repo string, mrNumber int, opts SubmitReviewOptions) error
	ReopenMRReview(ctx context.Context, owner, repo string, mrNumber int) error

	// --- Commit-level ops (/repository/commits/{sha}) ---
	GetCommitDiff(ctx context.Context, owner, repo, sha string, opts CommitDiffOptions) ([]*provider.ChangedFile, error)
	ListCommitComments(ctx context.Context, owner, repo, sha string, page, perPage int) ([]*CommitComment, error)
	CreateCommitComment(ctx context.Context, owner, repo, sha string, opts CreateCommitCommentOptions) (*CommitComment, error)
	GetCommitRefs(ctx context.Context, owner, repo, sha string, refType CommitRefType) (*CommitRefs, error)

	// --- Repository tree & blob ---
	GetRepoTree(ctx context.Context, owner, repo, path, ref string, recursive bool) ([]*TreeEntryNode, error)
	GetBlob(ctx context.Context, owner, repo, sha string) ([]byte, error)

	// --- Branch protection ---
	ProtectBranch(ctx context.Context, owner, repo, branch string, opts ProtectBranchOptions) error
	UnprotectBranch(ctx context.Context, owner, repo, branch string) error
	ListProtectedBranchMembers(ctx context.Context, owner, repo, branch string) ([]*ProtectedBranchMember, error)
	AddProtectedBranchMember(ctx context.Context, owner, repo, branch string, opts ProtectedBranchMemberOptions) error
	UpdateProtectedBranchMember(ctx context.Context, owner, repo, branch string, userID int, opts ProtectedBranchMemberOptions) error
	RemoveProtectedBranchMember(ctx context.Context, owner, repo, branch string, userID int) error
}

// Compile-time assertion that *tencentCodeProvider satisfies TencentCodeExtras.
var _ TencentCodeExtras = (*Provider)(nil)

// ---------------------------------------------------------------------------
// Shared option / result types
// ---------------------------------------------------------------------------

// ReviewerEvent is the verdict a reviewer submits for a code review or MR review.
type ReviewerEvent string

const (
	ReviewEventComment       ReviewerEvent = "comment"
	ReviewEventApprove       ReviewerEvent = "approve"
	ReviewEventRequireChange ReviewerEvent = "require_change"
	ReviewEventDeny          ReviewerEvent = "deny"
)

// SubmitReviewOptions submits a reviewer verdict with a summary.
type SubmitReviewOptions struct {
	Event   ReviewerEvent
	Summary string
}

// InviteReviewerOptions invites one or more reviewers.
type InviteReviewerOptions struct {
	ReviewerID          string // comma-separated user IDs (code review)
	NecessaryReviewerID string // comma-separated necessary user IDs (code review)
}

// CodeReview represents a Tencent 工蜂 native code review.
type CodeReview struct {
	ID           int      `json:"id"`
	IID          int      `json:"iid"`
	ProjectID    int      `json:"project_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	State        string   `json:"state"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	SourceCommit string   `json:"source_commit"`
	TargetCommit string   `json:"target_commit"`
	Author       *provider.CRUser  `json:"author"`
	Reviewers    []string `json:"reviewers"`
	WebURL       string   `json:"web_url"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// CreateCodeReviewOptions creates a native code review.
type CreateCodeReviewOptions struct {
	Title                 string
	SourceBranch          string
	TargetBranch          string
	SourceCommit          string
	TargetCommit          string
	Description           string
	ReviewerIDs           string // comma-separated reviewer user IDs
	NecessaryReviewerIDs  string // comma-separated necessary reviewer user IDs
	ApproverRule          int    // -1 = all approve, 1 = single, 2+ = N approve
	NecessaryApproverRule int
}

// ListCodeReviewsOptions filters the code review list.
type ListCodeReviewsOptions struct {
	State    string // approving, change_required, closed
	AuthorID int
	OrderBy  string // created_at, updated_at
	Sort     string // asc, desc
	Page     int
	PerPage  int
}

// UpdateCodeReviewOptions updates a code review's title/description.
type UpdateCodeReviewOptions struct {
	Title       string
	Description string
}

// MRReview represents the review state of a merge request.
type MRReview struct {
	ID           int     `json:"id"`
	IID          int     `json:"iid"`
	ProjectID    int     `json:"project_id"`
	State        string  `json:"state"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Author       *provider.CRUser `json:"author"`
	SourceBranch string  `json:"source_branch"`
	TargetBranch string  `json:"target_branch"`
	WebURL       string  `json:"web_url"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

// CommitDiffOptions controls a single-commit diff.
type CommitDiffOptions struct {
	Path             string
	IgnoreWhiteSpace bool
}

// CommitComment represents a comment on a commit.
type CommitComment struct {
	ID        int     `json:"id"`
	Body      string  `json:"body"`
	Author    *provider.CRUser `json:"author"`
	Path      string  `json:"path"`
	Line      int     `json:"line"`
	LineType  string  `json:"line_type"`
	CreatedAt string  `json:"created_at"`
}

// CreateCommitCommentOptions creates a comment on a commit.
type CreateCommitCommentOptions struct {
	Note     string
	Path     string
	Line     int
	LineType string // "old" or "new"
}

// CommitRefType selects which refs to return for a commit.
type CommitRefType string

const (
	CommitRefAll    CommitRefType = "all"
	CommitRefBranch CommitRefType = "branch"
	CommitRefTag    CommitRefType = "tag"
)

// CommitRefs lists branches and tags containing a commit.
type CommitRefs struct {
	Branches []string `json:"branches"`
	Tags     []string `json:"tags"`
}

// TreeEntryNode represents a single entry in a repository tree listing.
type TreeEntryNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "tree" or "blob"
	Mode string `json:"mode"`
	Path string `json:"path"`
}

// ProtectBranchOptions configures branch protection.
type ProtectBranchOptions struct {
	// Placeholder for future protection rules (merge/push access levels, etc.).
	// 工蜂's protect endpoint currently takes no required body fields.
}

// ProtectedBranchMember represents a member granted access to a protected branch.
type ProtectedBranchMember struct {
	UserID      int    `json:"user_id"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	AccessLevel int    `json:"access_level"`
}

// ProtectedBranchMemberOptions adds/updates a protected branch member.
type ProtectedBranchMemberOptions struct {
	UserID      int
	AccessLevel int
}

// ===========================================================================
// Native code review (/review)
// ===========================================================================

func (t *Provider) CreateCodeReview(ctx context.Context, owner, repo string, opts CreateCodeReviewOptions) (*CodeReview, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{
		"title":         opts.Title,
		"source_branch": opts.SourceBranch,
		"target_branch": opts.TargetBranch,
		"description":   opts.Description,
	}
	if opts.SourceCommit != "" {
		body["source_commit"] = opts.SourceCommit
	}
	if opts.TargetCommit != "" {
		body["target_commit"] = opts.TargetCommit
	}
	if opts.ReviewerIDs != "" {
		body["reviewer_ids"] = opts.ReviewerIDs
	}
	if opts.NecessaryReviewerIDs != "" {
		body["necessary_reviewer_ids"] = opts.NecessaryReviewerIDs
	}
	if opts.ApproverRule != 0 {
		body["approver_rule"] = opts.ApproverRule
	}
	if opts.NecessaryApproverRule != 0 {
		body["necessary_approver_rule"] = opts.NecessaryApproverRule
	}
	var cr CodeReview
	if err := t.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/review", encoded), body, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

func (t *Provider) ListCodeReviews(ctx context.Context, owner, repo string, opts ListCodeReviewsOptions) ([]*CodeReview, error) {
	encoded := encodeProjectPath(owner, repo)
	opts.Page, opts.PerPage = provider.NormalizePageOpts(opts.Page, opts.PerPage)
	q := url.Values{}
	q.Set("page", strconv.Itoa(opts.Page))
	q.Set("per_page", strconv.Itoa(opts.PerPage))
	if opts.State != "" {
		q.Set("state", opts.State)
	}
	if opts.AuthorID != 0 {
		q.Set("author_id", strconv.Itoa(opts.AuthorID))
	}
	if opts.OrderBy != "" {
		q.Set("order_by", opts.OrderBy)
	}
	if opts.Sort != "" {
		q.Set("sort", opts.Sort)
	}
	var reviews []*CodeReview
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/reviews?%s", encoded, q.Encode()), nil, &reviews); err != nil {
		return nil, err
	}
	return reviews, nil
}

func (t *Provider) GetCodeReview(ctx context.Context, owner, repo string, reviewID int) (*CodeReview, error) {
	encoded := encodeProjectPath(owner, repo)
	var cr CodeReview
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/review/%d", encoded, reviewID), nil, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

func (t *Provider) UpdateCodeReview(ctx context.Context, owner, repo string, reviewID int, opts UpdateCodeReviewOptions) (*CodeReview, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{"title": opts.Title, "description": opts.Description}
	var cr CodeReview
	if err := t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/review/%d", encoded, reviewID), body, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

func (t *Provider) InviteCodeReviewer(ctx context.Context, owner, repo string, reviewID int, opts InviteReviewerOptions) error {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{}
	if opts.ReviewerID != "" {
		body["reviewer_id"] = opts.ReviewerID
	}
	if opts.NecessaryReviewerID != "" {
		body["necessary_reviewer_id"] = opts.NecessaryReviewerID
	}
	return t.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/review/%d/invite", encoded, reviewID), body, nil)
}

func (t *Provider) RemoveCodeReviewer(ctx context.Context, owner, repo string, reviewID int, reviewerID string) error {
	encoded := encodeProjectPath(owner, repo)
	q := url.Values{}
	q.Set("reviewer_id", reviewerID)
	return t.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/review/%d/dismissals?%s", encoded, reviewID, q.Encode()), nil, nil)
}

func (t *Provider) SubmitCodeReview(ctx context.Context, owner, repo string, reviewID int, opts SubmitReviewOptions) error {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{"reviewer_event": string(opts.Event), "summary": opts.Summary}
	return t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/review/%d/reviewer/summary", encoded, reviewID), body, nil)
}

func (t *Provider) ReopenCodeReview(ctx context.Context, owner, repo string, reviewID int) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/review/%d/reopen", encoded, reviewID), nil, nil)
}

func (t *Provider) GetCodeReviewChangedFiles(ctx context.Context, owner, repo string, reviewID int) ([]*provider.ChangedFile, error) {
	encoded := encodeProjectPath(owner, repo)
	var files []*provider.ChangedFile
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/review/%d/changed_files", encoded, reviewID), nil, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// ===========================================================================
// MR review workflow (/merge_request/{iid}/review*)
// ===========================================================================

func (t *Provider) GetMRReview(ctx context.Context, owner, repo string, mrNumber int) (*MRReview, error) {
	encoded := encodeProjectPath(owner, repo)
	var mr MRReview
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_request/%d/review", encoded, mrNumber), nil, &mr); err != nil {
		return nil, err
	}
	return &mr, nil
}

func (t *Provider) InviteMRReviewer(ctx context.Context, owner, repo string, mrNumber int, opts InviteReviewerOptions) error {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{}
	if opts.ReviewerID != "" {
		if id, err := strconv.Atoi(opts.ReviewerID); err == nil {
			body["reviewer_id"] = id
		} else {
			body["reviewer_id"] = opts.ReviewerID
		}
	}
	if opts.NecessaryReviewerID != "" {
		if id, err := strconv.Atoi(opts.NecessaryReviewerID); err == nil {
			body["necessary_reviewer_id"] = id
		} else {
			body["necessary_reviewer_id"] = opts.NecessaryReviewerID
		}
	}
	return t.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/merge_request/%d/review/invite", encoded, mrNumber), body, nil)
}

func (t *Provider) RemoveMRReviewer(ctx context.Context, owner, repo string, mrNumber int, reviewerID int) error {
	encoded := encodeProjectPath(owner, repo)
	q := url.Values{}
	q.Set("reviewer_id", strconv.Itoa(reviewerID))
	return t.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/merge_request/%d/review/dismissals?%s", encoded, mrNumber, q.Encode()), nil, nil)
}

func (t *Provider) CancelMRReview(ctx context.Context, owner, repo string, mrNumber int) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/merge_request/%d/review/cancel", encoded, mrNumber), nil, nil)
}

func (t *Provider) SubmitMRReview(ctx context.Context, owner, repo string, mrNumber int, opts SubmitReviewOptions) error {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{"reviewer_event": string(opts.Event), "summary": opts.Summary}
	return t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_request/%d/reviewer/summary", encoded, mrNumber), body, nil)
}

func (t *Provider) ReopenMRReview(ctx context.Context, owner, repo string, mrNumber int) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_request/%d/review/reopen", encoded, mrNumber), nil, nil)
}

// ===========================================================================
// Commit-level operations (/repository/commits/{sha})
// ===========================================================================

func (t *Provider) GetCommitDiff(ctx context.Context, owner, repo, sha string, opts CommitDiffOptions) ([]*provider.ChangedFile, error) {
	encoded := encodeProjectPath(owner, repo)
	q := url.Values{}
	if opts.Path != "" {
		q.Set("path", opts.Path)
	}
	if opts.IgnoreWhiteSpace {
		q.Set("ignore_white_space", "true")
	}
	path := fmt.Sprintf("/projects/%s/repository/commits/%s/diff", encoded, url.PathEscape(sha))
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var files []*provider.ChangedFile
	if err := t.doRequest(ctx, "GET", path, nil, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (t *Provider) ListCommitComments(ctx context.Context, owner, repo, sha string, page, perPage int) ([]*CommitComment, error) {
	encoded := encodeProjectPath(owner, repo)
	page, perPage = provider.NormalizePageOpts(page, perPage)
	var comments []*CommitComment
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/commits/%s/comments?page=%d&per_page=%d", encoded, url.PathEscape(sha), page, perPage), nil, &comments); err != nil {
		return nil, err
	}
	return comments, nil
}

func (t *Provider) CreateCommitComment(ctx context.Context, owner, repo, sha string, opts CreateCommitCommentOptions) (*CommitComment, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{"note": opts.Note}
	if opts.Path != "" {
		body["path"] = opts.Path
	}
	if opts.Line > 0 {
		body["line"] = opts.Line
	}
	if opts.LineType != "" {
		body["line_type"] = opts.LineType
	}
	var c CommitComment
	if err := t.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/repository/commits/%s/comments", encoded, url.PathEscape(sha)), body, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (t *Provider) GetCommitRefs(ctx context.Context, owner, repo, sha string, refType CommitRefType) (*CommitRefs, error) {
	encoded := encodeProjectPath(owner, repo)
	q := url.Values{}
	if refType != "" {
		q.Set("type", string(refType))
	}
	path := fmt.Sprintf("/projects/%s/repository/commits/%s/refs", encoded, url.PathEscape(sha))
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	var refs struct {
		Type     string `json:"type"`
		Branches []struct {
			Name string `json:"name"`
		} `json:"branches"`
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
	}
	if err := t.doRequest(ctx, "GET", path, nil, &refs); err != nil {
		return nil, err
	}
	result := &CommitRefs{}
	for _, b := range refs.Branches {
		result.Branches = append(result.Branches, b.Name)
	}
	for _, tg := range refs.Tags {
		result.Tags = append(result.Tags, tg.Name)
	}
	return result, nil
}

// ===========================================================================
// Repository tree & blob
// ===========================================================================

func (t *Provider) GetRepoTree(ctx context.Context, owner, repo, path, ref string, recursive bool) ([]*TreeEntryNode, error) {
	encoded := encodeProjectPath(owner, repo)
	q := url.Values{}
	if path != "" {
		q.Set("path", path)
	}
	if ref != "" {
		q.Set("ref_name", ref)
	}
	if recursive {
		q.Set("recursive", "true")
	}
	reqPath := fmt.Sprintf("/projects/%s/repository/tree", encoded)
	if enc := q.Encode(); enc != "" {
		reqPath += "?" + enc
	}
	var nodes []*TreeEntryNode
	if err := t.doRequest(ctx, "GET", reqPath, nil, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (t *Provider) GetBlob(ctx context.Context, owner, repo, sha string) ([]byte, error) {
	encoded := encodeProjectPath(owner, repo)
	// The blob endpoint returns raw content; decode as base64 content first, then
	// fall back to raw bytes.
	var blob struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/blobs/%s", encoded, url.PathEscape(sha)), nil, &blob); err != nil {
		return nil, err
	}
	if blob.Encoding == "base64" {
		decoded, err := encoding.Base64Decode(strings.ReplaceAll(blob.Content, "\n", ""))
		if err != nil {
			return nil, err
		}
		return []byte(decoded), nil
	}
	return []byte(blob.Content), nil
}

// ===========================================================================
// Branch protection
// ===========================================================================

func (t *Provider) ProtectBranch(ctx context.Context, owner, repo, branch string, opts ProtectBranchOptions) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/repository/branches/%s/protect", encoded, url.PathEscape(branch)), nil, nil)
}

func (t *Provider) UnprotectBranch(ctx context.Context, owner, repo, branch string) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/repository/branches/%s/unprotect", encoded, url.PathEscape(branch)), nil, nil)
}

func (t *Provider) ListProtectedBranchMembers(ctx context.Context, owner, repo, branch string) ([]*ProtectedBranchMember, error) {
	encoded := encodeProjectPath(owner, repo)
	var members []*ProtectedBranchMember
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/branches/protected/%s/members", encoded, url.PathEscape(branch)), nil, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func (t *Provider) AddProtectedBranchMember(ctx context.Context, owner, repo, branch string, opts ProtectedBranchMemberOptions) error {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{"user_id": opts.UserID}
	if opts.AccessLevel != 0 {
		body["access_level"] = opts.AccessLevel
	}
	return t.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/branches/protected/%s/members", encoded, url.PathEscape(branch)), body, nil)
}

func (t *Provider) UpdateProtectedBranchMember(ctx context.Context, owner, repo, branch string, userID int, opts ProtectedBranchMemberOptions) error {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{}
	if opts.AccessLevel != 0 {
		body["access_level"] = opts.AccessLevel
	}
	return t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/branches/protected/%s/members/%d", encoded, url.PathEscape(branch), userID), body, nil)
}

func (t *Provider) RemoveProtectedBranchMember(ctx context.Context, owner, repo, branch string, userID int) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/branches/protected/%s/members/%d", encoded, url.PathEscape(branch), userID), nil, nil)
}
