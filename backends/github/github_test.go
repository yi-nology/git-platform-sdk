package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sdkgithub "github.com/google/go-github/v69/github"

	ghbackend "github.com/yi-nology/git-platform-sdk/backends/github"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func newTestProvider(t *testing.T, baseURL string) *ghbackend.Provider {
	t.Helper()
	p, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformGitHub,
		BaseURL:  baseURL,
		Token:    "test-token",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	gp, ok := p.(*ghbackend.Provider)
	if !ok {
		t.Fatalf("expected *ghbackend.Provider, got %T", p)
	}
	return gp
}

func TestNewProvider_Success(t *testing.T) {
	p, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformGitHub,
		BaseURL:  "http://example.test/api/v3",
		Token:    "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != provider.PlatformGitHub {
		t.Errorf("expected GitHub, got %s", p.Platform())
	}
}

func TestListRepos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]*sdkgithub.Repository{
			{ID: sdkgithub.Ptr(int64(1)), FullName: sdkgithub.Ptr("owner/r1"), Name: sdkgithub.Ptr("r1"),
				Owner: &sdkgithub.User{Login: sdkgithub.Ptr("owner")}, DefaultBranch: sdkgithub.Ptr("main")},
			{ID: sdkgithub.Ptr(int64(2)), FullName: sdkgithub.Ptr("owner/r2"), Name: sdkgithub.Ptr("r2"),
				Owner: &sdkgithub.User{Login: sdkgithub.Ptr("owner")}, DefaultBranch: sdkgithub.Ptr("main"), Private: sdkgithub.Ptr(true)},
		})
	}))
	defer srv.Close()

	p := newTestProvider(t, srv.URL+"/api/v3")
	repos, err := p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2, got %d", len(repos))
	}
	if repos[0].Platform != provider.PlatformGitHub {
		t.Errorf("expected GitHub platform, got %s", repos[0].Platform)
	}
	if !repos[1].Private {
		t.Error("expected r2 to be private")
	}
}

func TestGetRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(&sdkgithub.Repository{
			ID: sdkgithub.Ptr(int64(42)), FullName: sdkgithub.Ptr("owner/repo"), Name: sdkgithub.Ptr("repo"),
			Owner: &sdkgithub.User{Login: sdkgithub.Ptr("owner")}, DefaultBranch: sdkgithub.Ptr("main"),
		})
	}))
	defer srv.Close()

	p := newTestProvider(t, srv.URL+"/api/v3")
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
		_ = json.NewEncoder(w).Encode(&sdkgithub.PullRequest{
			Number: sdkgithub.Ptr(7), Title: sdkgithub.Ptr("test"), State: sdkgithub.Ptr("open"),
			Head:    &sdkgithub.PullRequestBranch{Ref: sdkgithub.Ptr("feature"), SHA: sdkgithub.Ptr("abc")},
			Base:    &sdkgithub.PullRequestBranch{Ref: sdkgithub.Ptr("main")},
			User:    &sdkgithub.User{ID: sdkgithub.Ptr(int64(1)), Login: sdkgithub.Ptr("dev"), AvatarURL: sdkgithub.Ptr("https://a/v")},
			HTMLURL: sdkgithub.Ptr("https://github.com/owner/repo/pull/7"),
		})
	}))
	defer srv.Close()

	p := newTestProvider(t, srv.URL+"/api/v3")
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
	if cr.State != provider.CRStateOpened {
		t.Errorf("expected opened, got %s", cr.State)
	}
}

func TestListCRs_MergedDetected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]*sdkgithub.PullRequest{
			{Number: sdkgithub.Ptr(1), Title: sdkgithub.Ptr("a"), State: sdkgithub.Ptr("open"),
				Head: &sdkgithub.PullRequestBranch{Ref: sdkgithub.Ptr("a")}, Base: &sdkgithub.PullRequestBranch{Ref: sdkgithub.Ptr("main")},
				User: &sdkgithub.User{ID: sdkgithub.Ptr(int64(1)), Login: sdkgithub.Ptr("u")}},
			{Number: sdkgithub.Ptr(2), Title: sdkgithub.Ptr("b"), State: sdkgithub.Ptr("closed"), Merged: sdkgithub.Ptr(true),
				Head: &sdkgithub.PullRequestBranch{Ref: sdkgithub.Ptr("b")}, Base: &sdkgithub.PullRequestBranch{Ref: sdkgithub.Ptr("main")},
				User: &sdkgithub.User{ID: sdkgithub.Ptr(int64(1)), Login: sdkgithub.Ptr("u")}},
		})
	}))
	defer srv.Close()

	p := newTestProvider(t, srv.URL+"/api/v3")
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

func TestGetCRDiff_Pagination(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			// First page: return 100 files to force pagination
			files := make([]*sdkgithub.CommitFile, 100)
			for i := range files {
				files[i] = &sdkgithub.CommitFile{Filename: sdkgithub.Ptr("f" + string(rune('a'+i%26))), Status: sdkgithub.Ptr("modified"), Additions: sdkgithub.Ptr(1), Deletions: sdkgithub.Ptr(0)}
			}
			_ = json.NewEncoder(w).Encode(files)
			return
		}
		// Second page: 1 file
		_ = json.NewEncoder(w).Encode([]*sdkgithub.CommitFile{
			{Filename: sdkgithub.Ptr("last"), Status: sdkgithub.Ptr("added"), Additions: sdkgithub.Ptr(5)},
		})
	}))
	defer srv.Close()

	p := newTestProvider(t, srv.URL+"/api/v3")
	diff, err := p.GetCRDiff(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 calls (pagination), got %d", got)
	}
	if diff.TotalAdd == 0 {
		t.Error("expected non-zero additions")
	}
}

