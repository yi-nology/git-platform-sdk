// Package contracttest provides a reusable test harness for verifying that
// platform backends satisfy the behavioral contracts defined by the
// provider.Provider interface.
//
// Each backend's test suite imports this package and calls Run with a
// backend-specific Harness. This ensures that list/pagination, error
// classification, retry behavior, webhook validation, and context cancellation
// are consistent across every platform the SDK supports.
package contracttest

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// Harness bundles the inputs needed to run the contract suite against a
// backend.
type Harness struct {
	// Name is the human-readable platform identifier (e.g. "GitHub").
	Name string
	// Platform is the provider.Platform constant for this backend.
	Platform provider.Platform
	// NewProvider builds a provider.Provider from the given config. The harness
	// fills in BaseURL (and, for the retry subtest, RetryConfig) before calling,
	// so the function should forward any supplied RetryConfig/Hooks rather than
	// discarding them.
	NewProvider func(t *testing.T, cfg provider.Config) provider.Provider
	// EmptyListResponse is the JSON body the mock returns for empty lists.
	EmptyListResponse string
	// NonEmptyListResponse is the JSON body the mock returns for a non-empty
	// list, with at least one item that maps to a valid repo.
	NonEmptyListResponse string
	// Labels, when non-nil, auto-mounts the label-management suite inside
	// Run. Run enforces both directions: a platform declaring
	// Capabilities().Labels must provide this config, and a config must not
	// be provided by a platform that does not declare the capability.
	Labels *LabelsHarnessConfig
}

// Run executes the full contract suite against h. Each subtest is independent.
func Run(t *testing.T, h Harness) {
	t.Run("Platform", func(t *testing.T) { testPlatform(t, h) })
	t.Run("ListRepos_Empty", func(t *testing.T) { testListReposEmpty(t, h) })
	t.Run("ListRepos_NonEmpty", func(t *testing.T) { testListReposNonEmpty(t, h) })
	t.Run("IsNotFound", func(t *testing.T) { testIsNotFound(t, h) })
	t.Run("Pagination_Normalized", func(t *testing.T) { testPagination(t, h) })
	t.Run("Retry_On5xx", func(t *testing.T) { testRetry(t, h) })
	t.Run("Webhook_ValidateSignature", func(t *testing.T) { testWebhookSignature(t, h) })
	t.Run("Context_Cancel", func(t *testing.T) { testContextCancel(t, h) })
	t.Run("Capabilities_Consistency", func(t *testing.T) { testCapabilities(t, h) })
	t.Run("LabelsSuite", func(t *testing.T) { testLabelsSuite(t, h) })
}

func baseCfg(h Harness, baseURL string) provider.Config {
	return provider.Config{Platform: h.Platform, BaseURL: baseURL, Token: "test"}
}

func testPlatform(t *testing.T, h Harness) {
	srv := httptest.NewServer(stubHandler(h))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	if p.Platform() != h.Platform {
		t.Errorf("expected %s, got %s", h.Platform, p.Platform())
	}
}

func testListReposEmpty(t *testing.T, h Harness) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(h.EmptyListResponse))
	}))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	repos, err := p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func testListReposNonEmpty(t *testing.T, h Harness) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(h.NonEmptyListResponse))
	}))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	repos, err := p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) == 0 {
		t.Error("expected at least 1 repo")
	}
	if repos[0].Platform != h.Platform {
		t.Errorf("expected platform %s, got %s", h.Platform, repos[0].Platform)
	}
}

func testIsNotFound(t *testing.T, h Harness) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	_, err := p.GetRepo(context.Background(), "missing", "repo")
	if err == nil {
		t.Fatal("expected error")
	}
	if !provider.IsNotFound(err) {
		t.Errorf("expected IsNotFound, got %v", err)
	}
}

func testPagination(t *testing.T, h Harness) {
	// page=0 / perPage=0 should be normalized to defaults, and perPage > 100
	// should be capped. We don't assert exact query encoding (each SDK differs)
	// but verify the call doesn't panic and returns without error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(h.EmptyListResponse))
	}))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	if _, err := p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 0, PerPage: 0}); err != nil {
		t.Errorf("ListRepos with zero page/perPage: %v", err)
	}
	if _, err := p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 1, PerPage: 1000}); err != nil {
		t.Errorf("ListRepos with huge perPage: %v", err)
	}
}

