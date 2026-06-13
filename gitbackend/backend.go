package gitbackend

import (
	"context"
	"io"
)

// AuthType represents the authentication method.
type AuthType string

const (
	AuthNone      AuthType = "none"
	AuthHTTPBasic AuthType = "http_basic"
	AuthHTTPToken AuthType = "http_token"
	AuthSSH       AuthType = "ssh"
)

// AuthConfig holds authentication credentials for git operations.
type AuthConfig struct {
	Type     AuthType
	Username string
	Password string
	Token    string
	SSHKey   string
}

// FetchOptions contains options for fetching from a remote.
type FetchOptions struct {
	RepoPath string
	Remote   string
	Branches []string
	Tags     bool
	Prune    bool
	Auth     AuthConfig
	Progress io.Writer
}

// PushOptions contains options for pushing to a remote.
type PushOptions struct {
	RepoPath string
	Remote   string
	RefSpecs []string
	Force    bool
	Mirror   bool
	Auth     AuthConfig
	Progress io.Writer
}

// CloneOptions contains options for cloning a repository.
type CloneOptions struct {
	URL        string
	Path       string
	Branch     string
	Depth      int
	Auth       AuthConfig
	Progress   io.Writer
	NoCheckout bool
}

// MergeOptions contains options for merging a branch.
type MergeOptions struct {
	Message      string
	Squash       bool
	NoCommit     bool
	FFOnly       bool
	AllowUnrelated bool
}

// DiffOptions contains options for getting a diff.
type DiffOptions struct {
	From  string
	To    string
	Paths []string
}

// RepoStatus represents the working tree status.
type RepoStatus struct {
	Branch       string
	IsClean      bool
	Staged       []FileStatus
	Unstaged     []FileStatus
	Untracked    []string
	Ahead        int
	Behind       int
}

// FileStatus represents the status of a single file.
type FileStatus struct {
	Path     string
	Worktree byte // M=modified, D=deleted, A=added, ?=untracked
	Staging  byte // M=modified, D=deleted, A=added
}

// FetchResult contains the result of a fetch operation.
type FetchResult struct {
	FetchedRefs   []string
	NewBranches   []string
	UpdatedBranch []string
	DeletedBranch []string
	NewTags       []string
}

// PushResult contains the result of a push operation.
type PushResult struct {
	PushedRefs []string
	Errors     []string
}

// CommitInfo represents a git commit.
type CommitInfo struct {
	Hash    string
	Message string
	Author  string
	Date    string
}

// GitBackend is the interface for low-level git operations.
type GitBackend interface {
	// Core operations
	Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error)
	Push(ctx context.Context, opts PushOptions) (*PushResult, error)
	Clone(ctx context.Context, opts CloneOptions) error
	Init(ctx context.Context, repoPath string) error

	// Branch operations
	ListRemoteBranches(ctx context.Context, repoPath string, remote string) ([]string, error)
	CreateBranch(ctx context.Context, repoPath string, branch string, ref string) error
	DeleteBranch(ctx context.Context, repoPath string, branch string) error
	Checkout(ctx context.Context, repoPath string, branch string) error
	GetCurrentBranch(ctx context.Context, repoPath string) (string, error)

	// Status and diff
	GetStatus(ctx context.Context, repoPath string) (*RepoStatus, error)
	Diff(ctx context.Context, repoPath string, opts DiffOptions) (string, error)

	// Commit operations
	GetCommitsBetween(ctx context.Context, repoPath string, from string, to string) ([]CommitInfo, error)
	IsAncestor(ctx context.Context, repoPath string, ancestor string, descendant string) (bool, error)
	Merge(ctx context.Context, repoPath string, branch string, opts MergeOptions) error

	// Remote operations
	AddRemote(ctx context.Context, repoPath string, name string, url string) error
	RemoveRemote(ctx context.Context, repoPath string, name string) error

	// Tag operations
	CreateTag(ctx context.Context, repoPath string, name string, ref string) error

	// File operations
	GetFileAtRevision(ctx context.Context, repoPath string, path string, ref string) ([]byte, error)
}
