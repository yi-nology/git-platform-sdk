package provider

import (
	"errors"
	"net/http"
	"testing"
)

func TestProviderError_Error(t *testing.T) {
	tests := []struct {
		name string
		e    *ProviderError
		want string
	}{
		{
			"minimal",
			&ProviderError{Op: "ListRepos"},
			"ListRepos",
		},
		{
			"with platform",
			&ProviderError{Platform: PlatformGitHub, Op: "ListRepos"},
			"github ListRepos",
		},
		{
			"with resource",
			&ProviderError{Platform: PlatformGitHub, Op: "GetRepo", Resource: "owner/repo"},
			"github GetRepo owner/repo",
		},
		{
			"with status",
			&ProviderError{Platform: PlatformGitHub, Op: "GetRepo", Resource: "owner/repo", StatusCode: 404, Cause: ErrNotFound},
			"github GetRepo owner/repo: HTTP 404: resource not found",
		},
		{
			"with cause no status",
			&ProviderError{Platform: PlatformGitHub, Op: "TestConnection", Cause: errors.New("dial tcp: timeout")},
			"github TestConnection: dial tcp: timeout",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.e.Error()
			if got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProviderError_Is(t *testing.T) {
	pe := &ProviderError{Platform: PlatformGitHub, Op: "GetRepo", Cause: ErrNotFound}
	if !errors.Is(pe, ErrNotFound) {
		t.Error("expected errors.Is to match wrapped sentinel")
	}
	if errors.Is(pe, ErrAuthentication) {
		t.Error("expected not to match different sentinel")
	}
}

func TestProviderError_IsStatus(t *testing.T) {
	pe := &ProviderError{StatusCode: 404}
	if !pe.IsStatus(404) {
		t.Error("expected IsStatus(404) true")
	}
	if pe.IsStatus(500) {
		t.Error("expected IsStatus(500) false")
	}
}

func TestProviderError_ClassBoundaries(t *testing.T) {
	for _, c := range []struct {
		code   int
		client bool
		server bool
	}{
		{399, false, false},
		{400, true, false},
		{499, true, false},
		{500, false, true},
		{599, false, true},
		{600, false, false},
	} {
		pe := &ProviderError{StatusCode: c.code}
		if got := pe.IsClientError(); got != c.client {
			t.Errorf("status %d IsClientError=%v, want %v", c.code, got, c.client)
		}
		if got := pe.IsServerError(); got != c.server {
			t.Errorf("status %d IsServerError=%v, want %v", c.code, got, c.server)
		}
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{http.StatusNotFound, ErrNotFound},
		{http.StatusUnauthorized, ErrAuthentication},
		{http.StatusForbidden, ErrForbidden},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusConflict, ErrConflict},
		{http.StatusInternalServerError, nil},
		{http.StatusBadGateway, nil},
		{418, nil},
	}
	for _, tc := range tests {
		err := New(PlatformGitHub, "Test", tc.status, "")
		if tc.want != nil && !errors.Is(err, tc.want) {
			t.Errorf("status %d: expected %v, got %v", tc.status, tc.want, err)
		}
	}
}

func TestNew_WithBody(t *testing.T) {
	err := New(PlatformGitHub, "Test", 404, "not found")
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected ErrNotFound")
	}
	if got := err.Error(); !containsSubstr(got, "not found") {
		t.Errorf("expected body in error, got %q", got)
	}
}

func TestWrap(t *testing.T) {
	// Wrap preserves the cause chain
	cause := errors.New("boom")
	err := Wrap(PlatformGitHub, "GetRepo", cause)
	if !errors.Is(err, cause) {
		t.Error("expected cause to be preserved")
	}
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Error("expected *ProviderError")
	}
	if pe.Op != "GetRepo" {
		t.Errorf("expected op GetRepo, got %q", pe.Op)
	}
}

func TestWrap_WithStatusCoder(t *testing.T) {
	statusErr := &fakeStatusError{code: 404}
	err := Wrap(PlatformGitHub, "GetRepo", statusErr)
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatal("expected *ProviderError")
	}
	if pe.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", pe.StatusCode)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Error("expected ErrNotFound after classification")
	}
}

func TestWrapf(t *testing.T) {
	err := Wrapf(PlatformGitHub, "GetRepo", "%s/%s", "owner", "repo")
	if !errors.Is(err, err) {
		t.Error("non-nil error should self-match")
	}
	if !containsSubstr(err.Error(), "owner/repo") {
		t.Errorf("expected formatted message, got %q", err.Error())
	}
}

func TestWrap_AlreadyWrapped(t *testing.T) {
	inner := Wrap(PlatformGitHub, "ListRepos", errors.New("boom"))
	outer := Wrap(PlatformGitHub, "ListRepos", inner)
	if outer != inner {
		t.Error("Wrap should be idempotent on identical (platform, op)")
	}
}

func TestClassifyStatus(t *testing.T) {
	if !errors.Is(ClassifyStatus(404), ErrNotFound) {
		t.Error("expected 404 to be ErrNotFound")
	}
	// 5xx returns a non-sentinel error; just verify no panic.
	_ = ClassifyStatus(502)
}

// fakeStatusError is a test double for the statusCoder interface.
type fakeStatusError struct {
	code int
}

func (e *fakeStatusError) Error() string   { return "fake" }
func (e *fakeStatusError) StatusCode() int { return e.code }
func (e *fakeStatusError) Unwrap() error   { return nil }

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