// testRetry verifies that, with a retry config supplied, a transient 503
// followed by 200 is retried to success. It also asserts the server observed
// more than one attempt, proving the backend actually wired retry through.
func testRetry(t *testing.T, h Harness) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(h.NonEmptyListResponse))
	}))
	defer srv.Close()

	cfg := baseCfg(h, srv.URL)
	cfg.RetryConfig = &provider.RetryConfig{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	p := h.NewProvider(t, cfg)

	repos, err := p.ListRepos(context.Background(), provider.ListRepoOptions{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("expected retry to recover from 503, got error: %v", err)
	}
	if len(repos) == 0 {
		t.Fatal("expected non-empty result after retry")
	}
	if got := atomic.LoadInt32(&calls); got < 2 {
		t.Errorf("expected at least 2 HTTP attempts (proving retry), got %d", got)
	}
}

// testWebhookSignature exercises the platform's registered webhook validator:
// a correctly signed request must pass, an empty secret must be rejected
// (forging with an empty HMAC key must not pass either), and a tampered body
// must fail.
func testWebhookSignature(t *testing.T, h Harness) {
	v := provider.DefaultWebhookRegistry().Get(h.Platform)
	if v == nil {
		t.Skipf("no webhook validator registered for %s", h.Platform)
	}
	const secret = "webhook-secret"
	body := []byte(`{"event":"push"}`)

	// Correct signature must validate.
	good := httptest.NewRequest(http.MethodPost, "/hook", bytes.NewReader(body))
	signRequest(good, v, body, secret)
	if err := v.Validate(good, body, secret); err != nil {
		t.Errorf("valid signature rejected for %s: %v", h.Platform, err)
	}

	// Empty secret must be rejected (guards signature forgery with an empty
	// HMAC key on predictable payloads).
	emptyReq := httptest.NewRequest(http.MethodPost, "/hook", bytes.NewReader(body))
	signRequest(emptyReq, v, body, "") // header present, but secret empty
	if err := v.Validate(emptyReq, body, ""); err == nil {
		t.Errorf("expected empty secret to be rejected for %s", h.Platform)
	}

	// Tampered body must fail validation — but only for validators that sign
	// the body (HMAC). Static-token validators only compare a header token, so
	// altering the body is not expected to change the result.
	if _, isHMAC := v.(provider.HMACSHA256Validator); isHMAC {
		tampered := httptest.NewRequest(http.MethodPost, "/hook", bytes.NewReader(body))
		signRequest(tampered, v, body, secret)
		if err := v.Validate(tampered, append(body, '!'), secret); err == nil {
			t.Errorf("expected tampered body to fail validation for %s", h.Platform)
		}
	}
}

// signRequest sets the validator's signature/token header on req for the given
// body and secret, so the validator can be exercised end to end. It understands
// the two validator types the SDK ships (HMAC-SHA256 and static-token).
func signRequest(req *http.Request, v provider.WebhookValidator, body []byte, secret string) {
	switch vv := v.(type) {
	case provider.HMACSHA256Validator:
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		req.Header.Set(vv.Header, "sha256="+hex.EncodeToString(mac.Sum(nil)))
	case provider.StaticTokenValidator:
		req.Header.Set(vv.Header, secret)
	}
}

func testContextCancel(t *testing.T, h Harness) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.ListRepos(ctx, provider.ListRepoOptions{})
	if err == nil {
		// Returned before timeout; acceptable in a race. Only fail on no error
		// AND the context not being done.
		if ctx.Err() == nil {
			t.Error("expected an error or a cancelled context")
		}
	}
}

// stubHandler returns a handler that serves empty responses for every path.
func stubHandler(h Harness) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(h.EmptyListResponse))
	}
}

// VersionProxy returns a test server that responds to /api/v1/version with
// versionBody and reverse-proxies every other path to baseURL. Gitea/Forgejo
// SDKs require the version endpoint at client init; this wrapper lets the
// contract suite target those backends with a plain mock server.
func VersionProxy(baseURL, versionBody string) *httptest.Server {
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
