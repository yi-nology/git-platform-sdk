package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestTCProvider(srv *httptest.Server) *tencentCodeProvider {
	bp := newBaseProvider(srv.URL, "test-token", false, authHeaderPrivateToken, "Tencent Code")
	return &tencentCodeProvider{baseProvider: bp}
}

func tcMRResponse(iid int, state, title, source, target string) map[string]interface{} {
	return map[string]interface{}{
		"iid": iid, "title": title, "description": "desc", "state": state,
		"source_branch": source, "target_branch": target,
		"author":        map[string]interface{}{"id": 1, "username": "dev", "name": "Developer"},
		"labels":        []string{"bug"},
		"merge_status":  "can_be_merged",
		"web_url":       fmt.Sprintf("https://git.code.tencent.com/o/r/merge_requests/%d", iid),
		"created_at":    "2024-01-01T12:00:00+0800",
		"updated_at":    "2024-01-01T12:00:00+0800",
	}
}

func TestTC_Platform(t *testing.T) {
	p, _ := NewProvider(Config{Platform: PlatformTencentCode, Token: "token"})
	if p.Platform() != PlatformTencentCode {
		t.Errorf("expected tencent_code, got %s", p.Platform())
	}
}

func TestTC_TestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/user" {
			writeJSON(w, map[string]string{"username": "testuser"})
			return
		}
		writeJSON(w, []interface{}{})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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

func TestTC_ListRepos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "2")
		writeJSON(w, []map[string]interface{}{
			{"id": 1, "name": "repo1", "path_with_namespace": "owner/repo1", "http_url_to_repo": "https://git.code.tencent.com/owner/repo1.git", "ssh_url_to_repo": "git@git.code.tencent.com:owner/repo1.git", "default_branch": "main", "visibility_level": 0},
			{"id": 2, "name": "repo2", "path_with_namespace": "owner/repo2", "http_url_to_repo": "https://git.code.tencent.com/owner/repo2.git", "default_branch": "main", "visibility_level": 20},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	repos, err := p.ListRepos(context.Background(), ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2, got %d", len(repos))
	}
	if repos[0].Platform != PlatformTencentCode {
		t.Errorf("unexpected platform: %s", repos[0].Platform)
	}
	if !repos[0].Private {
		t.Error("visibility_level=0 should be private")
	}
	if repos[1].Private {
		t.Error("visibility_level=20 should be public")
	}
}

func TestTC_GetRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/projects/") {
			writeJSON(w, map[string]interface{}{
				"id": 42, "name": "repo", "path_with_namespace": "owner/repo",
				"http_url_to_repo": "https://git.code.tencent.com/owner/repo.git",
				"default_branch": "main", "visibility_level": 0,
			})
			return
		}
		http.Error(w, "not found", 404)
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	repo, err := p.GetRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID != 42 || repo.Owner != "owner" {
		t.Errorf("unexpected: %+v", repo)
	}
}

func TestTC_CreateCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, tcMRResponse(7, "opened", "test mr", "feature", "main"))
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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
	if len(cr.Labels) != 1 || cr.Labels[0] != "bug" {
		t.Errorf("expected [bug], got %v", cr.Labels)
	}
}

func TestTC_GetCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, tcMRResponse(5, "opened", "found", "feat", "main"))
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	cr, err := p.GetCR(context.Background(), "owner", "repo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if cr.Number != 5 {
		t.Errorf("expected 5, got %d", cr.Number)
	}
	if cr.MergeStatus != "can_be_merged" {
		t.Errorf("expected can_be_merged, got %s", cr.MergeStatus)
	}
}

func TestTC_ListCRs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "3")
		writeJSON(w, []map[string]interface{}{
			tcMRResponse(1, "opened", "mr1", "a", "main"),
			tcMRResponse(2, "merged", "mr2", "b", "main"),
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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

func TestTC_CloseCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, tcMRResponse(3, "closed", "closed", "f", "main"))
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	cr, err := p.CloseCR(context.Background(), "owner", "repo", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateClosed {
		t.Errorf("expected closed, got %s", cr.State)
	}
}

func TestTC_MergeCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/merge") && r.Method == "PUT" {
			writeJSON(w, tcMRResponse(5, "merged", "merged", "f", "main"))
			return
		}
		writeJSON(w, tcMRResponse(5, "opened", "mr", "f", "main"))
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	cr, err := p.MergeCR(context.Background(), "owner", "repo", 5, MergeCROptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateMerged {
		t.Errorf("expected merged, got %s", cr.State)
	}
}

