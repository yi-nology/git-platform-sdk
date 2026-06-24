package provider

import (
	"context"
	"encoding/json"
	"time"
)

// Platform represents a Git hosting platform.
type Platform string

const (
	PlatformGitLab      Platform = "gitlab"
	PlatformGitHub      Platform = "github"
	PlatformGitea       Platform = "gitea"
	PlatformGitee       Platform = "gitee"
	PlatformForgejo     Platform = "forgejo"
	PlatformTencentCode Platform = "tencent_code"
	PlatformGitCode     Platform = "gitcode"
)

// Provider is the unified interface for all Git hosting platforms.
// It composes 8 focused sub-interfaces for high cohesion and low coupling.
//
// Consumers can depend on smaller interfaces (e.g., WebhookManager)
// when they don't need full Provider capabilities.
type Provider interface {
	// Platform returns the platform type.
	Platform() Platform
	// TestConnection verifies the connection and checks capabilities.
	TestConnection(ctx context.Context) (*TestConnectionResult, error)

	RepoManager
	ChangeRequestManager
	WebhookManager
	BranchManager
	DiffManager
	CommitManager
	FileManager
	ReleaseManager
}

// PlatformBranch represents a branch on a platform.
type PlatformBranch struct {
	Name string `json:"name"`
}

// PlatformRepo represents a repository on a platform.
type PlatformRepo struct {
	ID            int64    `json:"id"`
	FullName      string   `json:"full_name"`
	Name          string   `json:"name"`
	Owner         string   `json:"owner"`
	Description   string   `json:"description"`
	CloneURL      string   `json:"clone_url"`
	SSHURL        string   `json:"ssh_url"`
	DefaultBranch string   `json:"default_branch"`
	Private       bool     `json:"private"`
	Platform      Platform `json:"platform"`
}

// CRState represents the state of a change request.
type CRState string

const (
	CRStateOpened CRState = "opened"
	CRStateMerged CRState = "merged"
	CRStateClosed CRState = "closed"
)

// ChangeRequest represents a pull request or merge request.
type ChangeRequest struct {
	ID           int64     `json:"id"`
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	State        CRState   `json:"state"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	HeadSHA      string    `json:"head_sha,omitempty"`
	BaseSHA      string    `json:"base_sha,omitempty"`
	StartSHA     string    `json:"start_sha,omitempty"`
	Author       *CRUser   `json:"author"`
	Reviewers    []*CRUser `json:"reviewers"`
	Labels       []string  `json:"labels"`
	MergeStatus  string    `json:"merge_status"`
	WebURL       string    `json:"web_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CRUser represents a user on a platform.
type CRUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// ListRepoOptions contains options for listing repositories.
type ListRepoOptions struct {
	Owner   string `json:"owner"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

// CreateCROptions contains options for creating a change request.
type CreateCROptions struct {
	Owner              string   `json:"owner"`
	Repo               string   `json:"repo"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	SourceBranch       string   `json:"source_branch"`
	TargetBranch       string   `json:"target_branch"`
	Labels             []string `json:"labels"`
	RemoveSourceBranch bool     `json:"remove_source_branch"`
}

// ListCROptions contains options for listing change requests.
type ListCROptions struct {
	Owner        string  `json:"owner"`
	Repo         string  `json:"repo"`
	State        CRState `json:"state"`
	SourceBranch string  `json:"source_branch"`
	TargetBranch string  `json:"target_branch"`
	Page         int     `json:"page"`
	PerPage      int     `json:"per_page"`
}

// MergeCROptions contains options for merging a change request.
type MergeCROptions struct {
	MergeCommitMessage string `json:"merge_commit_message"`
	Squash             bool   `json:"squash"`
	RemoveSourceBranch bool   `json:"remove_source_branch"`
}

// CreateWebhookOptions contains options for creating a webhook.
type CreateWebhookOptions struct {
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	URL    string   `json:"url"`
	Secret string   `json:"secret"`
	Events []string `json:"events"`
}

// PlatformWebhook represents a webhook on a platform.
type PlatformWebhook struct {
	ID     int64    `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// NormalizedEvent represents a normalized webhook event from any platform.
type NormalizedEvent struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Source     Platform        `json:"source"`
	Timestamp  time.Time       `json:"timestamp"`
	Actor      *CRUser         `json:"actor"`
	Repo       *EventRepo      `json:"repo"`
	CR         *ChangeRequest  `json:"cr,omitempty"`
	Branch     string          `json:"branch,omitempty"`
	Tag        string          `json:"tag,omitempty"`
	CommitSHA  string          `json:"commit_sha,omitempty"`
	Action     string          `json:"action,omitempty"`
	RawPayload json.RawMessage `json:"raw_payload"`
}

// EventRepo represents the repository in a webhook event.
type EventRepo struct {
	FullName string `json:"full_name"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
}

// TestConnectionResult contains the result of a connection test.
type TestConnectionResult struct {
	Connected    bool   `json:"connected"`
	Platform     string `json:"platform"`
	UserName     string `json:"user_name"`
	Message      string `json:"message,omitempty"`
	CanListRepos bool   `json:"can_list_repos"`
	CanReadCR    bool   `json:"can_read_cr"`
	CanWriteCR   bool   `json:"can_write_cr"`
	CanWebhook   bool   `json:"can_webhook"`
}

// MergeDiff represents the diff of a change request.
type MergeDiff struct {
	Files    []*ChangedFile
	TotalAdd int
	TotalDel int
	RawDiff  string
}

// ChangedFile represents a file changed in a change request.
type ChangedFile struct {
	OldPath   string `json:"old_path"`
	NewPath   string `json:"new_path"`
	Diff      string `json:"diff"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	IsNew     bool   `json:"new_file"`
	IsDeleted bool   `json:"deleted_file"`
	IsRenamed bool   `json:"renamed_file"`
	IsBinary  bool   `json:"binary"`
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

// ReviewComment represents a comment in a code review.
type ReviewComment struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Line      int    `json:"line,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Side      string `json:"side,omitempty"`
}

// CreateReviewOptions contains options for creating a review.
type CreateReviewOptions struct {
	CommitID string          `json:"commit_id"`
	Event    string          `json:"event"`
	Body     string          `json:"body"`
	Comments []ReviewComment `json:"comments,omitempty"`
}

// ReviewResult represents the result of creating a review.
type ReviewResult struct {
	ID       string                 `json:"id"`
	Body     string                 `json:"body,omitempty"`
	HTMLURL  string                 `json:"html_url,omitempty"`
	User     *CRUser                `json:"user,omitempty"`
	Comments []ReviewCommentResult  `json:"comments,omitempty"`
}

// ReviewCommentResult represents the result of posting a single inline comment.
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
	Description string `json:"description"`
	TargetURL   string `json:"target_url,omitempty"`
}

