package gitbackend

import (
	"context"
	"fmt"
	"strings"
)

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
		staging := StatusCode(line[0])
		worktree := StatusCode(line[1])
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
	args = append(args, "--")
	args = append(args, opts.Paths...)

	stdout, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		return "", newGitError("Diff", repoPath, stderr, err)
	}
	return stdout, nil
}

// --- Advanced diff operations ---

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

// --- Revision parsing ---

func (b *NativeGitBackend) RevParse(ctx context.Context, repoPath, ref string) (string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"rev-parse", ref}, AuthConfig{})
	if err != nil {
		return "", newGitError("RevParse", repoPath, stderr, err)
	}
	return strings.TrimSpace(stdout), nil
}

func (b *NativeGitBackend) MergeBase(ctx context.Context, repoPath, a, other string) (string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"merge-base", a, other}, AuthConfig{})
	if err != nil {
		return "", newGitError("MergeBase", repoPath, stderr, err)
	}
	return strings.TrimSpace(stdout), nil
}
