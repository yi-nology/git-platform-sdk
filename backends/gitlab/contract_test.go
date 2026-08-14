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
	})
}

// TestGitLab_LabelsContract runs the label-management contract suite against
// the GitLab backend.
func TestGitLab_LabelsContract(t *testing.T) {
	contracttest.RunLabelsSuite(t, contracttest.LabelsHarness{
		Name:     "GitLab",
		Platform: provider.PlatformGitLab,
		NewProvider: func(t *testing.T, cfg provider.Config) provider.Provider {
			p, err := provider.NewProvider(cfg)
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			return p
		},
		ListResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
		MutateResponse: `{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}`,
	})
}
