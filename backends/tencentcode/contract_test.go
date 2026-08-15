package tencentcode_test

import (
	"testing"

	"github.com/yi-nology/git-platform-sdk/backends/contracttest"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func TestTencentCode_Contract(t *testing.T) {
	contracttest.Run(t, contracttest.Harness{
		Name:     "Tencent Code",
		Platform: provider.PlatformTencentCode,
		NewProvider: func(t *testing.T, cfg provider.Config) provider.Provider {
			p, err := provider.NewProvider(cfg)
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			return p
		},
		EmptyListResponse:    `[]`,
		NonEmptyListResponse: `[{"id":1,"name":"repo","path_with_namespace":"owner/repo","http_url_to_repo":"https://example.com/owner/repo.git","default_branch":"main","visibility_level":20}]`,
		// Gongfeng milestone shape (GitLab-shaped): id-addressed, wire state
		// "active", date-only due_date.
		Milestones: &contracttest.MilestonesHarnessConfig{
			ListResponse:   `[{"id":1,"iid":1,"project_id":1,"title":"v1","state":"active","description":"first","due_date":"2026-01-01"}]`,
			MutateResponse: `{"id":1,"iid":1,"project_id":1,"title":"v1","state":"active","description":"first","due_date":"2026-01-01"}`,
		},
		// Gongfeng release shape: tag_name/description only — the model has
		// no name/id/url fields, and the update surface cannot carry a name
		// (registered limitation), so the suite asserts the description key.
		Releases: &contracttest.ReleasesHarnessConfig{
			ByTagResponse:              `{"tag_name":"v1.0.0","description":"release notes","created_at":"2026-01-01T00:00:00Z"}`,
			UpdateResponse:             `{"tag_name":"v1.0.0","description":"updated notes","created_at":"2026-01-01T00:00:00Z"}`,
			UpdateSendsDescriptionOnly: true,
		},
	})
}
