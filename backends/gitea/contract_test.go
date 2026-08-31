package gitea_test

import (
	"testing"

	"github.com/yi-nology/git-platform-sdk/backends/contracttest"
	"github.com/yi-nology/git-platform-sdk/backends/gitea"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func TestGitea_Contract(t *testing.T) {
	contracttest.Run(t, contracttest.Harness{
		Name:     "Gitea",
		Platform: provider.PlatformGitea,
		NewProvider: func(t *testing.T, cfg provider.Config) provider.Provider {
			// The Gitea SDK requires /api/v1/version on client init. Wrap the
			// mock server so the version endpoint is intercepted and every
			// other request is reverse-proxied through.
			wrapper := contracttest.VersionProxy(cfg.BaseURL, `{"version":"1.22.0"}`)
			t.Cleanup(wrapper.Close)
			cfg.BaseURL = wrapper.URL
			p, err := gitea.New(cfg)
			if err != nil {
				t.Fatalf("gitea.New: %v", err)
			}
			return p
		},
		EmptyListResponse:    "[]",
		NonEmptyListResponse: `[{"id":1,"full_name":"owner/repo","name":"repo","owner":{"username":"owner"},"default_branch":"main"}]`,
		Labels: &contracttest.LabelsHarnessConfig{
			ListResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
			MutateResponse: `{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}`,
		},
		// Gitea-shaped fixtures: github-like keys (number/user/state/html_url)
		// except the milestone, which gitea keys by id, not number.
		Issues: &contracttest.IssuesHarnessConfig{
			ListResponse:     `[{"number":1,"title":"bug","state":"open","user":{"login":"dev"},"milestone":{"id":1,"title":"v1"},"labels":[{"id":1,"name":"bug","color":"#4cc917"}],"html_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			GetResponse:      `{"number":1,"title":"bug","state":"open","user":{"login":"dev"},"milestone":{"id":1,"title":"v1"},"html_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			MutateResponse:   `{"number":1,"title":"bug","state":"open","user":{"login":"dev"},"milestone":{"id":1,"title":"v1"},"html_url":"https://example.com/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`,
			CommentsResponse: `[{"id":1,"body":"a comment","user":{"login":"dev"},"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`,
			LabelsResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
		},
		// Gitea PullReview shape: github-like keys with UPPERCASE wire
		// states ("APPROVED"/"REQUEST_CHANGES"/"COMMENT"/"PENDING").
		Reviews: &contracttest.ReviewsHarnessConfig{
			ListResponse:   `[{"id":1,"user":{"id":7,"login":"dev"},"state":"APPROVED","body":"looks good","submitted_at":"2026-01-01T00:00:00Z","html_url":"https://example.com/pull/1#review-1"}]`,
			GetResponse:    `{"id":1,"user":{"id":7,"login":"dev"},"state":"APPROVED","body":"looks good","submitted_at":"2026-01-01T00:00:00Z","html_url":"https://example.com/pull/1#review-1"}`,
			MutateResponse: `{"id":1,"user":{"id":7,"login":"dev"},"state":"APPROVED","body":"looks good","submitted_at":"2026-01-01T00:00:00Z","html_url":"https://example.com/pull/1#review-1"}`,
			// CreateReview maps the APPROVE verdict to the gitea SDK's
			// ReviewStateApproved ("APPROVED"), posted under the "event" key
			// (CreatePullReviewOptions.State json tag).
			CreateEvent: "APPROVED",
		},
		// Gitea milestone shape: keyed by id (milestones are ID-addressed),
		// github-like state/due_on keys.
		Milestones: &contracttest.MilestonesHarnessConfig{
			ListResponse:   `[{"id":1,"title":"v1","state":"open","description":"first","due_on":"2026-01-01T00:00:00Z"}]`,
			MutateResponse: `{"id":1,"title":"v1","state":"open","description":"first","due_on":"2026-01-01T00:00:00Z"}`,
		},
		// Gitea release shape: github-like keys (tag_name/name/body/draft/
		// prerelease), url for the release page.
		Releases: &contracttest.ReleasesHarnessConfig{
			ByTagResponse:  `{"id":1,"tag_name":"v1.0.0","name":"v1.0.0","body":"release notes","draft":false,"prerelease":false,"url":"https://gitea.example.com/owner/repo/releases/tag/v1.0.0","created_at":"2026-01-01T00:00:00Z","published_at":"2026-01-01T00:00:00Z"}`,
			UpdateResponse: `{"id":1,"tag_name":"v1.0.0","name":"v1.0.0-renamed","body":"updated notes","draft":false,"prerelease":false,"url":"https://gitea.example.com/owner/repo/releases/tag/v1.0.0","created_at":"2026-01-01T00:00:00Z","published_at":"2026-01-01T00:00:00Z"}`,
		},
		// Gitea search: repo and user results wrap items in {"data":[..]}
		// (the /repos/search and /users/search envelopes); issue search
		// (/repos/issues/search) is a bare array keyed by number.
		Search: &contracttest.SearchHarnessConfig{
			ReposResponse:  `{"ok":true,"data":[{"id":1,"full_name":"owner/repo","description":"search hit","html_url":"https://gitea.example.com/owner/repo","stars_count":5,"forks_count":2,"default_branch":"main","private":false,"owner":{"login":"owner"}}]}`,
			IssuesResponse: `[{"number":1,"title":"found","state":"open","body":"b","html_url":"https://gitea.example.com/owner/repo/issues/1","labels":[{"name":"bug"}],"comments":2,"created_at":"2026-01-01T00:00:00Z","repository":{"full_name":"owner/repo"}}]`,
			UsersResponse:  `{"ok":true,"data":[{"login":"dev","full_name":"Dev","avatar_url":"https://gitea.example.com/avatars/1","html_url":"https://gitea.example.com/dev"}]}`,
		},
		// CommitStatus is zero-config: the suite self-drives against the
		// recording server and asserts a single status-reporting request.
		CommitStatus: &contracttest.CommitStatusHarnessConfig{},
		Notifications: &contracttest.NotificationsHarnessConfig{
			ListResponse: `[{"id":1,"unread":true,"subject":{"title":"Bug report","type":"Issue","url":"https://gitea.com/api/v1/repos/owner/repo/issues/1"},"repository":{"id":1,"full_name":"owner/repo"},"updated_at":"2026-01-01T00:00:00Z"}]`,
		},
		Reactions: &contracttest.ReactionsHarnessConfig{
			ListResponse:   `[{"content":"+1","user":{"id":1,"login":"dev"}}]`,
			CreateResponse: `{"content":"heart","user":{"id":1,"login":"dev"}}`,
		},
	})
}
