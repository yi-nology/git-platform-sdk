package gitbackend

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type NativeGitBackend struct {
	gitPath string
	logger  Logger
}

func NewNativeGitBackend(opts Options) (*NativeGitBackend, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitNotFound, err)
	}
	logger := opts.Logger
	if logger == nil {
		logger = NewNoopLogger()
	}
	return &NativeGitBackend{gitPath: path, logger: logger}, nil
}

// --- Core operations ---

func (b *NativeGitBackend) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	args := []string{"fetch"}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.Prune {
		args = append(args, "--prune")
	}
	if opts.Tags {
		args = append(args, "--tags")
	} else {
		args = append(args, "--no-tags")
	}
	args = append(args, opts.Remote)
	if len(opts.RefSpecs) > 0 {
		args = append(args, opts.RefSpecs...)
	} else if len(opts.Branches) > 0 {
		args = append(args, opts.Branches...)
	}

	stdout, stderr, err := b.runGit(ctx, opts.RepoPath, args, opts.Auth)
	if err != nil {
		return nil, newGitError("Fetch", opts.RepoPath, stderr, err)
	}
	return &FetchResult{FetchedRefs: parseFetchRefs(stdout + stderr)}, nil
}

func (b *NativeGitBackend) Push(ctx context.Context, opts PushOptions) (*PushResult, error) {
	args := []string{"push", opts.Remote}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.Mirror {
		args = []string{"push", "--mirror", opts.Remote}
	} else {
		args = append(args, opts.RefSpecs...)
	}

	stdout, stderr, err := b.runGit(ctx, opts.RepoPath, args, opts.Auth)
	if err != nil {
		return nil, newGitError("Push", opts.RepoPath, stderr, err)
	}
	return &PushResult{PushedRefs: parsePushRefs(stdout + stderr)}, nil
}

func (b *NativeGitBackend) Clone(ctx context.Context, opts CloneOptions) error {
	args := []string{"clone"}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}
	if opts.NoCheckout {
		args = append(args, "--no-checkout")
	}
	args = append(args, opts.URL, opts.Path)

	_, stderr, err := b.runGit(ctx, "", args, opts.Auth)
	if err != nil {
		return newGitError("Clone", opts.Path, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) Init(ctx context.Context, repoPath string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"init"}, AuthConfig{})
	if err != nil {
		return newGitError("Init", repoPath, stderr, err)
	}
	return nil
}

// --- Branch operations ---

func (b *NativeGitBackend) ListRemoteBranches(ctx context.Context, repoPath, remote string) ([]string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"branch", "-r", "--list", remote + "/*"}, AuthConfig{})
	if err != nil {
		return nil, newGitError("ListRemoteBranches", repoPath, stderr, err)
	}

	var branches []string
	prefix := remote + "/"
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, prefix)
		if line != "" && !strings.Contains(line, "HEAD") {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func (b *NativeGitBackend) CreateBranch(ctx context.Context, repoPath, branch, ref string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"branch", branch, ref}, AuthConfig{})
	if err != nil {
		if strings.Contains(stderr, "already exists") {
			return newGitError("CreateBranch", repoPath, stderr, ErrBranchExists)
		}
		return newGitError("CreateBranch", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) DeleteBranch(ctx context.Context, repoPath, branch string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"branch", "-D", branch}, AuthConfig{})
	if err != nil {
		if strings.Contains(stderr, "not found") || strings.Contains(stderr, "branch") {
			return newGitError("DeleteBranch", repoPath, stderr, ErrBranchNotFound)
		}
		return newGitError("DeleteBranch", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) Checkout(ctx context.Context, repoPath, branch string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"checkout", branch}, AuthConfig{})
	if err != nil {
		return newGitError("Checkout", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"rev-parse", "--abbrev-ref", "HEAD"}, AuthConfig{})
	if err != nil {
		return "", newGitError("GetCurrentBranch", repoPath, stderr, err)
	}
	return strings.TrimSpace(stdout), nil
}

// --- Status and diff ---

