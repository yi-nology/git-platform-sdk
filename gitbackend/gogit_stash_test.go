package gitbackend

import (
	"context"
	"errors"
	"testing"
)

// The go-git backend no longer implements stash: git's stash lives in the
// refs/stash reflog, which the go-git storage layer cannot represent (the
// previous refs/stash@{N} encoding was an invalid reference name invisible
// to native git, and StashPop could drop an entry whose apply half-failed).
// Every stash operation must fail loudly with ErrStashUnsupported instead
// of silently maintaining a parallel, incompatible data set.
func TestGoGit_StashUnsupported(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	ctx := context.Background()

	if _, err := b.StashList(ctx, repo); !errors.Is(err, ErrStashUnsupported) {
		t.Errorf("StashList err = %v, want ErrStashUnsupported", err)
	}
	if err := b.StashSave(ctx, repo, "msg"); !errors.Is(err, ErrStashUnsupported) {
		t.Errorf("StashSave err = %v, want ErrStashUnsupported", err)
	}
	if err := b.StashApply(ctx, repo, 0); !errors.Is(err, ErrStashUnsupported) {
		t.Errorf("StashApply err = %v, want ErrStashUnsupported", err)
	}
	if err := b.StashPop(ctx, repo, 0); !errors.Is(err, ErrStashUnsupported) {
		t.Errorf("StashPop err = %v, want ErrStashUnsupported", err)
	}
	if err := b.StashDrop(ctx, repo, 0); !errors.Is(err, ErrStashUnsupported) {
		t.Errorf("StashDrop err = %v, want ErrStashUnsupported", err)
	}
	if err := b.StashClear(ctx, repo); !errors.Is(err, ErrStashUnsupported) {
		t.Errorf("StashClear err = %v, want ErrStashUnsupported", err)
	}
}
