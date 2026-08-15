package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// IssuesHarnessConfig carries the fixtures a backend's main Harness needs to
// auto-mount the issue-management suite via Harness.Issues.
type IssuesHarnessConfig struct {
	// ListResponse is the JSON array for issue-list GETs. First item: number
	// 1, title "bug", state "open", milestone {number:1, title:"v1"}.
	ListResponse string
	// GetResponse is the JSON object for single-issue GETs (number 1, title
	// "bug", state "open", milestone {number:1, title:"v1"}).
	GetResponse string
	// MutateResponse is the JSON object for POST/PATCH/PUT (same shape as
	// GetResponse).
	MutateResponse string
	// CommentsResponse is the JSON array for issue-comment GETs. First item:
	// id 1, body "a comment".
	CommentsResponse string
	// LabelsResponse is the JSON array for repository-label GETs. First
	// item: id 1, name "bug", color "#4cc917".
	LabelsResponse string
}

// IssuesHarness is the full harness RunIssuesSuite consumes; auto-mounting
// builds it from the enclosing Harness plus IssuesHarnessConfig.
type IssuesHarness struct {
	Name        string
	Platform    provider.Platform
	NewProvider func(t *testing.T, cfg provider.Config) provider.Provider
	IssuesHarnessConfig
}

// testIssuesSuite auto-mounts RunIssuesSuite from a main Harness with the
// same bidirectional drift checks as the labels suite.
func testIssuesSuite(t *testing.T, h Harness) {
	srv := httptest.NewServer(stubHandler(h))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	declared := p.Capabilities().Issues
	switch {
	case h.Issues == nil && !declared:
		t.Skipf("%s declares no Issues capability", h.Name)
	case h.Issues == nil:
		t.Errorf("%s declares Capabilities().Issues but its Harness provides no Issues config — the issues suite is not wired", h.Name)
	case !declared:
		t.Errorf("%s Harness provides an Issues config but the platform does not declare Capabilities().Issues", h.Name)
	default:
		RunIssuesSuite(t, IssuesHarness{
			Name: h.Name, Platform: h.Platform, NewProvider: h.NewProvider,
			IssuesHarnessConfig: *h.Issues,
		})
	}
}

// RunIssuesSuite executes the issue-management contract suite. The mock
// routes by method and path shape so platform-specific paths don't matter:
// GET ending /issues/{alphanumeric} → GetResponse (issue numbers are not
// always numeric — Gitee's are alphanumeric); GET containing
// comments|notes →
// CommentsResponse; GET containing labels (not issues) → LabelsResponse;
// other GET → ListResponse; POST → 201 + MutateResponse (a POST to a labels
// path — adding labels to an issue — returns LabelsResponse, since
// GitHub-shaped APIs answer with the label array); PATCH/PUT →
// MutateResponse; DELETE → 204. Requests are recorded for wire assertions.
func RunIssuesSuite(t *testing.T, h IssuesHarness) {
	newIM := func(t *testing.T) (provider.IssueManager, *[]recordedRequest) {
		srv, requests := issueStubServer(h)
		t.Cleanup(srv.Close)
		p := h.NewProvider(t, provider.Config{Platform: h.Platform, BaseURL: srv.URL, Token: "test"})
		im, ok := p.(provider.IssueManager)
		if !ok {
			t.Fatalf("%s does not implement provider.IssueManager", h.Name)
		}
		return im, requests
	}

	t.Run("List_ParsesAndNormalizes", func(t *testing.T) {
		im, _ := newIM(t)
		assertIssueListNormalized(t, im)
	})
	t.Run("Get_ReturnsIssue", func(t *testing.T) {
		im, _ := newIM(t)
		assertIssueGet(t, im)
	})
	t.Run("Create_PostsTitle", func(t *testing.T) {
		im, requests := newIM(t)
		assertIssueCreateWire(t, im, requests)
	})
	t.Run("Update_PatchesTitle", func(t *testing.T) {
		im, requests := newIM(t)
		assertIssueUpdateWire(t, im, requests)
	})
	t.Run("CloseAndReopen_Succeed", func(t *testing.T) {
		im, _ := newIM(t)
		assertIssueCloseReopen(t, im)
	})
	t.Run("Comments_ListAndCreate", func(t *testing.T) {
		im, requests := newIM(t)
		assertIssueComments(t, im, requests)
	})
	t.Run("IssueLabels_ListAddRemove", func(t *testing.T) {
		im, requests := newIM(t)
		assertIssueLabelOps(t, im, requests)
	})
}

