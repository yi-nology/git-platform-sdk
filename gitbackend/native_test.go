package gitbackend

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func newTestNativeBackend(t *testing.T) *NativeGitBackend {
	t.Helper()
	backend, err := NewNativeGitBackend(Options{})
	if err != nil {
		t.Skip("git not available:", err)
	}
	return backend
}

func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", dir, "config", "user.email", "test@test.com")
	cmd.Run()
	cmd = exec.Command("git", "-C", dir, "config", "user.name", "Test")
	cmd.Run()

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0644)
	cmd = exec.Command("git", "-C", dir, "add", ".")
	cmd.Run()
	cmd = exec.Command("git", "-C", dir, "commit", "-m", "init")
	cmd.Run()
	return dir
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimRight(string(out), "\n")
}

func TestNative_Init(t *testing.T) {
	b := newTestNativeBackend(t)
	dir := t.TempDir()
	err := b.Init(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatal("expected .git directory")
	}
}

func TestNative_GetCurrentBranch(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	branch, err := b.GetCurrentBranch(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" && branch != "master" {
		t.Fatalf("expected main or master, got %q", branch)
	}
}

func TestNative_CreateAndDeleteBranch(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	err := b.CreateBranch(context.Background(), repo, "feature", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	err = b.DeleteBranch(context.Background(), repo, "feature")
	if err != nil {
		t.Fatal(err)
	}
}

func TestNative_CreateBranchExists(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	err := b.CreateBranch(context.Background(), repo, "feature", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	err = b.CreateBranch(context.Background(), repo, "feature", "HEAD")
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
	ge, ok := err.(*GitError)
	if !ok || ge.Err != ErrBranchExists {
		t.Fatalf("expected ErrBranchExists, got %v", err)
	}
}

func TestNative_Checkout(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	b.CreateBranch(context.Background(), repo, "feature", "HEAD")
	err := b.Checkout(context.Background(), repo, "feature")
	if err != nil {
		t.Fatal(err)
	}

	branch, _ := b.GetCurrentBranch(context.Background(), repo)
	if branch != "feature" {
		t.Fatalf("expected feature, got %q", branch)
	}
}

func TestNative_GetStatus(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	status, err := b.GetStatus(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if !status.IsClean {
		t.Fatal("expected clean repo")
	}

	os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty"), 0644)
	status, err = b.GetStatus(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if status.IsClean {
		t.Fatal("expected dirty repo")
	}
	if len(status.Untracked) != 1 {
		t.Fatalf("expected 1 untracked, got %d", len(status.Untracked))
	}
}

func TestNative_GetCommitsBetween(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	initHash := gitOutput(t, repo, "rev-parse", "HEAD")

	os.WriteFile(filepath.Join(repo, "file2.txt"), []byte("hello"), 0644)
	exec.Command("git", "-C", repo, "add", ".").Run()
	exec.Command("git", "-C", repo, "commit", "-m", "second").Run()

	secondHash := gitOutput(t, repo, "rev-parse", "HEAD")

	commits, err := b.GetCommitsBetween(context.Background(), repo, initHash, secondHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if commits[0].Message != "second" {
		t.Fatalf("expected 'second', got %q", commits[0].Message)
	}
}

func TestNative_IsAncestor(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	initHash := gitOutput(t, repo, "rev-parse", "HEAD")

	os.WriteFile(filepath.Join(repo, "file2.txt"), []byte("hello"), 0644)
	exec.Command("git", "-C", repo, "add", ".").Run()
	exec.Command("git", "-C", repo, "commit", "-m", "second").Run()

	secondHash := gitOutput(t, repo, "rev-parse", "HEAD")

	ok, err := b.IsAncestor(context.Background(), repo, initHash, secondHash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected initHash to be ancestor of secondHash")
	}

	ok, err = b.IsAncestor(context.Background(), repo, secondHash, initHash)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected secondHash NOT to be ancestor of initHash")
	}
}

func TestNative_CreateTag(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	err := b.CreateTag(context.Background(), repo, "v1.0", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	out := gitOutput(t, repo, "tag", "-l", "v1.0")
	if out != "v1.0" {
		t.Fatalf("expected tag v1.0, got %q", out)
	}
}

func TestNative_GetFileAtRevision(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	content, err := b.GetFileAtRevision(context.Background(), repo, "README.md", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "init" {
		t.Fatalf("expected 'init', got %q", string(content))
	}
}

func TestNative_GetFileNotFound(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	_, err := b.GetFileAtRevision(context.Background(), repo, "nonexistent.txt", "HEAD")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNative_AddRemoveRemote(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)

	err := b.AddRemote(context.Background(), repo, "myremote", "https://example.com/repo.git")
	if err != nil {
		t.Fatal(err)
	}

	err = b.RemoveRemote(context.Background(), repo, "myremote")
	if err != nil {
		t.Fatal(err)
	}
}

func TestNative_FetchNonexistentRepo(t *testing.T) {
	b := newTestNativeBackend(t)

	_, err := b.Fetch(context.Background(), FetchOptions{
		RepoPath: "/nonexistent/path",
		Remote:   "origin",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFactory_NewGitBackend(t *testing.T) {
	backend, err := NewGitBackend(Options{Type: "native"})
	if err != nil {
		t.Fatal(err)
	}
	if backend == nil {
		t.Fatal("expected non-nil backend")
	}
}

func TestFactory_AutoDetect(t *testing.T) {
	backend, err := NewGitBackend(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if backend == nil {
		t.Fatal("expected non-nil backend")
	}
}

func TestFactory_UnknownType(t *testing.T) {
	_, err := NewGitBackend(Options{Type: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}