// UpdateCROptions contains options for updating a change request.
type UpdateCROptions struct {
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	TargetBranch string `json:"target_branch,omitempty"`
}

// CRComment represents a comment on a change request.
type CRComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	Author    *CRUser   `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CRCommit represents a commit in a change request.
type CRCommit struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    *CRUser   `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

// ForkRepoOptions contains options for forking a repository.
type ForkRepoOptions struct {
	Organization string `json:"organization,omitempty"`
	Name         string `json:"name,omitempty"`
}

// UpdateRepoOptions contains options for updating a repository.
type UpdateRepoOptions struct {
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	Private       *bool  `json:"private,omitempty"`
}

// CommitInfo represents a commit.
type CommitInfo struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    *CRUser   `json:"author"`
	Committer *CRUser   `json:"committer"`
	CreatedAt time.Time `json:"created_at"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
}

// ListCommitsOptions contains options for listing commits.
type ListCommitsOptions struct {
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Branch  string `json:"branch,omitempty"`
	Since   string `json:"since,omitempty"`
	Until   string `json:"until,omitempty"`
}

// CompareResult represents the result of comparing two commits.
type CompareResult struct {
	Commits      []*CommitInfo  `json:"commits"`
	Files        []*ChangedFile `json:"files"`
	TotalCommits int            `json:"total_commits"`
	AheadBy      int            `json:"ahead_by"`
	BehindBy     int            `json:"behind_by"`
}

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

// FileResult represents the result of a file operation.
type FileResult struct {
	SHA       string `json:"sha"`
	CommitSHA string `json:"commit_sha"`
}

// TagInfo represents a tag.
type TagInfo struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
}

// ReleaseInfo represents a release.
type ReleaseInfo struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tag_name"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	URL         string    `json:"url"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
}

// CreateReleaseOptions contains options for creating a release.
type CreateReleaseOptions struct {
	TagName    string `json:"tag_name"`
	Target     string `json:"target,omitempty"`
	Title      string `json:"title"`
	Body       string `json:"body,omitempty"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}
