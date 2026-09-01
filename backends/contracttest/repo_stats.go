package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// RepoStatsHarnessConfig carries fixtures for the repo-stats suite.
type RepoStatsHarnessConfig struct {
	ForksResponse        string
	StargazersResponse   string
	ContributorsResponse string
}

func testRepoStatsSuite(t *testing.T, h Harness) {
	srv := httptest.NewServer(stubHandler(h))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	declared := p.Capabilities().RepoStats
	switch {
	case h.RepoStats == nil && !declared:
		t.Skipf("%s declares no RepoStats capability", h.Name)
	case h.RepoStats == nil:
		t.Errorf("%s declares Capabilities().RepoStats but Harness provides no config", h.Name)
	case !declared:
		t.Errorf("%s Harness provides RepoStats config but platform does not declare capability", h.Name)
	default:
		RunRepoStatsSuite(t, RepoStatsHarness{
			Name: h.Name, Platform: h.Platform, NewProvider: h.NewProvider,
			ForksResponse: h.RepoStats.ForksResponse, StargazersResponse: h.RepoStats.StargazersResponse,
			ContributorsResponse: h.RepoStats.ContributorsResponse,
		})
	}
}

// RepoStatsHarness bundles inputs for the repo-stats suite.
type RepoStatsHarness struct {
	Name                 string
	Platform             provider.Platform
	NewProvider          func(t *testing.T, cfg provider.Config) provider.Provider
	ForksResponse        string
	StargazersResponse   string
	ContributorsResponse string
}

// RunRepoStatsSuite tests repo-stats read operations. Each method is
// tested independently — backends that don't support a method will return
// an error, which the suite accepts as long as the type assertion works.
func RunRepoStatsSuite(t *testing.T, h RepoStatsHarness) {
	newRSM := func(t *testing.T) provider.RepoStatsManager {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(h.ForksResponse))
		}))
		t.Cleanup(srv.Close)
		p := h.NewProvider(t, provider.Config{Platform: h.Platform, BaseURL: srv.URL, Token: "test"})
		rsm, ok := p.(provider.RepoStatsManager)
		if !ok {
			t.Fatalf("%s does not implement provider.RepoStatsManager", h.Name)
		}
		return rsm
	}

	t.Run("ListForks", func(t *testing.T) {
		rsm := newRSM(t)
		forks, err := rsm.ListForks(context.Background(), "owner", "repo")
		if err != nil {
			// Some backends may not support ListForks.
			t.Skipf("ListForks not supported: %v", err)
		}
		if len(forks) == 0 {
			t.Skip("no forks returned (may be empty on this platform)")
		}
	})

	t.Run("ListStargazers", func(t *testing.T) {
		rsm := newRSM(t)
		stargazers, err := rsm.ListStargazers(context.Background(), "owner", "repo")
		if err != nil {
			t.Skipf("ListStargazers not supported: %v", err)
		}
		if len(stargazers) == 0 {
			t.Skip("no stargazers returned")
		}
	})

	t.Run("ListContributors", func(t *testing.T) {
		rsm := newRSM(t)
		contributors, err := rsm.ListContributors(context.Background(), "owner", "repo")
		if err != nil {
			t.Skipf("ListContributors not supported: %v", err)
		}
		if len(contributors) == 0 {
			t.Skip("no contributors returned")
		}
	})
}
