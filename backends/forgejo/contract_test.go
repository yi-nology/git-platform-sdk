package forgejo_test

import (
	"testing"

	"github.com/yi-nology/git-platform-sdk/backends/contracttest"
	"github.com/yi-nology/git-platform-sdk/backends/forgejo"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func TestForgejo_Contract(t *testing.T) {
	contracttest.Run(t, contracttest.Harness{
		Name:     "Forgejo",
		Platform: provider.PlatformForgejo,
		NewProvider: func(t *testing.T, cfg provider.Config) provider.Provider {
			wrapper := contracttest.VersionProxy(cfg.BaseURL, `{"version":"8.0.0"}`)
			t.Cleanup(wrapper.Close)
			cfg.BaseURL = wrapper.URL
			p, err := forgejo.New(cfg)
			if err != nil {
				t.Fatalf("forgejo.New: %v", err)
			}
			return p
		},
		EmptyListResponse:    "[]",
		NonEmptyListResponse: `[{"id":1,"full_name":"owner/repo","name":"repo","owner":{"username":"owner"},"default_branch":"main"}]`,
	})
}

// TestForgejo_LabelsContract runs the label-management contract suite
// against the Forgejo backend.
func TestForgejo_LabelsContract(t *testing.T) {
	contracttest.RunLabelsSuite(t, contracttest.LabelsHarness{
		Name:     "Forgejo",
		Platform: provider.PlatformForgejo,
		NewProvider: func(t *testing.T, cfg provider.Config) provider.Provider {
			wrapper := contracttest.VersionProxy(cfg.BaseURL, `{"version":"8.0.0"}`)
			t.Cleanup(wrapper.Close)
			cfg.BaseURL = wrapper.URL
			p, err := forgejo.New(cfg)
			if err != nil {
				t.Fatalf("forgejo.New: %v", err)
			}
			return p
		},
		ListResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
		MutateResponse: `{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}`,
	})
}
