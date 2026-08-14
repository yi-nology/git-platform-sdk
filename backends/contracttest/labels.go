package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// LabelsHarness bundles the inputs needed to run the label-management
// contract suite against a backend that implements provider.LabelManager.
type LabelsHarness struct {
	// Name is the human-readable platform identifier (e.g. "GitHub").
	Name string
	// Platform is the provider.Platform constant for this backend.
	Platform provider.Platform
	// NewProvider builds a provider.Provider; the harness fills in BaseURL.
	// It must construct the provider the same way the platform's main
	// harness does (including any VersionProxy wrapping for Gitea/Forgejo).
	NewProvider func(t *testing.T, cfg provider.Config) provider.Provider
	// ListResponse is the JSON array the mock returns for GET requests
	// (label listings and name→ID resolution lookups). Its first item must
	// have name "bug" and color "#4cc917" so the suite can assert color
	// normalization.
	ListResponse string
	// MutateResponse is the JSON object the mock returns for POST/PATCH
	// requests. It must have name "bug" and color "#4cc917".
	MutateResponse string
}

// RunLabelsSuite executes the label-management contract suite. The mock
// server routes by HTTP method so platform-specific paths don't matter:
// GET returns ListResponse, POST returns 201 + MutateResponse, PATCH/PUT
// return 200 + MutateResponse, DELETE returns 204.
func RunLabelsSuite(t *testing.T, h LabelsHarness) {
	newLM := func(t *testing.T) provider.LabelManager {
		srv := labelStubServer(h)
		t.Cleanup(srv.Close)
		p := h.NewProvider(t, provider.Config{Platform: h.Platform, BaseURL: srv.URL, Token: "test"})
		lm, ok := p.(provider.LabelManager)
		if !ok {
			t.Fatalf("%s does not implement provider.LabelManager", h.Name)
		}
		return lm
	}

	t.Run("List_NormalizesColor", func(t *testing.T) {
		lm := newLM(t)
		labels, err := lm.ListLabels(context.Background(), "owner", "repo", provider.ListLabelsOptions{Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("ListLabels: %v", err)
		}
		if len(labels) == 0 {
			t.Fatal("expected at least one label")
		}
		if labels[0].Name != "bug" {
			t.Errorf("expected first label name %q, got %q", "bug", labels[0].Name)
		}
		if labels[0].Color != "4cc917" {
			t.Errorf("expected normalized color %q, got %q — backends must strip the leading '#'", "4cc917", labels[0].Color)
		}
	})

	t.Run("Create_ReturnsLabel", func(t *testing.T) {
		lm := newLM(t)
		l, err := lm.CreateLabel(context.Background(), "owner", "repo", provider.CreateLabelOptions{
			Name: "bug", Color: "4cc917", Description: "something broke",
		})
		if err != nil {
			t.Fatalf("CreateLabel: %v", err)
		}
		if l == nil || l.Name != "bug" {
			t.Fatalf("expected created label named bug, got %+v", l)
		}
		if l.Color != "4cc917" {
			t.Errorf("expected normalized color 4cc917, got %q", l.Color)
		}
	})

	t.Run("Update_Succeeds", func(t *testing.T) {
		lm := newLM(t)
		newName := "bug-2"
		l, err := lm.UpdateLabel(context.Background(), "owner", "repo", "bug", provider.UpdateLabelOptions{NewName: &newName})
		if err != nil {
			t.Fatalf("UpdateLabel: %v", err)
		}
		if l == nil {
			t.Fatal("expected updated label, got nil")
		}
	})

	t.Run("Delete_Succeeds", func(t *testing.T) {
		lm := newLM(t)
		if err := lm.DeleteLabel(context.Background(), "owner", "repo", "bug"); err != nil {
			t.Fatalf("DeleteLabel: %v", err)
		}
	})
}

// labelStubServer returns a method-routed mock for the labels suite.
func labelStubServer(h LabelsHarness) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(h.ListResponse))
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(h.MutateResponse))
		case http.MethodPatch, http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(h.MutateResponse))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}
