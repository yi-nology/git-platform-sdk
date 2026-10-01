package gitbackend

import (
	"context"
	"strings"
	"testing"
)

// TestNative_CommitParsingSurvivesPipeInSubject 回归：提交标题含 "|" 时，
// 旧的 `%H|%s|%an|%ai` 管道分隔会把 Author/Date 解析错位（实测
// "fix: handle a | b in parser | more" 曾解析出 Author=" b"）。
// v0.71.0 改 NUL（%x00）分隔修复。
func TestNative_CommitParsingSurvivesPipeInSubject(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)
	subject := "fix: handle a | b in parser | more"
	commitFile(t, repo, "a.txt", "v2", subject)

	commits, err := b.GetCommitsBetween(context.Background(), repo, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	c := commits[0]
	if c.Message != subject {
		t.Errorf("Message misparsed: %q", c.Message)
	}
	if c.Author != "Test" {
		t.Errorf("Author misparsed: %q", c.Author)
	}
	if !strings.HasPrefix(c.Date, "20") {
		t.Errorf("Date misparsed: %q", c.Date)
	}

	one, err := b.GetCommit(context.Background(), repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if one.Message != subject || one.Author != "Test" {
		t.Errorf("GetCommit misparsed: %+v", one)
	}

	hist, err := b.GetFileHistory(context.Background(), repo, "a.txt", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) == 0 || hist[0].Message != subject {
		t.Errorf("GetFileHistory misparsed: %+v", hist)
	}
}

// TestNative_GetTagList_PipeInMessage 回归：标签消息含 "|" 时旧实现把
// Author 挤错位（subject 原在倒数第二位）。现在 subject 置尾整体吸收。
func TestNative_GetTagList_PipeInMessage(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)
	msg := "release | v1 | notes"
	gitOutput(t, repo, "tag", "-a", "v1.0", "-m", msg)

	tags, err := b.GetTagList(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Name != "v1.0" || tags[0].Message != msg {
		t.Errorf("tag misparsed: %+v", tags[0])
	}
	if tags[0].Author != "Test" {
		t.Errorf("Author misparsed: %q", tags[0].Author)
	}
}

// TestNative_ListBranches_PipeInSubject：subject 置尾 + TAB 分隔后，
// 标题里的任意分隔符不再错位。
func TestNative_ListBranches_PipeInSubject(t *testing.T) {
	b := newTestNativeBackend(t)
	repo := createTestRepo(t)
	subject := "feat: route a | b | c"
	commitFile(t, repo, "b.txt", "x", subject)

	branches, err := b.ListBranches(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 1 {
		t.Fatalf("expected 1 branch, got %d", len(branches))
	}
	if branches[0].Message != subject {
		t.Errorf("subject misparsed: %q", branches[0].Message)
	}
	if branches[0].Author != "Test" {
		t.Errorf("Author misparsed: %q", branches[0].Author)
	}
}

// TestIsConflictOutput：真实的 git 冲突必带大写 "CONFLICT (" 标记；
// 小写 "conflict" 子串也出现在无关错误里（如合并不存在的
// "conflict-fix" 分支），旧实现会误判成 ErrMergeConflict。
func TestIsConflictOutput(t *testing.T) {
	cases := []struct {
		name   string
		stdout string
		stderr string
		want   bool
	}{
		{"content conflict", "Auto-merging f\nCONFLICT (content): Merge conflict in f\nAutomatic merge failed", "", true},
		{"modify/delete on stderr", "", "CONFLICT (modify/delete): f deleted", true},
		{"nonexistent branch named conflict-*", "", "merge: conflict-fix - not something we can merge", false},
		{"successful merge touching conflict-named path", "Merge made by the 'ort' strategy.\n conflict-fix/test.txt | 2 ++", "", false},
		{"unrelated failure", "", "fatal: not a git repository", false},
	}
	for _, c := range cases {
		if got := isConflictOutput(c.stdout, c.stderr); got != c.want {
			t.Errorf("%s: isConflictOutput = %v, want %v", c.name, got, c.want)
		}
	}
}
