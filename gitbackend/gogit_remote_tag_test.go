package gitbackend

import (
	"context"
	"errors"
	"testing"
)

// TestGoGit_CreateTag_AtBranchRef verifies a non-empty, non-hash ref tags
// THAT rev. The old plumbing.NewHash path zeroed branch names and silently
// tagged HEAD instead of the requested ref.
func TestGoGit_CreateTag_AtBranchRef(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	ctx := context.Background()
	initHash := headHash(t, repo)
	gitOutput(t, repo, "branch", "old", initHash)
	commitFile(t, repo, "file2.txt", "hello", "second") // HEAD moves ahead of "old"

	if err := b.CreateTag(ctx, repo, "v0", "old"); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if got := gitOutput(t, repo, "rev-parse", "v0^{commit}"); got != initHash {
		t.Errorf("expected tag v0 at branch old (%s), got %s", initHash, got)
	}

	// Tagging HEAD via an empty ref still works.
	if err := b.CreateTag(ctx, repo, "v1", ""); err != nil {
		t.Fatalf("CreateTag(HEAD): %v", err)
	}
	if got := gitOutput(t, repo, "rev-parse", "v1^{commit}"); got != headHash(t, repo) {
		t.Errorf("expected tag v1 at HEAD, got %s", got)
	}
}

// TestGoGit_CreateTag_AlreadyExists verifies the duplicate-tag error maps to
// the backend ErrTagExists sentinel, like the native backend.
func TestGoGit_CreateTag_AlreadyExists(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	ctx := context.Background()

	if err := b.CreateTag(ctx, repo, "v1", "HEAD"); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	err := b.CreateTag(ctx, repo, "v1", "HEAD")
	if err == nil || !errors.Is(err, ErrTagExists) {
		t.Fatalf("expected ErrTagExists, got %v", err)
	}
}

// TestGoGit_CreateTag_UnresolvableRef verifies a bogus ref errors instead of
// silently tagging HEAD.
func TestGoGit_CreateTag_UnresolvableRef(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)

	err := b.CreateTag(context.Background(), repo, "v9", "no-such-branch")
	if err == nil || !errors.Is(err, errCannotResolveRev) {
		t.Fatalf("expected a cannot-resolve error, got %v", err)
	}
}
