package gitbackend

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// divergedRepo (see gogit_merge_test.go) provides the default branch and a
// feature branch diverged from a common base: the default branch committed
// main-only.txt, feature committed feature-only.txt. The default branch's
// name is environment-dependent (master/main), so resolve it dynamically.

// TestGoGit_Diff_BranchRevs verifies the diff family accepts branch names.
// The old plumbing.NewHash path zeroed them, so Diff/DiffNames/DeletedFiles
// either errored or (in GetCommitsBetween's case) walked the wrong history.
func TestGoGit_Diff_BranchRevs(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := divergedRepo(t)
	main := currentBranch(t, repo)
	ctx := context.Background()

	diff, err := b.Diff(ctx, repo, DiffOptions{From: main, To: "feature"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(diff, "feature-only.txt") {
		t.Errorf("expected feature-only.txt in the %s→feature diff, got %q", main, diff)
	}

	names, err := b.DiffNames(ctx, repo, main, "feature")
	if err != nil {
		t.Fatalf("DiffNames: %v", err)
	}
	if !contains(names, "feature-only.txt") || !contains(names, "main-only.txt") {
		t.Errorf("expected both branch-side files in %v", names)
	}

	deleted, err := b.DeletedFiles(ctx, repo, main, "feature")
	if err != nil {
		t.Fatalf("DeletedFiles: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "main-only.txt" {
		t.Errorf("expected only main-only.txt deleted on the way to feature, got %v", deleted)
	}
}

func TestGoGit_MergeBase_BranchRevs(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := divergedRepo(t)
	main := currentBranch(t, repo)
	fork := gitOutput(t, repo, "merge-base", main, "feature")

	got, err := b.MergeBase(context.Background(), repo, main, "feature")
	if err != nil {
		t.Fatalf("MergeBase: %v", err)
	}
	if got != fork {
		t.Errorf("expected the fork point %s, got %s", fork, got)
	}
}

// TestGoGit_MergeBase_UnresolvableRev verifies unknown revs error out instead
// of producing misleading zero-commit results.
func TestGoGit_MergeBase_UnresolvableRev(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := divergedRepo(t)

	_, err := b.MergeBase(context.Background(), repo, "no-such-branch", currentBranch(t, repo))
	if err == nil || !errors.Is(err, errCannotResolveRev) {
		t.Fatalf("expected a cannot-resolve error, got %v", err)
	}
}
