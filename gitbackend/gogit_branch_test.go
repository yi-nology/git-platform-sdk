package gitbackend

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// currentBranch returns the repo's checked-out branch name.
func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	return gitOutput(t, dir, "symbolic-ref", "--short", "HEAD")
}

// headHash returns the current HEAD commit hash.
func headHash(t *testing.T, dir string) string {
	t.Helper()
	return gitOutput(t, dir, "rev-parse", "HEAD")
}

// commitFile writes (or overwrites) a file and commits it, returning the
// new HEAD hash.
func commitFile(t *testing.T, dir, name, content, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	gitOutput(t, dir, "add", name)
	gitOutput(t, dir, "commit", "-m", msg)
	return headHash(t, dir)
}

// cloneRepo clones src into dst (a fresh working copy whose origin points
// back at src) with a hermetic commit identity so commits made in the clone
// do not depend on the machine's global git config.
func cloneRepo(t *testing.T, src, dst string) {
	t.Helper()
	out, err := exec.Command("git", "clone", src, dst).CombinedOutput()
	if err != nil {
		t.Fatalf("git clone %s %s: %v\n%s", src, dst, err, out)
	}
	gitOutput(t, dst, "config", "user.email", "test@test.com")
	gitOutput(t, dst, "config", "user.name", "Test")
}

func TestGoGit_ListLocalBranches(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	main := currentBranch(t, repo)
	gitOutput(t, repo, "branch", "feature")
	gitOutput(t, repo, "branch", "hotfix")

	branches, err := b.ListLocalBranches(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, name := range branches {
		set[name] = true
	}
	for _, want := range []string{main, "feature", "hotfix"} {
		if !set[want] {
			t.Errorf("expected branch %q in %v", want, branches)
		}
	}
}

func TestGoGit_RenameBranch(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	gitOutput(t, repo, "branch", "old-name")

	if err := b.RenameBranch(context.Background(), repo, "old-name", "new-name"); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	gitOutput(t, repo, "show-ref", "--verify", "refs/heads/new-name") // panics-free existence check
	if refErr := exec.Command("git", "-C", repo, "show-ref", "--verify", "refs/heads/old-name").Run(); refErr == nil {
		t.Error("expected the old branch ref to be gone after the rename")
	}

	err := b.RenameBranch(context.Background(), repo, "missing", "whatever")
	if err == nil || !IsNotFound(err) {
		t.Errorf("renaming a missing branch: expected a NotFound error, got %v", err)
	}
}

func TestGoGit_ListBranches_Details(t *testing.T) {
	b := newTestGoGitBackend(t)
	origin := createTestRepo(t)
	main := currentBranch(t, origin)
	clone := t.TempDir()
	cloneRepo(t, origin, clone)

	details, err := b.ListBranches(context.Background(), clone)
	if err != nil {
		t.Fatal(err)
	}
	var current, remote *BranchDetail
	for i := range details {
		switch {
		case details[i].Name == main && !details[i].IsRemote:
			current = &details[i]
		case details[i].IsRemote:
			remote = &details[i]
		}
	}
	if current == nil {
		t.Fatalf("expected the %q branch in %v", main, details)
	}
	if !current.IsCurrent || current.IsRemote {
		t.Errorf("local branch misclassified: %+v", current)
	}
	if current.Author != "Test" || current.Message != "init" || current.Hash == "" {
		t.Errorf("unexpected commit metadata: %+v", current)
	}
	if remote == nil {
		t.Fatalf("expected a remote-tracking branch after clone, got %v", details)
	}
	if remote.Remote != "origin" || !strings.HasPrefix(remote.Name, "origin/") {
		t.Errorf("remote branch mislabeled: %+v", remote)
	}
}

func TestGoGit_GetBranchSyncInfo_AheadAndBehind(t *testing.T) {
	b := newTestGoGitBackend(t)
	origin := createTestRepo(t)
	main := currentBranch(t, origin)
	clone := t.TempDir()
	cloneRepo(t, origin, clone)

	// Origin moves forward (the clone falls behind by one)…
	commitFile(t, origin, "origin-side.txt", "from origin", "origin work")
	// …and the clone moves forward too (one ahead).
	commitFile(t, clone, "clone-side.txt", "from clone", "clone work")
	gitOutput(t, clone, "fetch", "origin")

	ahead, behind, err := b.GetBranchSyncInfo(context.Background(), clone, main, "origin/"+main)
	if err != nil {
		t.Fatal(err)
	}
	if ahead != 1 || behind != 1 {
		t.Errorf("expected ahead=1 behind=1, got ahead=%d behind=%d", ahead, behind)
	}

	_, _, err = b.GetBranchSyncInfo(context.Background(), clone, "missing", "origin/"+main)
	if err == nil || !IsNotFound(err) {
		t.Errorf("missing branch: expected a NotFound error, got %v", err)
	}
}

func TestGoGit_ListRemoteBranches(t *testing.T) {
	b := newTestGoGitBackend(t)
	origin := createTestRepo(t)
	main := currentBranch(t, origin)
	gitOutput(t, origin, "branch", "feature")
	clone := t.TempDir()
	cloneRepo(t, origin, clone)

	branches, err := b.ListRemoteBranches(context.Background(), clone, "origin")
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, name := range branches {
		set[name] = true
	}
	if !set[main] || !set["feature"] {
		t.Errorf("expected remote heads %q and feature, got %v", main, branches)
	}

	_, err = b.ListRemoteBranches(context.Background(), clone, "nonexistent")
	if err == nil || !IsNotFound(err) {
		t.Errorf("missing remote: expected a NotFound error, got %v", err)
	}

	_, err = b.ListRemoteBranches(context.Background(), t.TempDir(), "origin")
	if err == nil || !IsNotFound(err) {
		t.Errorf("missing repo: expected a NotFound error, got %v", err)
	}
}