// assertIssueListNormalized checks that ListIssues returns parsed, normalized
// issues: title, open state, and the milestone ref.
func assertIssueListNormalized(t *testing.T, im provider.IssueManager) {
	t.Helper()
	issues, _, err := im.ListIssues(context.Background(), provider.ListIssuesOptions{Owner: "owner", Repo: "repo", Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) == 0 || issues[0].Title != "bug" {
		t.Fatalf("expected first issue titled bug, got %+v", issues)
	}
	if issues[0].State != provider.IssueStateOpen {
		t.Errorf("expected state open, got %q", issues[0].State)
	}
	if issues[0].Milestone == nil || issues[0].Milestone.Number != "1" || issues[0].Milestone.Title != "v1" {
		t.Errorf("expected milestone ref {1, v1}, got %+v", issues[0].Milestone)
	}
}

// assertIssueGet checks that GetIssue returns the fixture issue.
func assertIssueGet(t *testing.T, im provider.IssueManager) {
	t.Helper()
	issue, err := im.GetIssue(context.Background(), "owner", "repo", "1")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue == nil || issue.Title != "bug" {
		t.Fatalf("expected issue titled bug, got %+v", issue)
	}
}

// assertIssueCreateWire checks that CreateIssue succeeds and posts the title.
func assertIssueCreateWire(t *testing.T, im provider.IssueManager, requests *[]recordedRequest) {
	t.Helper()
	issue, err := im.CreateIssue(context.Background(), provider.CreateIssueOptions{Owner: "owner", Repo: "repo", Title: "bug", Body: "broke"})
	if err != nil || issue == nil {
		t.Fatalf("CreateIssue: issue=%v err=%v", issue, err)
	}
	assertBodyHas(t, requests, http.MethodPost, "title", "bug")
}

// assertIssueUpdateWire checks that UpdateIssue mutates via PATCH or PUT and
// carries the new title in the body.
func assertIssueUpdateWire(t *testing.T, im provider.IssueManager, requests *[]recordedRequest) {
	t.Helper()
	title := "bug-2"
	if _, err := im.UpdateIssue(context.Background(), "owner", "repo", "1", provider.UpdateIssueOptions{Title: title}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if !sawMethod(*requests, http.MethodPatch, http.MethodPut) {
		t.Errorf("expected a PATCH/PUT update, recorded %s", methodsOf(*requests))
	}
	assertBodyHas(t, requests, http.MethodPatch, http.MethodPut, "title", "bug-2") // PUT also accepted
}

// assertIssueCloseReopen checks that both state transitions succeed.
func assertIssueCloseReopen(t *testing.T, im provider.IssueManager) {
	t.Helper()
	if _, err := im.CloseIssue(context.Background(), "owner", "repo", "1"); err != nil {
		t.Fatalf("CloseIssue: %v", err)
	}
	if _, err := im.ReopenIssue(context.Background(), "owner", "repo", "1"); err != nil {
		t.Fatalf("ReopenIssue: %v", err)
	}
}

// assertIssueComments checks comment listing (body) and creation (wire body).
func assertIssueComments(t *testing.T, im provider.IssueManager, requests *[]recordedRequest) {
	t.Helper()
	comments, err := im.ListIssueComments(context.Background(), "owner", "repo", "1")
	if err != nil || len(comments) == 0 {
		t.Fatalf("ListIssueComments: comments=%d err=%v", len(comments), err)
	}
	if comments[0].Body != "a comment" {
		t.Errorf("expected comment body %q, got %q", "a comment", comments[0].Body)
	}
	if _, err := im.CreateIssueComment(context.Background(), "owner", "repo", "1", "a comment"); err != nil {
		t.Fatalf("CreateIssueComment: %v", err)
	}
	assertBodyHas(t, requests, http.MethodPost, "body", "a comment")
}

// assertIssueLabelOps checks repository-label listing (color normalization),
// adding labels to an issue, and removing one — via a dedicated DELETE
// endpoint (GitHub-style) or via a PATCH/PUT write (GitLab's remove_labels).
func assertIssueLabelOps(t *testing.T, im provider.IssueManager, requests *[]recordedRequest) {
	t.Helper()
	labels, err := im.ListIssueLabels(context.Background(), "owner", "repo")
	if err != nil || len(labels) == 0 {
		t.Fatalf("ListIssueLabels: labels=%d err=%v", len(labels), err)
	}
	if labels[0].Color != "4cc917" {
		t.Errorf("expected normalized color 4cc917, got %q", labels[0].Color)
	}
	if err := im.AddIssueLabels(context.Background(), "owner", "repo", "1", []string{"bug"}); err != nil {
		t.Fatalf("AddIssueLabels: %v", err)
	}
	if err := im.RemoveIssueLabel(context.Background(), "owner", "repo", "1", "bug"); err != nil {
		t.Fatalf("RemoveIssueLabel: %v", err)
	}
	if !sawLabelRemoval(*requests) {
		t.Errorf("expected label removal via DELETE or a PATCH/PUT write carrying remove_labels, recorded %s", methodsOf(*requests))
	}
}

// sawLabelRemoval reports whether label removal used one of the two wire
// shapes platforms have: a dedicated DELETE endpoint (GitHub, Gitea,
// Forgejo, GitCode), or a PATCH/PUT write whose body carries a non-empty
// remove_labels key (GitLab — go-gitlab sends the update form-encoded, and
// the recorded Body map normalizes form keys). Bare method matching is not
// enough: GitLab's AddIssueLabels is also a PUT, so a RemoveIssueLabel that
// silently stopped sending remove_labels would otherwise pass.
func sawLabelRemoval(requests []recordedRequest) bool {
	for _, r := range requests {
		switch r.Method {
		case http.MethodDelete:
			return true
		case http.MethodPatch, http.MethodPut:
			if v, ok := r.Body["remove_labels"]; ok && nonEmptyBodyValue(v) {
				return true
			}
		}
	}
	return false
}

// nonEmptyBodyValue reports whether a decoded body value carries content:
// strings must be non-empty, non-nil values of any other decoded type (e.g.
// a JSON array) count as present.
func nonEmptyBodyValue(v any) bool {
	if v == nil {
		return false
	}
	if s, ok := v.(string); ok {
		return s != ""
	}
	return true
}

// Alphanumeric (not just digits) because Gitee issue numbers are alphanumeric (e.g. I3XU7A).
var issuePathNum = regexp.MustCompile(`/issues/[A-Za-z0-9]+$`)

// issueStubServer returns the method/shape-routed recording mock.
func issueStubServer(h IssuesHarness) (*httptest.Server, *[]recordedRequest) {
	var mu sync.Mutex
	var requests []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := decodeRecordedBody(r)
		mu.Lock()
		requests = append(requests, recordedRequest{Method: r.Method, Path: r.URL.RequestURI(), Body: body})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "labels"):
			// Adding labels to an issue: GitHub-shaped APIs answer with the
			// label array (e.g. []*Label), not a single issue object.
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(h.LabelsResponse))
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(h.MutateResponse))
		case r.Method == http.MethodPatch || r.Method == http.MethodPut:
			_, _ = w.Write([]byte(h.MutateResponse))
		case issuePathNum.MatchString(r.URL.Path):
			_, _ = w.Write([]byte(h.GetResponse))
		case strings.Contains(r.URL.Path, "comments"), strings.Contains(r.URL.Path, "notes"):
			_, _ = w.Write([]byte(h.CommentsResponse))
		case strings.Contains(r.URL.Path, "labels") && !strings.Contains(r.URL.Path, "issues"):
			_, _ = w.Write([]byte(h.LabelsResponse))
		default:
			_, _ = w.Write([]byte(h.ListResponse))
		}
	}))
	return srv, &requests
}

// assertBodyHas asserts that some request with one of the verbs carried want
// under key. Its arguments follow the convention verbs..., key, want: the
// last two variadic elements are the body key and expected value, everything
// before them the accepted HTTP methods.
func assertBodyHas(t *testing.T, requests *[]recordedRequest, methods ...string) {
	t.Helper()
	key, want := methods[len(methods)-2], methods[len(methods)-1]
	verbSet := methods[:len(methods)-2]
	for i := range *requests {
		r := &(*requests)[i]
		for _, m := range verbSet {
			if r.Method != m {
				continue
			}
			if v, ok := r.Body[key]; ok {
				if s, ok := v.(string); ok && strings.Contains(s, want) {
					return
				}
			}
		}
	}
	t.Errorf("expected a %v request whose body carries %q=%q; recorded: %v", verbSet, key, want, *requests)
}

// sawMethod reports whether any recorded request used one of the methods.
func sawMethod(requests []recordedRequest, methods ...string) bool {
	for _, r := range requests {
		for _, m := range methods {
			if r.Method == m {
				return true
			}
		}
	}
	return false
}
