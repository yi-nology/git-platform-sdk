package branchfilter

import (
	"errors"
	"path/filepath"
	"testing"
)

func mustNew(t *testing.T, filterStr string) *BranchFilter {
	t.Helper()
	f, err := New(filterStr)
	if err != nil {
		t.Fatalf("New(%q): %v", filterStr, err)
	}
	return f
}

func TestNew_Empty(t *testing.T) {
	f := mustNew(t, "")
	if !f.IsEmpty() {
		t.Error("expected empty filter")
	}
}

func TestNew_SinglePattern(t *testing.T) {
	f := mustNew(t, "main")
	if f.IsEmpty() {
		t.Error("expected non-empty filter")
	}
	if len(f.Patterns()) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(f.Patterns()))
	}
}

func TestNew_BadPatternRejected(t *testing.T) {
	f, err := New("main,[bad")
	if err == nil {
		t.Fatal("expected an error for the malformed pattern")
	}
	if f != nil {
		t.Errorf("expected a nil filter alongside the error, got %+v", f)
	}
	if !errors.Is(err, filepath.ErrBadPattern) {
		t.Errorf("expected the error to wrap filepath.ErrBadPattern, got %v", err)
	}
	if _, err := New("["); err == nil || !errors.Is(err, filepath.ErrBadPattern) {
		t.Errorf("expected an unclosed bracket to be rejected, got %v", err)
	}
}

func TestMatch_EmptyMatchesAll(t *testing.T) {
	f := mustNew(t, "")
	if !f.Match("any-branch") {
		t.Error("empty filter should match all branches")
	}
}

func TestMatch_ExactPattern(t *testing.T) {
	f := mustNew(t, "main")
	if !f.Match("main") {
		t.Error("expected match for 'main'")
	}
	if f.Match("develop") {
		t.Error("expected no match for 'develop'")
	}
}

func TestMatch_GlobPattern(t *testing.T) {
	f := mustNew(t, "release/*")
	if !f.Match("release/v1.0") {
		t.Error("expected match for 'release/v1.0'")
	}
	if f.Match("main") {
		t.Error("expected no match for 'main'")
	}
}

// TestMatch_StarDoesNotCrossSlash pins the documented semantics: `*` matches
// only within one path level (filepath.Match), so "release/*" never reaches
// "release/a/b" and there is no `**`.
func TestMatch_StarDoesNotCrossSlash(t *testing.T) {
	f := mustNew(t, "release/*")
	if f.Match("release/a/b") {
		t.Error(`expected "release/*" NOT to match "release/a/b": * does not cross /`)
	}
	deep := mustNew(t, "release/*/*")
	if !deep.Match("release/a/b") {
		t.Error(`expected "release/*/*" to match "release/a/b"`)
	}
}

func TestMatch_MultiplePatterns(t *testing.T) {
	f := mustNew(t, "main, develop, release/*")
	if !f.Match("main") {
		t.Error("expected match for 'main'")
	}
	if !f.Match("develop") {
		t.Error("expected match for 'develop'")
	}
	if !f.Match("release/v2.0") {
		t.Error("expected match for 'release/v2.0'")
	}
	if f.Match("feature/something") {
		t.Error("expected no match for 'feature/something'")
	}
}

func TestFilterBranches(t *testing.T) {
	f := mustNew(t, "main, release/*")
	branches := []string{"main", "develop", "release/v1.0", "feature/x"}
	filtered := f.FilterBranches(branches)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(filtered))
	}
	if filtered[0] != "main" || filtered[1] != "release/v1.0" {
		t.Errorf("unexpected filtered branches: %v", filtered)
	}
}

func TestFilterBranches_EmptyFilter(t *testing.T) {
	f := mustNew(t, "")
	branches := []string{"main", "develop"}
	filtered := f.FilterBranches(branches)
	if len(filtered) != 2 {
		t.Errorf("empty filter should return all branches, got %d", len(filtered))
	}
}

func TestNew_WhitespaceHandling(t *testing.T) {
	f := mustNew(t, " main , develop , feature/* ")
	if len(f.Patterns()) != 3 {
		t.Errorf("expected 3 patterns after trimming, got %d", len(f.Patterns()))
	}
}