func (b *NativeGitBackend) GetStatus(ctx context.Context, repoPath string) (*RepoStatus, error) {
	status := &RepoStatus{}

	// Get current branch
	branch, err := b.GetCurrentBranch(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	status.Branch = branch

	// Get ahead/behind
	ahead, behind, _ := b.getAheadBehind(ctx, repoPath)
	status.Ahead = ahead
	status.Behind = behind

	// Parse porcelain status
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"status", "--porcelain=v1"}, AuthConfig{})
	if err != nil {
		return nil, newGitError("GetStatus", repoPath, stderr, err)
	}

	for _, line := range strings.Split(stdout, "\n") {
		if len(line) < 3 {
			continue
		}
		staging := line[0]
		worktree := line[1]
		path := strings.TrimSpace(line[3:])

		if staging == '?' && worktree == '?' {
			status.Untracked = append(status.Untracked, path)
		} else {
			fs := FileStatus{Path: path, Staging: staging, Worktree: worktree}
			if staging != ' ' && staging != '?' {
				status.Staged = append(status.Staged, fs)
			}
			if worktree != ' ' && worktree != '?' {
				status.Unstaged = append(status.Unstaged, fs)
			}
		}
	}

	status.IsClean = len(status.Staged) == 0 && len(status.Unstaged) == 0 && len(status.Untracked) == 0
	return status, nil
}

func (b *NativeGitBackend) getAheadBehind(ctx context.Context, repoPath string) (int, int, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"rev-list", "--left-right", "--count", "HEAD...@{upstream}"}, AuthConfig{})
	if err != nil {
		return 0, 0, newGitError("getAheadBehind", repoPath, stderr, err)
	}
	parts := strings.Fields(strings.TrimSpace(stdout))
	if len(parts) != 2 {
		return 0, 0, nil
	}
	ahead, behind := 0, 0
	fmt.Sscanf(parts[0], "%d", &ahead)
	fmt.Sscanf(parts[1], "%d", &behind)
	return ahead, behind, nil
}

func (b *NativeGitBackend) Diff(ctx context.Context, repoPath string, opts DiffOptions) (string, error) {
	args := []string{"diff"}
	if opts.From != "" && opts.To != "" {
		args = append(args, opts.From, opts.To)
	}
	args = append(args, "--", )
	args = append(args, opts.Paths...)

	stdout, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		return "", newGitError("Diff", repoPath, stderr, err)
	}
	return stdout, nil
}

// --- Commit operations ---

func (b *NativeGitBackend) GetCommitsBetween(ctx context.Context, repoPath, from, to string) ([]CommitInfo, error) {
	rangeArg := fmt.Sprintf("%s..%s", from, to)
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{
		"log", rangeArg, "--pretty=format:%H|%s|%an|%ai",
	}, AuthConfig{})
	if err != nil {
		return nil, newGitError("GetCommitsBetween", repoPath, stderr, err)
	}

	var commits []CommitInfo
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, CommitInfo{
			Hash: parts[0], Message: parts[1], Author: parts[2], Date: parts[3],
		})
	}
	return commits, nil
}

func (b *NativeGitBackend) IsAncestor(ctx context.Context, repoPath, ancestor, descendant string) (bool, error) {
	_, _, err := b.runGit(ctx, repoPath, []string{
		"merge-base", "--is-ancestor", ancestor, descendant,
	}, AuthConfig{})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (b *NativeGitBackend) Merge(ctx context.Context, repoPath, branch string, opts MergeOptions) error {
	args := []string{"merge"}
	if opts.Squash {
		args = append(args, "--squash")
	}
	if opts.FFOnly {
		args = append(args, "--ff-only")
	}
	if opts.NoCommit {
		args = append(args, "--no-commit")
	}
	if opts.AllowUnrelated {
		args = append(args, "--allow-unrelated-histories")
	}
	if opts.Message != "" {
		args = append(args, "-m", opts.Message)
	}
	args = append(args, branch)

	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		if strings.Contains(stderr, "CONFLICT") || strings.Contains(stderr, "conflict") {
			return newGitError("Merge", repoPath, stderr, ErrMergeConflict)
		}
		return newGitError("Merge", repoPath, stderr, err)
	}
	return nil
}

// --- Remote operations ---

func (b *NativeGitBackend) AddRemote(ctx context.Context, repoPath, name, url string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"remote", "add", name, url}, AuthConfig{})
	if err != nil {
		return newGitError("AddRemote", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) RemoveRemote(ctx context.Context, repoPath, name string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"remote", "remove", name}, AuthConfig{})
	if err != nil {
		return newGitError("RemoveRemote", repoPath, stderr, err)
	}
	return nil
}

// --- Tag operations ---

func (b *NativeGitBackend) CreateTag(ctx context.Context, repoPath, name, ref string) error {
	args := []string{"tag", name}
	if ref != "" {
		args = append(args, ref)
	}
	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		if strings.Contains(stderr, "already exists") {
			return newGitError("CreateTag", repoPath, stderr, ErrTagExists)
		}
		return newGitError("CreateTag", repoPath, stderr, err)
	}
	return nil
}

