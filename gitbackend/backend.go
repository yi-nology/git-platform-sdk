package gitbackend

import (
	"context"
	"io"
)

// This file defines the public surface of the gitbackend package: the
// authentication model, the option/result types and the GitBackend interface
// that every backend implementation must satisfy.
//
// Two implementations are provided out of the box:
//   - "native": shells out to the local git binary (full feature coverage).
//   - "gogit":  pure-Go implementation based on go-git/v5 (no git binary
//     required, but some operations such as rebase/stash/raw are stubbed).
//
// Backends are created through the factory (NewGitBackend) which auto-selects
// native when available and falls back to gogit.

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

// AuthType represents the authentication method used for a git operation.
type AuthType string

const (
	// AuthNone disables authentication (public repositories, local repos).
	AuthNone AuthType = "none"
	// AuthHTTPBasic authenticates over HTTPS using HTTP Basic. The username is
	// an arbitrary placeholder and the secret is carried in Password.
	AuthHTTPBasic AuthType = "http_basic"
	// AuthHTTPToken authenticates over HTTPS using a bearer/token sent as the
	// password of HTTP Basic auth (the convention used by most git hosts).
	AuthHTTPToken AuthType = "http_token"
	// AuthSSH authenticates over the SSH protocol using a private key.
	AuthSSH AuthType = "ssh"
)

// AuthConfig holds authentication credentials for git operations. Build one
// with the New*Auth helpers in auth.go when possible.
type AuthConfig struct {
	// Type selects the authentication method. AuthNone performs no auth.
	Type AuthType

	// Username is the HTTP Basic username (ignored for SSH).
	Username string
	// Password is the HTTP Basic password (used together with Username).
	Password string
	// Token is the HTTPS access token (used by AuthHTTPToken).
	Token string

	// SSHKey is the on-disk path to an SSH private key.
	SSHKey string

	// SSHKeyContent holds the raw PEM content of an SSH private key.
	// When set (and SSHKey is empty), a temporary key file is created
	// automatically for the git operation. This is useful for keys stored
	// in a database rather than on disk.
	SSHKeyContent string

	// Passphrase is the passphrase for decrypting the SSH private key.
	// Used together with SSHKey or SSHKeyContent.
	Passphrase string

	// InsecureSkipTLS disables TLS certificate verification for HTTPS
	// operations (equivalent to http.sslVerify=false). It has no effect on
	// SSH operations. Carried on AuthConfig so every network operation that
	// takes auth (Fetch, Push, Clone, Pull, FetchAll, PushTag,
	// TestConnection, ...) honors it uniformly.
	InsecureSkipTLS bool
}

// ---------------------------------------------------------------------------
// Option types
// ---------------------------------------------------------------------------

// FetchOptions contains options for fetching from a remote.
type FetchOptions struct {
	// RepoPath is the local working tree to operate on.
	RepoPath string
	// Remote is the remote name to fetch from (e.g. "origin"). Defaults to
	// the remote configured for the current branch when empty.
	Remote string
	// Branches limits the fetch to the given branch/ref names. When empty all
	// branches are fetched. Full 40-char SHAs are ignored as they are not
	// valid fetch refspecs.
	Branches []string
	// Tags fetches all tags when true.
	Tags bool
	// Prune removes remote-tracking refs that no longer exist on the remote.
	Prune bool
	// Depth limits the fetch to the given number of commits (shallow fetch).
	// Zero means no depth limit.
	Depth           int
	InsecureSkipTLS bool
	Auth            AuthConfig
	// Progress receives human-readable progress output (optional).
	Progress io.Writer
}

// PushOptions contains options for pushing to a remote.
type PushOptions struct {
	RepoPath string
	// Remote is the remote name to push to (e.g. "origin").
	Remote string
	// RefSpecs are the refs to push (e.g. "refs/heads/main:refs/heads/main").
	// Ignored when Mirror is true.
	RefSpecs []string
	// Force forces the push (overwrites remote refs).
	Force bool
	// Mirror pushes all refs as a mirror (--mirror).
	Mirror          bool
	InsecureSkipTLS bool
	Auth            AuthConfig
	// Progress receives human-readable progress output (optional).
	Progress io.Writer
}

// CloneOptions contains options for cloning a repository.
type CloneOptions struct {
	// URL is the remote URL (HTTPS or SSH) to clone from.
	URL string
	// Path is the destination directory for the new working tree.
	Path string
	// Branch checks out the given branch after clone (optional).
	Branch string
	// Depth creates a shallow clone with the given history depth.
	// Zero means a full clone.
	Depth int
	Auth  AuthConfig
	// Progress receives human-readable progress output (optional).
	Progress        io.Writer
	NoCheckout      bool
	SingleBranch    bool
	InsecureSkipTLS bool
}

