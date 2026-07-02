package gitbackend

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
)

// --- Stash operations ---
// Note: stash is not supported by the go-git backend; use the native backend.

func (b *GoGitBackend) StashList(ctx context.Context, repoPath string) ([]StashEntry, error) {
	return nil, newGitError("StashList", repoPath, "", fmt.Errorf("stash not supported in go-git backend, use native backend"))
}

func (b *GoGitBackend) StashSave(ctx context.Context, repoPath, message string) error {
	return newGitError("StashSave", repoPath, "", fmt.Errorf("stash not supported in go-git backend, use native backend"))
}

func (b *GoGitBackend) StashApply(ctx context.Context, repoPath string, index int) error {
	return newGitError("StashApply", repoPath, "", fmt.Errorf("stash not supported in go-git backend, use native backend"))
}

func (b *GoGitBackend) StashPop(ctx context.Context, repoPath string, index int) error {
	return newGitError("StashPop", repoPath, "", fmt.Errorf("stash not supported in go-git backend, use native backend"))
}

func (b *GoGitBackend) StashDrop(ctx context.Context, repoPath string, index int) error {
	return newGitError("StashDrop", repoPath, "", fmt.Errorf("stash not supported in go-git backend, use native backend"))
}

func (b *GoGitBackend) StashClear(ctx context.Context, repoPath string) error {
	return newGitError("StashClear", repoPath, "", fmt.Errorf("stash not supported in go-git backend, use native backend"))
}

// --- Config operations ---

func (b *GoGitBackend) GetConfig(ctx context.Context, repoPath, key string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", newGitError("GetConfig", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	cfg, err := repo.Config()
	if err != nil {
		return "", newGitError("GetConfig", repoPath, "", err)
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return "", newGitError("GetConfig", repoPath, "", fmt.Errorf("invalid config key: %s", key))
	}

	section := parts[0]
	subsection := parts[1]

	if section == "user" {
		switch subsection {
		case "name":
			return cfg.Author.Name, nil
		case "email":
			return cfg.Author.Email, nil
		}
	}

	return "", newGitError("GetConfig", repoPath, "", fmt.Errorf("config key not found: %s", key))
}

func (b *GoGitBackend) SetConfig(ctx context.Context, repoPath, key, value string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("SetConfig", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	cfg, err := repo.Config()
	if err != nil {
		return newGitError("SetConfig", repoPath, "", err)
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return newGitError("SetConfig", repoPath, "", fmt.Errorf("invalid config key: %s", key))
	}

	section := parts[0]
	subsection := parts[1]

	switch section {
	case "user":
		switch subsection {
		case "name":
			cfg.Author.Name = value
		case "email":
			cfg.Author.Email = value
		default:
			return newGitError("SetConfig", repoPath, "", fmt.Errorf("unsupported config key: %s", key))
		}
	default:
		return newGitError("SetConfig", repoPath, "", fmt.Errorf("unsupported config section: %s", section))
	}

	return repo.Storer.SetConfig(cfg)
}
