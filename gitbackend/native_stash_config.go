package gitbackend

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
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

// validateConfigKey rejects config keys that git's option parser could
// swallow. The key is the first positional after the `config` subcommand
// — still inside option-parsing territory for git's interspersed parser —
// so `--list` (dump all config), `--global` (cross-scope access),
// `--file=/etc/passwd` (read arbitrary INI files) or `--remove-section`
// must never be able to travel through it. A key must be
// section.option or section.subsection.option with no segment starting
// with "-"; this mirrors the go-git backend's parseConfigKey shape.
func validateConfigKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty config key", ErrInvalidGitArg)
	}
	if _, _, _, err := parseConfigKey(key); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidGitArg, err)
	}
	for _, part := range strings.Split(key, ".") {
		if part == "" {
			return fmt.Errorf("%w: config key %q has an empty segment", ErrInvalidGitArg, key)
		}
		if strings.HasPrefix(part, "-") {
			return fmt.Errorf("%w: config key segment %q must not start with '-'", ErrInvalidGitArg, part)
		}
	}
	return nil
}

func (b *NativeGitBackend) GetConfig(ctx context.Context, repoPath, key string) (string, error) {
	if err := validateConfigKey(key); err != nil {
		return "", newGitError("GetConfig", repoPath, "", err)
	}
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"config", key}, AuthConfig{})
	if err != nil {
		// `git config <key>` exits 1 when the key has no value — the
		// documented not-found signal. stderr is localized and usually
		// empty, so the exit code (not its text) is the contract.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", newGitError("GetConfig", repoPath, stderr,
				fmt.Errorf("%w: %s", ErrConfigKeyNotFound, key))
		}
		return "", newGitError("GetConfig", repoPath, stderr, err)
	}
	return strings.TrimSpace(stdout), nil
}

func (b *NativeGitBackend) SetConfig(ctx context.Context, repoPath, key, value string) error {
	if err := validateConfigKey(key); err != nil {
		return newGitError("SetConfig", repoPath, "", err)
	}
	// A value starting with "-" could be permuted into option position by
	// git's parser (--global, --file=..., --unset...). Legitimate config
	// values rarely start with a dash; callers that need one verbatim can
	// go through RunRaw.
	if strings.HasPrefix(value, "-") {
		return newGitError("SetConfig", repoPath, "",
			fmt.Errorf("%w: config value must not start with '-'", ErrInvalidGitArg))
	}
	_, stderr, err := b.runGit(ctx, repoPath, []string{"config", key, value}, AuthConfig{})
	if err != nil {
		return newGitError("SetConfig", repoPath, stderr, err)
	}
	return nil
}
