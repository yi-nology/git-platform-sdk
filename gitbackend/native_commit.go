package gitbackend

import (
	"context"
	"fmt"
	"strings"
)

// --- Commit operations ---

func (b *NativeGitBackend) GetCommitsBetween(ctx context.Context, repoPath, from, to string) ([]CommitInfo, error) {
	var rangeArg string
	if from == "" {
		rangeArg = to
	} else {
		rangeArg = fmt.Sprintf("%s..%s", from, to)
	}
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

	stdout, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		if isConflictOutput(stdout, stderr) {
			return newGitError("Merge", repoPath, stderr, ErrMergeConflict)
		}
		return newGitError("Merge", repoPath, stderr, err)
	}
	return nil
}

// isConflictOutput reports whether git's output announces merge conflicts.
// git prints CONFLICT lines on stdout (stderr stays empty on a content
// conflict), so both streams must be inspected.
func isConflictOutput(stdout, stderr string) bool {
	out := stdout + stderr
	return strings.Contains(out, "CONFLICT") || strings.Contains(out, "conflict")
}

func (b *NativeGitBackend) CherryPick(ctx context.Context, repoPath, commitHash string) error {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"cherry-pick", commitHash}, AuthConfig{})
	if err != nil {
		if isConflictOutput(stdout, stderr) {
			return newGitError("CherryPick", repoPath, stderr, ErrMergeConflict)
		}
		return newGitError("CherryPick", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) Rebase(ctx context.Context, repoPath, onto string) error {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"rebase", onto}, AuthConfig{})
	if err != nil {
		if isConflictOutput(stdout, stderr) {
			return newGitError("Rebase", repoPath, stderr, ErrMergeConflict)
		}
		return newGitError("Rebase", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) RebaseAbort(ctx context.Context, repoPath string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"rebase", "--abort"}, AuthConfig{})
	if err != nil {
		return newGitError("RebaseAbort", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) RebaseContinue(ctx context.Context, repoPath string) error {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"rebase", "--continue"}, AuthConfig{})
	if err != nil {
		if isConflictOutput(stdout, stderr) {
			return newGitError("RebaseContinue", repoPath, stderr, ErrMergeConflict)
		}
		return newGitError("RebaseContinue", repoPath, stderr, err)
	}
	return nil
}

// --- Commit query and index operations ---

func (b *NativeGitBackend) GetCommit(ctx context.Context, repoPath, hashStr string) (*CommitInfo, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{
		"log", "-1", "--pretty=format:%H|%s|%an|%ai", hashStr,
	}, AuthConfig{})
	if err != nil {
		return nil, newGitError("GetCommit", repoPath, stderr, err)
	}
	parts := strings.SplitN(strings.TrimSpace(stdout), "|", 4)
	if len(parts) < 4 {
		return nil, newGitError("GetCommit", repoPath, "", fmt.Errorf("unexpected log format"))
	}
	return &CommitInfo{
		Hash:    parts[0],
		Message: parts[1],
		Author:  parts[2],
		Date:    parts[3],
	}, nil
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
