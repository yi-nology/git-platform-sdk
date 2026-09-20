package gitbackend

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoGit_GetFileHistory(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	commitFile(t, repo, "a.txt", "v2", "second touch")
	commitFile(t, repo, "a.txt", "v3", "third touch")

	commits, err := b.GetFileHistory(context.Background(), repo, "a.txt", 0)
	if err != nil {
		t.Fatalf("GetFileHistory: %v", err)
	}
	if len(commits) != 2 { // a.txt exists only in the last two commits
		t.Fatalf("expected 2 commits touching a.txt, got %d: %+v", len(commits), commits)
	}
	if commits[0].Message != "third touch" {
		t.Errorf("expected newest-first ordering, got %q first", commits[0].Message)
	}

	limited, err := b.GetFileHistory(context.Background(), repo, "a.txt", 1)
	if err != nil || len(limited) != 1 {
		t.Errorf("expected limit honored (1 commit), got %d (%v)", len(limited), err)
	}
}

func TestGoGit_GetTree(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "docs", "guide.md"), []byte("docs"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, repo, "add", ".")
	gitOutput(t, repo, "commit", "-m", "add docs")

	// Flat listing at the root carries the directory with its type.
	entries, err := b.GetTree(context.Background(), repo, "HEAD", "", false)
	if err != nil {
		t.Fatalf("GetTree: %v", err)
	}
	var docs, readme *TreeEntry
	for i := range entries {
		switch entries[i].Name {
		case "docs":
			docs = &entries[i]
		case "README.md":
			readme = &entries[i]
		}
	}
	if docs == nil || docs.Type != TreeEntryDir {
		t.Errorf("expected a docs directory entry, got %+v", entries)
	}
	if readme == nil || readme.Type != TreeEntryFile || readme.Mode != "0100644" {
		t.Errorf("expected a regular README.md entry, got %+v", entries)
	}

	// Recursive listing reaches the nested file with its full path.
	entries, err = b.GetTree(context.Background(), repo, "HEAD", "", true)
	if err != nil {
		t.Fatalf("GetTree (recursive): %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Path == "docs/guide.md" {
			found = true
			if e.Size != int64(len("docs")) {
				t.Errorf("expected size %d for docs/guide.md, got %d", len("docs"), e.Size)
			}
		}
	}
	if !found {
		t.Errorf("expected docs/guide.md in the recursive listing, got %+v", entries)
	}

	// dirPath scopes the listing into the subtree.
	entries, err = b.GetTree(context.Background(), repo, "HEAD", "docs", false)
	if err != nil || len(entries) != 1 || entries[0].Path != "docs/guide.md" {
		t.Errorf("expected the scoped docs listing, got %+v (%v)", entries, err)
	}
}

func TestGoGit_GetBlob(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "logo.bin"), []byte{0x00, 0x01, 0x02, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, repo, "add", ".")
	gitOutput(t, repo, "commit", "-m", "binary")

	text, err := b.GetBlob(context.Background(), repo, "HEAD", "README.md")
	if err != nil {
		t.Fatalf("GetBlob (text): %v", err)
	}
	if text.IsBinary || text.Encoding != EncodingUTF8 || text.Content != "init" {
		t.Errorf("unexpected text blob: %+v", text)
	}

	bin, err := b.GetBlob(context.Background(), repo, "HEAD", "logo.bin")
	if err != nil {
		t.Fatalf("GetBlob (binary): %v", err)
	}
	if !bin.IsBinary || bin.Encoding != EncodingBase64 {
		t.Errorf("expected a base64 binary blob, got %+v", bin)
	}
	if decoded, err := base64.StdEncoding.DecodeString(bin.Content); err != nil || len(decoded) != 4 {
		t.Errorf("expected the 4-byte payload round-tripped, got %x (%v)", decoded, err)
	}

	_, err = b.GetBlob(context.Background(), repo, "HEAD", "missing.txt")
	if err == nil || !IsNotFound(err) {
		t.Errorf("missing blob: expected a NotFound error, got %v", err)
	}
}