// --- File operations ---

func (b *NativeGitBackend) GetFileAtRevision(ctx context.Context, repoPath, path, ref string) ([]byte, error) {
	spec := fmt.Sprintf("%s:%s", ref, path)
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"show", spec}, AuthConfig{})
	if err != nil {
		if strings.Contains(stderr, "does not exist") || strings.Contains(stderr, "exists on disk") {
			return nil, newGitError("GetFileAtRevision", repoPath, stderr, ErrFileNotFound)
		}
		return nil, newGitError("GetFileAtRevision", repoPath, stderr, err)
	}
	return []byte(stdout), nil
}

// --- Internal helpers ---

func (b *NativeGitBackend) runGit(ctx context.Context, repoPath string, args []string, auth AuthConfig) (string, string, error) {
	cmd := exec.CommandContext(ctx, b.gitPath, args...)
	if repoPath != "" {
		cmd.Dir = repoPath
	}
	b.configureAuth(cmd, auth)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func (b *NativeGitBackend) configureAuth(cmd *exec.Cmd, auth AuthConfig) {
	if cmd.Env == nil {
		cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	}

	switch auth.Type {
	case AuthHTTPBasic, AuthHTTPToken:
		token := auth.Token
		if token == "" {
			token = auth.Password
		}
		if token != "" {
			cmd.Env = append(cmd.Env, "GIT_ASKPASS=echo")
			cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_PASSWORD=%s", token))
		}
	case AuthSSH:
		if auth.SSHKey != "" {
			sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", auth.SSHKey)
			cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))
		}
	}
}

// --- Advanced operations ---

func (b *NativeGitBackend) RevParse(ctx context.Context, repoPath, ref string) (string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"rev-parse", ref}, AuthConfig{})
	if err != nil {
		return "", newGitError("RevParse", repoPath, stderr, err)
	}
	return strings.TrimSpace(stdout), nil
}

func (b *NativeGitBackend) MergeBase(ctx context.Context, repoPath, a, commitB string) (string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"merge-base", a, commitB}, AuthConfig{})
	if err != nil {
		return "", newGitError("MergeBase", repoPath, stderr, err)
	}
	result := strings.TrimSpace(stdout)
	return result, nil
}

func (b *NativeGitBackend) DiffNames(ctx context.Context, repoPath, from, to string) ([]string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"diff", "--name-only", from, to}, AuthConfig{})
	if err != nil {
		return nil, newGitError("DiffNames", repoPath, stderr, err)
	}
	var result []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func (b *NativeGitBackend) DeletedFiles(ctx context.Context, repoPath, from, to string) ([]string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"diff", "--name-only", "--diff-filter=D", from, to}, AuthConfig{})
	if err != nil {
		return nil, newGitError("DeletedFiles", repoPath, stderr, err)
	}
	var result []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func (b *NativeGitBackend) CheckoutRef(ctx context.Context, repoPath, ref string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"checkout", "--force", ref}, AuthConfig{})
	if err != nil {
		return newGitError("CheckoutRef", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) CheckoutFiles(ctx context.Context, repoPath, ref string, files []string) error {
	args := append([]string{"checkout", ref, "--"}, files...)
	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		return newGitError("CheckoutFiles", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) Add(ctx context.Context, repoPath string, files []string) error {
	args := append([]string{"add"}, files...)
	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		return newGitError("Add", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) CommitWithIdentity(ctx context.Context, repoPath, name, email, message string) error {
	args := []string{
		"-c", fmt.Sprintf("user.name=%s", name),
		"-c", fmt.Sprintf("user.email=%s", email),
		"-c", "commit.gpgsign=false",
		"commit", "--allow-empty", "-m", message,
	}
	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		return newGitError("CommitWithIdentity", repoPath, stderr, err)
	}
	return nil
}

// --- Output parsers ---

func parseFetchRefs(output string) []string {
	var refs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, " ") && strings.Contains(line, "->") {
			parts := strings.SplitN(line, "->", 2)
			if len(parts) == 2 {
				refs = append(refs, strings.TrimSpace(parts[1]))
			}
		}
	}
	return refs
}

func parsePushRefs(output string) []string {
	var refs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "To ") || strings.Contains(line, "..") {
			if strings.Contains(line, " ") {
				for _, p := range strings.Fields(line) {
					if strings.Contains(p, ":") || strings.Contains(p, "..") {
						refs = append(refs, p)
					}
				}
			}
		}
	}
	return refs
}
