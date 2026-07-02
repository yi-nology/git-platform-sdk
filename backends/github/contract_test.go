package github_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkgithub "github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/backends/contracttest"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// TestGitHub_Contract runs the cross-platform contract suite against the
// GitHub backend.
func TestGitHub_Contract(t *testing.T) {
	contracttest.Run(t, contracttest.Harness{
		Name:     "GitHub",
		Platform: provider.PlatformGitHub,
		NewProvider: func(t *testing.T, baseURL string) provider.Provider {
			p, err := provider.NewProvider(provider.Config{
				Platform: provider.PlatformGitHub,
				BaseURL:  baseURL + "/api/v3",
				Token:    "test",
			})
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			return p
		},
		EmptyListResponse:    "[]",
		NonEmptyListResponse: githubNonEmptyList(),
	})
}

func githubNonEmptyList() string {
	repos := []*sdkgithub.Repository{
		{
			ID:       sdkgithub.Ptr(int64(1)),
			FullName: sdkgithub.Ptr("owner/repo"),
			Name:     sdkgithub.Ptr("repo"),
			Owner:    &sdkgithub.User{Login: sdkgithub.Ptr("owner")},
		},
	}
	b, _ := json.Marshal(repos)
	return string(b)
}

// Ensure httptest is referenced (used by the harness internally).
var _ = httptest.NewServer
var _ = http.StatusOK
