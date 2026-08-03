package tencentcode_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yi-nology/git-platform-sdk/backends/tencentcode"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// stripAPIPrefix removes the /api/v3 prefix that the gongfeng SDK
// automatically appends to all request paths.
func stripAPIPrefix(path string) string {
	return strings.TrimPrefix(path, "/api/v3")
}

func newTestProvider(t *testing.T, srv *httptest.Server) *tencentcode.Provider {
	t.Helper()
	p, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformTencentCode,
		BaseURL:  srv.URL,
		Token:    "test-token",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	tcp, ok := p.(*tencentcode.Provider)
	if !ok {
		t.Fatalf("expected *tencentcode.Provider, got %T", p)
	}
	return tcp
}

func tcMRResponse(iid int, state, title, source, target string) map[string]any {
	return map[string]any{
		"iid": iid, "title": title, "description": "desc", "state": state,
		"source_branch": source, "target_branch": target,
		"author":       map[string]any{"id": 1, "username": "dev", "name": "Developer"},
		"labels":       []string{"bug"},
		"merge_status": "can_be_merged",
		"web_url":      fmt.Sprintf("https://git.code.tencent.com/o/r/merge_requests/%d", iid),
		"created_at":   "2024-01-01T12:00:00+0800",
		"updated_at":   "2024-01-01T12:00:00+0800",
	}
}

func TestPlatform(t *testing.T) {
	p, err := provider.NewProvider(provider.Config{Platform: provider.PlatformTencentCode, Token: "token"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != provider.PlatformTencentCode {
		t.Errorf("expected tencent_code, got %s", p.Platform())
	}
}

func TestTestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := stripAPIPrefix(r.URL.Path)
		if path == "/user" {
			writeJSON(w, map[string]string{"username": "testuser"})
			return
		}
		writeJSON(w, []any{})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	result, err := p.TestConnection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Connected || result.UserName != "testuser" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestListRepos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{"id": 1, "name": "r1", "path_with_namespace": "owner/r1",
				"http_url_to_repo": "https://example.com/owner/r1.git",
				"default_branch":   "main", "visibility_level": 20},
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	repos, err := p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Owner != "owner" {
		t.Errorf("unexpected: %+v", repos)
	}
}

func TestCreateCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, tcMRResponse(7, "opened", "test", "feature", "main"))
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
	if cr.Number != 7 || cr.State != provider.CRStateOpened {
		t.Errorf("unexpected: %+v", cr)
	}
}

func TestListCRs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Count", "2")
		writeJSON(w, []map[string]any{
			tcMRResponse(1, "opened", "a", "a", "main"),
			tcMRResponse(2, "merged", "b", "b", "main"),
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	crs, total, err := p.ListCRs(context.Background(), provider.ListCROptions{Owner: "owner", Repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if crs[1].State != provider.CRStateMerged {
		t.Errorf("expected merged, got %s", crs[1].State)
	}
}

func TestParseWebhookEvent_MergeRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	body := `{"object_kind":"merge_request","user":{"id":1,"username":"dev","name":"Dev"},"project":{"path_with_namespace":"owner/repo"},"object_attributes":{"iid":7,"title":"t","description":"d","state":"opened","source_branch":"f","target_branch":"main","action":"open","merge_status":"can_be_merged","url":"https://git.code.tencent.com/o/r/merge_requests/7","last_commit":{"id":"abc"},"created_at":"2024-01-01T12:00:00+0800","updated_at":"2024-01-01T12:00:00+0800"}}`
	r, _ := http.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ne, err := p.ParseWebhookEvent(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if ne.Type != "cr.open" {
		t.Errorf("expected cr.open, got %s", ne.Type)
	}
	if ne.CR == nil || ne.CR.Number != 7 {
		t.Errorf("expected CR with number 7, got %+v", ne.CR)
	}
}

func TestParseWebhookEvent_Push(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	body := `{"object_kind":"push","user":{"id":1,"username":"dev","name":"Dev"},"project":{"path_with_namespace":"owner/repo"},"ref":"refs/heads/main","after":"abc123"}`
	r, _ := http.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ne, err := p.ParseWebhookEvent(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if ne.Type != "push" || ne.Branch != "main" || ne.CommitSHA != "abc123" {
		t.Errorf("unexpected event: %+v", ne)
	}
}

func TestValidateWebhookSignature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	r, _ := http.NewRequest(http.MethodPost, "/hook", nil)
	r.Header.Set("X-Token", "the-secret")
	if err := p.ValidateWebhookSignature(r, "the-secret"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	r2, _ := http.NewRequest(http.MethodPost, "/hook", nil)
	r2.Header.Set("X-Token", "wrong")
	if err := p.ValidateWebhookSignature(r2, "the-secret"); err == nil {
		t.Error("expected error for wrong token")
	}
}

func TestGetFileContent_Base64(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{
			"content":  base64.StdEncoding.EncodeToString([]byte("hello world")),
			"encoding": "base64",
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	content, err := p.GetFileContent(context.Background(), "owner", "repo", "README.md", "main")
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello world" {
		t.Errorf("expected 'hello world', got %q", content)
	}
}

func TestListBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]string{{"name": "main"}, {"name": "dev"}})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	branches, err := p.ListBranches(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Errorf("expected 2, got %d", len(branches))
	}
}

func TestListTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]any{
			{"name": "v1.0", "commit": map[string]any{"id": "abc"}},
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	tags, err := p.ListTags(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Commit != "abc" {
		t.Errorf("unexpected: %+v", tags)
	}
}

func TestRetry_TriggersOn5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		writeJSON(w, []map[string]any{})
	}))
	defer srv.Close()
	p, err := tencentcode.New(provider.Config{
		Platform:    provider.PlatformTencentCode,
		BaseURL:     srv.URL,
		Token:       "test",
		RetryConfig: &provider.RetryConfig{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.ListRepos(context.Background(), provider.ListRepoOptions{})
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
		writeJSON(w, map[string]string{"message": "404 Not Found"})
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

func TestProvider_ImplementsProvider(t *testing.T) {
	var _ provider.Provider = (*tencentcode.Provider)(nil)
}

func TestProvider_ImplementsExtras(t *testing.T) {
	var _ tencentcode.TencentCodeExtras = (*tencentcode.Provider)(nil)
}
