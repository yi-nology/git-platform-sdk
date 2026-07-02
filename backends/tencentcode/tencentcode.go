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
	"crypto/subtle"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// Provider is the Tencent 工蜂 implementation of provider.Provider.
type Provider struct {
	client *transport.Client
	logger provider.Logger
}

// New builds a Tencent Code Provider from the given config.
func New(cfg provider.Config) (provider.Provider, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = provider.NewNoopLogger()
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://git.code.tencent.com/api/v3"
	}

	c := transport.NewClient(baseURL, transport.PrivateToken{Token: cfg.Token})
	c.Logger = toTransportLogger(logger)
	c.Timeout = 30 * time.Second
	if cfg.SkipTLS {
		c.Transport = &http.Transport{
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
		c.Retry = &rc
	}
	if cfg.Hooks != nil {
		c.Hooks = convertHooks(cfg.Hooks)
	}
	return &Provider{client: c, logger: logger}, nil
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

// encodeProjectPath URL-encodes the "owner/repo" path used by Tencent 工蜂's
// project-scoped endpoints. The API uses the encoded form in the URL rather
// than a path parameter.
func encodeProjectPath(owner, repo string) string {
	return url.PathEscape(owner + "/" + repo)
}

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformTencentCode }

// TestConnection implements provider.Provider.
func (p *Provider) TestConnection(ctx context.Context) (*provider.TestConnectionResult, error) {
	var user struct {
		Username string `json:"username"`
	}
	if err := p.doRequest(ctx, "GET", "/user", nil, &user); err != nil {
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

// doRequest is a tiny convenience wrapper for JSON-in / JSON-out calls.
func (p *Provider) doRequest(ctx context.Context, method, path string, body, result any) error {
	_, err := p.client.DoJSON(ctx, &transport.Request{
		Method: method, Path: path, Body: body, Result: result,
	})
	if err != nil {
		return provider.Wrap(provider.PlatformTencentCode, opFromPath(method, path), err)
	}
	return nil
}

// doRequestWithHeaders is the same as doRequest but returns the response
// headers. Used for paginated endpoints that expose X-Total-Count.
func (p *Provider) doRequestWithHeaders(ctx context.Context, method, path string, body, result any) (http.Header, error) {
	resp, err := p.client.DoJSON(ctx, &transport.Request{
		Method: method, Path: path, Body: body, Result: result,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformTencentCode, opFromPath(method, path), err)
	}
	return resp.Header, nil
}

// doRawRequest executes a request and returns the raw body bytes. Used for
// archive downloads.
func (p *Provider) doRawRequest(ctx context.Context, method, path string) ([]byte, error) {
	resp, err := p.client.Do(ctx, &transport.Request{Method: method, Path: path})
	if err != nil {
		return nil, provider.Wrap(platform(), "doRawRequest", err)
	}
	return resp.Body, nil
}

// opFromPath derives a short operation name from the method+path for error
// wrapping. Example: "GET /projects/owner%2Frepo/merge_requests" ->
// "GET merge_requests".
func opFromPath(method, path string) string {
	// Take the last segment after the final "/".
	last := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		last = path[i+1:]
	}
	// Drop any query string.
	if i := strings.IndexByte(last, '?'); i >= 0 {
		last = last[:i]
	}
	return method + " " + last
}

// platform is a tiny helper so doRawRequest doesn't need to be a method on
// Provider (it is, but the helper avoids repetition).
func platform() provider.Platform { return provider.PlatformTencentCode }

// Compile-time guarantee that *Provider satisfies provider.Provider and
// TencentCodeExtras. The extras methods live in extras.go.
var (
	_ provider.Provider = (*Provider)(nil)
	_ TencentCodeExtras = (*Provider)(nil)
)

// avoid unused-import warnings when subtle is only used in webhook code.
var _ = subtle.ConstantTimeCompare
var _ = fmt.Sprintf
