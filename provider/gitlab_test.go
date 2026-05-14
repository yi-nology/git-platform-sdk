package provider

import (
	"context"
	"encoding/json"

	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func newTestGitLabProvider(srv *httptest.Server) *gitlabProvider {
	client, _ := gitlab.NewClient("test-token", gitlab.WithBaseURL(srv.URL), gitlab.WithHTTPClient(srv.Client()))
	return &gitlabProvider{client: client}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func TestGitLab_ListRepos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total", "1")
		writeJSON(w, []map[string]interface{}{
			{"id": 1, "name": "repo1", "path_with_namespace": "owner/repo1", "http_url_to_repo": "https://gitlab.com/owner/repo1.git", "ssh_url_to_repo": "git@gitlab.com:owner/repo1.git", "default_branch": "main", "visibility": "public"},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	repos, err := p.ListRepos(context.Background(), ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].FullName != "owner/repo1" {
		t.Errorf("unexpected: %s", repos[0].FullName)
	}
	if repos[0].Platform != PlatformGitLab {
		t.Errorf("unexpected platform: %s", repos[0].Platform)
	}
	if repos[0].Private {
		t.Error("public project should not be private")
	}
}

func TestGitLab_GetRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"id": 42, "name": "repo", "path_with_namespace": "owner/repo", "default_branch": "main", "visibility": "private",
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	repo, err := p.GetRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID != 42 {
		t.Errorf("expected 42, got %d", repo.ID)
	}
	if !repo.Private {
		t.Error("expected private")
	}
}

func TestGitLab_CreateCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"iid": 7, "title": "test mr", "description": "desc", "state": "opened",
			"source_branch": "feature", "target_branch": "main",
			"author":  map[string]interface{}{"id": 1, "username": "dev", "name": "Developer", "avatar_url": "https://avatar/dev.png"},
			"reviewers": []map[string]interface{}{
				{"id": 2, "username": "rev1", "name": "Reviewer", "avatar_url": "https://avatar/rev.png"},
			},
			"labels":                    []string{"bug", "critical"},
			"web_url":                   "https://gitlab.com/owner/repo/-/merge_requests/7",
			"created_at":                time.Now().Format(time.RFC3339),
			"updated_at":                time.Now().Format(time.RFC3339),
			"detailed_merge_status":     "mergeable",
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	cr, err := p.CreateCR(context.Background(), CreateCROptions{
		Owner: "owner", Repo: "repo", Title: "test mr", Description: "desc",
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
	if len(cr.Reviewers) != 1 || cr.Reviewers[0].Username != "rev1" {
		t.Errorf("expected 1 reviewer, got %v", cr.Reviewers)
	}
	if len(cr.Labels) != 2 {
		t.Errorf("expected 2 labels, got %v", cr.Labels)
	}
}

func TestGitLab_GetCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"iid": 5, "title": "found", "state": "opened",
			"source_branch": "feat", "target_branch": "main",
			"author":                    map[string]interface{}{"id": 1, "username": "u"},
			"detailed_merge_status":     "mergeable",
			"web_url":                   "https://gitlab.com/o/r/-/merge_requests/5",
			"created_at":                time.Now().Format(time.RFC3339),
			"updated_at":                time.Now().Format(time.RFC3339),
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	cr, err := p.GetCR(context.Background(), "owner", "repo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if cr.Number != 5 {
		t.Errorf("expected 5, got %d", cr.Number)
	}
	if cr.MergeStatus != "mergeable" {
		t.Errorf("expected mergeable, got %s", cr.MergeStatus)
	}
}

func TestGitLab_ListCRs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total", "5")
		now := time.Now().Format(time.RFC3339)
		writeJSON(w, []map[string]interface{}{
			{"iid": 1, "title": "mr1", "state": "opened", "source_branch": "a", "target_branch": "main", "author": map[string]interface{}{"id": 1, "username": "u"}, "created_at": now, "updated_at": now},
			{"iid": 2, "title": "mr2", "state": "merged", "source_branch": "b", "target_branch": "main", "author": map[string]interface{}{"id": 1, "username": "u"}, "created_at": now, "updated_at": now},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

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
	if crs[0].State != CRStateOpened {
		t.Errorf("expected opened, got %s", crs[0].State)
	}
	if crs[1].State != CRStateMerged {
		t.Errorf("expected merged, got %s", crs[1].State)
	}
}

func TestGitLab_CloseCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().Format(time.RFC3339)
		writeJSON(w, map[string]interface{}{
			"iid": 3, "state": "closed", "source_branch": "f", "target_branch": "main",
			"author": map[string]interface{}{"id": 1, "username": "u"}, "created_at": now, "updated_at": now,
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	cr, err := p.CloseCR(context.Background(), "owner", "repo", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateClosed {
		t.Errorf("expected closed, got %s", cr.State)
	}
}

func TestGitLab_MergeCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().Format(time.RFC3339)
		if r.Method == "PUT" && strings.Contains(r.URL.Path, "/merge") {
			writeJSON(w, map[string]interface{}{
				"iid": 5, "state": "merged", "source_branch": "f", "target_branch": "main",
				"author": map[string]interface{}{"id": 1, "username": "u"}, "created_at": now, "updated_at": now,
			})
			return
		}
		writeJSON(w, map[string]interface{}{
			"iid": 5, "state": "opened", "detailed_merge_status": "mergeable",
			"source_branch": "f", "target_branch": "main",
			"author": map[string]interface{}{"id": 1, "username": "u"}, "created_at": now, "updated_at": now,
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	cr, err := p.MergeCR(context.Background(), "owner", "repo", 5, MergeCROptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateMerged {
		t.Errorf("expected merged, got %s", cr.State)
	}
}

func TestGitLab_ListBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "main"}, {"name": "develop"},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	branches, err := p.ListBranches(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 || branches[0].Name != "main" {
		t.Errorf("unexpected: %v", branches)
	}
}

func TestGitLab_CreateWebhook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"id": 100, "url": "https://callback.example.com/hook",
			"push_events": true, "merge_requests_events": true,
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	hook, err := p.CreateWebhook(context.Background(), CreateWebhookOptions{
		Owner: "owner", Repo: "repo", URL: "https://callback.example.com/hook", Secret: "s3cret",
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

func TestGitLab_ListWebhooks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"id": 1, "push_events": true},
			{"id": 2, "push_events": true, "merge_requests_events": true, "tag_push_events": true},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	hooks, err := p.ListWebhooks(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2, got %d", len(hooks))
	}
	if len(hooks[1].Events) != 3 {
		t.Errorf("expected 3 events, got %v", hooks[1].Events)
	}
}

func TestGitLab_DeleteWebhook(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			called = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	err := p.DeleteWebhook(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestGitLab_TestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "user") {
			writeJSON(w, map[string]interface{}{"username": "testuser"})
			return
		}
		writeJSON(w, []map[string]interface{}{})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

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

func TestGitLab_ValidateWebhookSignature(t *testing.T) {
	p := &gitlabProvider{}
	req := httptest.NewRequest("POST", "/hook", strings.NewReader("{}"))
	req.Header.Set("X-Gitlab-Token", "mysecret")

	if err := p.ValidateWebhookSignature(req, "mysecret"); err != nil {
		t.Errorf("valid token should pass: %v", err)
	}
	if err := p.ValidateWebhookSignature(req, "wrong"); err == nil {
		t.Error("invalid token should fail")
	}
}

func TestGitLab_ParseWebhookEvent_Push(t *testing.T) {
	p := &gitlabProvider{}
	body := `{"object_kind":"push","ref":"refs/heads/main","after":"abc123","user":{"id":1,"username":"dev","name":"Developer"},"project":{"path_with_namespace":"owner/repo"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Gitlab-Token", "secret")

	event, err := p.ParseWebhookEvent(req, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "push" {
		t.Errorf("expected push, got %s", event.Type)
	}
	if event.Branch != "main" {
		t.Errorf("expected main, got %s", event.Branch)
	}
	if event.Actor.Username != "dev" {
		t.Errorf("expected dev, got %s", event.Actor.Username)
	}
}

func TestGitLab_ParseWebhookEvent_MR(t *testing.T) {
	p := &gitlabProvider{}
	body := `{"object_kind":"merge_request","user":{"id":1,"username":"dev"},"project":{"path_with_namespace":"o/r"},"object_attributes":{"iid":5,"title":"test","description":"desc","state":"opened","source_branch":"feat","target_branch":"main","action":"open","merge_status":"mergeable","url":"https://gitlab.com/o/r/-/merge_requests/5","last_commit":{"id":"sha1"},"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Gitlab-Token", "secret")

	event, err := p.ParseWebhookEvent(req, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "cr.open" {
		t.Errorf("expected cr.open, got %s", event.Type)
	}
	if event.CR == nil || event.CR.Number != 5 {
		t.Fatal("expected CR with number 5")
	}
	if event.CR.MergeStatus != "mergeable" {
		t.Errorf("expected mergeable, got %s", event.CR.MergeStatus)
	}
}

func TestGitLab_ParseWebhookEvent_TagPush(t *testing.T) {
	p := &gitlabProvider{}
	body := `{"object_kind":"tag_push","ref":"refs/tags/v1.0","user":{"id":1,"username":"dev"},"project":{"path_with_namespace":"o/r"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Gitlab-Token", "secret")

	event, err := p.ParseWebhookEvent(req, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "tag.created" {
		t.Errorf("expected tag.created, got %s", event.Type)
	}
	if event.Tag != "v1.0" {
		t.Errorf("expected v1.0, got %s", event.Tag)
	}
}

func TestGitLab_CreateNote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"id": 999})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	id, err := p.CreateNote(context.Background(), "owner", "repo", 5, "nice work")
	if err != nil {
		t.Fatal(err)
	}
	if id != "999" {
		t.Errorf("expected 999, got %s", id)
	}
}

func TestGitLab_GetFileContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("file content"))
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	content, err := p.GetFileContent(context.Background(), "owner", "repo", "README.md", "main")
	if err != nil {
		t.Fatal(err)
	}
	if content != "file content" {
		t.Errorf("unexpected: %s", content)
	}
}

func TestGitLab_CreateBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"name": "new-branch"})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	b, err := p.CreateBranch(context.Background(), "owner", "repo", "new-branch", "main")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "new-branch" {
		t.Errorf("expected new-branch, got %s", b.Name)
	}
}

func TestGitLab_DeleteBranch(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			called = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	err := p.DeleteBranch(context.Background(), "owner", "repo", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestGitLab_UpdateCRLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().Format(time.RFC3339)
		writeJSON(w, map[string]interface{}{"iid": 3, "labels": []string{"bug"}, "created_at": now, "updated_at": now})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	err := p.UpdateCRLabels(context.Background(), "owner", "repo", 3, []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGitLab_CreateCommitStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"id": 1})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	err := p.CreateCommitStatus(context.Background(), "owner", "repo", "sha123", CommitStatusOptions{
		State: "success", Context: "ci", Description: "passed",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGitLab_CreateDiscussion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"id": "disc-123"})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	id, err := p.CreateDiscussion(context.Background(), "owner", "repo", 5, DiscussionOptions{Body: "comment"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "disc-123" {
		t.Errorf("expected disc-123, got %s", id)
	}
}

func TestGitLab_UpdateCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			writeJSON(w, map[string]interface{}{
				"iid": 7, "title": "updated", "description": "new desc", "state": "opened",
				"source_branch": "feature", "target_branch": "develop",
				"author": map[string]interface{}{"id": 1, "username": "u", "name": "User"},
				"web_url": "https://gitlab.com/o/r/-/merge_requests/7",
			})
		}
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	cr, err := p.UpdateCR(context.Background(), "owner", "repo", 7, UpdateCROptions{Title: "updated", Description: "new desc"})
	if err != nil {
		t.Fatal(err)
	}
	if cr.Title != "updated" {
		t.Errorf("expected updated, got %s", cr.Title)
	}
}

func TestGitLab_ReopenCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"iid": 3, "title": "reopened", "description": "", "state": "opened",
			"source_branch": "f", "target_branch": "main",
			"author": map[string]interface{}{"id": 1, "username": "u", "name": "User"},
			"web_url": "https://gitlab.com/o/r/-/merge_requests/3",
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	cr, err := p.ReopenCR(context.Background(), "owner", "repo", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateOpened {
		t.Errorf("expected opened, got %s", cr.State)
	}
}

func TestGitLab_ListCRComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"id": 10, "body": "comment 1", "author": map[string]interface{}{"id": 1, "username": "u1", "name": "User1"},
				"created_at": "2024-01-01T12:00:00Z", "updated_at": "2024-01-01T12:00:00Z"},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	comments, err := p.ListCRComments(context.Background(), "owner", "repo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1, got %d", len(comments))
	}
	if comments[0].Body != "comment 1" {
		t.Errorf("expected comment 1, got %s", comments[0].Body)
	}
}

func TestGitLab_ListCRCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"id": "abc123", "short_id": "abc", "title": "fix bug", "message": "fix bug\n\ndetail",
				"author_name": "dev", "created_at": "2024-01-01T12:00:00Z"},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	commits, err := p.ListCRCommits(context.Background(), "owner", "repo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1, got %d", len(commits))
	}
	if commits[0].SHA != "abc" {
		t.Errorf("expected abc, got %s", commits[0].SHA)
	}
}

func TestGitLab_ForkRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"id": 99, "name": "repo", "path_with_namespace": "user/repo",
			"http_url_to_repo": "https://gitlab.com/user/repo.git", "default_branch": "main", "visibility": "public",
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	repo, err := p.ForkRepo(context.Background(), "owner", "repo", ForkRepoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID != 99 {
		t.Errorf("expected 99, got %d", repo.ID)
	}
}

func TestGitLab_DeleteRepo(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			called = true
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	err := p.DeleteRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestGitLab_UpdateRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"id": 1, "name": "new-name", "path_with_namespace": "owner/new-name",
			"http_url_to_repo": "https://gitlab.com/owner/new-name.git", "default_branch": "develop", "visibility": "public",
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	repo, err := p.UpdateRepo(context.Background(), "owner", "repo", UpdateRepoOptions{Name: "new-name", DefaultBranch: "develop"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.Name != "new-name" {
		t.Errorf("expected new-name, got %s", repo.Name)
	}
}

func TestGitLab_GetCommit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"id": "abc123", "message": "init", "author_name": "dev", "created_at": "2024-01-01T12:00:00Z",
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

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

func TestGitLab_ListCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"id": "sha1", "message": "first", "author_name": "dev", "created_at": "2024-01-01T12:00:00Z"},
			{"id": "sha2", "message": "second", "author_name": "dev", "created_at": "2024-01-01T12:00:00Z"},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

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

func TestGitLab_CompareCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"commits": []map[string]interface{}{
				{"id": "sha1", "message": "c1", "author_name": "dev", "created_at": "2024-01-01T12:00:00Z"},
			},
			"diffs": []map[string]interface{}{
				{"old_path": "a.txt", "new_path": "b.txt", "diff": "+1\n-2", "new_file": true, "deleted_file": false, "renamed_file": false},
			},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	result, err := p.CompareCommits(context.Background(), "owner", "repo", "base", "head")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCommits != 1 {
		t.Errorf("expected 1, got %d", result.TotalCommits)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
}

func TestGitLab_CreateFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Gitlab-Commit-Id", "newsha")
		writeJSON(w, map[string]interface{}{"file_path": "path.txt", "branch": "main"})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	result, err := p.CreateFile(context.Background(), "owner", "repo", FileOptions{
		Path: "path.txt", Message: "add", Content: "hello", Branch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "newsha" {
		t.Errorf("expected newsha, got %s", result.CommitSHA)
	}
}

func TestGitLab_UpdateFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Gitlab-Commit-Id", "updsha")
		writeJSON(w, map[string]interface{}{"file_path": "path.txt", "branch": "main"})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	result, err := p.UpdateFile(context.Background(), "owner", "repo", FileOptions{
		Path: "path.txt", Message: "update", Content: "updated",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "updsha" {
		t.Errorf("expected updsha, got %s", result.CommitSHA)
	}
}

func TestGitLab_DeleteFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Gitlab-Commit-Id", "delsha")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	result, err := p.DeleteFile(context.Background(), "owner", "repo", FileDeleteOptions{
		Path: "path.txt", Message: "delete",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "delsha" {
		t.Errorf("expected delsha, got %s", result.CommitSHA)
	}
}

func TestGitLab_ListTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"name": "v1.0", "commit": map[string]interface{}{"id": "sha1"}},
			{"name": "v2.0", "commit": map[string]interface{}{"id": "sha2"}},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	tags, err := p.ListTags(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2, got %d", len(tags))
	}
	if tags[0].Name != "v1.0" || tags[0].Commit != "sha1" {
		t.Errorf("unexpected: %v", tags[0])
	}
}

func TestGitLab_ListReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []map[string]interface{}{
			{"tag_name": "v1.0", "name": "Release 1.0", "description": "First",
				"released_at": "2024-01-01T12:00:00Z", "created_at": "2024-01-01T12:00:00Z",
				"_links": map[string]interface{}{"self": "https://gitlab.com/o/r/-/releases/v1.0"}},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

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

func TestGitLab_CreateRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"tag_name": "v2.0", "name": "Release 2.0", "description": "body",
			"released_at": "2024-01-01T12:00:00Z", "created_at": "2024-01-01T12:00:00Z",
			"_links": map[string]interface{}{"self": "https://gitlab.com/o/r/-/releases/v2.0"},
		})
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

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

func TestGitLab_GetArchive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("tar-data"))
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	data, err := p.GetArchive(context.Background(), "owner", "repo", "main", "tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "tar-data" {
		t.Errorf("expected tar-data, got %s", string(data))
	}
}

func TestGitLab_DeleteNote(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			called = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestGitLabProvider(srv)

	err := p.DeleteNote(context.Background(), "owner", "repo", 5, "123")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}
