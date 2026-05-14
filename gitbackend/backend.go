package gitbackend

import (
	"context"
	"io"
)

type AuthConfig struct {
	Type     string // ssh, http_basic, http_token, none
	Username string
	Password string
	Token    string
	SSHKey   string
}

type FetchOptions struct {
	RepoPath string
	Remote   string
	Branches []string
	Tags     bool
	Prune    bool
	Auth     AuthConfig
	Progress io.Writer
}

type PushOptions struct {
	RepoPath string
	Remote   string
	RefSpecs []string
	Force    bool
	Mirror   bool
	Auth     AuthConfig
	Progress io.Writer
}

type FetchResult struct {
	FetchedRefs   []string
	NewBranches   []string
	UpdatedBranch []string
	DeletedBranch []string
	NewTags       []string
}

type PushResult struct {
	PushedRefs []string
	Errors     []string
}

type CommitInfo struct {
	Hash    string
	Message string
	Author  string
	Date    string
}

type GitBackend interface {
	Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error)
	Push(ctx context.Context, opts PushOptions) (*PushResult, error)
	ListRemoteBranches(ctx context.Context, repoPath, remote string) ([]string, error)
	GetCommitsBetween(ctx context.Context, repoPath, from, to string) ([]CommitInfo, error)
	IsAncestor(ctx context.Context, repoPath, ancestor, descendant string) (bool, error)
}
