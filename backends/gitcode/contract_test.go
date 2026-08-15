package gitcode_test

import (
	"testing"

	"github.com/yi-nology/git-platform-sdk/backends/contracttest"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func TestGitCode_Contract(t *testing.T) {
	contracttest.Run(t, contracttest.Harness{
		Name:     "GitCode",
		Platform: provider.PlatformGitCode,
		NewProvider: func(t *testing.T, cfg provider.Config) provider.Provider {
			p, err := provider.NewProvider(cfg)
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			return p
		},
		EmptyListResponse:    `[]`,
		NonEmptyListResponse: `[{"id":1,"name":"repo","full_name":"owner/repo","owner":{"login":"owner"},"default_branch":"main"}]`,
		Labels: &contracttest.LabelsHarnessConfig{
			ListResponse:   `[{"id":1,"name":"bug","color":"#4cc917"}]`,
			MutateResponse: `{"id":1,"name":"bug","color":"#4cc917"}`,
			// The gitcode SDK's ListIssueLabels exposes no page/page-size
			// parameters, so the backend cannot forward pagination on the wire.
			IgnoresListPagination: true,
		},
		// GitCode's issue payloads are GitHub-shaped; milestones are
		// addressed by id, and labels carry '#'-prefixed colors.
		Issues: &contracttest.IssuesHarnessConfig{
			ListResponse:     `[{"number":1,"title":"bug","state":"open","user":{"login":"dev"},"milestone":{"id":1,"title":"v1"},"labels":[{"id":1,"name":"bug","color":"#4cc917"}],"html_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			GetResponse:      `{"number":1,"title":"bug","state":"open","user":{"login":"dev"},"milestone":{"id":1,"title":"v1"},"html_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			MutateResponse:   `{"number":1,"title":"bug","state":"open","user":{"login":"dev"},"milestone":{"id":1,"title":"v1"},"html_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			CommentsResponse: `[{"id":1,"body":"a comment","user":{"login":"dev"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			LabelsResponse:   `[{"id":1,"name":"bug","color":"#4cc917"}]`,
		},
		// GitCode review payloads are GitHub-shaped; the review author rides
		// the "user" (fallback "author") key and the timestamp is created_at.
		Reviews: &contracttest.ReviewsHarnessConfig{
			ListResponse:   `[{"id":1,"user":{"login":"dev"},"state":"APPROVED","body":"looks good","created_at":"2026-01-01T00:00:00Z"}]`,
			GetResponse:    `{"id":1,"user":{"login":"dev"},"state":"APPROVED","body":"looks good","created_at":"2026-01-01T00:00:00Z"}`,
			MutateResponse: `{"id":1,"user":{"login":"dev"},"state":"APPROVED","body":"looks good","created_at":"2026-01-01T00:00:00Z"}`,
		},
		// GitCode release payloads are GitHub-shaped (id/tag_name/name/body/
		// draft/prerelease/html_url).
		Releases: &contracttest.ReleasesHarnessConfig{
			ByTagResponse:  `{"id":1,"tag_name":"v1.0.0","name":"v1.0.0","body":"release notes","draft":false,"prerelease":false,"html_url":"https://gitcode.com/owner/repo/releases/v1.0.0","created_at":"2026-01-01T00:00:00Z","published_at":"2026-01-01T00:00:00Z"}`,
			UpdateResponse: `{"id":1,"tag_name":"v1.0.0","name":"v1.0.0-renamed","body":"updated notes","draft":false,"prerelease":false,"html_url":"https://gitcode.com/owner/repo/releases/v1.0.0","created_at":"2026-01-01T00:00:00Z","published_at":"2026-01-01T00:00:00Z"}`,
		},
	})
}
