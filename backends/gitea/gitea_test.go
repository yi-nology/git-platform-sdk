package gitea_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	giteasdk "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/backends/gitea"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// newTestProvider wraps the test server so the Gitea SDK's version-check
// request on construction succeeds. Tests that don't care about specific
// paths just respond with empty arrays.
func newTestProvider(t *testing.T, srv *httptest.Server) *gitea.Provider {
	t.Helper()
	// The Gitea SDK calls GET /api/v1/version on NewClient; serve it.
	versionSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			writeJSON(w, map[string]string{"version": "1.22.0"})
			return
		}
		srv.Config.Handler.ServeHTTP(w, r)
	}))
	pp, err := gitea.New(provider.Config{
		Platform: provider.PlatformGitea,
		BaseURL:  versionSrv.URL,
		Token:    "test-token",
	})
	if err != nil {
		t.Fatalf("gitea.New: %v", err)
	}
	gp, ok := pp.(*gitea.Provider)
	if !ok {
		t.Fatalf("expected *gitea.Provider, got %T", pp)
	}
	return gp
}

func TestListRepos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []*giteasdk.Repository{
			{ID: 1, FullName: "owner/r1", Name: "r1", Owner: &giteasdk.User{UserName: "owner"}, DefaultBranch: "main"},
			{ID: 2, FullName: "owner/r2", Name: "r2", Owner: &giteasdk.User{UserName: "owner"}, DefaultBranch: "main", Private: true},
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	repos, err := p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2, got %d", len(repos))
	}
	if repos[0].Platform != provider.PlatformGitea {
		t.Errorf("expected Gitea, got %s", repos[0].Platform)
	}
}

func TestGetRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, giteasdk.Repository{
			ID: 42, FullName: "owner/repo", Name: "repo",
			Owner: &giteasdk.User{UserName: "owner"}, DefaultBranch: "main",
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	repo, err := p.GetRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID != 42 || repo.Owner != "owner" {
		t.Errorf("unexpected repo: %+v", repo)
	}
}

func TestCreateCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, giteasdk.PullRequest{
			ID: 7, Index: 7, Title: "test", State: giteasdk.StateOpen,
			Head: &giteasdk.PRBranchInfo{Ref: "feature", Sha: "abc"},
			Base: &giteasdk.PRBranchInfo{Ref: "main"},
			Poster: &giteasdk.User{ID: 1, UserName: "dev"},
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

func TestListCRs_MergedDetected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []*giteasdk.PullRequest{
			{ID: 1, Index: 1, Title: "a", State: giteasdk.StateOpen,
				Head: &giteasdk.PRBranchInfo{Ref: "a"}, Base: &giteasdk.PRBranchInfo{Ref: "main"}},
			{ID: 2, Index: 2, Title: "b", State: giteasdk.StateClosed, HasMerged: true,
				Head: &giteasdk.PRBranchInfo{Ref: "b"}, Base: &giteasdk.PRBranchInfo{Ref: "main"}},
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	crs, _, err := p.ListCRs(context.Background(), provider.ListCROptions{Owner: "owner", Repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	if crs[1].State != provider.CRStateMerged {
		t.Errorf("expected merged, got %s", crs[1].State)
	}
}

func TestListBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []*giteasdk.Branch{{Name: "main"}, {Name: "dev"}})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	branches, err := p.ListBranches(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Fatalf("expected 2, got %d", len(branches))
	}
}

func TestParseWebhookEvent_PullRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	body := `{"action":"opened","number":1,"sender":{"id":1,"login":"dev"},"repository":{"full_name":"owner/repo"},"pull_request":{"number":1,"title":"t","state":"open","head":{"ref":"f","sha":"abc"},"base":{"ref":"main"},"html_url":"https://gitea.com/owner/repo/pulls/1","user":{"id":1,"login":"dev"},"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}}`
	r, _ := http.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Header.Set("X-Gitea-Event", "pull_request")
	r.Header.Set("Content-Type", "application/json")
	ne, err := p.ParseWebhookEvent(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if ne.Type != "cr.opened" {
		t.Errorf("expected cr.opened, got %s", ne.Type)
	}
	if ne.CR == nil || ne.CR.Number != 1 {
		t.Errorf("expected CR with number 1, got %+v", ne.CR)
	}
}

func TestProvider_ImplementsProvider(t *testing.T) {
	var _ provider.Provider = (*gitea.Provider)(nil)
}

func TestRetry_TriggersOn5xx(t *testing.T) {
	var calls int32
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, []*giteasdk.Repository{})
	}))
	defer inner.Close()
	// Wrap inner with version endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			writeJSON(w, map[string]string{"version": "1.22.0"})
			return
		}
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	defer srv.Close()
	p, err := gitea.New(provider.Config{
		Platform:    provider.PlatformGitea,
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
