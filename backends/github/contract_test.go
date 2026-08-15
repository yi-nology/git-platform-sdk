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
		Issues: &contracttest.IssuesHarnessConfig{
			ListResponse:     `[{"number":1,"title":"bug","state":"open","user":{"login":"dev"},"milestone":{"number":1,"title":"v1"},"labels":[{"id":1,"name":"bug","color":"4cc917"}],"html_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			GetResponse:      `{"number":1,"title":"bug","state":"open","user":{"login":"dev"},"milestone":{"number":1,"title":"v1"},"html_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			MutateResponse:   `{"number":1,"title":"bug","state":"open","user":{"login":"dev"},"milestone":{"number":1,"title":"v1"},"html_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			CommentsResponse: `[{"id":1,"body":"a comment","user":{"login":"dev"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			LabelsResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
		},
		Reviews: &contracttest.ReviewsHarnessConfig{
			ListResponse:   `[{"id":1,"user":{"login":"dev"},"state":"APPROVED","body":"looks good","submitted_at":"2026-01-01T00:00:00Z","html_url":"https://example.com/pull/1#review-1"}]`,
			GetResponse:    `{"id":1,"user":{"login":"dev"},"state":"APPROVED","body":"looks good","submitted_at":"2026-01-01T00:00:00Z","html_url":"https://example.com/pull/1#review-1"}`,
			MutateResponse: `{"id":1,"user":{"login":"dev"},"state":"APPROVED","body":"looks good","submitted_at":"2026-01-01T00:00:00Z","html_url":"https://example.com/pull/1#review-1"}`,
		},
		// GitHub milestone shape: github-like keys (number/state/due_on);
		// milestones are number-addressed.
		Milestones: &contracttest.MilestonesHarnessConfig{
			ListResponse:   `[{"number":1,"title":"v1","state":"open","description":"first","due_on":"2026-01-01T00:00:00Z"}]`,
			MutateResponse: `{"number":1,"title":"v1","state":"open","description":"first","due_on":"2026-01-01T00:00:00Z"}`,
		},
		Releases: &contracttest.ReleasesHarnessConfig{
			ByTagResponse:  `{"id":1,"tag_name":"v1.0.0","name":"v1.0.0","body":"release notes","draft":false,"prerelease":false,"html_url":"https://example.com/releases/v1.0.0","created_at":"2026-01-01T00:00:00Z","published_at":"2026-01-01T00:00:00Z"}`,
			UpdateResponse: `{"id":1,"tag_name":"v1.0.0","name":"v1.0.0-renamed","body":"updated notes","draft":false,"prerelease":false,"html_url":"https://example.com/releases/v1.0.0","created_at":"2026-01-01T00:00:00Z","published_at":"2026-01-01T00:00:00Z"}`,
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
