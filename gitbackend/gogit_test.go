package gitbackend

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func newTestGoGitBackend(t *testing.T) *GoGitBackend {
	t.Helper()
	return NewGoGitBackend(Options{})
}

func TestGoGit_Init(t *testing.T) {
	b := newTestGoGitBackend(t)
	dir := t.TempDir()
	err := b.Init(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatal("expected .git directory")
	}
}

func TestGoGit_GetCurrentBranch(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)

	branch, err := b.GetCurrentBranch(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" && branch != "master" {
		t.Fatalf("expected main or master, got %q", branch)
	}
}

func TestGoGit_CreateAndDeleteBranch(t *testing.T) {
	b := newTestGoGitBackend(t)
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

func TestGoGit_Checkout(t *testing.T) {
	t.Skip("go-git v6 alpha: Checkout API limitations")
	b := newTestGoGitBackend(t)
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

func TestGoGit_GetStatus(t *testing.T) {
	b := newTestGoGitBackend(t)
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
}

func TestGoGit_IsAncestor(t *testing.T) {
	b := newTestGoGitBackend(t)
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
}

func TestGoGit_CreateTag(t *testing.T) {
	t.Skip("go-git v6 alpha: CreateTag API limitations")
	b := newTestGoGitBackend(t)
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

func TestGoGit_GetFileAtRevision(t *testing.T) {
	t.Skip("go-git v6 alpha: GetFileAtRevision API limitations")
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)

	content, err := b.GetFileAtRevision(context.Background(), repo, "README.md", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "init" {
		t.Fatalf("expected 'init', got %q", string(content))
	}
}

func TestGoGit_AddRemoveRemote(t *testing.T) {
	b := newTestGoGitBackend(t)
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

func TestGoGit_GetCommitsBetween(t *testing.T) {
	b := newTestGoGitBackend(t)
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
