package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestGitCodeProvider(t *testing.T, handler http.Handler) Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p, err := NewProvider(Config{
		Platform: PlatformGitCode,
		BaseURL:  srv.URL,
		Token:    "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeJSONGitCode(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func TestGitCode_TestConnection(t *testing.T) {
	p := newTestGitCodeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			writeJSONGitCode(w, map[string]string{"login": "testuser"})
		case "/repositories":
			writeJSONGitCode(w, []interface{}{})
		default:
			http.NotFound(w, r)
		}
	}))
	result, err := p.TestConnection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Connected {
		t.Error("expected connected")
	}
	if result.UserName != "testuser" {
		t.Errorf("expected testuser, got %s", result.UserName)
	}
}

func TestGitCode_ListBranches(t *testing.T) {
	p := newTestGitCodeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/branches" {
			writeJSONGitCode(w, []map[string]interface{}{
				{"name": "main"},
				{"name": "develop"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	branches, err := p.ListBranches(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(branches))
	}
	if branches[0].Name != "main" {
		t.Errorf("expected main, got %s", branches[0].Name)
	}
}

func TestGitCode_CreateBranch(t *testing.T) {
	p := newTestGitCodeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/repos/owner/repo/branches" {
			writeJSONGitCode(w, map[string]interface{}{"name": "feature"})
			return
		}
		http.NotFound(w, r)
	}))
	branch, err := p.CreateBranch(context.Background(), "owner", "repo", "feature", "main")
	if err != nil {
		t.Fatal(err)
	}
	if branch.Name != "feature" {
		t.Errorf("expected feature, got %s", branch.Name)
	}
}

func TestGitCode_ValidateWebhookSignature(t *testing.T) {
	p := newTestGitCodeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	// Test with empty secret (skip validation)
	r, _ := http.NewRequest("POST", "/", nil)
	err := p.ValidateWebhookSignature(r, "")
	if err != nil {
		t.Fatalf("expected nil with empty secret, got %v", err)
	}

	// Test with missing signature header
	r2, _ := http.NewRequest("POST", "/", nil)
	err = p.ValidateWebhookSignature(r2, "mysecret")
	if err == nil {
		t.Fatal("expected error for missing signature")
	}
}

func TestGitCode_CreateDiscussion(t *testing.T) {
	p := newTestGitCodeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/repos/owner/repo/issues/1/comments" {
			writeJSONGitCode(w, map[string]interface{}{"id": 100})
			return
		}
		http.NotFound(w, r)
	}))
	id, err := p.CreateDiscussion(context.Background(), "owner", "repo", 1, DiscussionOptions{
		Body: "test comment",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}
}
