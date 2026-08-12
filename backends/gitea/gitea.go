// Package gitea implements the Gitea Provider for the git-platform-sdk.
//
// It builds on top of the official code.gitea.io/sdk/gitea SDK and adds
// transport-layer cross-cutting behavior (auth, retry, hooks, logging)
// provided by the parent project's transport package.
package gitea

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// Provider is the Gitea implementation of provider.Provider.
type Provider struct {
	client *gitea.Client
	logger provider.Logger
}

// New builds a Gitea Provider from the given config.
func New(cfg provider.Config) (provider.Provider, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = provider.NewNoopLogger()
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://gitea.com"
	}
	baseURL = normalizeBaseURL(baseURL)

	transportClient := transport.NewClient(baseURL, transport.TokenHeader{Token: cfg.Token})
	transportClient.Logger = logger
	// Set TLS-skipping transport on the transport client so that all
	// HTTP requests (including retries) honour SkipTLS.
	if cfg.SkipTLS {
		transportClient.Transport = httpTransport(cfg.SkipTLS)
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

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: chainTransport(
			httpTransport(cfg.SkipTLS),
			transportClient.NewRetryingRoundTripper(),
		),
	}

	client, err := gitea.NewClient(baseURL, gitea.SetToken(cfg.Token), gitea.SetHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("gitea: failed to create client: %w", err)
	}
	return &Provider{client: client, logger: logger}, nil
}

// normalizeBaseURL strips any trailing slash and the /api/v1 suffix from
// base URLs. The gitea SDK re-adds the suffix internally.
func normalizeBaseURL(base string) string {
	base = strings.TrimSuffix(base, "/")
	base = strings.TrimSuffix(base, "/api/v1")
	return base
}

func httpTransport(skipTLS bool) http.RoundTripper {
	if !skipTLS {
		return http.DefaultTransport
	}
	return &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
}

func chainTransport(inner, outer http.RoundTripper) http.RoundTripper {
	return &chainedTransport{inner: inner, outer: outer}
}

type chainedTransport struct {
	inner http.RoundTripper
	outer http.RoundTripper
}

func (c *chainedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if c.outer != nil {
		if resp, err := c.outer.RoundTrip(req); resp != nil || err != nil {
			return resp, err
		}
	}
	return c.inner.RoundTrip(req)
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

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformGitea }

// TestConnection implements provider.Provider.
func (p *Provider) TestConnection(ctx context.Context) (*provider.TestConnectionResult, error) {
	user, _, err := p.client.GetMyUserInfo()
	if err != nil {
		return &provider.TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &provider.TestConnectionResult{
		Connected: true,
		Platform:  string(p.Platform()),
		UserName:  user.UserName,
	}
	_, err = p.ListRepos(ctx, provider.ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}
