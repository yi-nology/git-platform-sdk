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
	})
}

// TestGitea_LabelsContract runs the label-management contract suite against
// the Gitea backend.
func TestGitea_LabelsContract(t *testing.T) {
	contracttest.RunLabelsSuite(t, contracttest.LabelsHarness{
		Name:     "Gitea",
		Platform: provider.PlatformGitea,
		NewProvider: func(t *testing.T, cfg provider.Config) provider.Provider {
			// The Gitea SDK requires /api/v1/version on client init; wrap
			// the mock server the same way the main harness does.
			wrapper := contracttest.VersionProxy(cfg.BaseURL, `{"version":"1.22.0"}`)
			t.Cleanup(wrapper.Close)
			cfg.BaseURL = wrapper.URL
			p, err := gitea.New(cfg)
			if err != nil {
				t.Fatalf("gitea.New: %v", err)
			}
			return p
		},
		ListResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
		MutateResponse: `{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}`,
	})
}
