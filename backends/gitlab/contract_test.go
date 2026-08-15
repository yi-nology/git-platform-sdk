package gitlab_test

import (
	"testing"

	"github.com/yi-nology/git-platform-sdk/backends/contracttest"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func TestGitLab_Contract(t *testing.T) {
	contracttest.Run(t, contracttest.Harness{
		Name:     "GitLab",
		Platform: provider.PlatformGitLab,
		NewProvider: func(t *testing.T, cfg provider.Config) provider.Provider {
			p, err := provider.NewProvider(cfg)
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			return p
		},
		EmptyListResponse:    "[]",
		NonEmptyListResponse: `[{"id":1,"name":"repo","path_with_namespace":"owner/repo","http_url_to_repo":"https://gitlab.com/owner/repo.git","default_branch":"main","visibility":"public"}]`,
		Labels: &contracttest.LabelsHarnessConfig{
			ListResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
			MutateResponse: `{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}`,
		},
		Issues: &contracttest.IssuesHarnessConfig{
			// GitLab-shaped fixtures: iid/author/description/web_url instead of
			// github's number/user/body/html_url, plain-string labels, milestone
			// keyed by id, and state "opened" (the backend maps opened→open).
			ListResponse:     `[{"id":1,"iid":1,"title":"bug","state":"opened","author":{"id":1,"username":"dev","name":"dev"},"milestone":{"id":1,"title":"v1"},"labels":["bug"],"web_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			GetResponse:      `{"id":1,"iid":1,"title":"bug","state":"opened","author":{"id":1,"username":"dev","name":"dev"},"milestone":{"id":1,"title":"v1"},"labels":["bug"],"web_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			MutateResponse:   `{"id":1,"iid":1,"title":"bug","state":"opened","author":{"id":1,"username":"dev","name":"dev"},"milestone":{"id":1,"title":"v1"},"labels":["bug"],"web_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			CommentsResponse: `[{"id":1,"body":"a comment","author":{"id":1,"username":"dev","name":"dev"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			LabelsResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
		},
		Reviews: &contracttest.ReviewsHarnessConfig{
			// GitLab approval-state shape: ListReviews/GetReview hit the
			// approval_state endpoint whose rules[].approved_by entries
			// (BasicUser) become synthesized single "approved" reviews keyed
			// by the MR IID. There is no per-review GET on GitLab, so
			// GetResponse mirrors ListResponse (the suite's /reviews/{id}
			// route is never hit).
			ListResponse: `{"approval_rules_overwritten":false,"rules":[{"id":1,"name":"All Eligible Users","rule_type":"any_approver","approvals_required":1,"approved_by":[{"id":1,"username":"dev","name":"dev","state":"active"}],"approved":true}]}`,
			GetResponse:  `{"approval_rules_overwritten":false,"rules":[{"id":1,"name":"All Eligible Users","rule_type":"any_approver","approvals_required":1,"approved_by":[{"id":1,"username":"dev","name":"dev","state":"active"}],"approved":true}]}`,
			// CreateReview posts a merge-request note; MutateResponse is the
			// created Note (also served, and ignored, to the unapprove POST).
			MutateResponse: `{"id":1,"body":"looks good","author":{"id":1,"username":"dev","name":"dev"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			// RequestReviewers is a registered ignore on GitLab (reviewer_ids
			// needs username→ID resolution the SDK surface does not offer), so
			// the wire subtest only asserts a silent no-op.
			IgnoresRequestReviewers: true,
		},
		// GitLab release shape: name/description/_links.self (released_at is
		// the publish timestamp). Update is a tag-addressed PUT carrying
		// name/description; Draft/Prerelease have no GitLab counterpart
		// (registered ignore in the backend).
		Releases: &contracttest.ReleasesHarnessConfig{
			ByTagResponse:  `{"tag_name":"v1.0.0","name":"v1.0.0","description":"release notes","created_at":"2026-01-01T00:00:00Z","released_at":"2026-01-01T00:00:00Z","_links":{"self":"https://gitlab.example.com/owner/repo/-/releases/v1.0.0"}}`,
			UpdateResponse: `{"tag_name":"v1.0.0","name":"v1.0.0-renamed","description":"updated notes","created_at":"2026-01-01T00:00:00Z","released_at":"2026-01-01T00:00:00Z","_links":{"self":"https://gitlab.example.com/owner/repo/-/releases/v1.0.0"}}`,
		},
	})
}
