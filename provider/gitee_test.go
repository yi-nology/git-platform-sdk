package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestGiteeProvider(t *testing.T, handler http.Handler) Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p, err := NewProvider(Config{
		Platform: PlatformGitee,
		BaseURL:  srv.URL + "/api/v5",
		Token:    "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeJSONGitee(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func TestGitee_TestConnection(t *testing.T) {
	p := newTestGiteeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v5/user":
			writeJSONGitee(w, map[string]string{"login": "testuser"})
		case "/api/v5/user/repos":
			writeJSONGitee(w, []interface{}{})
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

func TestGitee_ListRepos(t *testing.T) {
	p := newTestGiteeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v5/user/repos" {
			writeJSONGitee(w, []map[string]interface{}{
				{"id": 1, "full_name": "owner/repo", "name": "repo", "owner": map[string]string{"login": "owner"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	repos, err := p.ListRepos(context.Background(), ListRepoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].FullName != "owner/repo" {
		t.Errorf("expected owner/repo, got %s", repos[0].FullName)
	}
}

func TestGitee_GetRepo(t *testing.T) {
	p := newTestGiteeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v5/repos/owner/repo" {
			writeJSONGitee(w, map[string]interface{}{
				"id": 1, "full_name": "owner/repo", "name": "repo",
				"owner": map[string]string{"login": "owner"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	repo, err := p.GetRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if repo.FullName != "owner/repo" {
		t.Errorf("expected owner/repo, got %s", repo.FullName)
	}
}

func TestGitee_CreateCR(t *testing.T) {
	p := newTestGiteeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v5/repos/owner/repo/pulls" {
			writeJSONGitee(w, map[string]interface{}{
				"id": 1, "number": 1, "title": "test PR", "state": "opened",
				"source_branch": "feature", "target_branch": "main",
				"user": map[string]interface{}{"id": 1, "login": "author"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	cr, err := p.CreateCR(context.Background(), CreateCROptions{
		Owner: "owner", Repo: "repo", Title: "test PR",
		SourceBranch: "feature", TargetBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cr.Title != "test PR" {
		t.Errorf("expected 'test PR', got %q", cr.Title)
	}
	if cr.State != CRStateOpened {
		t.Errorf("expected opened, got %q", cr.State)
	}
}

func TestGitee_ListBranches(t *testing.T) {
	p := newTestGiteeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v5/repos/owner/repo/branches" {
			writeJSONGitee(w, []map[string]interface{}{
				{"name": "main"},
				{"name": "feature"},
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
}

func TestGitee_ValidateWebhookSignature(t *testing.T) {
	p := newTestGiteeProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	// Test with secret
	r, _ := http.NewRequest("POST", "/", nil)
	r.Header.Set("X-Gitee-Token", "mysecret")
	err := p.ValidateWebhookSignature(r, "mysecret")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Test with wrong secret
	r2, _ := http.NewRequest("POST", "/", nil)
	r2.Header.Set("X-Gitee-Token", "wrong")
	err = p.ValidateWebhookSignature(r2, "mysecret")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}

	// Test with empty secret (skip validation)
	r3, _ := http.NewRequest("POST", "/", nil)
	err = p.ValidateWebhookSignature(r3, "")
	if err != nil {
		t.Fatalf("expected no error with empty secret, got %v", err)
	}
}
