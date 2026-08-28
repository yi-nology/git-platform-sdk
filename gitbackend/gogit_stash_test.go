package gitbackend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// dirtyRepo returns a repo whose worktree carries an uncommitted change.
func dirtyRepo(t *testing.T) string {
	t.Helper()
	repo := createTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "wip.txt"), []byte("work in progress"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestGoGit_StashLifecycle(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := dirtyRepo(t)
	ctx := context.Background()

	// Saving stashes the change away and leaves a clean worktree.
	if err := b.StashSave(ctx, repo, "my stash"); err != nil {
		t.Fatalf("StashSave: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "wip.txt")); !os.IsNotExist(err) {
		t.Errorf("expected wip.txt gone after the stash, stat err = %v", err)
	}
	entries, err := b.StashList(ctx, repo)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one stash entry, got %+v (%v)", entries, err)
	}
	if entries[0].Index != 0 || entries[0].Message != "my stash" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}

	// A second save stacks on top: stash@{0} is the newest.
	if err := os.WriteFile(filepath.Join(repo, "wip2.txt"), []byte("more wip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.StashSave(ctx, repo, "second stash"); err != nil {
		t.Fatalf("StashSave #2: %v", err)
	}
	entries, err = b.StashList(ctx, repo)
	if err != nil || len(entries) != 2 {
		t.Fatalf("expected two stash entries, got %+v (%v)", entries, err)
	}
	if entries[0].Message != "second stash" || entries[1].Message != "my stash" {
		t.Errorf("unexpected stack order: %+v", entries)
	}

	// Popping the newest restores its change and drops the entry.
	if err := b.StashPop(ctx, repo, 0); err != nil {
		t.Fatalf("StashPop: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "wip2.txt")); err != nil || string(body) != "more wip" {
		t.Errorf("expected wip2.txt restored, got %q (%v)", body, err)
	}
	entries, err = b.StashList(ctx, repo)
	if err != nil || len(entries) != 1 || entries[0].Message != "my stash" {
		t.Errorf("expected the older stash to remain, got %+v (%v)", entries, err)
	}

	// Dropping clears the remaining entry without touching the worktree.
	if err := b.StashDrop(ctx, repo, 0); err != nil {
		t.Fatalf("StashDrop: %v", err)
	}
	entries, err = b.StashList(ctx, repo)
	if err != nil || len(entries) != 0 {
		t.Errorf("expected no stash entries after the drop, got %+v (%v)", entries, err)
	}

	// Stash out-of-range indices must fail, not silently no-op.
	if err := b.StashApply(ctx, repo, 5); err == nil {
		t.Error("expected an error applying a missing stash entry")
	}
	if err := b.StashDrop(ctx, repo, 0); err == nil {
		t.Error("expected an error dropping a missing stash entry")
	}
}

func TestGoGit_StashSaveCleanWorktreeNoop(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)

	if err := b.StashSave(context.Background(), repo, ""); err != nil {
		t.Fatalf("StashSave on a clean worktree: %v", err)
	}
	entries, err := b.StashList(context.Background(), repo)
	if err != nil || len(entries) != 0 {
		t.Errorf("expected no stash entries on a clean worktree, got %+v (%v)", entries, err)
	}
}

// TestGoGit_StashMultiFileTree exercises the stash tree builder's directory
// handling and entry sorting with several files spread across directories.
func TestGoGit_StashMultiFileTree(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	ctx := context.Background()

	files := map[string]string{
		"b.txt":           "b",
		"a.txt":           "a",
		"dir/zed.txt":     "z",
		"dir/mid/sub.txt": "s",
	}
	for name, content := range files {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.StashSave(ctx, repo, "multi"); err != nil {
		t.Fatalf("StashSave: %v", err)
	}
	for name := range files {
		if _, err := os.Stat(filepath.Join(repo, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s stashed away, stat err = %v", name, err)
		}
	}
	if err := b.StashPop(ctx, repo, 0); err != nil {
		t.Fatalf("StashPop: %v", err)
	}
	for name, content := range files {
		body, err := os.ReadFile(filepath.Join(repo, name))
		if err != nil || string(body) != content {
			t.Errorf("expected %s restored to %q, got %q (%v)", name, content, body, err)
		}
	}
}

func TestGoGit_StashApplyRestoresContent(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := dirtyRepo(t)
	ctx := context.Background()

	if err := b.StashSave(ctx, repo, ""); err != nil {
		t.Fatalf("StashSave: %v", err)
	}
	// Auto-generated message follows git's WIP-on convention.
	entries, err := b.StashList(ctx, repo)
	if err != nil || len(entries) != 1 || entries[0].Message == "" {
		t.Fatalf("expected an auto-generated message, got %+v (%v)", entries, err)
	}

	if err := b.StashApply(ctx, repo, 0); err != nil {
		t.Fatalf("StashApply: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "wip.txt")); err != nil || string(body) != "work in progress" {
		t.Errorf("expected wip.txt restored, got %q (%v)", body, err)
	}

	if err := b.StashClear(ctx, repo); err != nil {
		t.Fatalf("StashClear: %v", err)
	}
	entries, err = b.StashList(ctx, repo)
	if err != nil || len(entries) != 0 {
		t.Errorf("expected no stash entries after clear, got %+v (%v)", entries, err)
	}
}
