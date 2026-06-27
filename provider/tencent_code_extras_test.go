package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// readJSONBody reads and JSON-decodes the request body into a map.
func readJSONBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
	}
	return m
}

// tcExtrasSrv builds a mock 工蜂 server that records the last request into
// *last and returns canned JSON per path. Pass nil for *last if recording is
// not needed.
func tcExtrasSrv(t *testing.T, last **http.Request, routes map[string]func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if last != nil {
			*last = r
		}
		w.Header().Set("Content-Type", "application/json")
		for pattern, handler := range routes {
			if r.URL.EscapedPath() == pattern {
				handler(w, r)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]string{"error": "no route for " + r.URL.Path})
	}))
	return srv
}

func TestTC_TypeAssertTencentCodeExtras(t *testing.T) {
	p, err := NewProvider(Config{Platform: PlatformTencentCode, Token: "token"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.(TencentCodeExtras); !ok {
		t.Fatal("expected *tencentCodeProvider to satisfy TencentCodeExtras")
	}
}

func TestTC_CreateCodeReview(t *testing.T) {
	var gotBody map[string]interface{}
	srv := tcExtrasSrv(t, nil, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/review": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			gotBody = readJSONBody(t, r)
			writeJSON(w, map[string]interface{}{"id": 10, "iid": 7, "title": "rev", "state": "approving"})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	cr, err := p.CreateCodeReview(context.Background(), "o", "r", CreateCodeReviewOptions{
		Title: "rev", SourceBranch: "feat", TargetBranch: "main",
		ReviewerIDs: "1,2", ApproverRule: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cr.ID != 10 || cr.IID != 7 || cr.State != "approving" {
		t.Fatalf("unexpected review: %+v", cr)
	}
	if gotBody["source_branch"] != "feat" || gotBody["approver_rule"] != float64(1) || gotBody["reviewer_ids"] != "1,2" {
		t.Fatalf("unexpected body: %+v", gotBody)
	}
}

func TestTC_ListCodeReviews(t *testing.T) {
	var last *http.Request
	srv := tcExtrasSrv(t, &last, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/reviews": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"id": 1, "title": "a"},
				{"id": 2, "title": "b"},
			})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	reviews, err := p.ListCodeReviews(context.Background(), "o", "r", ListCodeReviewsOptions{State: "approving", Page: 2, PerPage: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(reviews))
	}
	if q := last.URL.Query(); q.Get("state") != "approving" || q.Get("page") != "2" || q.Get("per_page") != "5" {
		t.Fatalf("unexpected query: %s", last.URL.RawQuery)
	}
}

func TestTC_SubmitCodeReview(t *testing.T) {
	var gotBody map[string]interface{}
	srv := tcExtrasSrv(t, nil, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/review/3/reviewer/summary": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			gotBody = readJSONBody(t, r)
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	if err := p.SubmitCodeReview(context.Background(), "o", "r", 3, SubmitReviewOptions{Event: ReviewEventApprove, Summary: "lgtm"}); err != nil {
		t.Fatal(err)
	}
	if gotBody["reviewer_event"] != "approve" || gotBody["summary"] != "lgtm" {
		t.Fatalf("unexpected body: %+v", gotBody)
	}
}

func TestTC_GetCodeReviewChangedFiles(t *testing.T) {
	srv := tcExtrasSrv(t, nil, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/review/3/changed_files": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"old_path": "a.go", "new_path": "a.go", "new_file": false, "deleted_file": false, "renamed_file": false},
			})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	files, err := p.GetCodeReviewChangedFiles(context.Background(), "o", "r", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].NewPath != "a.go" {
		t.Fatalf("unexpected files: %+v", files)
	}
}

func TestTC_RemoveCodeReviewer(t *testing.T) {
	var last *http.Request
	srv := tcExtrasSrv(t, &last, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/review/3/dismissals": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	if err := p.RemoveCodeReviewer(context.Background(), "o", "r", 3, "42"); err != nil {
		t.Fatal(err)
	}
	if q := last.URL.Query(); q.Get("reviewer_id") != "42" {
		t.Fatalf("expected reviewer_id=42, got %s", last.URL.RawQuery)
	}
}

func TestTC_GetMRReview(t *testing.T) {
	srv := tcExtrasSrv(t, nil, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/merge_request/5/review": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{"id": 5, "iid": 5, "state": "approving", "title": "mr"})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	mr, err := p.GetMRReview(context.Background(), "o", "r", 5)
	if err != nil {
		t.Fatal(err)
	}
	if mr.State != "approving" || mr.IID != 5 {
		t.Fatalf("unexpected mr review: %+v", mr)
	}
}

func TestTC_SubmitMRReview(t *testing.T) {
	srv := tcExtrasSrv(t, nil, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/merge_request/5/reviewer/summary": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	if err := p.SubmitMRReview(context.Background(), "o", "r", 5, SubmitReviewOptions{Event: ReviewEventRequireChange, Summary: "pls fix"}); err != nil {
		t.Fatal(err)
	}
}

func TestTC_GetCommitDiff(t *testing.T) {
	var last *http.Request
	srv := tcExtrasSrv(t, &last, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/repository/commits/abc123/diff": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"old_path": "main.go", "new_path": "main.go", "diff": "-old\n+new"},
			})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	files, err := p.GetCommitDiff(context.Background(), "o", "r", "abc123", CommitDiffOptions{Path: "main.go", IgnoreWhiteSpace: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Diff != "-old\n+new" {
		t.Fatalf("unexpected diff: %+v", files)
	}
	if q := last.URL.Query(); q.Get("path") != "main.go" || q.Get("ignore_white_space") != "true" {
		t.Fatalf("unexpected query: %s", last.URL.RawQuery)
	}
}

func TestTC_CreateCommitComment(t *testing.T) {
	var gotBody map[string]interface{}
	srv := tcExtrasSrv(t, nil, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/repository/commits/abc123/comments": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			gotBody = readJSONBody(t, r)
			writeJSON(w, map[string]interface{}{"id": 99, "body": "nice", "path": "a.go"})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	c, err := p.CreateCommitComment(context.Background(), "o", "r", "abc123", CreateCommitCommentOptions{Note: "nice", Path: "a.go", Line: 10, LineType: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != 99 || c.Path != "a.go" {
		t.Fatalf("unexpected comment: %+v", c)
	}
	if gotBody["note"] != "nice" || gotBody["line"] != float64(10) || gotBody["line_type"] != "new" {
		t.Fatalf("unexpected body: %+v", gotBody)
	}
}

func TestTC_GetCommitRefs(t *testing.T) {
	var last *http.Request
	srv := tcExtrasSrv(t, &last, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/repository/commits/abc123/refs": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{
				"branches": []map[string]interface{}{{"name": "main"}, {"name": "dev"}},
				"tags":     []map[string]interface{}{{"name": "v1"}},
			})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	refs, err := p.GetCommitRefs(context.Background(), "o", "r", "abc123", CommitRefBranch)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs.Branches) != 2 || refs.Branches[0] != "main" || len(refs.Tags) != 1 || refs.Tags[0] != "v1" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
	if q := last.URL.Query(); q.Get("type") != "branch" {
		t.Fatalf("expected type=branch, got %s", last.URL.RawQuery)
	}
}

func TestTC_GetRepoTree(t *testing.T) {
	var last *http.Request
	srv := tcExtrasSrv(t, &last, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/repository/tree": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"id": "1", "name": "src", "type": "tree", "path": "src"},
				{"id": "2", "name": "main.go", "type": "blob", "path": "main.go"},
			})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	nodes, err := p.GetRepoTree(context.Background(), "o", "r", "src", "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Type != "tree" || nodes[1].Path != "main.go" {
		t.Fatalf("unexpected tree: %+v", nodes)
	}
	q := last.URL.Query()
	if q.Get("path") != "src" || q.Get("ref_name") != "main" || q.Get("recursive") != "true" {
		t.Fatalf("unexpected query: %s", last.URL.RawQuery)
	}
}

func TestTC_GetBlob(t *testing.T) {
	raw := "hello blob"
	srv := tcExtrasSrv(t, nil, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/repository/blobs/deadbeef": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]interface{}{
				"content":  base64.StdEncoding.EncodeToString([]byte(raw)),
				"encoding": "base64",
			})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	data, err := p.GetBlob(context.Background(), "o", "r", "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != raw {
		t.Fatalf("expected %q, got %q", raw, string(data))
	}
}

func TestTC_ProtectBranch(t *testing.T) {
	var last *http.Request
	srv := tcExtrasSrv(t, &last, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/repository/branches/main/protect": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	if err := p.ProtectBranch(context.Background(), "o", "r", "main", ProtectBranchOptions{}); err != nil {
		t.Fatal(err)
	}
	if last.URL.EscapedPath() != "/projects/o%2Fr/repository/branches/main/protect" {
		t.Fatalf("unexpected path: %s", last.URL.EscapedPath())
	}
}

func TestTC_ProtectedBranchMembers(t *testing.T) {
	srv := tcExtrasSrv(t, nil, map[string]func(http.ResponseWriter, *http.Request){
		"/projects/o%2Fr/branches/protected/main/members": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, []map[string]interface{}{
				{"user_id": 1, "username": "alice", "access_level": 40},
			})
		},
	})
	defer srv.Close()
	p := newTestTCProvider(srv)

	members, err := p.ListProtectedBranchMembers(context.Background(), "o", "r", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].Username != "alice" || members[0].AccessLevel != 40 {
		t.Fatalf("unexpected members: %+v", members)
	}
}
