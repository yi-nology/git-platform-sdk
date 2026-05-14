package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gitea "code.gitea.io/sdk/gitea"
)

func newGiteaTestServer(routes map[string]func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"version": "1.22.0"})
	})
	for path, handler := range routes {
		mux.HandleFunc(path, handler)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

func newTestGiteaProvider(srv *httptest.Server) *giteaProvider {
	client, _ := gitea.NewClient(srv.URL, gitea.SetToken("test-token"), gitea.SetHTTPClient(srv.Client()))
	if client == nil {
		panic("gitea client is nil")
	}
	return &giteaProvider{client: client}
}

func TestGitea_Platform(t *testing.T) {
	srv := newGiteaTestServer(nil)
	defer srv.Close()
	p := newTestGiteaProvider(srv)
	if p.Platform() != PlatformGitea {
		t.Errorf("expected gitea, got %s", p.Platform())
	}
}

func TestGitea_TestConnection(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/user": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, gitea.User{UserName: "testuser"})
		},
		"/api/v1/repos/search": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*gitea.Repository{})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

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

func TestGitea_ListRepos(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/search": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "2")
			writeJSON(w, []*gitea.Repository{
				{ID: 1, FullName: "owner/repo1", Name: "repo1", Owner: &gitea.User{UserName: "owner"}, CloneURL: "https://gitea.com/owner/repo1.git", DefaultBranch: "main"},
			})
		},
		"/api/v1/user/repos": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "2")
			writeJSON(w, []*gitea.Repository{
				{ID: 1, FullName: "owner/repo1", Name: "repo1", Owner: &gitea.User{UserName: "owner"}, CloneURL: "https://gitea.com/owner/repo1.git", DefaultBranch: "main"},
				{ID: 2, FullName: "owner/repo2", Name: "repo2", Owner: &gitea.User{UserName: "owner"}, CloneURL: "https://gitea.com/owner/repo2.git", DefaultBranch: "main", Private: true},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	repos, err := p.ListRepos(context.Background(), ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2, got %d", len(repos))
	}
	if repos[0].Platform != PlatformGitea {
		t.Errorf("unexpected platform: %s", repos[0].Platform)
	}
}

func TestGitea_GetRepo(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, gitea.Repository{ID: 42, FullName: "owner/repo", Name: "repo", Owner: &gitea.User{UserName: "owner"}, DefaultBranch: "main"})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	repo, err := p.GetRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID != 42 {
		t.Errorf("expected 42, got %d", repo.ID)
	}
}

