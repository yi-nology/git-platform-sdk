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
		NewProvider: func(t *testing.T, baseURL string) provider.Provider {
			p, err := provider.NewProvider(provider.Config{
				Platform: provider.PlatformTencentCode,
				BaseURL:  baseURL,
				Token:    "test",
			})
			if err != nil {
				t.Fatalf("NewProvider: %v", err)
			}
			return p
		},
		EmptyListResponse:    `[]`,
		NonEmptyListResponse: `[{"id":1,"name":"repo","path_with_namespace":"owner/repo","http_url_to_repo":"https://example.com/owner/repo.git","default_branch":"main","visibility_level":20}]`,
	})
}
