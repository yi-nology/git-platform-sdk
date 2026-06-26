package gitbackend

import (
	"context"
	"fmt"
	"strings"
)

// --- Stash operations ---

func (b *NativeGitBackend) StashList(ctx context.Context, repoPath string) ([]StashEntry, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"stash", "list"}, AuthConfig{})
	if err != nil {
		return nil, newGitError("StashList", repoPath, stderr, err)
	}

	var entries []StashEntry
	for i, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, StashEntry{Index: i, Message: line})
	}
	return entries, nil
}

func (b *NativeGitBackend) StashSave(ctx context.Context, repoPath, message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		return newGitError("StashSave", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) StashApply(ctx context.Context, repoPath string, index int) error {
	args := []string{"stash", "apply", fmt.Sprintf("stash@{%d}", index)}
	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		return newGitError("StashApply", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) StashPop(ctx context.Context, repoPath string, index int) error {
	args := []string{"stash", "pop", fmt.Sprintf("stash@{%d}", index)}
	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		return newGitError("StashPop", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) StashDrop(ctx context.Context, repoPath string, index int) error {
	args := []string{"stash", "drop", fmt.Sprintf("stash@{%d}", index)}
	_, stderr, err := b.runGit(ctx, repoPath, args, AuthConfig{})
	if err != nil {
		return newGitError("StashDrop", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) StashClear(ctx context.Context, repoPath string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"stash", "clear"}, AuthConfig{})
	if err != nil {
		return newGitError("StashClear", repoPath, stderr, err)
	}
	return nil
}

// --- Config operations ---

func (b *NativeGitBackend) GetConfig(ctx context.Context, repoPath, key string) (string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"config", key}, AuthConfig{})
	if err != nil {
		if strings.Contains(stderr, "not found") || strings.Contains(stderr, "No such") {
			return "", newGitError("GetConfig", repoPath, stderr, fmt.Errorf("config key not found: %s", key))
		}
		return "", newGitError("GetConfig", repoPath, stderr, err)
	}
	return strings.TrimSpace(stdout), nil
}

func (b *NativeGitBackend) SetConfig(ctx context.Context, repoPath, key, value string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"config", key, value}, AuthConfig{})
	if err != nil {
		return newGitError("SetConfig", repoPath, stderr, err)
	}
	return nil
}