func TestTC_MergeCR_Conflicting(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{
			"iid": 5, "state": "opened", "merge_status": "cannot_be_merged",
			"source_branch": "f", "target_branch": "main",
			"author": map[string]interface{}{"id": 1, "username": "u"},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	_, err := p.MergeCR(context.Background(), "owner", "repo", 5, MergeCROptions{})
	if err == nil {
		t.Error("expected error for conflicting MR")
	}
}

func TestTC_ListBranches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, []map[string]string{{"name": "main"}, {"name": "develop"}})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	branches, err := p.ListBranches(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 || branches[0].Name != "main" {
		t.Errorf("unexpected: %v", branches)
	}
}

func TestTC_CreateBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]string{"name": "new-branch"})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	b, err := p.CreateBranch(context.Background(), "owner", "repo", "new-branch", "main")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "new-branch" {
		t.Errorf("expected new-branch, got %s", b.Name)
	}
}

func TestTC_DeleteBranch(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			called = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	err := p.DeleteBranch(context.Background(), "owner", "repo", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestTC_CreateWebhook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{"id": 100, "url": "https://cb.com/hook"})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	hook, err := p.CreateWebhook(context.Background(), CreateWebhookOptions{
		Owner: "owner", Repo: "repo", URL: "https://cb.com/hook", Secret: "s3cret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if hook.ID != 100 {
		t.Errorf("expected 100, got %d", hook.ID)
	}
}

func TestTC_ListWebhooks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, []map[string]interface{}{
			{"id": 1, "url": "https://h1.com"},
			{"id": 2, "url": "https://h2.com"},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	hooks, err := p.ListWebhooks(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2, got %d", len(hooks))
	}
}

func TestTC_DeleteWebhook(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			called = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	err := p.DeleteWebhook(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestTC_CreateNote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]int{"id": 999})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	id, err := p.CreateNote(context.Background(), "owner", "repo", 5, "nice")
	if err != nil {
		t.Fatal(err)
	}
	if id != "999" {
		t.Errorf("expected 999, got %s", id)
	}
}

