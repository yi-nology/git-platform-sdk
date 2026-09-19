package gitbackend

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestGoGit_GetCommitsBetween_BranchRevs verifies from/to accept branch
// names: a from-branch sitting below to must yield only the in-between
// commits. The old plumbing.NewHash path zeroed branch names, so the walk
// never matched and silently returned to's entire history.
func TestGoGit_GetCommitsBetween_BranchRevs(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	main := currentBranch(t, repo)
	initHash := headHash(t, repo)
	gitOutput(t, repo, "branch", "base", initHash)
	commitFile(t, repo, "file2.txt", "hello", "second")

	commits, err := b.GetCommitsBetween(context.Background(), repo, "base", main)
	if err != nil {
		t.Fatalf("GetCommitsBetween: %v", err)
	}
	if len(commits) != 1 || commits[0].Message != "second" {
		t.Fatalf("expected only [second] between base and %s, got %+v", main, commits)
	}
}

// TestGoGit_GetCommitsBetween_UnresolvableRev verifies an unknown rev is a
// hard error instead of an empty-range / full-history guess.
func TestGoGit_GetCommitsBetween_UnresolvableRev(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)

	_, err := b.GetCommitsBetween(context.Background(), repo, "no-such-branch", "HEAD")
	if err == nil || !errors.Is(err, errCannotResolveRev) {
		t.Fatalf("expected a cannot-resolve error, got %v", err)
	}
}

// TestGoGit_RebaseAbort_RestoresWorktree verifies abort restores not only the
// branch ref but also the worktree and index: restoring only the ref used to
// leave the half-rebase state in place, and the next commit polluted the old
// branch.
func TestGoGit_RebaseAbort_RestoresWorktree(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo, main, topicTip := rebaseSetup(t)
	ctx := context.Background()
	mainTip := gitOutput(t, repo, "rev-parse", main)

	// Simulate a rebase stopped mid-way: the branch ref moved onto main with
	// a conflicted topic.txt staged on top of it.
	gitOutput(t, repo, "checkout", "-q", "-B", "topic", main)
	if err := os.WriteFile(filepath.Join(repo, "topic.txt"), []byte("<<<<<<< conflict"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, repo, "add", "topic.txt")

	stateDir := filepath.Join(repo, ".git", "rebase-merge")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatal(err)
	}
	write := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(stateDir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("orig-head", topicTip)
	write("head-name", "refs/heads/topic")
	write("onto", mainTip)

	if err := b.RebaseAbort(ctx, repo); err != nil {
		t.Fatalf("RebaseAbort: %v", err)
	}

	if got := headHash(t, repo); got != topicTip {
		t.Errorf("expected HEAD restored to %s, got %s", topicTip, got)
	}
	// Worktree content is back at origHead: the conflicted file shows its
	// original body and the onto-side file is gone.
	if body, err := os.ReadFile(filepath.Join(repo, "topic.txt")); err != nil || string(body) != "topic work" {
		t.Errorf("expected topic.txt restored to the origHead content, got %q (%v)", body, err)
	}
	if _, err := os.Stat(filepath.Join(repo, "main.txt")); !os.IsNotExist(err) {
		t.Errorf("expected the onto-side main.txt gone after abort, stat err = %v", err)
	}
	// A clean status proves the next commit cannot pollute the branch.
	status, err := b.GetStatus(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	if !status.IsClean {
		t.Fatalf("expected a clean status after abort, got %+v", status)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("expected the rebase state cleaned up, stat err = %v", err)
	}
}
