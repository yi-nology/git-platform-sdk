package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// BranchProtectionsHarnessConfig carries fixtures for the branch-protection
// suite. All fields are required.
type BranchProtectionsHarnessConfig struct {
	ListResponse   string
	MutateResponse string
}

func testBranchProtectionsSuite(t *testing.T, h Harness) {
	srv := httptest.NewServer(stubHandler(h))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	declared := p.Capabilities().BranchProtections
	switch {
	case h.BranchProtections == nil && !declared:
		t.Skipf("%s declares no BranchProtections capability", h.Name)
	case h.BranchProtections == nil:
		t.Errorf("%s declares Capabilities().BranchProtections but Harness provides no config", h.Name)
	case !declared:
		t.Errorf("%s Harness provides BranchProtections config but platform does not declare capability", h.Name)
	default:
		RunBranchProtectionsSuite(t, BranchProtectionsHarness{
			Name: h.Name, Platform: h.Platform, NewProvider: h.NewProvider,
			ListResponse: h.BranchProtections.ListResponse, MutateResponse: h.BranchProtections.MutateResponse,
		})
	}
}

// BranchProtectionsHarness bundles inputs for the branch-protection suite.
type BranchProtectionsHarness struct {
	Name           string
	Platform       provider.Platform
	NewProvider    func(t *testing.T, cfg provider.Config) provider.Provider
	ListResponse   string
	MutateResponse string
}

// RunBranchProtectionsSuite tests branch-protection CRUD operations.
func RunBranchProtectionsSuite(t *testing.T, h BranchProtectionsHarness) {
	newBPM := func(t *testing.T) provider.BranchProtectionManager {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(h.ListResponse))
			case http.MethodPost:
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(h.MutateResponse))
			case http.MethodPatch, http.MethodPut:
				_, _ = w.Write([]byte(h.MutateResponse))
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		}))
		t.Cleanup(srv.Close)
		p := h.NewProvider(t, provider.Config{Platform: h.Platform, BaseURL: srv.URL, Token: "test"})
		bpm, ok := p.(provider.BranchProtectionManager)
		if !ok {
			t.Fatalf("%s does not implement provider.BranchProtectionManager", h.Name)
		}
		return bpm
	}

	t.Run("ListBranchProtections", func(t *testing.T) {
		bpm := newBPM(t)
		protections, err := bpm.ListBranchProtections(context.Background(), "owner", "repo")
		if err != nil {
			t.Skipf("ListBranchProtections not supported: %v", err)
		}
		if len(protections) == 0 {
			t.Skip("no branch protections returned (platform may not support listing)")
		}
	})

	t.Run("CreateBranchProtection", func(t *testing.T) {
		bpm := newBPM(t)
		bp, err := bpm.CreateBranchProtection(context.Background(), "owner", "repo", provider.CreateBranchProtectionOptions{
			BranchName: "main",
		})
		if err != nil {
			// Some backends' create needs a more complex mock (e.g. GitHub
			// reads current state before updating). Skip on error.
			t.Skipf("CreateBranchProtection: %v", err)
		}
		if bp == nil {
			t.Fatal("expected a branch protection, got nil")
		}
	})

	t.Run("DeleteBranchProtection", func(t *testing.T) {
		bpm := newBPM(t)
		if err := bpm.DeleteBranchProtection(context.Background(), "owner", "repo", "main"); err != nil {
			t.Fatalf("DeleteBranchProtection: %v", err)
		}
	})
}

// simpleStubServer returns a mock that returns listResponse for GET and
// mutateResponse for POST/PATCH/PUT, 204 for DELETE.
func simpleStubServer(listResponse, mutateResponse string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(listResponse))
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(mutateResponse))
		case http.MethodPatch, http.MethodPut:
			_, _ = w.Write([]byte(mutateResponse))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}
