package gitbackend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRepository_InsecureCarriedOnAuth(t *testing.T) {
	b := newTestGoGitBackend(t)
	r := OpenRepository(b, t.TempDir(), AuthConfig{Type: AuthHTTPBasic, Username: "u", Password: "p"}, true)
	if !r.Auth().InsecureSkipTLS {
		t.Error("expected insecure to ride on the bound AuthConfig so every call honors it")
	}
	if r.Auth().Username != "u" {
		t.Errorf("expected the auth config preserved, got %+v", r.Auth())
	}
}

func TestCloneRepository_BindsDir(t *testing.T) {
	origin, _, _ := pushSetup(t)
	dst := filepath.Join(t.TempDir(), "clone")

	r, err := CloneRepository(context.Background(), newTestGoGitBackend(t), origin, dst, AuthConfig{}, false)
	if err != nil {
		t.Fatalf("CloneRepository: %v", err)
	}
	defer r.Close()
	if r.Dir() != dst {
		t.Errorf("expected Dir() = %s, got %s", dst, r.Dir())
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); err != nil {
		t.Errorf("expected the clone on disk: %v", err)
	}
}

func TestRepository_FetchAndFetchAll(t *testing.T) {
	origin, seed, _ := pushSetup(t)
	dst := filepath.Join(t.TempDir(), "clone")
	r, err := CloneRepository(context.Background(), newTestGoGitBackend(t), origin, dst, AuthConfig{}, false)
	if err != nil {
		t.Fatalf("CloneRepository: %v", err)
	}
	defer r.Close()

	commitFile(t, seed, "later.txt", "later", "later work")
	gitOutput(t, seed, "push", "-q", "origin", "main")

	if err := r.Fetch(context.Background(), "main"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if err := r.FetchAll(context.Background()); err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if got := gitOutput(t, dst, "rev-parse", "origin/main"); got == "" {
		t.Error("expected the origin/main remote-tracking ref after the fetches")
	}
}

func TestRepository_DelegatesAndCommits(t *testing.T) {
	b := newTestGoGitBackend(t)
	ctx := context.Background()
	repo := createTestRepo(t)
	r := OpenRepository(b, repo, AuthConfig{}, false)
	defer r.Close()
	main := currentBranch(t, repo)

	commitFile(t, repo, "a.txt", "one", "first")
	base := headHash(t, repo)
	commitFile(t, repo, "b.txt", "two", "second")
	gitOutput(t, repo, "rm", "-q", "README.md")
	gitOutput(t, repo, "commit", "-m", "remove readme")
	tip := headHash(t, repo)

	if got, err := r.RevParse(ctx, "HEAD"); err != nil || got != tip {
		t.Errorf("RevParse: got %q (%v), want %s", got, err, tip)
	}
	if got, err := r.MergeBase(ctx, base, tip); err != nil || got != base {
		t.Errorf("MergeBase: got %q (%v), want %s", got, err, base)
	}

	names, err := r.DiffNameOnly(ctx, base, tip)
	if err != nil || !contains(names, "b.txt") || !contains(names, "README.md") {
		t.Errorf("DiffNameOnly: got %v (%v)", names, err)
	}
	deleted, err := r.DeletedFiles(ctx, base, tip)
	if err != nil || len(deleted) != 1 || deleted[0] != "README.md" {
		t.Errorf("DeletedFiles: got %v (%v)", deleted, err)
	}
	diff, err := r.Diff(ctx, base, tip)
	if err != nil || !strings.Contains(diff, "b.txt") {
		t.Errorf("Diff: expected b.txt in the patch, got %q (%v)", diff, err)
	}

	// Checkout delegation: rewinding to base restores that state.
	if err := r.Checkout(ctx, base); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "b.txt")); !os.IsNotExist(err) {
		t.Errorf("expected b.txt gone at the base checkout, stat err = %v", err)
	}
	if err := r.CheckoutDetached(ctx, main); err != nil {
		t.Fatalf("CheckoutDetached: %v", err)
	}

	// CheckoutFiles delegation restores a file deleted by a later commit
	// from an earlier ref that still carries it.
	if err := r.CheckoutFiles(ctx, base, []string{"README.md"}); err != nil {
		t.Fatalf("CheckoutFiles: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "README.md")); err != nil {
		t.Errorf("expected README.md restored: %v", err)
	}

	// Add + CommitWithIdentity + GetCommit round-trip an explicit author.
	if err := os.WriteFile(filepath.Join(repo, "c.txt"), []byte("three"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.Add(ctx, repo, []string{"c.txt"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := r.CommitWithIdentity(ctx, "Bot", "bot@example.com", "bot work"); err != nil {
		t.Fatalf("CommitWithIdentity: %v", err)
	}
	if got := gitOutput(t, repo, "log", "-1", "--pretty=%an"); got != "Bot" {
		t.Errorf("expected the Bot author recorded, got %q", got)
	}
	if c, err := b.GetCommit(ctx, repo, headHash(t, repo)); err != nil || c.Message != "bot work" || c.Author != "Bot" {
		t.Errorf("GetCommit: got %+v (%v)", c, err)
	}

	if err := r.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
