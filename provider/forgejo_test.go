package provider

import (
	"context"

	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func newForgejoTestServer(routes map[string]func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	for path, handler := range routes {
		mux.HandleFunc(path, handler)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			writeJSON(w, map[string]string{"version": "9.0.0"})
			return
		}
		for prefix, handler := range routes {
			if strings.HasPrefix(r.URL.Path, prefix) || r.URL.Path == prefix {
				handler(w, r)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})
	return httptest.NewServer(mux)
}

func newTestForgejoProvider(srv *httptest.Server) *forgejoProvider {
	client, _ := forgejo.NewClient(srv.URL, forgejo.SetToken("test-token"), forgejo.SetHTTPClient(srv.Client()))
	if client == nil {
		panic("forgejo client is nil - check test server")
	}
	return &forgejoProvider{client: client}
}

func TestForgejo_Platform(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){})
	defer srv.Close()
	p := newTestForgejoProvider(srv)
	if p.Platform() != PlatformForgejo {
		t.Errorf("expected forgejo, got %s", p.Platform())
	}
}

func TestForgejo_TestConnection(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/user": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, forgejo.User{UserName: "testuser"})
		},
		"/api/v1/repos": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*forgejo.Repository{})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

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

func TestForgejo_ListRepos(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/search": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "2")
			writeJSON(w, []*forgejo.Repository{
				{ID: 1, FullName: "owner/repo1", Name: "repo1", Owner: &forgejo.User{UserName: "owner"}, CloneURL: "https://codeberg.org/owner/repo1.git", DefaultBranch: "main"},
				{ID: 2, FullName: "owner/repo2", Name: "repo2", Owner: &forgejo.User{UserName: "owner"}, CloneURL: "https://codeberg.org/owner/repo2.git", DefaultBranch: "main", Private: true},
			})
		},
		"/api/v1/user/repos": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "2")
			writeJSON(w, []*forgejo.Repository{
				{ID: 1, FullName: "owner/repo1", Name: "repo1", Owner: &forgejo.User{UserName: "owner"}, CloneURL: "https://codeberg.org/owner/repo1.git", DefaultBranch: "main"},
				{ID: 2, FullName: "owner/repo2", Name: "repo2", Owner: &forgejo.User{UserName: "owner"}, CloneURL: "https://codeberg.org/owner/repo2.git", DefaultBranch: "main", Private: true},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	repos, err := p.ListRepos(context.Background(), ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2, got %d", len(repos))
	}
	if repos[0].Platform != PlatformForgejo {
		t.Errorf("unexpected platform: %s", repos[0].Platform)
	}
}

func TestForgejo_GetRepo(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, forgejo.Repository{ID: 42, FullName: "owner/repo", Name: "repo", Owner: &forgejo.User{UserName: "owner"}, DefaultBranch: "main"})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	repo, err := p.GetRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID != 42 {
		t.Errorf("expected 42, got %d", repo.ID)
	}
}

