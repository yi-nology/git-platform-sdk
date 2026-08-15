package gitcode_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// TestGitCode_MilestoneMutations_OmitDueOn verifies the wire form of the
// milestone mutation bodies: gitcode_api's create/update option structs
// marshal `due_on` without omitempty, so riding the SDK would post
// `"due_on": ""` on every call without a due date — on GitCode's
// GitHub-shaped API that conventionally clears (or errors on) the value,
// wiping the milestone's due date on a title-only update. The backend must
// therefore omit the key entirely when DueOn is nil (raw-transport
// detour; see backends/gitcode/milestones.go).
func TestGitCode_MilestoneMutations_OmitDueOn(t *testing.T) {
	var gotBodies = map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			raw, _ := io.ReadAll(r.Body)
			var m map[string]any
			_ = json.Unmarshal(raw, &m)
			gotBodies[r.Method] = m
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"title":"v1","state":"open"}`))
	}))
	defer srv.Close()

	p, err := provider.NewProvider(provider.Config{Platform: provider.PlatformGitCode, BaseURL: srv.URL, Token: "t"})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	mm := p.(provider.MilestoneManager)

	// Title-only update must not carry a due_on key — the finding's core
	// assertion: an empty-string due_on would clear GitCode's stored date.
	newTitle := "v2-renamed"
	if _, err := mm.UpdateMilestone(context.Background(), "owner", "repo", "1", provider.UpdateMilestoneOptions{Title: &newTitle}); err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}
	updateBody, ok := gotBodies[http.MethodPatch]
	if !ok {
		t.Fatalf("expected a PATCH request, recorded methods: create=%v", gotBodies[http.MethodPost] != nil)
	}
	if v, present := updateBody["due_on"]; present {
		t.Errorf("title-only update body carries due_on=%v; the key must be omitted so GitCode keeps the stored due date", v)
	}
	if v, _ := updateBody["title"].(string); v != newTitle {
		t.Errorf("update body title = %q, want %q", v, newTitle)
	}

	// Same omission on create without a due date.
	if _, err := mm.CreateMilestone(context.Background(), "owner", "repo", provider.CreateMilestoneOptions{Title: "v2"}); err != nil {
		t.Fatalf("CreateMilestone: %v", err)
	}
	createBody, ok := gotBodies[http.MethodPost]
	if !ok {
		t.Fatal("expected a POST request")
	}
	if v, present := createBody["due_on"]; present {
		t.Errorf("due-on-less create body carries due_on=%v; the key must be omitted", v)
	}
	if v, _ := createBody["title"].(string); v != "v2" {
		t.Errorf("create body title = %q, want %q", v, "v2")
	}
}
