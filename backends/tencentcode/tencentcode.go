// Package tencentcode implements the Tencent 工蜂 Provider for the
// git-platform-sdk.
//
// Tencent 工蜂 exposes a GitLab-compatible REST API with some platform-
// specific extensions (native code reviews, branch protection, repository
// tree/blob). This package implements the cross-platform provider.Provider
// interface and additionally exposes a TencentCodeExtras interface for the
// platform-specific capabilities.
package tencentcode

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	gongfeng "github.com/studyzy/gongfeng-sdk-go"
	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// Provider is the Tencent 工蜂 implementation of provider.Provider.
type Provider struct {
	client    *gongfeng.Client
	transport *transport.Client
	logger    provider.Logger
}

// New builds a Tencent Code Provider from the given config.
func New(cfg provider.Config) (provider.Provider, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = provider.NewNoopLogger()
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://git.code.tencent.com"
	}

	// Build a transport.Client so we can leverage the retry/hooks/auth pipeline.
	transportClient := transport.NewClient(baseURL+"/api/v3", transport.PrivateToken{Token: cfg.Token})
	transportClient.Logger = toTransportLogger(logger)
	transportClient.Timeout = 30 * time.Second
	if cfg.SkipTLS {
		transportClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
				MaxVersion:         tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				},
			},
		}
	}
	if cfg.RetryConfig != nil {
		rc := transport.RetryConfig{
			MaxAttempts: cfg.RetryConfig.MaxRetries + 1,
			BaseDelay:   cfg.RetryConfig.BaseDelay,
			MaxDelay:    cfg.RetryConfig.MaxDelay,
			Statuses:    cfg.RetryConfig.RetryOn,
		}
		transportClient.Retry = &rc
	}
	if cfg.Hooks != nil {
		transportClient.Hooks = convertHooks(cfg.Hooks)
	}

	// Build an *http.Client whose Transport uses the transport layer's
	// RoundTripper (with auth, hooks, and optional retry).
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transportClient.NewRetryingRoundTripper(),
	}

	// Normalize the base URL for the gongfeng SDK (it appends /api/v3/ itself).
	sdkBaseURL := strings.TrimRight(baseURL, "/")
	sdkBaseURL = strings.TrimSuffix(sdkBaseURL, "/api/v3")

	// Create the gongfeng SDK client with the custom HTTP client.
	gfClient, err := gongfeng.NewClient(cfg.Token,
		gongfeng.WithHTTPClient(httpClient),
		gongfeng.WithBaseURL(sdkBaseURL),
	)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformTencentCode, "New", err)
	}

	return &Provider{client: gfClient, transport: transportClient, logger: logger}, nil
}

// toTransportLogger adapts provider.Logger to transport.Logger.
func toTransportLogger(l provider.Logger) transport.Logger {
	if l == nil {
		return nil
	}
	return transport.LoggerFunc{
		DebugFunc: func(msg string, kv ...any) { l.Debug(msg, kv...) },
		InfoFunc:  func(msg string, kv ...any) { l.Info(msg, kv...) },
		WarnFunc:  func(msg string, kv ...any) { l.Warn(msg, kv...) },
		ErrorFunc: func(msg string, kv ...any) { l.Error(msg, kv...) },
	}
}

func convertHooks(h *provider.Hooks) *transport.Hooks {
	if h == nil {
		return nil
	}
	out := &transport.Hooks{}
	for _, rh := range h.Request {
		if rh == nil {
			continue
		}
		rhCopy := rh
		out.AddRequest(func(ctx context.Context, req *http.Request) error {
			_ = rhCopy(ctx, req)
			return nil
		})
	}
	for _, rh := range h.Response {
		if rh == nil {
			continue
		}
		rhCopy := rh
		out.AddResponse(func(ctx context.Context, req *http.Request, resp *http.Response, d time.Duration, err error) {
			rhCopy(ctx, req, resp, d, err)
		})
	}
	return out
}

// sdkError wraps an error as a provider.ProviderError for the TencentCode platform.
func sdkError(op string, err error) error {
	if err == nil {
		return nil
	}
	return provider.Wrap(provider.PlatformTencentCode, op, err)
}

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformTencentCode }

// TestConnection implements provider.Provider.
func (p *Provider) TestConnection(ctx context.Context) (*provider.TestConnectionResult, error) {
	var user struct {
		Username string `json:"username"`
	}
	if err := p.doRequest(ctx, "TestConnection", "GET", "user", nil, &user); err != nil {
		return &provider.TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &provider.TestConnectionResult{
		Connected: true,
		Platform:  string(p.Platform()),
		UserName:  user.Username,
	}
	_, listErr := p.ListRepos(ctx, provider.ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = listErr == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}

// Compile-time guarantee that *Provider satisfies provider.Provider and
// TencentCodeExtras. The extras methods live in extras.go.
var (
	_ provider.Provider = (*Provider)(nil)
	_ TencentCodeExtras = (*Provider)(nil)
)

// doRequest executes a JSON request through the gongfeng SDK client.
// Used by extras.go and diffs.go for endpoints not covered by SDK services.
func (p *Provider) doRequest(ctx context.Context, op, method, path string, body, result any) error {
	req, err := p.client.NewRequest(ctx, method, path, body)
	if err != nil {
		return provider.Wrap(provider.PlatformTencentCode, op, err)
	}
	if _, err := p.client.Do(req, result); err != nil {
		return sdkError(op, err)
	}
	return nil
}

// extractTotalCount returns the total item count from the SDK response,
// falling back to the length of the result slice.
func extractTotalCount(resp *gongfeng.Response, fallback int) int {
	if resp != nil && resp.TotalItems > 0 {
		return resp.TotalItems
	}
	return fallback
}

// pid returns the project identifier in "owner/repo" format for SDK calls.
func pid(owner, repo string) string {
	return owner + "/" + repo
}

// avoid unused-import warnings.
var _ = fmt.Sprintf
