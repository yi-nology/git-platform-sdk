package branchfilter

import "testing"

func TestNew_Empty(t *testing.T) {
	f := New("")
	if !f.IsEmpty() {
		t.Error("expected empty filter")
	}
}

func TestNew_SinglePattern(t *testing.T) {
	f := New("main")
	if f.IsEmpty() {
		t.Error("expected non-empty filter")
	}
	if len(f.Patterns()) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(f.Patterns()))
	}
}

func TestMatch_EmptyMatchesAll(t *testing.T) {
	f := New("")
	if !f.Match("any-branch") {
		t.Error("empty filter should match all branches")
	}
}

func TestMatch_ExactPattern(t *testing.T) {
	f := New("main")
	if !f.Match("main") {
		t.Error("expected match for 'main'")
	}
	if f.Match("develop") {
		t.Error("expected no match for 'develop'")
	}
}

func TestMatch_GlobPattern(t *testing.T) {
	f := New("release/*")
	if !f.Match("release/v1.0") {
		t.Error("expected match for 'release/v1.0'")
	}
	if f.Match("main") {
		t.Error("expected no match for 'main'")
	}
}

func TestMatch_MultiplePatterns(t *testing.T) {
	f := New("main, develop, release/*")
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
	f := New("main, release/*")
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
	f := New("")
	branches := []string{"main", "develop"}
	filtered := f.FilterBranches(branches)
	if len(filtered) != 2 {
		t.Errorf("empty filter should return all branches, got %d", len(filtered))
	}
}

func TestNew_WhitespaceHandling(t *testing.T) {
	f := New(" main , develop , feature/* ")
	if len(f.Patterns()) != 3 {
		t.Errorf("expected 3 patterns after trimming, got %d", len(f.Patterns()))
	}
}
