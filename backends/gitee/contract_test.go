package gitee_test

import (
	"testing"

	"github.com/yi-nology/git-platform-sdk/backends/contracttest"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func TestGitee_Contract(t *testing.T) {
	contracttest.Run(t, contracttest.Harness{
		Name:     "Gitee",
		Platform: provider.PlatformGitee,
		NewProvider: func(t *testing.T, baseURL string) provider.Provider {
			p, err := provider.NewProvider(provider.Config{
				Platform: provider.PlatformGitee,
				BaseURL:  baseURL,
				Token:    "test",
			})
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			return p
		},
		EmptyListResponse:    `[]`,
		NonEmptyListResponse: `[{"id":1,"name":"repo","full_name":"owner/repo","owner":{"login":"owner"},"default_branch":"main"}]`,
	})
}
