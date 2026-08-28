package gitbackend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNative_ListAndRenameBranches(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)
	main := currentBranch(t, repo)
	gitOutput(t, repo, "branch", "feature")
	gitOutput(t, repo, "branch", "wip")

	locals, err := b.ListLocalBranches(context.Background(), repo)
	if err != nil || !contains(locals, main) || !contains(locals, "feature") {
		t.Errorf("ListLocalBranches: got %v (%v)", locals, err)
	}

	if err := b.RenameBranch(context.Background(), repo, "wip", "renamed"); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	locals, err = b.ListLocalBranches(context.Background(), repo)
	if err != nil || !contains(locals, "renamed") || contains(locals, "wip") {
		t.Errorf("expected wip renamed, got %v (%v)", locals, err)
	}

	origin := createTestRepo(t)
	clone := t.TempDir()
	cloneRepo(t, origin, clone)
	// Like the gogit backend, remote branch names come back bare (the
	// origin/ prefix is stripped) and the symbolic origin/HEAD is skipped.
	remotes, err := b.ListRemoteBranches(context.Background(), clone, "origin")
	if err != nil || len(remotes) != 1 || remotes[0] != main {
		t.Errorf("ListRemoteBranches: got %v (%v)", remotes, err)
	}
}

func TestNative_ListBranches_Details(t *testing.T) {
	b := newTestNativeBackend(t)
	origin := createTestRepo(t)
	main := currentBranch(t, origin)
	clone := t.TempDir()
	cloneRepo(t, origin, clone)
	gitOutput(t, clone, "branch", "--set-upstream-to=origin/"+main, main)

	details, err := b.ListBranches(context.Background(), clone)
	if err != nil {
		t.Fatalf("ListBranches: %v", err)
	}
	var local, remote *BranchDetail
	for i := range details {
		if details[i].IsRemote {
			remote = &details[i]
		} else if details[i].Name == main {
			local = &details[i]
		}
	}
	if local == nil || !local.IsCurrent || local.Upstream != "origin/"+main || local.Author != "Test" || local.Message != "init" {
		t.Errorf("unexpected local detail: %+v", local)
	}
	if remote == nil || remote.Remote != "origin" {
		t.Errorf("unexpected remote detail: %+v", remote)
	}
}

func TestNative_GetBranchSyncInfo(t *testing.T) {
	b := newTestNativeBackend(t)
	origin := createTestRepo(t)
	main := currentBranch(t, origin)
	clone := t.TempDir()
	cloneRepo(t, origin, clone)
	commitFile(t, origin, "o.txt", "origin", "origin work")
	commitFile(t, clone, "c.txt", "clone", "clone work")
	gitOutput(t, clone, "fetch", "-q", "origin")

	ahead, behind, err := b.GetBranchSyncInfo(context.Background(), clone, main, "origin/"+main)
	if err != nil {
		t.Fatalf("GetBranchSyncInfo: %v", err)
	}
	if ahead != 1 || behind != 1 {
		t.Errorf("expected ahead=1 behind=1, got %d/%d", ahead, behind)
	}
}

func TestNative_MergeVariantsAndCherryPick(t *testing.T) {
	b := newTestNativeBackend(t)
	ctx := context.Background()

	repo := divergedRepo(t)
	if err := b.Merge(ctx, repo, "feature", MergeOptions{Message: "native merge"}); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got := gitOutput(t, repo, "log", "-1", "--pretty=%s"); got != "native merge" {
		t.Errorf("expected the merge commit on HEAD, got %q", got)
	}
	for _, file := range []string{"main-only.txt", "feature-only.txt"} {
		if _, err := os.Stat(filepath.Join(repo, file)); err != nil {
			t.Errorf("expected %s after the merge: %v", file, err)
		}
	}

	// FF-only on diverged branches fails.
	repo2 := divergedRepo(t)
	if err := b.Merge(ctx, repo2, "feature", MergeOptions{FFOnly: true}); err == nil {
		t.Fatal("expected the ff-only merge to fail on diverged branches")
	}

	// Same-file edits on both sides classify as a merge conflict.
	repo3 := createTestRepo(t)
	main := currentBranch(t, repo3)
	gitOutput(t, repo3, "checkout", "-b", "feature")
	commitFile(t, repo3, "shared.txt", "feature version", "feature edit")
	gitOutput(t, repo3, "checkout", main)
	commitFile(t, repo3, "shared.txt", "main version", "main edit")
	err := b.Merge(ctx, repo3, "feature", MergeOptions{})
	if err == nil || !IsMergeConflict(err) {
		t.Fatalf("expected a merge-conflict error, got %v", err)
	}

	// Cherry-pick replays a side commit onto the default branch.
	repo4 := createTestRepo(t)
	main4 := currentBranch(t, repo4)
	gitOutput(t, repo4, "checkout", "-b", "side")
	picked := commitFile(t, repo4, "note.txt", "picked", "side commit")
	gitOutput(t, repo4, "checkout", main4)
	if err := b.CherryPick(ctx, repo4, picked); err != nil {
		t.Fatalf("CherryPick: %v", err)
	}
	if got := gitOutput(t, repo4, "log", "-1", "--pretty=%s"); got != "side commit" {
		t.Errorf("expected the picked message, got %q", got)
	}
}

