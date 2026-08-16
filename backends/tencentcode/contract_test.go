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
		// Gongfeng label shape (GitLab-shaped): name-addressed, colors carry
		// a leading '#'.
		Labels: &contracttest.LabelsHarnessConfig{
			ListResponse:   `[{"name":"bug","color":"#4cc917","description":"something broke"}]`,
			MutateResponse: `{"name":"bug","color":"#4cc917","description":"something broke"}`,
		},
		// Gongfeng issue shape (GitLab-shaped): iid/description, state
		// "opened", plain-string labels, milestone keyed by id, no web_url
		// (registered). GetResponse carries two labels so label removal has
		// a survivor — removing an issue's last label is a registered no-op
		// (the empty csv cannot travel on the update body).
		Issues: &contracttest.IssuesHarnessConfig{
			ListResponse:     `[{"id":1,"iid":1,"title":"bug","description":"broke","state":"opened","author":{"id":1,"username":"dev","name":"Developer"},"labels":["bug","enhancement"],"milestone":{"id":1,"title":"v1"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			GetResponse:      `{"id":1,"iid":1,"title":"bug","description":"broke","state":"opened","author":{"id":1,"username":"dev","name":"Developer"},"labels":["bug","enhancement"],"milestone":{"id":1,"title":"v1"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			MutateResponse:   `{"id":1,"iid":1,"title":"bug","description":"broke","state":"opened","author":{"id":1,"username":"dev","name":"Developer"},"labels":["bug","enhancement"],"milestone":{"id":1,"title":"v1"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			CommentsResponse: `[{"id":1,"body":"a comment","author":{"id":1,"username":"dev","name":"Developer"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			LabelsResponse:   `[{"name":"bug","color":"#4cc917","description":"something broke"}]`,
		},
		// Gongfeng review-note shape (GitLab-shaped notes carrying the
		// review): id/body/author/created_at with system bookkeeping notes
		// mixed into lists (filtered out by ListReviews). The note model
		// carries no state field, so reads normalize to commented
		// (registered) and the List subtest asserts that; create verdicts
		// travel as reviewer_state — not under the suite's "event" key — so
		// CreateEvent stays empty. RequestReviewers is a registered ignore
		// and DismissReview a registered stub, both flagged.
		Reviews: &contracttest.ReviewsHarnessConfig{
			ListResponse:            `[{"id":1,"body":"looks good","author":{"id":1,"username":"dev","name":"Developer"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","system":false},{"id":2,"body":"milestone removed","author":{"id":1,"username":"dev","name":"Developer"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","system":true}]`,
			GetResponse:             `{"id":1,"body":"looks good","author":{"id":1,"username":"dev","name":"Developer"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","system":false}`,
			MutateResponse:          `{"id":1,"body":"looks good","author":{"id":1,"username":"dev","name":"Developer"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","system":false}`,
			IgnoresRequestReviewers: true,
			IgnoresDismissal:        true,
			ListStateIsCommented:    true,
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