func TestGitea_CreateCR(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			writeJSON(w, gitea.PullRequest{
				ID: 7, Index: 7, Title: "test pr", Body: "desc", State: gitea.StateOpen,
				Head: &gitea.PRBranchInfo{Ref: "feature"}, Base: &gitea.PRBranchInfo{Ref: "main"},
				Poster: &gitea.User{ID: 1, UserName: "dev", AvatarURL: "https://avatar/dev.png"},
				RequestedReviewers: []*gitea.User{
					{ID: 2, UserName: "rev1", AvatarURL: "https://avatar/rev.png"},
				},
				Labels: []*gitea.Label{{Name: "bug"}, {Name: "urgent"}},
				HTMLURL: "https://gitea.com/owner/repo/pulls/7",
				Created: &now, Updated: &now,
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

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
	if len(cr.Reviewers) != 1 || cr.Reviewers[0].Username != "rev1" {
		t.Errorf("expected 1 reviewer, got %v", cr.Reviewers)
	}
	if len(cr.Labels) != 2 || cr.Labels[0] != "bug" {
		t.Errorf("expected [bug urgent], got %v", cr.Labels)
	}
}

func TestGitea_GetCR(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/5": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			writeJSON(w, gitea.PullRequest{
				ID: 5, Index: 5, Title: "found", State: gitea.StateOpen,
				Head: &gitea.PRBranchInfo{Ref: "feat"}, Base: &gitea.PRBranchInfo{Ref: "main"},
				Poster: &gitea.User{ID: 1, UserName: "u", AvatarURL: "https://avatar/u.png"},
				HTMLURL: "https://gitea.com/o/r/pulls/5",
				Created: &now, Updated: &now,
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

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

func TestGitea_ListCRs(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Total-Count", "5")
			now := time.Now()
			writeJSON(w, []*gitea.PullRequest{
				{ID: 1, Index: 1, Title: "pr1", State: gitea.StateOpen, Head: &gitea.PRBranchInfo{Ref: "a"}, Base: &gitea.PRBranchInfo{Ref: "main"}, Poster: &gitea.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now},
				{ID: 2, Index: 2, Title: "pr2", State: gitea.StateClosed, HasMerged: true, Head: &gitea.PRBranchInfo{Ref: "b"}, Base: &gitea.PRBranchInfo{Ref: "main"}, Poster: &gitea.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	crs, total, err := p.ListCRs(context.Background(), ListCROptions{Owner: "owner", Repo: "repo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(crs) != 2 {
		t.Fatalf("expected 2, got %d", len(crs))
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
}

func TestGitea_CloseCR(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/3": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			writeJSON(w, gitea.PullRequest{ID: 3, Index: 3, State: gitea.StateClosed, Head: &gitea.PRBranchInfo{Ref: "f"}, Base: &gitea.PRBranchInfo{Ref: "main"}, Poster: &gitea.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	cr, err := p.CloseCR(context.Background(), "owner", "repo", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateClosed {
		t.Errorf("expected closed, got %s", cr.State)
	}
}

func TestGitea_ListBranches(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/branches": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*gitea.Branch{{Name: "main"}, {Name: "develop"}})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	branches, err := p.ListBranches(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 || branches[0].Name != "main" {
		t.Errorf("unexpected: %v", branches)
	}
}

func TestGitea_CreateWebhook(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/hooks": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, gitea.Hook{ID: 100, Events: []string{"push", "pull_request"}, Config: map[string]string{"url": "https://cb.com/hook"}})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	hook, err := p.CreateWebhook(context.Background(), CreateWebhookOptions{
		Owner: "owner", Repo: "repo", URL: "https://cb.com/hook", Secret: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if hook.ID != 100 || len(hook.Events) != 2 {
		t.Errorf("unexpected: %+v", hook)
	}
}

func TestGitea_ListWebhooks(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/hooks": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*gitea.Hook{
				{ID: 1, Events: []string{"push"}, Config: map[string]string{"url": "u1"}},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	hooks, err := p.ListWebhooks(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1, got %d", len(hooks))
	}
}

func TestGitea_DeleteWebhook(t *testing.T) {
	called := false
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/hooks/42": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				called = true
			}
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	err := p.DeleteWebhook(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestGitea_CreateNote(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/issues/5/comments": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, gitea.Comment{ID: 999})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	id, err := p.CreateNote(context.Background(), "owner", "repo", 5, "nice")
	if err != nil {
		t.Fatal(err)
	}
	if id != "999" {
		t.Errorf("expected 999, got %s", id)
	}
}

func TestGitea_GetFileContent(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/raw/test.txt": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("hello world"))
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	content, err := p.GetFileContent(context.Background(), "owner", "repo", "test.txt", "main")
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello world" {
		t.Errorf("unexpected: %s", content)
	}
}

func TestGitea_ValidateWebhookSignature(t *testing.T) {
	p := &giteaProvider{}
	err := p.ValidateWebhookSignature(httptest.NewRequest("POST", "/", nil), "")
	if err != nil {
		t.Errorf("empty secret should pass: %v", err)
	}
}

func TestGitea_ParseWebhookEvent_Push(t *testing.T) {
	p := &giteaProvider{}
	body := `{"ref":"refs/heads/main","after":"abc123","sender":{"id":1,"login":"dev"},"repository":{"full_name":"owner/repo"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Gitea-Event", "push")

	event, err := p.ParseWebhookEvent(req, "")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "push" || event.Branch != "main" {
		t.Errorf("unexpected: type=%s branch=%s", event.Type, event.Branch)
	}
}

func TestGitea_ParseWebhookEvent_PR(t *testing.T) {
	p := &giteaProvider{}
	body := fmt.Sprintf(`{"action":"opened","number":1,"pull_request":{"id":1,"number":1,"title":"test","body":"desc","state":"open","head":{"ref":"feat","sha":"sha1"},"base":{"ref":"main"},"merged":false,"html_url":"https://gitea.com/o/r/pulls/1","user":{"id":1,"login":"dev"},"created_at":"%s","updated_at":"%s"},"sender":{"id":1,"login":"dev"},"repository":{"full_name":"o/r"}}`,
		time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Gitea-Event", "pull_request")

	event, err := p.ParseWebhookEvent(req, "")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "cr.opened" || event.CR == nil || event.CR.Number != 1 {
		t.Fatalf("unexpected: type=%s cr=%v", event.Type, event.CR)
	}
}

func TestGitea_CreateBranch(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/branches": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, gitea.Branch{Name: "new-branch"})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	b, err := p.CreateBranch(context.Background(), "owner", "repo", "new-branch", "main")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "new-branch" {
		t.Errorf("expected new-branch, got %s", b.Name)
	}
}

func TestGitea_DeleteBranch(t *testing.T) {
	called := false
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/branches/feature": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				called = true
			}
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	err := p.DeleteBranch(context.Background(), "owner", "repo", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestGitea_UpdateCR(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/7": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			writeJSON(w, gitea.PullRequest{
				ID: 7, Index: 7, Title: "updated", Body: "new desc", State: gitea.StateOpen,
				Head: &gitea.PRBranchInfo{Ref: "feature"}, Base: &gitea.PRBranchInfo{Ref: "develop"},
				Poster: &gitea.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now,
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	cr, err := p.UpdateCR(context.Background(), "owner", "repo", 7, UpdateCROptions{Title: "updated", Description: "new desc", TargetBranch: "develop"})
	if err != nil {
		t.Fatal(err)
	}
	if cr.Title != "updated" {
		t.Errorf("expected updated, got %s", cr.Title)
	}
}

func TestGitea_ReopenCR(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/3": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			writeJSON(w, gitea.PullRequest{
				ID: 3, Index: 3, Title: "reopened", State: gitea.StateOpen,
				Head: &gitea.PRBranchInfo{Ref: "f"}, Base: &gitea.PRBranchInfo{Ref: "main"},
				Poster: &gitea.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now,
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	cr, err := p.ReopenCR(context.Background(), "owner", "repo", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateOpened {
		t.Errorf("expected opened, got %s", cr.State)
	}
}

func TestGitea_ListCRComments(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/issues/5/comments": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*gitea.Comment{
				{ID: 10, Body: "comment 1", Poster: &gitea.User{ID: 1, UserName: "u1"}},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	comments, err := p.ListCRComments(context.Background(), "owner", "repo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1, got %d", len(comments))
	}
	if comments[0].Author.Username != "u1" {
		t.Errorf("expected u1, got %s", comments[0].Author.Username)
	}
}

func TestGitea_ListCRCommits(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/5/commits": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*gitea.Commit{
				{
					CommitMeta: &gitea.CommitMeta{SHA: "abc123"},
					RepoCommit: &gitea.RepoCommit{Message: "fix bug", Author: &gitea.CommitUser{Identity: gitea.Identity{Name: "dev"}}},
				},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

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
}

func TestGitea_ForkRepo(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/forks": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, &gitea.Repository{ID: 99, FullName: "user/repo", Name: "repo", Owner: &gitea.User{UserName: "user"}, DefaultBranch: "main"})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	repo, err := p.ForkRepo(context.Background(), "owner", "repo", ForkRepoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID != 99 {
		t.Errorf("expected 99, got %d", repo.ID)
	}
}

func TestGitea_DeleteRepo(t *testing.T) {
	called := false
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				called = true
			}
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	err := p.DeleteRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestGitea_UpdateRepo(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, gitea.Repository{ID: 1, FullName: "owner/new-name", Name: "new-name", Owner: &gitea.User{UserName: "owner"}, DefaultBranch: "develop"})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	repo, err := p.UpdateRepo(context.Background(), "owner", "repo", UpdateRepoOptions{Name: "new-name", DefaultBranch: "develop"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.Name != "new-name" {
		t.Errorf("expected new-name, got %s", repo.Name)
	}
}

func TestGitea_GetCommit(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/git/commits/abc123": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, gitea.Commit{
				CommitMeta: &gitea.CommitMeta{SHA: "abc123"},
				RepoCommit: &gitea.RepoCommit{Message: "init", Author: &gitea.CommitUser{Identity: gitea.Identity{Name: "dev"}}},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	ci, err := p.GetCommit(context.Background(), "owner", "repo", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if ci.SHA != "abc123" {
		t.Errorf("expected abc123, got %s", ci.SHA)
	}
}

func TestGitea_ListCommits(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/commits": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*gitea.Commit{
				{CommitMeta: &gitea.CommitMeta{SHA: "sha1"}, RepoCommit: &gitea.RepoCommit{Message: "first"}},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	commits, err := p.ListCommits(context.Background(), "owner", "repo", ListCommitsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1, got %d", len(commits))
	}
}

func TestGitea_CompareCommits(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/compare/base...head": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, &gitea.Compare{
				TotalCommits: 1,
				Commits: []*gitea.Commit{
					{CommitMeta: &gitea.CommitMeta{SHA: "sha1"}, RepoCommit: &gitea.RepoCommit{Message: "c1"}},
				},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	result, err := p.CompareCommits(context.Background(), "owner", "repo", "base", "head")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCommits != 1 {
		t.Errorf("expected 1, got %d", result.TotalCommits)
	}
}

func TestGitea_CreateFile(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/contents/path.txt": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, gitea.FileResponse{
				Commit: &gitea.FileCommitResponse{CommitMeta: gitea.CommitMeta{SHA: "newsha"}},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	result, err := p.CreateFile(context.Background(), "owner", "repo", FileOptions{
		Path: "path.txt", Message: "add", Content: "aGVsbG8=", Branch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "newsha" {
		t.Errorf("expected newsha, got %s", result.CommitSHA)
	}
}

func TestGitea_UpdateFile(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/contents/path.txt": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, gitea.FileResponse{
				Commit: &gitea.FileCommitResponse{CommitMeta: gitea.CommitMeta{SHA: "updsha"}},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	result, err := p.UpdateFile(context.Background(), "owner", "repo", FileOptions{
		Path: "path.txt", Message: "update", Content: "dXBk", SHA: "oldsha",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "updsha" {
		t.Errorf("expected updsha, got %s", result.CommitSHA)
	}
}

func TestGitea_DeleteFile(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/contents/path.txt": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				w.WriteHeader(http.StatusOK)
			}
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	result, err := p.DeleteFile(context.Background(), "owner", "repo", FileDeleteOptions{
		Path: "path.txt", Message: "delete", SHA: "oldsha",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestGitea_ListTags(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/tags": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*gitea.Tag{
				{Name: "v1.0", Commit: &gitea.CommitMeta{SHA: "sha1"}},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	tags, err := p.ListTags(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1, got %d", len(tags))
	}
	if tags[0].Name != "v1.0" {
		t.Errorf("expected v1.0, got %s", tags[0].Name)
	}
}

func TestGitea_ListReleases(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/releases": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []*gitea.Release{
				{ID: 1, TagName: "v1.0", Title: "Release 1.0", Note: "First", CreatedAt: time.Now(), PublishedAt: time.Now()},
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

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

func TestGitea_CreateRelease(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/releases": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, &gitea.Release{ID: 5, TagName: "v2.0", Title: "Release 2.0", Note: "body", CreatedAt: time.Now(), PublishedAt: time.Now()})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

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

func TestGitea_GetArchive(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/archive": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("tar-data"))
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	data, err := p.GetArchive(context.Background(), "owner", "repo", "main", "tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "tar-data" {
		t.Errorf("expected tar-data, got %s", string(data))
	}
}

func TestGitea_MergeCR(t *testing.T) {
	srv := newGiteaTestServer(map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/v1/repos/owner/repo/pulls/5": func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			if r.Method == "POST" && strings.Contains(r.URL.Path, "merge") {
				w.WriteHeader(http.StatusOK)
				return
			}
			writeJSON(w, gitea.PullRequest{
				ID: 5, Index: 5, State: gitea.StateClosed, HasMerged: true,
				Head: &gitea.PRBranchInfo{Ref: "f"}, Base: &gitea.PRBranchInfo{Ref: "main"},
				Poster: &gitea.User{ID: 1, UserName: "u"}, Created: &now, Updated: &now,
			})
		},
	})
	defer srv.Close()
	p := newTestGiteaProvider(srv)

	cr, err := p.MergeCR(context.Background(), "owner", "repo", 5, MergeCROptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateMerged {
		t.Errorf("expected merged, got %s", cr.State)
	}
}