func TestTC_DeleteNote(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			called = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	err := p.DeleteNote(context.Background(), "owner", "repo", 5, "123")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestTC_GetFileContent(t *testing.T) {
	content := base64.StdEncoding.EncodeToString([]byte("hello world"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]string{"content": content, "encoding": "base64"})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	result, err := p.GetFileContent(context.Background(), "owner", "repo", "test.txt", "main")
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestTC_GetCRDiff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{
			"changes": []map[string]interface{}{
				{"old_path": "main.go", "new_path": "main.go", "diff": "+new line\n-old line", "new_file": false, "deleted_file": false, "renamed_file": false},
			},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	diff, err := p.GetCRDiff(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Files) != 1 || diff.Files[0].NewPath != "main.go" {
		t.Errorf("unexpected: %v", diff.Files)
	}
	if diff.TotalAdd != 1 || diff.TotalDel != 1 {
		t.Errorf("expected add=1 del=1, got add=%d del=%d", diff.TotalAdd, diff.TotalDel)
	}
}

func TestTC_CreateCommitStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	err := p.CreateCommitStatus(context.Background(), "owner", "repo", "sha123", CommitStatusOptions{
		State: "success", Context: "ci", Description: "passed",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTC_UpdateCRLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	err := p.UpdateCRLabels(context.Background(), "owner", "repo", 3, []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTC_ValidateWebhookSignature(t *testing.T) {
	p := &tencentCodeProvider{}
	req := httptest.NewRequest("POST", "/hook", nil)
	req.Header.Set("X-Token", "mysecret")

	if err := p.ValidateWebhookSignature(req, "mysecret"); err != nil {
		t.Errorf("valid token should pass: %v", err)
	}
	if err := p.ValidateWebhookSignature(req, "wrong"); err == nil {
		t.Error("invalid token should fail")
	}
}

func TestTC_ParseWebhookEvent_Push(t *testing.T) {
	p := &tencentCodeProvider{}
	body := `{"object_kind":"push","ref":"refs/heads/main","after":"abc123","user":{"id":1,"username":"dev","name":"Developer"},"project":{"path_with_namespace":"owner/repo"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Token", "secret")

	event, err := p.ParseWebhookEvent(req, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "push" || event.Branch != "main" {
		t.Errorf("unexpected: type=%s branch=%s", event.Type, event.Branch)
	}
	if event.CommitSHA != "abc123" {
		t.Errorf("expected abc123, got %s", event.CommitSHA)
	}
}

func TestTC_ParseWebhookEvent_MR(t *testing.T) {
	p := &tencentCodeProvider{}
	body := `{"object_kind":"merge_request","user":{"id":1,"username":"dev"},"project":{"path_with_namespace":"o/r"},"object_attributes":{"iid":5,"title":"test","description":"desc","state":"opened","source_branch":"feat","target_branch":"main","action":"open","merge_status":"can_be_merged","url":"https://git.code.tencent.com/o/r/merge_requests/5","last_commit":{"id":"sha1"},"created_at":"2024-01-01T12:00:00+0800","updated_at":"2024-01-01T12:00:00+0800"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Token", "secret")

	event, err := p.ParseWebhookEvent(req, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "cr.open" {
		t.Errorf("expected cr.open, got %s", event.Type)
	}
	if event.CR == nil || event.CR.Number != 5 {
		t.Fatal("expected CR 5")
	}
}

func TestTC_ParseWebhookEvent_TagPush(t *testing.T) {
	p := &tencentCodeProvider{}
	body := `{"object_kind":"tag_push","ref":"refs/tags/v1.0","user":{"id":1,"username":"dev"},"project":{"path_with_namespace":"o/r"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Token", "secret")

	event, err := p.ParseWebhookEvent(req, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "tag.created" || event.Tag != "v1.0" {
		t.Errorf("unexpected: type=%s tag=%s", event.Type, event.Tag)
	}
}

func TestTC_CreateDiscussion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]int{"id": 555})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	id, err := p.CreateDiscussion(context.Background(), "owner", "repo", 3, DiscussionOptions{Body: "comment"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "555" {
		t.Errorf("expected 555, got %s", id)
	}
}

func TestTC_EncodeProjectPath(t *testing.T) {
	encoded := encodeProjectPath("owner", "repo")
	if encoded != "owner%2Frepo" {
		t.Errorf("expected owner%%2Frepo, got %s", encoded)
	}
}

func TestTC_UpdateCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "PUT" {
			writeJSON(w, tcMRResponse(7, "opened", "updated title", "feature", "develop"))
		}
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	cr, err := p.UpdateCR(context.Background(), "owner", "repo", 7, UpdateCROptions{Title: "updated title", TargetBranch: "develop"})
	if err != nil {
		t.Fatal(err)
	}
	if cr.Title != "updated title" {
		t.Errorf("expected updated title, got %s", cr.Title)
	}
}

func TestTC_ReopenCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, tcMRResponse(3, "opened", "reopened", "f", "main"))
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	cr, err := p.ReopenCR(context.Background(), "owner", "repo", 3)
	if err != nil {
		t.Fatal(err)
	}
	if cr.State != CRStateOpened {
		t.Errorf("expected opened, got %s", cr.State)
	}
}

func TestTC_ListCRComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, []map[string]interface{}{
			{"id": 10, "body": "comment 1", "author": map[string]interface{}{"id": 1, "username": "u1", "name": "User1"}, "created_at": "2024-01-01T12:00:00+0800", "updated_at": "2024-01-01T12:00:00+0800"},
			{"id": 11, "body": "comment 2", "author": map[string]interface{}{"id": 2, "username": "u2", "name": "User2"}, "created_at": "2024-01-01T12:00:00+0800", "updated_at": "2024-01-01T12:00:00+0800"},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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

func TestTC_ListCRCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, []map[string]interface{}{
			{"id": "abc123", "message": "fix bug", "author": map[string]interface{}{"name": "dev"}},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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
	if commits[0].Author.Name != "dev" {
		t.Errorf("expected dev, got %s", commits[0].Author.Name)
	}
}

func TestTC_ForkRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{
			"id": 99, "name": "repo", "path_with_namespace": "user/repo",
			"http_url_to_repo": "https://git.code.tencent.com/user/repo.git",
			"default_branch": "main",
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	repo, err := p.ForkRepo(context.Background(), "owner", "repo", ForkRepoOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.ID != 99 {
		t.Errorf("expected 99, got %d", repo.ID)
	}
}

func TestTC_DeleteRepo(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			called = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	err := p.DeleteRepo(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("expected DELETE")
	}
}

func TestTC_UpdateRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{
			"id": 1, "name": "new-name", "path_with_namespace": "owner/new-name",
			"http_url_to_repo": "https://git.code.tencent.com/owner/new-name.git",
			"default_branch": "develop",
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	repo, err := p.UpdateRepo(context.Background(), "owner", "repo", UpdateRepoOptions{Name: "new-name", DefaultBranch: "develop"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.Name != "new-name" {
		t.Errorf("expected new-name, got %s", repo.Name)
	}
}

func TestTC_GetCommit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{
			"id": "abc123", "message": "init", "author": map[string]interface{}{"name": "dev"},
			"created_at": "2024-01-01T12:00:00+0800",
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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

func TestTC_ListCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, []map[string]interface{}{
			{"id": "sha1", "message": "first", "author": map[string]interface{}{"name": "dev"}, "created_at": "2024-01-01T12:00:00+0800"},
			{"id": "sha2", "message": "second", "author": map[string]interface{}{"name": "dev"}, "created_at": "2024-01-01T12:00:00+0800"},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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

func TestTC_CompareCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{
			"commits": []map[string]interface{}{
				{"id": "sha1", "message": "c1"},
			},
			"diffs": []map[string]interface{}{
				{"old_path": "a.txt", "new_path": "b.txt", "diff": "+1\n-2", "new_file": true, "deleted_file": false, "renamed_file": false},
			},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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
	if result.Files[0].NewPath != "b.txt" {
		t.Errorf("expected b.txt, got %s", result.Files[0].NewPath)
	}
}

func TestTC_CreateFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{"commit_id": "newsha"})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	result, err := p.CreateFile(context.Background(), "owner", "repo", FileOptions{
		Path: "path.txt", Message: "add", Content: base64.StdEncoding.EncodeToString([]byte("hello")), Branch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "newsha" {
		t.Errorf("expected newsha, got %s", result.CommitSHA)
	}
}

func TestTC_UpdateFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{"commit_id": "updsha"})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	result, err := p.UpdateFile(context.Background(), "owner", "repo", FileOptions{
		Path: "path.txt", Message: "update", Content: base64.StdEncoding.EncodeToString([]byte("updated")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "updsha" {
		t.Errorf("expected updsha, got %s", result.CommitSHA)
	}
}

func TestTC_DeleteFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{"commit_id": "delsha"})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	result, err := p.DeleteFile(context.Background(), "owner", "repo", FileDeleteOptions{
		Path: "path.txt", Message: "delete", Branch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommitSHA != "delsha" {
		t.Errorf("expected delsha, got %s", result.CommitSHA)
	}
}

func TestTC_ListTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, []map[string]interface{}{
			{"name": "v1.0", "commit": map[string]interface{}{"id": "sha1"}},
			{"name": "v2.0", "commit": map[string]interface{}{"id": "sha2"}},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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

func TestTC_ListReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, []map[string]interface{}{
			{"id": 1, "tag_name": "v1.0", "name": "Release 1.0", "description": "First"},
		})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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

func TestTC_CreateRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{"id": 5, "tag_name": "v2.0", "name": "Release 2.0", "description": "body"})
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

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

func TestTC_GetArchive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("tar-data"))
	}))
	defer srv.Close()
	p := newTestTCProvider(srv)

	data, err := p.GetArchive(context.Background(), "owner", "repo", "main", "tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "tar-data" {
		t.Errorf("expected tar-data, got %s", string(data))
	}
}

func TestTC_ParseWebhookEvent_Note(t *testing.T) {
	p := &tencentCodeProvider{}
	body := `{"object_kind":"note","user":{"id":1,"username":"dev"},"project":{"path_with_namespace":"o/r"}}`
	req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
	req.Header.Set("X-Token", "secret")

	event, err := p.ParseWebhookEvent(req, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "comment" {
		t.Errorf("expected comment, got %s", event.Type)
	}
}