func TestGoGit_CheckoutRef(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	old := headHash(t, repo)
	commitFile(t, repo, "README.md", "updated", "update readme")

	if err := b.CheckoutRef(context.Background(), repo, old); err != nil {
		t.Fatalf("CheckoutRef: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "README.md")); err != nil || string(body) != "init" {
		t.Errorf("expected the worktree back at the old content, got %q (%v)", body, err)
	}
	if got := headHash(t, repo); got != old {
		t.Errorf("expected HEAD at the checked-out commit %s, got %s", old, got)
	}
}

// TestGoGit_CheckoutRef_AttachesToBranch verifies a branch name checks out
// ATTACHED (HEAD follows the branch), matching the native backend's
// `git checkout <branch>` semantics. CheckoutRef used to detach
// unconditionally, so a branch checkout silently drifted off the branch.
func TestGoGit_CheckoutRef_AttachesToBranch(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	main := currentBranch(t, repo)
	gitOutput(t, repo, "branch", "feature")
	commitFile(t, repo, "on-main.txt", "main work", "main work")

	attached := func() bool {
		t.Helper()
		return exec.Command("git", "-C", repo, "symbolic-ref", "-q", "HEAD").Run() == nil
	}

	if err := b.CheckoutRef(context.Background(), repo, "feature"); err != nil {
		t.Fatalf("CheckoutRef(feature): %v", err)
	}
	if !attached() {
		t.Fatal("expected HEAD attached after CheckoutRef with a branch name")
	}
	if got := currentBranch(t, repo); got != "feature" {
		t.Errorf("expected HEAD on feature, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "on-main.txt")); !os.IsNotExist(err) {
		t.Errorf("expected the feature worktree without main-side files, stat err = %v", err)
	}

	if err := b.CheckoutRef(context.Background(), repo, main); err != nil {
		t.Fatalf("CheckoutRef(%s): %v", main, err)
	}
	if got := currentBranch(t, repo); got != main {
		t.Errorf("expected HEAD back on %s, got %q", main, got)
	}
	if _, err := os.Stat(filepath.Join(repo, "on-main.txt")); err != nil {
		t.Errorf("expected on-main.txt back on %s: %v", main, err)
	}
}

// TestGoGit_CheckoutDetached_AlwaysDetaches verifies the explicit detached
// variant pins HEAD to the commit even when given a branch name.
func TestGoGit_CheckoutDetached_AlwaysDetaches(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	main := currentBranch(t, repo)

	if err := b.CheckoutDetached(context.Background(), repo, main); err != nil {
		t.Fatalf("CheckoutDetached: %v", err)
	}
	if err := exec.Command("git", "-C", repo, "symbolic-ref", "-q", "HEAD").Run(); err == nil {
		t.Error("expected HEAD detached after CheckoutDetached, found it attached")
	}
	if got := headHash(t, repo); got != gitOutput(t, repo, "rev-parse", main) {
		t.Errorf("expected HEAD at %s's tip, got %s", main, got)
	}
}

func TestGoGit_CheckoutFiles(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	commitFile(t, repo, "a.txt", "committed", "add a")
	head := headHash(t, repo)

	// Local edits: clobber a.txt, and ask for a file that does not exist.
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("clobbered"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := b.CheckoutFiles(context.Background(), repo, head, []string{"a.txt", "nope.txt"})
	if err == nil || !strings.Contains(err.Error(), "nope.txt") {
		t.Fatalf("expected the missing file to surface as an error, got %v", err)
	}
	if body, _ := os.ReadFile(filepath.Join(repo, "a.txt")); string(body) != "committed" {
		t.Errorf("expected a.txt restored despite the sibling failure, got %q", body)
	}
	// The restored file matches HEAD, so the restore leaves it fully clean.
	if dirty := gitOutput(t, repo, "status", "--porcelain", "--", "a.txt"); dirty != "" {
		t.Errorf("expected a.txt clean after the restore, got %q", dirty)
	}
}

// commitExecFile writes an executable file, commits it, and returns HEAD.
func commitExecFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if err := os.Chmod(filepath.Join(dir, name), 0o755); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, dir, "add", name)
	gitOutput(t, dir, "commit", "-m", msg)
}

// TestGoGit_CheckoutFiles_PreservesExecMode verifies CheckoutFiles restores a
// file's git mode: executables come back 0o755 even when the checkout target
// already existed as a plain 0o644 file (os.Create used to drop the bit).
func TestGoGit_CheckoutFiles_PreservesExecMode(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	commitExecFile(t, repo, "run.sh", "#!/bin/sh\n", "add script")
	head := headHash(t, repo)

	// Clobber the executable down to a plain file.
	if err := os.WriteFile(filepath.Join(repo, "run.sh"), []byte("clobbered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.CheckoutFiles(context.Background(), repo, head, []string{"run.sh"}); err != nil {
		t.Fatalf("CheckoutFiles: %v", err)
	}
	info, err := os.Stat(filepath.Join(repo, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("expected run.sh restored with mode 0755, got %v", info.Mode().Perm())
	}
	if body, _ := os.ReadFile(filepath.Join(repo, "run.sh")); string(body) != "#!/bin/sh\n" {
		t.Errorf("expected run.sh content restored, got %q", body)
	}
}

// TestGoGit_CheckoutFiles_SymlinkRecreatedAsLink verifies symlink entries are
// restored as links instead of plain files holding the target path.
func TestGoGit_CheckoutFiles_SymlinkRecreatedAsLink(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo := createTestRepo(t)
	commitFile(t, repo, "a.txt", "target", "add a")
	if err := os.Symlink("a.txt", filepath.Join(repo, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	gitOutput(t, repo, "add", "link")
	gitOutput(t, repo, "commit", "-m", "add link")
	head := headHash(t, repo)

	// Replace the link with a plain file to simulate the damage.
	if err := os.Remove(filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "link"), []byte("a.txt"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := b.CheckoutFiles(context.Background(), repo, head, []string{"link"}); err != nil {
		t.Fatalf("CheckoutFiles: %v", err)
	}
	info, err := os.Lstat(filepath.Join(repo, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected link to be restored as a symlink, got mode %v", info.Mode())
	}
	if target, err := os.Readlink(filepath.Join(repo, "link")); err != nil || target != "a.txt" {
		t.Errorf("expected link target a.txt, got %q (%v)", target, err)
	}
}
