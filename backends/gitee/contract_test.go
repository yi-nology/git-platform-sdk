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
			ListResponse:   `[{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}]`,
			MutateResponse: `{"id":1,"name":"bug","color":"#4cc917","description":"something broke"}`,
		},
		// issues.go implements IssueManager, but the capability stays
		// undeclared: live Gitee returns alphanumeric string issue numbers
		// (e.g. "IAINVA") the int-typed interface cannot address. See
		// Capabilities in gitee.go.
		IssuesImplementedButUndeclared: true,
	})
}
