package gitbackend

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestGitErrorError verifies the rendered message includes the op, path,
// and cause, and that stderr joins the message only when present.
func TestGitErrorError(t *testing.T) {
	cause := errors.New("boom")
	withStderr := &GitError{Op: "Fetch", Path: "/repo", Stderr: "fatal: no such ref", Err: cause}
	got := withStderr.Error()
	for _, want := range []string{"Fetch", "/repo", "fatal: no such ref", "boom"} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q, want it to contain %q", got, want)
		}
	}
	plain := (&GitError{Op: "Pull", Path: "/repo", Err: cause}).Error()
	if strings.Contains(plain, ": :") || !strings.Contains(plain, "Pull") || !strings.Contains(plain, "boom") {
		t.Errorf("stderr-less Error() = %q, want op and cause without empty stderr segment", plain)
	}
}

// TestGitErrorUnwrapAndIs verifies GitError participates in errors.Is /
// errors.As through both Unwrap and the Is override.
func TestGitErrorUnwrapAndIs(t *testing.T) {
	inner := newGitError("Checkout", "/repo", "", ErrBranchNotFound)
	if !errors.Is(inner, ErrBranchNotFound) {
		t.Error("expected errors.Is to see ErrBranchNotFound through GitError")
	}
	wrapped := fmt.Errorf("outer: %w", inner)
	if !errors.Is(wrapped, ErrBranchNotFound) {
		t.Error("expected errors.Is to survive an extra wrap layer")
	}
	var ge *GitError
	if !errors.As(wrapped, &ge) || ge.Op != "Checkout" {
		t.Errorf("expected errors.As to recover the *GitError, got %+v", ge)
	}
	if errors.Is(inner, ErrRepoNotFound) {
		t.Error("did not expect ErrRepoNotFound to match")
	}
}

// TestSentinelClassification verifies the three public predicates map the
// sentinel families they document, and nothing else.
func TestSentinelClassification(t *testing.T) {
	notFound := []error{ErrRepoNotFound, ErrBranchNotFound, ErrRemoteNotFound, ErrFileNotFound}
	for _, sentinel := range notFound {
		err := newGitError("Get", "/repo", "", sentinel)
		if !IsNotFound(err) {
			t.Errorf("IsNotFound(%v) = false, want true", err)
		}
		if IsAuthFailed(err) || IsMergeConflict(err) {
			t.Errorf("sentinel %v misclassified as auth/conflict", err)
		}
	}
	auth := newGitError("Push", "/repo", "fatal: Authentication failed", ErrAuthFailed)
	if !IsAuthFailed(auth) || IsNotFound(auth) || IsMergeConflict(auth) {
		t.Errorf("auth error misclassified: %v", auth)
	}
	conflict := newGitError("Merge", "/repo", "CONFLICT (content)", ErrMergeConflict)
	if !IsMergeConflict(conflict) || IsNotFound(conflict) || IsAuthFailed(conflict) {
		t.Errorf("conflict error misclassified: %v", conflict)
	}
	if IsNotFound(errors.New("plain")) || IsAuthFailed(nil) || IsMergeConflict(nil) {
		t.Error("plain/nil errors must not classify as anything")
	}
}