func TestForgejo_CreateCR(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			writeJSON(w, forgejo.PullRequest{
				ID: 7, Index: 7, Title: "test pr", Body: "desc", State: forgejo.StateOpen,
				Head: &forgejo.PRBranchInfo{Ref: "feature"}, Base: &forgejo.PRBranchInfo{Ref: "main"},
				Poster: &forgejo.User{ID: 1, UserName: "dev", AvatarURL: "https://avatar/dev.png"},
				Labels: []*forgejo.Label{{Name: "bug"}},
				HTMLURL: "https://codeberg.org/owner/repo/pulls/7",
				Created: &now, Updated: &now,
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	cr, err := p.CreateCR(context.Background(), CreateCROptions{
		Owner: "owner", Repo: "repo", Title: "test pr", Description: "desc",
		SourceBranch: "feature", TargetBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cr.Number != 7 {
		t.Errorf("expected 7, got %d", cr.Number)
	}
	if cr.Author.Username != "dev" {
		t.Errorf("expected dev, got %s", cr.Author.Username)
	}
	if cr.Author.AvatarURL != "https://avatar/dev.png" {
		t.Errorf("expected avatar URL, got %s", cr.Author.AvatarURL)
	}
}

func TestForgejo_GetCR(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/5": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			writeJSON(w, forgejo.PullRequest{
				ID: 5, Index: 5, Title: "found", State: forgejo.StateOpen,
				Head: &forgejo.PRBranchInfo{Ref: "feat"}, Base: &forgejo.PRBranchInfo{Ref: "main"},
				Poster: &forgejo.User{ID: 1, UserName: "u", AvatarURL: "https://avatar/u.png"},
				HTMLURL: "https://codeberg.org/o/r/pulls/5",
				Created: &now, Updated: &now,
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	cr, err := p.GetCR(context.Background(), "owner", "repo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if cr.Number != 5 {
		t.Errorf("expected 5, got %d", cr.Number)
	}
	if cr.Author.AvatarURL != "https://avatar/u.png" {
		t.Errorf("expected avatar URL, got %s", cr.Author.AvatarURL)
	}
}

func TestForgejo_ListCRs(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "3")
			now := time.Now()
			writeJSON(w, []*forgejo.PullRequest{
				{ID: 1, Index: 1, Title: "pr1", State: forgejo.StateOpen, Head: &forgejo.PRBranchInfo{Ref: "a"}, Base: &forgejo.PRBranchInfo{Ref: "main"}, Poster: &forgejo.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now},
				{ID: 2, Index: 2, Title: "pr2", State: forgejo.StateClosed, HasMerged: true, Head: &forgejo.PRBranchInfo{Ref: "b"}, Base: &forgejo.PRBranchInfo{Ref: "main"}, Poster: &forgejo.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	crs, total, err := p.ListCRs(context.Background(), ListCROptions{Owner: "owner", Repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(crs) != 2 {
		t.Fatalf("expected 2, got %d", len(crs))
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if crs[0].State != CRStateOpened {
		t.Errorf("expected opened, got %s", crs[0].State)
	}
	if crs[1].State != CRStateMerged {
		t.Errorf("expected merged, got %s", crs[1].State)
	}
}

func TestForgejo_CloseCR(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/3": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			writeJSON(w, forgejo.PullRequest{ID: 3, Index: 3, State: forgejo.StateClosed, Head: &forgejo.PRBranchInfo{Ref: "f"}, Base: &forgejo.PRBranchInfo{Ref: "main"}, Poster: &forgejo.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	cr, err := p.CloseCR(context.Background(), "owner", "repo", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateClosed {
		t.Errorf("expected closed, got %s", cr.State)
	}
}

func TestForgejo_ListBranches(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/branches": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*forgejo.Branch{{Name: "main"}, {Name: "develop"}})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	branches, err := p.ListBranches(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 || branches[0].Name != "main" {
		t.Errorf("unexpected: %v", branches)
	}
}

func TestForgejo_CreateWebhook(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/hooks": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, forgejo.Hook{ID: 100, Events: []string{"push", "pull_request"}, Config: map[string]string{"url": "https://cb.com/hook"}})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	hook, err := p.CreateWebhook(context.Background(), CreateWebhookOptions{
		Owner: "owner", Repo: "repo", URL: "https://cb.com/hook", Secret: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if hook.ID != 100 {
		t.Errorf("expected 100, got %d", hook.ID)
	}
	if len(hook.Events) != 2 {
		t.Errorf("expected 2 events, got %v", hook.Events)
	}
}

func TestForgejo_ListWebhooks(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/hooks": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*forgejo.Hook{
				{ID: 1, Events: []string{"push"}, Config: map[string]string{"url": "u1"}},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	hooks, err := p.ListWebhooks(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1, got %d", len(hooks))
	}
}

func TestForgejo_DeleteWebhook(t *testing.T) {
	called := false
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/hooks/42": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				called = true
			}
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	err := p.DeleteWebhook(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestForgejo_CreateNote(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/issues/5/comments": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, forgejo.Comment{ID: 999})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	id, err := p.CreateNote(context.Background(), "owner", "repo", 5, "nice")
	if err != nil {
		t.Fatal(err)
	}
	if id != "999" {
		t.Errorf("expected 999, got %s", id)
	}
}

func TestForgejo_ValidateWebhookSignature(t *testing.T) {
	p := &forgejoProvider{}
	err := p.ValidateWebhookSignature(httptest.NewRequest("POST", "/", nil), "")
	if err != nil {
		t.Errorf("empty secret should pass: %v", err)
	}
}

func TestForgejo_ParseWebhookEvent_Push(t *testing.T) {
	p := &forgejoProvider{}
	body := `{"ref":"refs/heads/main","after":"abc123","sender":{"id":1,"login":"dev"},"repository":{"full_name":"owner/repo"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Forgejo-Event", "push")

	event, err := p.ParseWebhookEvent(req, "")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "push" {
		t.Errorf("expected push, got %s", event.Type)
	}
	if event.Branch != "main" {
		t.Errorf("expected main, got %s", event.Branch)
	}
}

func TestForgejo_ParseWebhookEvent_PR(t *testing.T) {
	p := &forgejoProvider{}
	body := fmt.Sprintf(`{"action":"opened","number":1,"pull_request":{"id":1,"number":1,"title":"test","body":"desc","state":"open","head":{"ref":"feat","sha":"sha1"},"base":{"ref":"main"},"merged":false,"html_url":"https://codeberg.org/o/r/pulls/1","user":{"id":1,"login":"dev"},"created_at":"%s","updated_at":"%s"},"sender":{"id":1,"login":"dev"},"repository":{"full_name":"o/r"}}`,
		time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Forgejo-Event", "pull_request")

	event, err := p.ParseWebhookEvent(req, "")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "cr.opened" {
		t.Errorf("expected cr.opened, got %s", event.Type)
	}
	if event.CR == nil || event.CR.Number != 1 {
		t.Fatal("expected CR with number 1")
	}
}

func TestForgejo_UpdateCR(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/7": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "PATCH" {
				t.Errorf("expected PATCH, got %s", r.Method)
			}
			now := time.Now()
			writeJSON(w, forgejo.PullRequest{
				ID: 7, Index: 7, Title: "updated title", Body: "new desc", State: forgejo.StateOpen,
				Head: &forgejo.PRBranchInfo{Ref: "feature"}, Base: &forgejo.PRBranchInfo{Ref: "develop"},
				Poster: &forgejo.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now,
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	cr, err := p.UpdateCR(context.Background(), "owner", "repo", 7, UpdateCROptions{Title: "updated title", Description: "new desc", TargetBranch: "develop"})
	if err != nil {
		t.Fatal(err)
	}
	if cr.Title != "updated title" {
		t.Errorf("expected updated title, got %s", cr.Title)
	}
}

func TestForgejo_ReopenCR(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/3": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			writeJSON(w, forgejo.PullRequest{
				ID: 3, Index: 3, Title: "reopened", State: forgejo.StateOpen,
				Head: &forgejo.PRBranchInfo{Ref: "f"}, Base: &forgejo.PRBranchInfo{Ref: "main"},
				Poster: &forgejo.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now,
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	cr, err := p.ReopenCR(context.Background(), "owner", "repo", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateOpened {
		t.Errorf("expected opened, got %s", cr.State)
	}
}

func TestForgejo_ListCRComments(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/issues/5/comments": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*forgejo.Comment{
				{ID: 10, Body: "comment 1", Poster: &forgejo.User{ID: 1, UserName: "u1", AvatarURL: "a1.png"}},
				{ID: 11, Body: "comment 2", Poster: &forgejo.User{ID: 2, UserName: "u2"}},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	comments, err := p.ListCRComments(context.Background(), "owner", "repo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2, got %d", len(comments))
	}
	if comments[0].Body != "comment 1" {
		t.Errorf("expected comment 1, got %s", comments[0].Body)
	}
	if comments[0].Author.Username != "u1" {
		t.Errorf("expected u1, got %s", comments[0].Author.Username)
	}
}

func TestForgejo_ListCRCommits(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/5/commits": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*forgejo.Commit{
				{
					CommitMeta: &forgejo.CommitMeta{SHA: "abc123"},
					RepoCommit: &forgejo.RepoCommit{Message: "fix bug", Author: &forgejo.CommitUser{Identity: forgejo.Identity{Name: "dev"}}},
				},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	commits, err := p.ListCRCommits(context.Background(), "owner", "repo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1, got %d", len(commits))
	}
	if commits[0].SHA != "abc123" {
		t.Errorf("expected abc123, got %s", commits[0].SHA)
	}
	if commits[0].Message != "fix bug" {
		t.Errorf("expected 'fix bug', got %s", commits[0].Message)
	}
	if commits[0].Author.Name != "dev" {
		t.Errorf("expected dev, got %s", commits[0].Author.Name)
	}
}

func TestForgejo_ForkRepo(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/forks": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, &forgejo.Repository{ID: 99, FullName: "user/repo", Name: "repo", Owner: &forgejo.User{UserName: "user"}, DefaultBranch: "main"})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	repo, err := p.ForkRepo(context.Background(), "owner", "repo", ForkRepoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID != 99 {
		t.Errorf("expected 99, got %d", repo.ID)
	}
}

func TestForgejo_DeleteRepo(t *testing.T) {
	called := false
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				called = true
			}
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	err := p.DeleteRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestForgejo_UpdateRepo(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, forgejo.Repository{ID: 1, FullName: "owner/new-name", Name: "new-name", Owner: &forgejo.User{UserName: "owner"}, DefaultBranch: "develop"})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	repo, err := p.UpdateRepo(context.Background(), "owner", "repo", UpdateRepoOptions{Name: "new-name", DefaultBranch: "develop"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.Name != "new-name" {
		t.Errorf("expected new-name, got %s", repo.Name)
	}
}

func TestForgejo_GetCommit(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/git/commits/abc123": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, forgejo.Commit{
				CommitMeta: &forgejo.CommitMeta{SHA: "abc123"},
				RepoCommit: &forgejo.RepoCommit{Message: "init", Author: &forgejo.CommitUser{Identity: forgejo.Identity{Name: "dev"}}},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	ci, err := p.GetCommit(context.Background(), "owner", "repo", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if ci.SHA != "abc123" {
		t.Errorf("expected abc123, got %s", ci.SHA)
	}
	if ci.Message != "init" {
		t.Errorf("expected init, got %s", ci.Message)
	}
}

func TestForgejo_ListCommits(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/commits": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*forgejo.Commit{
				{CommitMeta: &forgejo.CommitMeta{SHA: "sha1"}, RepoCommit: &forgejo.RepoCommit{Message: "first"}},
				{CommitMeta: &forgejo.CommitMeta{SHA: "sha2"}, RepoCommit: &forgejo.RepoCommit{Message: "second"}},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	commits, err := p.ListCommits(context.Background(), "owner", "repo", ListCommitsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2, got %d", len(commits))
	}
	if commits[0].SHA != "sha1" {
		t.Errorf("expected sha1, got %s", commits[0].SHA)
	}
}

func TestForgejo_CompareCommits(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/compare/base...head": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, &forgejo.Compare{
				TotalCommits: 2,
				Commits: []*forgejo.Commit{
					{CommitMeta: &forgejo.CommitMeta{SHA: "sha1"}, RepoCommit: &forgejo.RepoCommit{Message: "c1"}},
					{CommitMeta: &forgejo.CommitMeta{SHA: "sha2"}, RepoCommit: &forgejo.RepoCommit{Message: "c2"}},
				},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	result, err := p.CompareCommits(context.Background(), "owner", "repo", "base", "head")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCommits != 2 {
		t.Errorf("expected 2, got %d", result.TotalCommits)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(result.Commits))
	}
}

func TestForgejo_CreateFile(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/contents/path.txt": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, forgejo.FileResponse{
				Commit: &forgejo.FileCommitResponse{CommitMeta: forgejo.CommitMeta{SHA: "newsha"}},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	result, err := p.CreateFile(context.Background(), "owner", "repo", FileOptions{
		Path: "path.txt", Message: "add file", Content: "aGVsbG8=", Branch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "newsha" {
		t.Errorf("expected newsha, got %s", result.CommitSHA)
	}
}

func TestForgejo_UpdateFile(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/contents/path.txt": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, forgejo.FileResponse{
				Commit: &forgejo.FileCommitResponse{CommitMeta: forgejo.CommitMeta{SHA: "updsha"}},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	result, err := p.UpdateFile(context.Background(), "owner", "repo", FileOptions{
		Path: "path.txt", Message: "update file", Content: "dXBkYXRlZA==", SHA: "oldsha", Branch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "updsha" {
		t.Errorf("expected updsha, got %s", result.CommitSHA)
	}
}

func TestForgejo_DeleteFile(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/contents/path.txt": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				w.WriteHeader(http.StatusOK)
			}
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	result, err := p.DeleteFile(context.Background(), "owner", "repo", FileDeleteOptions{
		Path: "path.txt", Message: "delete file", SHA: "oldsha", Branch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestForgejo_ListTags(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/tags": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*forgejo.Tag{
				{Name: "v1.0", Commit: &forgejo.CommitMeta{SHA: "sha1"}},
				{Name: "v2.0"},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	tags, err := p.ListTags(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2, got %d", len(tags))
	}
	if tags[0].Name != "v1.0" {
		t.Errorf("expected v1.0, got %s", tags[0].Name)
	}
	if tags[0].Commit != "sha1" {
		t.Errorf("expected sha1, got %s", tags[0].Commit)
	}
	if tags[1].Commit != "" {
		t.Errorf("expected empty commit, got %s", tags[1].Commit)
	}
}

func TestForgejo_ListReleases(t *testing.T) {
	now := time.Now()
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/releases": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*forgejo.Release{
				{ID: 1, TagName: "v1.0", Title: "Release 1.0", Note: "First", URL: "u1", CreatedAt: now, PublishedAt: now},
			})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	releases, err := p.ListReleases(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1, got %d", len(releases))
	}
	if releases[0].TagName != "v1.0" {
		t.Errorf("expected v1.0, got %s", releases[0].TagName)
	}
}

func TestForgejo_CreateRelease(t *testing.T) {
	now := time.Now()
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/releases": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, &forgejo.Release{ID: 5, TagName: "v2.0", Title: "Release 2.0", Note: "body", URL: "u2", CreatedAt: now, PublishedAt: now})
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	ri, err := p.CreateRelease(context.Background(), "owner", "repo", CreateReleaseOptions{
		TagName: "v2.0", Title: "Release 2.0", Body: "body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ri.TagName != "v2.0" {
		t.Errorf("expected v2.0, got %s", ri.TagName)
	}
}

func TestForgejo_GetArchive(t *testing.T) {
	srv := newForgejoTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/archive": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("tar-data"))
		},
	})
	defer srv.Close()
	p := newTestForgejoProvider(srv)

	data, err := p.GetArchive(context.Background(), "owner", "repo", "main", "tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "tar-data" {
		t.Errorf("expected tar-data, got %s", string(data))
	}
}

func TestForgejo_ParseWebhookEvent_FallbackToGiteaHeader(t *testing.T) {
	p := &forgejoProvider{}
	body := `{"ref":"refs/heads/main","after":"abc","sender":{"id":1,"login":"dev"},"repository":{"full_name":"o/r"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Gitea-Event", "push")

	event, err := p.ParseWebhookEvent(req, "")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "push" {
		t.Errorf("expected push (fallback), got %s", event.Type)
	}
}
