package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Platform string

const (
	PlatformGitLab      Platform = "gitlab"
	PlatformGitHub      Platform = "github"
	PlatformGitea       Platform = "gitea"
	PlatformGitee       Platform = "gitee"
	PlatformForgejo     Platform = "forgejo"
	PlatformTencentCode Platform = "tencent_code"
)

type Provider interface {
	Platform() Platform
	ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error)
	GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error)
	CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error)
	GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error)
	ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error)
	MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error)
	CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error)
	CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error)
	DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error
	ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error)
	ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error)
	ValidateWebhookSignature(r *http.Request, secret string) error
	TestConnection(ctx context.Context) (*TestConnectionResult, error)
	ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error)
	CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error)
	DeleteBranch(ctx context.Context, owner, repo, branch string) error

	GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error)
	GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error)
	CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error)
	DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error
	CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error)
	CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error
	GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error)
	UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error

	UpdateCR(ctx context.Context, owner, repo string, number int, opts UpdateCROptions) (*ChangeRequest, error)
	ReopenCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error)
	ListCRComments(ctx context.Context, owner, repo string, number int) ([]*CRComment, error)
	ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*CRCommit, error)
	ForkRepo(ctx context.Context, owner, repo string, opts ForkRepoOptions) (*PlatformRepo, error)
	DeleteRepo(ctx context.Context, owner, repo string) error
	UpdateRepo(ctx context.Context, owner, repo string, opts UpdateRepoOptions) (*PlatformRepo, error)
	GetCommit(ctx context.Context, owner, repo, sha string) (*CommitInfo, error)
	ListCommits(ctx context.Context, owner, repo string, opts ListCommitsOptions) ([]*CommitInfo, error)
	CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error)
	CreateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error)
	UpdateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error)
	DeleteFile(ctx context.Context, owner, repo string, opts FileDeleteOptions) (*FileResult, error)
	ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error)
	ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error)
	CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error)
	GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error)
}

type PlatformBranch struct {
	Name string `json:"name"`
}

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

type CRState string

const (
	CRStateOpened CRState = "opened"
	CRStateMerged CRState = "merged"
	CRStateClosed CRState = "closed"
)

type ChangeRequest struct {
	ID           int64     `json:"id"`
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	State        CRState   `json:"state"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	Author       *CRUser   `json:"author"`
	Reviewers    []*CRUser `json:"reviewers"`
	Labels       []string  `json:"labels"`
	MergeStatus  string    `json:"merge_status"`
	WebURL       string    `json:"web_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CRUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type ListRepoOptions struct {
	Owner   string `json:"owner"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

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

type ListCROptions struct {
	Owner        string  `json:"owner"`
	Repo         string  `json:"repo"`
	State        CRState `json:"state"`
	SourceBranch string  `json:"source_branch"`
	TargetBranch string  `json:"target_branch"`
	Page         int     `json:"page"`
	PerPage      int     `json:"per_page"`
}

type MergeCROptions struct {
	MergeCommitMessage string `json:"merge_commit_message"`
	Squash             bool   `json:"squash"`
	RemoveSourceBranch bool   `json:"remove_source_branch"`
}

type CreateWebhookOptions struct {
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	URL    string   `json:"url"`
	Secret string   `json:"secret"`
	Events []string `json:"events"`
}

type PlatformWebhook struct {
	ID     int64    `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

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

type EventRepo struct {
	FullName string `json:"full_name"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
}

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

type MergeDiff struct {
	Files    []*ChangedFile
	TotalAdd int
	TotalDel int
	RawDiff  string
}

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

type DiscussionOptions struct {
	Body     string `json:"body"`
	FilePath string `json:"file_path,omitempty"`
	NewLine  int    `json:"new_line,omitempty"`
	OldLine  int    `json:"old_line,omitempty"`
}

type CommitStatusOptions struct {
	State       string `json:"state"`
	Context     string `json:"context"`
	Description string `json:"description"`
	TargetURL   string `json:"target_url,omitempty"`
}

type UpdateCROptions struct {
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	TargetBranch string `json:"target_branch,omitempty"`
}

type CRComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	Author    *CRUser   `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CRCommit struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    *CRUser   `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

type ForkRepoOptions struct {
	Organization string `json:"organization,omitempty"`
	Name         string `json:"name,omitempty"`
}

type UpdateRepoOptions struct {
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	Private       *bool  `json:"private,omitempty"`
}

type CommitInfo struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    *CRUser   `json:"author"`
	Committer *CRUser   `json:"committer"`
	CreatedAt time.Time `json:"created_at"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
}

type ListCommitsOptions struct {
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Branch  string `json:"branch,omitempty"`
	Since   string `json:"since,omitempty"`
	Until   string `json:"until,omitempty"`
}

type CompareResult struct {
	Commits      []*CommitInfo `json:"commits"`
	Files        []*ChangedFile `json:"files"`
	TotalCommits int           `json:"total_commits"`
	AheadBy      int           `json:"ahead_by"`
	BehindBy     int           `json:"behind_by"`
}

type FileOptions struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Message string `json:"message"`
	Branch  string `json:"branch,omitempty"`
	SHA     string `json:"sha,omitempty"`
	Author  string `json:"author,omitempty"`
	Email   string `json:"email,omitempty"`
}

type FileDeleteOptions struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Branch  string `json:"branch,omitempty"`
	SHA     string `json:"sha,omitempty"`
	Author  string `json:"author,omitempty"`
	Email   string `json:"email,omitempty"`
}

type FileResult struct {
	SHA     string `json:"sha"`
	CommitSHA string `json:"commit_sha"`
}

type TagInfo struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
}

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

type CreateReleaseOptions struct {
	TagName     string `json:"tag_name"`
	Target      string `json:"target,omitempty"`
	Title       string `json:"title"`
	Body        string `json:"body,omitempty"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}
