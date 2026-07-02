package provider

// This file consolidates every option/result type used by the Provider
// sub-interfaces. The split between "options" (what callers pass in) and
// "result types" (what platforms return) is intentional so consumers can
// scan a single file to see the cross-platform API surface.

// --- Listing / pagination ---

// ListRepoOptions contains options for listing repositories on a platform.
type ListRepoOptions struct {
	Owner   string `json:"owner,omitempty"`
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"per_page,omitempty"`
}

// ListCommitsOptions contains options for listing commits in a repository.
type ListCommitsOptions struct {
	Page    int    `json:"page,omitempty"`
	PerPage int    `json:"per_page,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Since   string `json:"since,omitempty"` // RFC3339
	Until   string `json:"until,omitempty"` // RFC3339
}

// --- Change requests (PRs / MRs) ---

// CreateCROptions contains options for creating a change request.
type CreateCROptions struct {
	Owner              string   `json:"owner"`
	Repo               string   `json:"repo"`
	Title              string   `json:"title"`
	Description        string   `json:"description,omitempty"`
	SourceBranch       string   `json:"source_branch"`
	TargetBranch       string   `json:"target_branch"`
	Labels             []string `json:"labels,omitempty"`
	RemoveSourceBranch bool     `json:"remove_source_branch,omitempty"`
}

// ListCROptions contains options for listing change requests.
type ListCROptions struct {
	Owner        string  `json:"owner"`
	Repo         string  `json:"repo"`
	State        CRState `json:"state,omitempty"`
	SourceBranch string  `json:"source_branch,omitempty"`
	TargetBranch string  `json:"target_branch,omitempty"`
	Page         int     `json:"page,omitempty"`
	PerPage      int     `json:"per_page,omitempty"`
}

// MergeCROptions contains options for merging a change request.
type MergeCROptions struct {
	MergeCommitMessage string `json:"merge_commit_message,omitempty"`
	Squash             bool   `json:"squash,omitempty"`
	RemoveSourceBranch bool   `json:"remove_source_branch,omitempty"`
}

// UpdateCROptions contains options for updating a change request.
type UpdateCROptions struct {
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	TargetBranch string `json:"target_branch,omitempty"`
}

// DiscussionOptions contains options for creating a discussion comment.
type DiscussionOptions struct {
	Body         string `json:"body"`
	FilePath     string `json:"file_path,omitempty"`
	NewLine      int    `json:"new_line,omitempty"`
	OldLine      int    `json:"old_line,omitempty"`
	StartNewLine int    `json:"start_new_line,omitempty"`
	BaseSHA      string `json:"base_sha,omitempty"`
	StartSHA     string `json:"start_sha,omitempty"`
	HeadSHA      string `json:"head_sha,omitempty"`
}

// CreateReviewOptions contains options for creating a review.
type CreateReviewOptions struct {
	CommitID string          `json:"commit_id,omitempty"`
	Event    string          `json:"event,omitempty"`
	Body     string          `json:"body,omitempty"`
	Comments []ReviewComment `json:"comments,omitempty"`
}

// ReviewComment is a single inline comment in a code review.
type ReviewComment struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Line      int    `json:"line,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Side      string `json:"side,omitempty"`
}

// ReviewResult is the result of a CreateReview call.
type ReviewResult struct {
	ID       string                `json:"id"`
	Body     string                `json:"body,omitempty"`
	HTMLURL  string                `json:"html_url,omitempty"`
	User     *CRUser               `json:"user,omitempty"`
	Comments []ReviewCommentResult `json:"comments,omitempty"`
}

// ReviewCommentResult is the result of posting a single inline comment.
type ReviewCommentResult struct {
	Path       string `json:"path,omitempty"`
	Line       int    `json:"line,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
	Error      string `json:"error,omitempty"`
}

// CommitStatusOptions contains options for creating a commit status.
type CommitStatusOptions struct {
	State       string `json:"state"`
	Context     string `json:"context"`
	Description string `json:"description,omitempty"`
	TargetURL   string `json:"target_url,omitempty"`
}

// --- Repositories ---

// CreateRepoOptions contains options for creating a repository.
type CreateRepoOptions struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Private       bool   `json:"private,omitempty"`
	AutoInit      bool   `json:"auto_init,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

// UpdateRepoOptions contains options for updating a repository.
type UpdateRepoOptions struct {
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	Private       *bool  `json:"private,omitempty"`
}

// ForkRepoOptions contains options for forking a repository.
type ForkRepoOptions struct {
	Organization string `json:"organization,omitempty"`
	Name         string `json:"name,omitempty"`
}

// --- Files ---

// FileOptions contains options for creating or updating a file.
type FileOptions struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Message string `json:"message"`
	Branch  string `json:"branch,omitempty"`
	SHA     string `json:"sha,omitempty"`
	Author  string `json:"author,omitempty"`
	Email   string `json:"email,omitempty"`
}

// FileDeleteOptions contains options for deleting a file.
type FileDeleteOptions struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Branch  string `json:"branch,omitempty"`
	SHA     string `json:"sha,omitempty"`
	Author  string `json:"author,omitempty"`
	Email   string `json:"email,omitempty"`
}

// FileResult is the result of a file operation.
type FileResult struct {
	SHA       string `json:"sha,omitempty"`
	CommitSHA string `json:"commit_sha,omitempty"`
}

// --- Webhooks ---

// CreateWebhookOptions contains options for creating a webhook.
type CreateWebhookOptions struct {
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	URL    string   `json:"url"`
	Secret string   `json:"secret,omitempty"`
	Events []string `json:"events,omitempty"`
}

// --- Releases ---

// CreateReleaseOptions contains options for creating a release.
type CreateReleaseOptions struct {
	TagName    string `json:"tag_name"`
	Target     string `json:"target,omitempty"`
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	Draft      bool   `json:"draft,omitempty"`
	Prerelease bool   `json:"prerelease,omitempty"`
}
