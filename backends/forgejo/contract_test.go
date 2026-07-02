package forgejo_test

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/yi-nology/git-platform-sdk/backends/contracttest"
	"github.com/yi-nology/git-platform-sdk/backends/forgejo"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func TestForgejo_Contract(t *testing.T) {
	contracttest.Run(t, contracttest.Harness{
		Name:     "Forgejo",
		Platform: provider.PlatformForgejo,
		NewProvider: func(t *testing.T, baseURL string) provider.Provider {
			wrapper := newVersionProxy(baseURL, `{"version":"8.0.0"}`)
			t.Cleanup(wrapper.Close)
			p, err := forgejo.New(provider.Config{
				Platform: provider.PlatformForgejo,
				BaseURL:  wrapper.URL,
				Token:    "test",
			})
			if err != nil {
				t.Fatalf("forgejo.New: %v", err)
			}
			return p
		},
		EmptyListResponse:    "[]",
		NonEmptyListResponse: `[{"id":1,"full_name":"owner/repo","name":"repo","owner":{"username":"owner"},"default_branch":"main"}]`,
	})
}

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
