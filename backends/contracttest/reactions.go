package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ReactionsHarnessConfig carries the fixtures a backend's main Harness needs
// to auto-mount the reaction suite via Harness.Reactions.
type ReactionsHarnessConfig struct {
	// ListResponse is the JSON array the mock returns for reaction list
	// requests. It must contain at least one reaction with content "+1".
	ListResponse string
	// CreateResponse is the JSON object the mock returns for reaction create
	// requests.
	CreateResponse string
}

// testReactionsSuite auto-mounts RunReactionsSuite from a main Harness,
// enforcing bidirectional drift checks.
func testReactionsSuite(t *testing.T, h Harness) {
	srv := httptest.NewServer(stubHandler(h))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	declared := p.Capabilities().Reactions
	switch {
	case h.Reactions == nil && !declared:
		t.Skipf("%s declares no Reactions capability", h.Name)
	case h.Reactions == nil:
		t.Errorf("%s declares Capabilities().Reactions but its Harness provides no Reactions config", h.Name)
	case !declared:
		t.Errorf("%s Harness provides a Reactions config but the platform does not declare Capabilities().Reactions", h.Name)
	default:
		RunReactionsSuite(t, ReactionsHarness{
			Name:           h.Name,
			Platform:       h.Platform,
			NewProvider:    h.NewProvider,
			ListResponse:   h.Reactions.ListResponse,
			CreateResponse: h.Reactions.CreateResponse,
		})
	}
}

// ReactionsHarness bundles the inputs needed to run the reaction contract
// suite against a backend that implements provider.ReactionManager.
type ReactionsHarness struct {
	Name           string
	Platform       provider.Platform
	NewProvider    func(t *testing.T, cfg provider.Config) provider.Provider
	ListResponse   string
	CreateResponse string
}

// RunReactionsSuite executes the reaction contract suite.
func RunReactionsSuite(t *testing.T, h ReactionsHarness) {
	newRM := func(t *testing.T) provider.ReactionManager {
		srv := reactionStubServer(h.ListResponse, h.CreateResponse)
		t.Cleanup(srv.Close)
		p := h.NewProvider(t, provider.Config{Platform: h.Platform, BaseURL: srv.URL, Token: "test"})
		rm, ok := p.(provider.ReactionManager)
		if !ok {
			t.Fatalf("%s does not implement provider.ReactionManager", h.Name)
		}
		return rm
	}

	t.Run("ListIssueReactions_ReturnsResults", func(t *testing.T) {
		rm := newRM(t)
		reactions, err := rm.ListIssueReactions(context.Background(), "owner", "repo", "1")
		if err != nil {
			t.Fatalf("ListIssueReactions: %v", err)
		}
		if len(reactions) == 0 {
			t.Fatal("expected at least one reaction")
		}
		if reactions[0].Emoji == "" {
			t.Error("expected reaction emoji to be non-empty")
		}
	})

	t.Run("AddIssueReaction_ReturnsReaction", func(t *testing.T) {
		rm := newRM(t)
		r, err := rm.AddIssueReaction(context.Background(), "owner", "repo", "1", provider.ReactionHeart)
		if err != nil {
			t.Fatalf("AddIssueReaction: %v", err)
		}
		if r == nil {
			t.Fatal("expected a reaction, got nil")
		}
		if r.Emoji != provider.ReactionHeart {
			t.Errorf("expected emoji %q, got %q", provider.ReactionHeart, r.Emoji)
		}
	})

	t.Run("RemoveIssueReaction_Succeeds", func(t *testing.T) {
		rm := newRM(t)
		if err := rm.RemoveIssueReaction(context.Background(), "owner", "repo", "1", 1); err != nil {
			t.Fatalf("RemoveIssueReaction: %v", err)
		}
	})
}

// reactionStubServer returns a mock server for the reaction suite.
func reactionStubServer(listResponse, createResponse string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(listResponse))
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(createResponse))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}