// nativeRebaseSetup mirrors rebaseSetup but on a repo whose default branch is
// renamed to "main", so the competing-edit scenario below addresses it directly.
func nativeRebaseSetup(t *testing.T) (string, string, string) {
	t.Helper()
	repo := createTestRepo(t)
	defaultBranch := currentBranch(t, repo)
	gitOutput(t, repo, "branch", "-m", defaultBranch, "main")
	gitOutput(t, repo, "checkout", "-q", "-b", "topic")
	commitFile(t, repo, "topic.txt", "topic work", "topic work")
	topicTip := headHash(t, repo)
	gitOutput(t, repo, "checkout", "-q", "main")
	commitFile(t, repo, "main-only.txt", "main work", "main work")
	gitOutput(t, repo, "checkout", "-q", "topic")
	return repo, "main", topicTip
}

func TestNative_RebaseAndAbort(t *testing.T) {
	b := newTestNativeBackend(t)
	ctx := context.Background()

	repo, main, topicTip := nativeRebaseSetup(t)
	if err := b.Rebase(ctx, repo, main); err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	if got := headHash(t, repo); got == topicTip {
		t.Error("expected topic rewritten by the rebase")
	}
	for _, file := range []string{"topic.txt", "main-only.txt"} {
		if _, err := os.Stat(filepath.Join(repo, file)); err != nil {
			t.Errorf("expected %s after the rebase: %v", file, err)
		}
	}

	// A conflicting rebase leaves the repo mid-rebase; abort rolls it back.
	repo2, _, _ := nativeRebaseSetup(t)
	gitOutput(t, repo2, "checkout", "-q", "main")
	commitFile(t, repo2, "topic.txt", "competing", "main hijacks topic.txt")
	gitOutput(t, repo2, "checkout", "-q", "topic")
	before := headHash(t, repo2)
	if err := b.Rebase(ctx, repo2, "main"); !IsMergeConflict(err) {
		t.Fatalf("expected a conflict error, got %v", err)
	}
	if err := b.RebaseAbort(ctx, repo2); err != nil {
		t.Fatalf("RebaseAbort: %v", err)
	}
	if got := headHash(t, repo2); got != before {
		t.Errorf("expected HEAD restored to %s, got %s", before, got)
	}
}

func TestNative_GetCommitAddAndCommitWithIdentity(t *testing.T) {
	b := newTestNativeBackend(t)
	ctx := context.Background()
	repo := createTestRepo(t)

	if err := os.WriteFile(filepath.Join(repo, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := b.Add(ctx, repo, []string{"x.txt"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := b.CommitWithIdentity(ctx, repo, "NativeBot", "bot@example.com", "native bot work"); err != nil {
		t.Fatalf("CommitWithIdentity: %v", err)
	}
	c, err := b.GetCommit(ctx, repo, "HEAD")
	if err != nil || c.Message != "native bot work" || c.Author != "NativeBot" || c.Hash == "" {
		t.Errorf("GetCommit: got %+v (%v)", c, err)
	}
}

func TestGoGit_ConfigRoundTrip(t *testing.T) {
	b := newTestGoGitBackend(t)
	ctx := context.Background()
	origin, _, _ := pushSetup(t)
	clone := t.TempDir()
	cloneRepo(t, origin, clone)

	if err := b.SetConfig(ctx, clone, "custom.key", "value"); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	got, err := b.GetConfig(ctx, clone, "custom.key")
	if err != nil || got != "value" {
		t.Errorf("GetConfig(custom.key): got %q (%v)", got, err)
	}

	url, err := b.GetConfig(ctx, clone, "remote.origin.url")
	if err != nil || url != origin {
		t.Errorf("GetConfig(remote.origin.url): got %q (%v), want %q", url, err, origin)
	}

	if err := b.SetConfig(ctx, clone, "user.name", "CfgUser"); err != nil {
		t.Fatalf("SetConfig(user.name): %v", err)
	}
	if got, err := b.GetConfig(ctx, clone, "user.name"); err != nil || got != "CfgUser" {
		t.Errorf("GetConfig(user.name): got %q (%v)", got, err)
	}

	if _, err := b.GetConfig(ctx, clone, "not.a.valid.key"); err == nil {
		t.Error("expected an error for a malformed config key")
	}
}