func TestParseWebhookEvent_PullRequest(t *testing.T) {
	p := newTestProvider(t, "http://example.test/api/v3")
	body := `{"action":"opened","number":1,"pull_request":{"number":1,"state":"open","title":"t","head":{"ref":"f","sha":"abc"},"base":{"ref":"main"}},"repository":{"full_name":"owner/repo"},"sender":{"login":"dev"}}`
	r, _ := http.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Header.Set("X-GitHub-Event", "pull_request")
	r.Header.Set("Content-Type", "application/json")
	ne, err := p.ParseWebhookEvent(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if ne.Type != "cr.opened" {
		t.Errorf("expected cr.opened, got %s", ne.Type)
	}
	if ne.CR == nil || ne.CR.Number != 1 {
		t.Errorf("expected PR with number 1, got %+v", ne.CR)
	}
}

func TestParseWebhookEvent_Merged(t *testing.T) {
	p := newTestProvider(t, "http://example.test/api/v3")
	body := `{"action":"closed","number":1,"pull_request":{"number":1,"state":"closed","merged":true,"head":{"ref":"f","sha":"abc"},"base":{"ref":"main"}},"repository":{"full_name":"owner/repo"},"sender":{"login":"dev"}}`
	r, _ := http.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Header.Set("X-GitHub-Event", "pull_request")
	r.Header.Set("Content-Type", "application/json")
	ne, err := p.ParseWebhookEvent(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if ne.Type != "cr.merged" {
		t.Errorf("expected cr.merged, got %s", ne.Type)
	}
}

func TestParseWebhookEvent_Push(t *testing.T) {
	p := newTestProvider(t, "http://example.test/api/v3")
	body := `{"ref":"refs/heads/main","after":"abc123","repository":{"full_name":"owner/repo"},"sender":{"login":"dev"}}`
	r, _ := http.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Header.Set("X-GitHub-Event", "push")
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
	if ne.CommitSHA != "abc123" {
		t.Errorf("expected abc123, got %s", ne.CommitSHA)
	}
}

func TestListBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]*sdkgithub.Branch{
			{Name: sdkgithub.Ptr("main")},
			{Name: sdkgithub.Ptr("dev")},
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv.URL+"/api/v3")
	branches, err := p.ListBranches(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Fatalf("expected 2, got %d", len(branches))
	}
}

func TestCreateBranch_WithCommitSHA(t *testing.T) {
	var gotRef string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRef = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ref":"refs/heads/newbranch","object":{"sha":"abc"}}`))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv.URL+"/api/v3")
	sha := strings.Repeat("a", 40)
	_, err := p.CreateBranch(context.Background(), "owner", "repo", "newbranch", sha)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotRef, "git/refs") {
		t.Errorf("expected git/refs endpoint, got %s", gotRef)
	}
}

func TestListTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]*sdkgithub.RepositoryTag{
			{Name: sdkgithub.Ptr("v1.0"), Commit: &sdkgithub.Commit{SHA: sdkgithub.Ptr("abc")}},
		})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv.URL+"/api/v3")
	tags, err := p.ListTags(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "v1.0" {
		t.Errorf("unexpected tags: %+v", tags)
	}
}

func TestCreateCommitStatus(t *testing.T) {
	var gotState string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			State string `json:"state"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotState = body.State
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	p := newTestProvider(t, srv.URL+"/api/v3")
	err := p.CreateCommitStatus(context.Background(), "owner", "repo", "abc", provider.CommitStatusOptions{
		State: "success", Context: "ci", Description: "ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotState != "success" {
		t.Errorf("expected success, got %q", gotState)
	}
}

func TestProvider_ImplementsProvider(t *testing.T) {
	var _ provider.Provider = (*ghbackend.Provider)(nil)
}

// TestGetFileContent_NotFound verifies the error classification. The
// go-github SDK's DownloadContents makes 2 HTTP calls (metadata + content
// fetch), which is brittle to mock in full here. Instead we test the
// error path that flows through provider.Wrap.
func TestGetFileContent_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv.URL+"/api/v3")
	_, err := p.GetFileContent(context.Background(), "owner", "repo", "missing.md", "main")
	if err == nil {
		t.Fatal("expected error")
	}
	if !provider.IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestRetry_TriggersOn5xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode([]*sdkgithub.Repository{})
	}))
	defer srv.Close()

	p, err := provider.NewProvider(provider.Config{
		Platform:    provider.PlatformGitHub,
		BaseURL:     srv.URL + "/api/v3",
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
		t.Errorf("expected at least 2 calls (retry), got %d", got)
	}
}

func TestIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv.URL+"/api/v3")
	_, err := p.GetRepo(context.Background(), "missing", "repo")
	if err == nil {
		t.Fatal("expected error")
	}
	if !provider.IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func TestValidateWebhookSignature_NoSecret(t *testing.T) {
	p := newTestProvider(t, "http://example.test/api/v3")
	r, _ := http.NewRequest(http.MethodPost, "/hook", nil)
	if err := p.ValidateWebhookSignature(r, ""); err != nil {
		t.Errorf("expected no error with empty secret, got %v", err)
	}
}
