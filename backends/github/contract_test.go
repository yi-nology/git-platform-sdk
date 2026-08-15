package github_test

import (
	"encoding/json"
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
		NewProvider: func(t *testing.T, cfg provider.Config) provider.Provider {
			cfg.BaseURL = cfg.BaseURL + "/api/v3"
			p, err := provider.NewProvider(cfg)
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			return p
		},
		EmptyListResponse:    "[]",
		NonEmptyListResponse: githubNonEmptyList(),
		Labels: &contracttest.LabelsHarnessConfig{
			ListResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
			MutateResponse: `{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}`,
		},
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
