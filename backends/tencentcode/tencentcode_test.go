package tencentcode_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
		"created_at":   "2024-01-01T12:00:00+08:00",
		"updated_at":   "2024-01-01T12:00:00+08:00",
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
	if cr.Number != "7" || cr.State != provider.CRStateOpened {
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
	body := `{"object_kind":"merge_request","user":{"id":1,"username":"dev","name":"Dev"},"project":{"path_with_namespace":"owner/repo"},"object_attributes":{"iid":7,"title":"t","description":"d","state":"opened","source_branch":"f","target_branch":"main","action":"open","merge_status":"can_be_merged","url":"https://git.code.tencent.com/o/r/merge_requests/7","last_commit":{"id":"abc"},"created_at":"2024-01-01T12:00:00+08:00","updated_at":"2024-01-01T12:00:00+08:00"}}`
	r, _ := http.NewRequest(http.MethodPost, "/hook", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	ne, err := p.ParseWebhookEvent(r, "")
	if err != nil {
		t.Fatal(err)
	}
	if ne.Type != "cr.open" {
		t.Errorf("expected cr.open, got %s", ne.Type)
	}
	if ne.CR == nil || ne.CR.Number != "7" {
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

// TestUpdateRelease_AllNilShortCircuit verifies that an UpdateRelease
// carrying nothing this platform can express (only Body is forwarded;
// Name/Draft/Prerelease are ignored by registered limitation) skips the
// PUT entirely and returns the current release from GetReleaseByTag.
func TestUpdateRelease_AllNilShortCircuit(t *testing.T) {
	var mu sync.Mutex
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods = append(methods, r.Method)
		mu.Unlock()
		writeJSON(w, map[string]any{"tag_name": "v1.0", "description": "current notes"})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	rel, err := p.UpdateRelease(context.Background(), "owner", "repo", "v1.0", provider.UpdateReleaseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rel == nil || rel.TagName != "v1.0" || rel.Body != "current notes" {
		t.Errorf("unexpected release: %+v", rel)
	}
	for _, m := range methods {
		if m == http.MethodPut {
			t.Errorf("all-nil update must not PUT an empty body; recorded methods: %v", methods)
		}
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

// tcIssueResponse builds a gongfeng-shaped issue payload with the given
// iid, state, and label set.
func tcIssueResponse(iid int, state string, labels []string) map[string]any {
	return map[string]any{
		"id": 100, "iid": iid, "title": "bug", "description": "broke",
		"state": state, "labels": labels,
		"author":     map[string]any{"id": 1, "username": "dev", "name": "Developer"},
		"assignees":  []any{map[string]any{"id": 1, "username": "dev", "name": "Developer"}},
		"milestone":  map[string]any{"id": 5, "title": "v1"},
		"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
	}
}

// TestIssueStateTransitions_SendStateEventVerbs checks that CloseIssue and
// ReopenIssue travel as 工蜂's documented state_event verbs (close/reopen).
func TestIssueStateTransitions_SendStateEventVerbs(t *testing.T) {
	var mu sync.Mutex
	var events []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			events = append(events, body["state_event"].(string))
			mu.Unlock()
		}
		writeJSON(w, tcIssueResponse(3, "closed", []string{"bug"}))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	if _, err := p.CloseIssue(context.Background(), "owner", "repo", "3"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReopenIssue(context.Background(), "owner", "repo", "3"); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0] != "close" || events[1] != "reopen" {
		t.Errorf("expected state_event verbs [close reopen], got %v", events)
	}
}

// TestListIssues_FiltersCarryWireVocabulary checks the state and labels
// list filters reach the query string in 工蜂's vocabulary (state=opened,
// labels csv).
func TestListIssues_FiltersCarryWireVocabulary(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, []any{tcIssueResponse(3, "opened", []string{"bug"})})
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	if _, _, err := p.ListIssues(context.Background(), provider.ListIssuesOptions{
		Owner: "owner", Repo: "repo", State: provider.IssueStateOpen, Labels: "a,b",
	}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"state=opened", "labels=a%2Cb"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("expected query to carry %s, got %q", want, gotQuery)
		}
	}
}

// TestUpdateIssue_WireMappings checks the update mappings: state becomes a
// state_event verb, labels become a csv, and the milestone ref becomes the
// numeric milestone_id.
func TestUpdateIssue_WireMappings(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			body = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		writeJSON(w, tcIssueResponse(3, "opened", []string{"bug"}))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	if _, err := p.UpdateIssue(context.Background(), "owner", "repo", "3", provider.UpdateIssueOptions{
		Title: "fixed", Body: "no more", State: provider.IssueStateOpen,
		Labels: []string{"a", "b"}, Milestone: "5",
	}); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{"title": "fixed", "description": "no more", "state_event": "reopen", "labels": "a,b", "milestone_id": float64(5)} {
		if body[key] != want {
			t.Errorf("expected %s=%v on the wire, got body %v", key, want, body)
		}
	}
}

// TestCreateIssue_IgnoresAssignees checks the registered Assignees
// limitation: usernames cannot reach 工蜂's assignee_ids surface, so the
// create body carries no assignee_ids key while title/description/labels
// csv do travel.
func TestCreateIssue_IgnoresAssignees(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		writeJSON(w, tcIssueResponse(9, "opened", []string{"bug"}))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	if _, err := p.CreateIssue(context.Background(), provider.CreateIssueOptions{
		Owner: "owner", Repo: "repo", Title: "bug", Body: "broke",
		Assignees: []string{"dev"}, Labels: []string{"a", "b"}, Milestone: "5",
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["assignee_ids"]; ok {
		t.Errorf("expected no assignee_ids on the wire (registered ignore), got body %v", body)
	}
	if body["labels"] != "a,b" || body["milestone_id"] != float64(5) {
		t.Errorf("expected labels csv and milestone_id, got body %v", body)
	}
}

// TestGetIssue_MapsModelToProvider checks convertIssue's mappings: Number
// carries the IID, state opened→open, the milestone ref carries the
// milestone ID, and WebURL stays empty (the gongfeng model has no field).
func TestGetIssue_MapsModelToProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, tcIssueResponse(3, "opened", []string{"bug", "enhancement"}))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	issue, err := p.GetIssue(context.Background(), "owner", "repo", "3")
	if err != nil {
		t.Fatal(err)
	}
	if issue.Number != "3" || issue.State != provider.IssueStateOpen {
		t.Errorf("unexpected number/state: %+v", issue)
	}
	if issue.WebURL != "" {
		t.Errorf("expected empty WebURL (registered), got %q", issue.WebURL)
	}
	if issue.Milestone == nil || issue.Milestone.Number != "5" || issue.Milestone.Title != "v1" {
		t.Errorf("expected milestone ref {5, v1}, got %+v", issue.Milestone)
	}
	if len(issue.Assignees) != 1 || issue.Assignees[0] != "dev" {
		t.Errorf("expected assignee usernames [dev], got %v", issue.Assignees)
	}
}

// TestAddIssueLabels_UnionsAndRewrites checks the read-union-rewrite
// shape: 工蜂's update surface takes the full label set only.
func TestAddIssueLabels_UnionsAndRewrites(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			body = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		writeJSON(w, tcIssueResponse(3, "opened", []string{"bug", "enhancement"}))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	if err := p.AddIssueLabels(context.Background(), "owner", "repo", "3", []string{"urgent", "bug"}); err != nil {
		t.Fatal(err)
	}
	if body["labels"] != "bug,enhancement,urgent" {
		t.Errorf("expected union csv bug,enhancement,urgent, got body %v", body)
	}
}

// TestRemoveIssueLabel_RewritesWithoutName checks the read-filter-rewrite
// shape when a label survives the filter.
func TestRemoveIssueLabel_RewritesWithoutName(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			body = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		writeJSON(w, tcIssueResponse(3, "opened", []string{"bug", "enhancement"}))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	if err := p.RemoveIssueLabel(context.Background(), "owner", "repo", "3", "bug"); err != nil {
		t.Fatal(err)
	}
	if body["labels"] != "enhancement" {
		t.Errorf("expected surviving csv enhancement, got body %v", body)
	}
}

// TestRemoveIssueLabel_LastLabelNoOp checks the registered limitation:
// removing an issue's only label yields an empty csv that omitempty drops
// from the PUT body, so the rewrite carries no labels key at all.
func TestRemoveIssueLabel_LastLabelNoOp(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			body = map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		writeJSON(w, tcIssueResponse(3, "opened", []string{"bug"}))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	if err := p.RemoveIssueLabel(context.Background(), "owner", "repo", "3", "bug"); err != nil {
		t.Fatal(err)
	}
	if body == nil {
		t.Fatal("expected a PUT rewrite")
	}
	if _, ok := body["labels"]; ok {
		t.Errorf("expected no labels key on the wire (registered no-op), got body %v", body)
	}
}

// TestIssueNumber_AtoiGuard checks the string-entry parse guard.
func TestIssueNumber_AtoiGuard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, tcIssueResponse(1, "opened", nil))
	}))
	defer srv.Close()
	p := newTestProvider(t, srv)
	_, err := p.GetIssue(context.Background(), "owner", "repo", "not-a-number")
	if err == nil || !strings.Contains(err.Error(), `invalid issue number "not-a-number"`) {
		t.Errorf("expected invalid issue number error, got %v", err)
	}
}
