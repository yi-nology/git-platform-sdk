package gitbackend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
)

// divergedRepo builds a repo with main and feature diverged from a common
// base: main commits main-only.txt, feature commits feature-only.txt.
func divergedRepo(t *testing.T) string {
	t.Helper()
	repo := createTestRepo(t)
	main := currentBranch(t, repo)
	gitOutput(t, repo, "checkout", "-b", "feature")
	commitFile(t, repo, "feature-only.txt", "from feature", "feature work")
	gitOutput(t, repo, "checkout", main)
	commitFile(t, repo, "main-only.txt", "from main", "main work")
	return repo
}

func TestGoGit_Merge_FastForward(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	gitOutput(t, repo, "branch", "ff")
	gitOutput(t, repo, "checkout", "ff")
	tip := commitFile(t, repo, "ff.txt", "fast-forwarded", "ff work")
	gitOutput(t, repo, "checkout", currentBranch(t, repo))

	if err := b.Merge(context.Background(), repo, "ff", MergeOptions{}); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := headHash(t, repo); got != tip {
		t.Errorf("expected HEAD to fast-forward to %s, got %s", tip, got)
	}
}

func TestGoGit_Merge_AlreadyUpToDate(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	before := headHash(t, repo)
	if err := b.Merge(context.Background(), repo, currentBranch(t, repo), MergeOptions{}); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if after := headHash(t, repo); after != before {
		t.Errorf("self-merge moved HEAD from %s to %s", before, after)
	}
}

func TestGoGit_Merge_ThreeWayCreatesMergeCommit(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := divergedRepo(t)
	featureTip := gitOutput(t, repo, "rev-parse", "feature")

	if err := b.Merge(context.Background(), repo, "feature", MergeOptions{Message: "custom merge"}); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := gitOutput(t, repo, "log", "-1", "--pretty=%s"); got != "custom merge" {
		t.Errorf("expected the custom merge message, got %q", got)
	}
	parents := gitOutput(t, repo, "log", "-1", "--pretty=%P")
	if !strings.Contains(parents, featureTip) {
		t.Errorf("expected a two-parent merge commit recording %s, parents = %q", featureTip, parents)
	}
	for _, file := range []string{"main-only.txt", "feature-only.txt"} {
		if _, err := os.Stat(filepath.Join(repo, file)); err != nil {
			t.Errorf("expected %s in the working tree after the merge: %v", file, err)
		}
	}
}

func TestGoGit_Merge_FFOnlyRejectsDiverged(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := divergedRepo(t)
	before := headHash(t, repo)

	err := b.Merge(context.Background(), repo, "feature", MergeOptions{FFOnly: true})
	if err == nil || !strings.Contains(err.Error(), "fast-forward") {
		t.Fatalf("expected a not-a-fast-forward error, got %v", err)
	}
	if after := headHash(t, repo); after != before {
		t.Errorf("failed FFOnly merge must not move HEAD (%s → %s)", before, after)
	}
}

func TestGoGit_Merge_ConflictDetected(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	main := currentBranch(t, repo)
	gitOutput(t, repo, "checkout", "-b", "feature")
	commitFile(t, repo, "shared.txt", "feature version", "feature edit")
	gitOutput(t, repo, "checkout", main)
	commitFile(t, repo, "shared.txt", "main version", "main edit")

	err := b.Merge(context.Background(), repo, "feature", MergeOptions{})
	if err == nil || !IsMergeConflict(err) {
		t.Fatalf("expected a merge-conflict error, got %v", err)
	}
}

func TestGoGit_Merge_NoCommitLeavesStaged(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := divergedRepo(t)
	before := headHash(t, repo)

	if err := b.Merge(context.Background(), repo, "feature", MergeOptions{NoCommit: true}); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if after := headHash(t, repo); after != before {
		t.Errorf("NoCommit merge moved HEAD (%s → %s)", before, after)
	}
	if _, err := os.Stat(filepath.Join(repo, "feature-only.txt")); err != nil {
		t.Errorf("expected the branch changes applied to the worktree: %v", err)
	}
	staged := gitOutput(t, repo, "diff", "--cached", "--name-only")
	if !strings.Contains(staged, "feature-only.txt") {
		t.Errorf("expected feature-only.txt staged, got %q", staged)
	}
}

func TestGoGit_Merge_SquashSingleParent(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := divergedRepo(t)
	featureTip := gitOutput(t, repo, "rev-parse", "feature")

	if err := b.Merge(context.Background(), repo, "feature", MergeOptions{Squash: true}); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	parents := gitOutput(t, repo, "log", "-1", "--pretty=%P")
	if strings.Contains(parents, featureTip) || len(strings.Fields(parents)) != 1 {
		t.Errorf("squash merge must record a single parent excluding %s, parents = %q", featureTip, parents)
	}
}

func TestGoGit_Merge_DefaultMessageAndMissingBranch(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := divergedRepo(t)

	if err := b.Merge(context.Background(), repo, "feature", MergeOptions{}); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := gitOutput(t, repo, "log", "-1", "--pretty=%s"); got != "Merge branch 'feature'" {
		t.Errorf("expected the default merge message, got %q", got)
	}

	err := b.Merge(context.Background(), repo, "missing", MergeOptions{})
	if err == nil || !IsNotFound(err) {
		t.Errorf("missing branch: expected a NotFound error, got %v", err)
	}
}

func TestGoGit_CherryPick(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	main := currentBranch(t, repo)
	gitOutput(t, repo, "checkout", "-b", "side")
	picked := commitFile(t, repo, "note.txt", "picked content", "side commit")
	gitOutput(t, repo, "checkout", main)

	if err := b.CherryPick(context.Background(), repo, picked); err != nil {
		t.Fatalf("CherryPick: %v", err)
	}
	if got := gitOutput(t, repo, "log", "-1", "--pretty=%s"); got != "side commit" {
		t.Errorf("expected the picked message on HEAD, got %q", got)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "note.txt")); err != nil || string(body) != "picked content" {
		t.Errorf("expected note.txt applied to the worktree, got %q (%v)", body, err)
	}

	if err := b.CherryPick(context.Background(), repo, "0000000000000000000000000000000000000000"); err == nil {
		t.Error("expected an error cherry-picking an unknown commit")
	}
}

func TestResolveRef(t *testing.T) {
	repo := createTestRepo(t)
	gogitRepo, err := git.PlainOpen(repo)
	if err != nil {
		t.Fatal(err)
	}
	main := currentBranch(t, repo)
	want := headHash(t, repo)

	for _, ref := range []string{main, "refs/heads/" + main, want} {
		got, err := resolveRef(gogitRepo, ref)
		if err != nil {
			t.Errorf("resolveRef(%q): %v", ref, err)
			continue
		}
		if got.String() != want {
			t.Errorf("resolveRef(%q) = %s, want %s", ref, got, want)
		}
	}
	if _, err := resolveRef(gogitRepo, "no-such-ref"); err == nil {
		t.Error("expected an error for an unresolvable ref")
	}
}
