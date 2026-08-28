package gitbackend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// rebaseSetup builds a repo where topic (one commit: topic.txt) diverged
// from the default branch (which then advanced with main.txt), with topic
// checked out. Returns the repo path, the default branch's name, and the
// pre-rebase topic tip.
func rebaseSetup(t *testing.T) (string, string, string) {
	t.Helper()
	repo := createTestRepo(t)
	main := currentBranch(t, repo)
	gitOutput(t, repo, "checkout", "-b", "topic")
	commitFile(t, repo, "topic.txt", "topic work", "topic work")
	topicTip := headHash(t, repo)
	gitOutput(t, repo, "checkout", main)
	commitFile(t, repo, "main.txt", "main work", "main work")
	gitOutput(t, repo, "checkout", "topic")
	return repo, main, topicTip
}

func TestGoGit_Rebase_ReplaysOntoAdvancedMain(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo, main, topicTip := rebaseSetup(t)
	mainTip := gitOutput(t, repo, "rev-parse", main)

	if err := b.Rebase(context.Background(), repo, main); err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	if got := headHash(t, repo); got == topicTip {
		t.Error("expected topic to be rewritten onto main, HEAD unchanged")
	}
	if parents := gitOutput(t, repo, "log", "-1", "--pretty=%P"); parents != mainTip {
		t.Errorf("expected the replayed commit's parent to be main's tip %s, got %q", mainTip, parents)
	}
	// The rebased tip must carry BOTH sides: the replayed topic change and
	// the onto-side main change it now sits on.
	for _, file := range []string{"topic.txt", "main.txt"} {
		if _, err := os.Stat(filepath.Join(repo, file)); err != nil {
			t.Errorf("expected %s present after the rebase: %v", file, err)
		}
	}
	if got := gitOutput(t, repo, "log", "-1", "--pretty=%s"); got != "topic work" {
		t.Errorf("expected the replayed message on HEAD, got %q", got)
	}
}

func TestGoGit_Rebase_AlreadyUpToDate(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo, _, tip := rebaseSetup(t) // HEAD sits on topic at tip

	// Rebasing onto the commit HEAD already points at is a no-op.
	if err := b.Rebase(context.Background(), repo, "topic"); err != nil {
		t.Fatalf("Rebase onto the same commit: %v", err)
	}
	if got := headHash(t, repo); got != tip {
		t.Errorf("expected HEAD unchanged, got %s", got)
	}
}

func TestGoGit_RebaseAbort(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo, _, topicTip := rebaseSetup(t)
	ctx := context.Background()

	// No rebase in progress: abort must say so.
	if err := b.RebaseAbort(ctx, repo); err == nil {
		t.Error("expected an error aborting with no rebase in progress")
	}

	// A recorded rebase state is rolled back to the original HEAD.
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

	if err := b.RebaseAbort(ctx, repo); err != nil {
		t.Fatalf("RebaseAbort: %v", err)
	}
	if got := headHash(t, repo); got != topicTip {
		t.Errorf("expected HEAD restored to %s, got %s", topicTip, got)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("expected the rebase state cleaned up, stat err = %v", err)
	}
}

func TestGoGit_RebaseContinue_NoStateFails(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo, _, _ := rebaseSetup(t)
	if err := b.RebaseContinue(context.Background(), repo); err == nil {
		t.Error("expected an error continuing with no rebase in progress")
	}
}

// TestGoGit_RebaseContinue_ReplaysTheBranchSide verifies that --continue
// resumes replaying the ORIGINAL branch's commits (the set Rebase planned),
// not the onto side's.
func TestGoGit_RebaseContinue_ReplaysTheBranchSide(t *testing.T) {
	b := newTestGoGitBackend(t)
	repo, main, topicTip := rebaseSetup(t)
	ctx := context.Background()
	mainTip := gitOutput(t, repo, "rev-parse", main)

	// Simulate a rebase stopped after HEAD moved onto the target: the
	// branch sits on main's tip with the first replayed change resolved and
	// staged, and the state files record where the replay stands.
	gitOutput(t, repo, "checkout", "-q", "-B", "topic", main)
	if err := os.WriteFile(filepath.Join(repo, "topic.txt"), []byte("topic work"), 0o644); err != nil {
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
	write("onto", mainTip)
	write("head-name", "refs/heads/topic")
	write("msgnum", "0")
	write("end", "1")

	if err := b.RebaseContinue(ctx, repo); err != nil {
		t.Fatalf("RebaseContinue: %v", err)
	}
	if got := gitOutput(t, repo, "log", "-1", "--pretty=%s"); got != "topic work" {
		t.Errorf("expected the resolved branch-side commit replayed, got %q", got)
	}
	if parents := gitOutput(t, repo, "log", "-1", "--pretty=%P"); parents != mainTip {
		t.Errorf("expected the replayed commit on top of onto %s, parents = %q", mainTip, parents)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("expected the rebase state cleaned up, stat err = %v", err)
	}
}
