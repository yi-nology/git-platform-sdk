package gitee_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yi-nology/git-platform-sdk/backends/gitee"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func newTestProvider(t *testing.T, srv *httptest.Server) *gitee.Provider {
	t.Helper()
	p, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformGitee,
		BaseURL:  srv.URL,
		Token:    "test-token",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	gp, ok := p.(*gitee.Provider)
	if !ok {
		t.Fatalf("expected *gitee.Provider, got %T", p)
	}
	return gp
}

func TestListRepos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{"id": 1, "name": "r1", "full_name": "owner/r1",
				"owner": map[string]any{"login": "owner"}, "default_branch": "main"},
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	repos, err := p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1, got %d", len(repos))
	}
	if repos[0].Platform != provider.PlatformGitee {
		t.Errorf("expected Gitee, got %s", repos[0].Platform)
	}
}

func TestCreateCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"id": 7, "number": 7, "title": "test", "state": "open",
			"source_branch": "feature", "target_branch": "main",
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	cr, err := p.CreateCR(context.Background(), provider.CreateCROptions{
		Owner: "owner", Repo: "repo", Title: "test",
		SourceBranch: "feature", TargetBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cr.Number != 7 {
		t.Errorf("expected 7, got %d", cr.Number)
	}
}

func TestParseWebhookEvent_PullRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	body := `{"action":"open","number":1,"title":"t","state":"open","source_branch":"f","target_branch":"main","html_url":"https://gitee.com/owner/repo/pulls/1","user":{"id":1,"login":"dev","name":"Dev"},"repository":{"full_name":"owner/repo"}}`
	r, _ := http.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Header.Set("X-Gitee-Event", "pull_request")
	r.Header.Set("Content-Type", "application/json")
	ne, err := p.ParseWebhookEvent(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if ne.Type != "cr.open" {
		t.Errorf("expected cr.open, got %s", ne.Type)
	}
}

func TestParseWebhookEvent_Push(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	body := `{"ref":"refs/heads/main","after":"abc123","user":{"id":1,"login":"dev","name":"Dev"},"repository":{"full_name":"owner/repo"}}`
	r, _ := http.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Header.Set("X-Gitee-Event", "push")
	r.Header.Set("Content-Type", "application/json")
	ne, err := p.ParseWebhookEvent(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if ne.Type != "push" {
		t.Errorf("expected push, got %s", ne.Type)
	}
	if ne.Branch != "main" {
		t.Errorf("expected main, got %s", ne.Branch)
	}
}

func TestProvider_ImplementsProvider(t *testing.T) {
	var _ provider.Provider = (*gitee.Provider)(nil)
}

func TestRetry_TriggersOn5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, []map[string]any{})
	}))
	defer srv.Close()
	p, err := gitee.New(provider.Config{
		Platform:    provider.PlatformGitee,
		BaseURL:     srv.URL,
		Token:       "test",
		RetryConfig: &provider.RetryConfig{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 1, PerPage: 5})
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got < 2 {
		t.Errorf("expected at least 2 calls, got %d", got)
	}
}

func TestIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	_, err := p.GetRepo(context.Background(), "missing", "repo")
	if err == nil {
		t.Fatal("expected error")
	}
	if !provider.IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}
