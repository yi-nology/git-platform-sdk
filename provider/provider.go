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

// CompareResult represents the result of comparing two commits.
type CompareResult struct {
	Commits      []*CommitInfo  `json:"commits"`
	Files        []*ChangedFile `json:"files"`
	TotalCommits int            `json:"total_commits"`
	AheadBy      int            `json:"ahead_by"`
	BehindBy     int            `json:"behind_by"`
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