// MergeOptions contains options for merging a branch into HEAD.
type MergeOptions struct {
	// Message overrides the auto-generated merge commit message.
	Message string
	// Squash merges the branch as a single squashed change without
	// recording merge ancestry.
	Squash bool
	// NoCommit performs the merge but leaves the result staged without
	// committing.
	NoCommit bool
	// FFOnly only allows a fast-forward merge; aborts otherwise.
	FFOnly bool
	// AllowUnrelated permits merging histories with no common ancestor.
	AllowUnrelated bool
}

// DiffOptions contains options for getting a diff.
type DiffOptions struct {
	// From is the starting commit SHA. When both From and To are empty the
	// diff is computed between the working tree and HEAD.
	From string
	// To is the ending commit SHA.
	To string
	// Paths restricts the diff to the given paths (optional).
	Paths []string
}

// ---------------------------------------------------------------------------
// Result types
// ---------------------------------------------------------------------------

// RepoStatus represents the working tree status.
type RepoStatus struct {
	// Branch is the name of the current branch (without the refs/heads/ prefix).
	Branch string
	// IsClean is true when there are no staged, unstaged or untracked changes.
	IsClean bool
	// Staged lists files with changes staged for commit.
	Staged []FileStatus
	// Unstaged lists files with changes in the working tree (not yet staged).
	Unstaged []FileStatus
	// Untracked lists untracked files (not ignored).
	Untracked []string
	// Ahead is the number of local commits not pushed to upstream.
	Ahead int
	// Behind is the number of upstream commits not pulled locally.
	Behind int
}

// StatusCode is the porcelain status code used for a file in the staging area
// or the working tree. These mirror the codes produced by
// `git status --porcelain`.
type StatusCode byte

const (
	StatusUnmodified StatusCode = ' '
	StatusModified   StatusCode = 'M'
	StatusAdded      StatusCode = 'A'
	StatusDeleted    StatusCode = 'D'
	StatusRenamed    StatusCode = 'R'
	StatusCopied     StatusCode = 'C'
	StatusUntracked  StatusCode = '?'
	StatusIgnored    StatusCode = '!'
)

// FileStatus represents the status of a single file.
type FileStatus struct {
	Path string
	// Worktree is the status of the file in the working tree.
	Worktree StatusCode
	// Staging is the status of the file in the staging area (index).
	Staging StatusCode
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
	// PushedRefs are the refs that were pushed.
	PushedRefs []string
	// Errors collects per-ref errors reported by the remote (if any).
	Errors []string
}

// CommitInfo represents a git commit.
type CommitInfo struct {
	// Hash is the full 40-character commit SHA.
	Hash string
	// Message is the commit message.
	Message string
	// Author is the commit author's name.
	Author string
	// Date is the author date formatted as RFC3339.
	Date string
}

// TagInfo represents a git tag.
type TagInfo struct {
	Name string
	// Hash is the SHA the tag points at (the tag object for annotated tags,
	// or the commit for lightweight tags).
	Hash    string
	Message string
	Author  string
}

// StashEntry represents a stash entry.
type StashEntry struct {
	// Index is the stash position (0 is the most recent, i.e. stash@{0}).
	Index int
	// Message is the stash list line as produced by `git stash list`.
	Message string
}

// BranchDetail represents a branch with extended information.
type BranchDetail struct {
	Name string
	Hash string
	// IsCurrent is true when this is the checked-out branch.
	IsCurrent bool
	// IsRemote is true for remote-tracking branches (refs/remotes/*).
	IsRemote bool
	// Remote is the remote name for remote branches (e.g. "origin").
	Remote string
	// Upstream is the configured upstream tracking branch (e.g. "origin/main").
	Upstream string
	Author   string
	Email    string
	// Date is the tip commit's author date formatted as RFC3339.
	Date string
	// Message is the subject line of the tip commit.
	Message string
}

// TreeEntryType identifies whether a TreeEntry is a file or a directory.
type TreeEntryType string

const (
	TreeEntryFile TreeEntryType = "file"
	TreeEntryDir  TreeEntryType = "dir"
)

// TreeEntry represents a file or directory in a git tree.
type TreeEntry struct {
	Name string
	// Path is the full path relative to the repository root.
	Path string
	// Type is either TreeEntryFile or TreeEntryDir.
	Type TreeEntryType
	// Mode is the git file mode (e.g. "100644").
	Mode string
	Hash string
	// Size is the file size in bytes (populated for files in recursive mode).
	Size int64
}

