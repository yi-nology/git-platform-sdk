package gitea_test

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/yi-nology/git-platform-sdk/backends/contracttest"
	"github.com/yi-nology/git-platform-sdk/backends/gitea"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func TestGitea_Contract(t *testing.T) {
	contracttest.Run(t, contracttest.Harness{
		Name:     "Gitea",
		Platform: provider.PlatformGitea,
		NewProvider: func(t *testing.T, baseURL string) provider.Provider {
			// The Gitea SDK requires /api/v1/version on client init. We
			// build a reverse proxy that intercepts the version endpoint
			// and forwards every other request to baseURL.
			wrapper := newVersionProxy(baseURL, `{"version":"1.22.0"}`)
			t.Cleanup(wrapper.Close)
			p, err := gitea.New(provider.Config{
				Platform: provider.PlatformGitea,
				BaseURL:  wrapper.URL,
				Token:    "test",
			})
			if err != nil {
				t.Fatalf("gitea.New: %v", err)
			}
			return p
		},
		EmptyListResponse:    "[]",
		NonEmptyListResponse: `[{"id":1,"full_name":"owner/repo","name":"repo","owner":{"username":"owner"},"default_branch":"main"}]`,
	})
}

// newVersionProxy returns a test server that responds to /api/v1/version with
// versionBody and reverse-proxies every other path to baseURL.
func newVersionProxy(baseURL, versionBody string) *httptest.Server {
	target, _ := url.Parse(baseURL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(versionBody))
	})
	mux.HandleFunc("/", proxy.ServeHTTP)
	return httptest.NewServer(mux)
}
