package gitbackend

import (
	"context"
	"fmt"
	"strings"
)

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

func (b *NativeGitBackend) RenameBranch(ctx context.Context, repoPath, oldName, newName string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"branch", "-m", oldName, newName}, AuthConfig{})
	if err != nil {
		return newGitError("RenameBranch", repoPath, stderr, err)
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

// --- Extended branch operations ---

func (b *NativeGitBackend) ListLocalBranches(ctx context.Context, repoPath string) ([]string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"branch", "--format=%(refname:short)"}, AuthConfig{})
	if err != nil {
		return nil, newGitError("ListLocalBranches", repoPath, stderr, err)
	}
	var branches []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func (b *NativeGitBackend) ListBranches(ctx context.Context, repoPath string) ([]BranchDetail, error) {
	// The leading %(refname) column carries the full ref because
	// %(refname:short) drops the refs/remotes/ prefix under --format, which
	// would make remote-ness undetectable.
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{
		"branch", "-a",
		"--format=%(refname)|%(refname:short)|%(objectname:short)|%(HEAD)|%(upstream:short)|%(authorname)|%(authoremail)|%(authordate:iso-strict)|%(subject)",
	}, AuthConfig{})
	if err != nil {
		return nil, newGitError("ListBranches", repoPath, stderr, err)
	}

	var branches []BranchDetail
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 9)
		if len(parts) < 9 {
			continue
		}
		isRemote := strings.HasPrefix(parts[0], "refs/remotes/")
		name := parts[1]
		isCurrent := parts[3] == "*"
		remote := ""
		if isRemote {
			if p := strings.SplitN(name, "/", 2); len(p) == 2 {
				remote = p[0]
			}
		}
		branches = append(branches, BranchDetail{
			Name:      name,
			Hash:      parts[2],
			IsCurrent: isCurrent,
			IsRemote:  isRemote,
			Remote:    remote,
			Upstream:  parts[4],
			Author:    parts[5],
			Email:     parts[6],
			Date:      parts[7],
			Message:   parts[8],
		})
	}
	return branches, nil
}

func (b *NativeGitBackend) GetBranchSyncInfo(ctx context.Context, repoPath, branch, upstream string) (int, int, error) {
	aRef := "refs/heads/" + branch
	bRef := "refs/remotes/" + upstream
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{
		"rev-list", "--left-right", "--count", aRef + "..." + bRef,
	}, AuthConfig{})
	if err != nil {
		return 0, 0, newGitError("GetBranchSyncInfo", repoPath, stderr, err)
	}
	parts := strings.Fields(strings.TrimSpace(stdout))
	if len(parts) != 2 {
		return 0, 0, nil
	}
	var ahead, behind int
	_, _ = fmt.Sscanf(parts[0], "%d", &ahead)
	_, _ = fmt.Sscanf(parts[1], "%d", &behind)
	return ahead, behind, nil
}