// BlobEncoding is the encoding used for BlobContent.Content.
type BlobEncoding string

const (
	EncodingUTF8   BlobEncoding = "utf-8"
	EncodingBase64 BlobEncoding = "base64"
)

// BlobContent represents the content of a file at a given revision.
type BlobContent struct {
	// Content is the file payload. When IsBinary it is base64-encoded.
	Content string
	// Encoding is either EncodingUTF8 or EncodingBase64.
	Encoding BlobEncoding
	// Size is the decoded payload size in bytes.
	Size int64
	// IsBinary is true when the blob is detected as binary.
	IsBinary bool
}

// ---------------------------------------------------------------------------
// GitBackend interface
// ---------------------------------------------------------------------------

// GitBackend is the interface for low-level, local git operations.
//
// Every method takes a context (for cancellation/timeouts) and the absolute
// path of a working tree. Commit/ref arguments are full 40-character SHAs
// unless stated otherwise; dates are RFC3339 formatted strings. Methods
// return wrapped errors created by newGitError (see errors.go); use the
// IsNotFound / IsMergeConflict helpers to classify them.
type GitBackend interface {
	// --- Core operations ---

	// Fetch downloads objects and refs from a remote into the local repository
	// without touching the working tree.
	Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error)
	// FetchAll fetches tags and every branch from all configured remotes.
	FetchAll(ctx context.Context, repoPath string, auth AuthConfig) error
	// Push uploads local refs to a remote.
	Push(ctx context.Context, opts PushOptions) (*PushResult, error)
	// Pull fetches from and integrates with the current branch (fetch + merge).
	Pull(ctx context.Context, repoPath string, remote string, branch string, auth AuthConfig) error
	// Clone clones a remote repository into a new local working tree.
	Clone(ctx context.Context, opts CloneOptions) error
	// Init creates a new empty git repository at repoPath.
	Init(ctx context.Context, repoPath string) error
	// RunRaw executes an arbitrary git command and returns its combined output.
	// Only supported by the native backend.
	RunRaw(ctx context.Context, repoPath string, args []string) (stdout string, stderr string, err error)

	// --- Branch operations ---

	// ListRemoteBranches lists branches that exist on the given remote.
	ListRemoteBranches(ctx context.Context, repoPath string, remote string) ([]string, error)
	// ListLocalBranches lists local branches (refs/heads/*).
	ListLocalBranches(ctx context.Context, repoPath string) ([]string, error)
	// ListBranches lists both local and remote branches with extended details.
	ListBranches(ctx context.Context, repoPath string) ([]BranchDetail, error)
	// CreateBranch creates a branch pointing at ref (or HEAD when empty).
	CreateBranch(ctx context.Context, repoPath string, branch string, ref string) error
	// DeleteBranch deletes a local branch.
	DeleteBranch(ctx context.Context, repoPath string, branch string) error
	// RenameBranch renames a local branch.
	RenameBranch(ctx context.Context, repoPath string, oldName string, newName string) error
	// Checkout switches the working tree to the given branch.
	Checkout(ctx context.Context, repoPath string, branch string) error
	// GetCurrentBranch returns the name of the checked-out branch.
	GetCurrentBranch(ctx context.Context, repoPath string) (string, error)
	// GetBranchSyncInfo returns how many commits branch is ahead/behind upstream.
	GetBranchSyncInfo(ctx context.Context, repoPath string, branch string, upstream string) (ahead int, behind int, err error)

	// --- Status and diff ---

	// GetStatus returns the working tree status.
	GetStatus(ctx context.Context, repoPath string) (*RepoStatus, error)
	// Diff returns a textual diff. With empty From/To it diffs the working tree
	// against HEAD; otherwise it diffs the two commits.
	Diff(ctx context.Context, repoPath string, opts DiffOptions) (string, error)

	// --- Commit operations ---

	// GetCommitsBetween returns commits in the range (from, to].
	GetCommitsBetween(ctx context.Context, repoPath string, from string, to string) ([]CommitInfo, error)
	// IsAncestor reports whether ancestor is an ancestor of descendant.
	IsAncestor(ctx context.Context, repoPath string, ancestor string, descendant string) (bool, error)
	// Merge merges branch into the current HEAD according to opts.
	Merge(ctx context.Context, repoPath string, branch string, opts MergeOptions) error
	// CherryPick applies the changes of commitHash onto the current HEAD.
	CherryPick(ctx context.Context, repoPath string, commitHash string) error
	// Rebase rebases the current branch onto onto.
	Rebase(ctx context.Context, repoPath string, onto string) error
	// RebaseAbort aborts an in-progress rebase.
	RebaseAbort(ctx context.Context, repoPath string) error
	// RebaseContinue continues an in-progress rebase after resolving conflicts.
	RebaseContinue(ctx context.Context, repoPath string) error

	// --- Remote operations ---

	// GetRemotes returns the names of all configured remotes.
	GetRemotes(ctx context.Context, repoPath string) ([]string, error)
	// AddRemote adds a new remote.
	AddRemote(ctx context.Context, repoPath string, name string, url string) error
	// RemoveRemote removes a configured remote.
	RemoveRemote(ctx context.Context, repoPath string, name string) error
	// GetRemoteURL returns the URL of a named remote.
	GetRemoteURL(ctx context.Context, repoPath string, name string) (string, error)
	// TestConnection verifies that url is reachable with the given credentials.
	TestConnection(ctx context.Context, url string, auth AuthConfig) error

	// --- Tag operations ---

	// CreateTag creates a tag pointing at ref (or HEAD when empty).
	CreateTag(ctx context.Context, repoPath string, name string, ref string) error
	// DeleteTag deletes a local tag.
	DeleteTag(ctx context.Context, repoPath string, name string) error
	// PushTag pushes a single tag to remote.
	PushTag(ctx context.Context, repoPath string, remote string, name string, auth AuthConfig) error
	// GetTagList lists all tags with their metadata.
	GetTagList(ctx context.Context, repoPath string) ([]TagInfo, error)

	// --- File operations ---

	// GetFileAtRevision returns the raw bytes of path at the given ref.
	GetFileAtRevision(ctx context.Context, repoPath string, path string, ref string) ([]byte, error)
	// GetFileHistory returns commits that touched path (most recent first),
	// limited to limit entries (0 means all).
	GetFileHistory(ctx context.Context, repoPath string, path string, limit int) ([]CommitInfo, error)
	// GetTree lists entries under dirPath at ref. When recursive is true it
	// lists all files (not directories) below dirPath.
	GetTree(ctx context.Context, repoPath string, ref string, dirPath string, recursive bool) ([]TreeEntry, error)
	// GetBlob returns the content of filePath at ref.
	GetBlob(ctx context.Context, repoPath string, ref string, filePath string) (*BlobContent, error)
	// GetCommit returns metadata for a single commit.
	GetCommit(ctx context.Context, repoPath string, hash string) (*CommitInfo, error)

	// --- Stash operations ---

	// StashList lists the stash entries.
	StashList(ctx context.Context, repoPath string) ([]StashEntry, error)
	// StashSave saves the working tree and index changes to the stash.
	StashSave(ctx context.Context, repoPath string, message string) error
	// StashApply applies a stash entry without removing it from the stash list.
	StashApply(ctx context.Context, repoPath string, index int) error
	// StashPop applies a stash entry and then drops it from the stash list.
	StashPop(ctx context.Context, repoPath string, index int) error
	// StashDrop removes a single stash entry.
	StashDrop(ctx context.Context, repoPath string, index int) error
	// StashClear removes all stash entries.
	StashClear(ctx context.Context, repoPath string) error

	// --- Config operations ---

	// GetConfig reads a config value by dotted key (e.g. "user.name").
	GetConfig(ctx context.Context, repoPath string, key string) (string, error)
	// SetConfig sets a config value by dotted key (e.g. "user.email").
	SetConfig(ctx context.Context, repoPath string, key string, value string) error

	// --- Advanced operations ---

	// RevParse resolves a revision (ref/short-SHA/etc.) to a full SHA.
	RevParse(ctx context.Context, repoPath string, ref string) (string, error)
	// MergeBase returns the best common ancestor of two commits.
	MergeBase(ctx context.Context, repoPath string, a string, b string) (string, error)
	// DiffNames returns the names of files changed between two commits.
	DiffNames(ctx context.Context, repoPath string, from string, to string) ([]string, error)
	// DeletedFiles returns files deleted between two commits.
	DeletedFiles(ctx context.Context, repoPath string, from string, to string) ([]string, error)
	// CheckoutRef force-checks out an arbitrary ref in detached HEAD.
	CheckoutRef(ctx context.Context, repoPath string, ref string) error
	// CheckoutFiles restores the given files from ref into the working tree.
	CheckoutFiles(ctx context.Context, repoPath string, ref string, files []string) error
	// Add stages the given files into the index.
	Add(ctx context.Context, repoPath string, files []string) error
	// CommitWithIdentity creates a commit using an explicit author identity.
	CommitWithIdentity(ctx context.Context, repoPath string, name string, email string, message string) error
}
