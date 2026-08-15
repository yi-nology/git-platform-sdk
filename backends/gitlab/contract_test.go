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
	})
}
