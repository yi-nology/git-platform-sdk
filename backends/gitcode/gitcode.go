// Package gitcode implements the GitCode Provider for the git-platform-sdk.
//
// It builds on top of the yi-nology/gitcode_api client SDK and adds
// transport-layer cross-cutting behavior (auth, retry, hooks, logging)
// provided by the parent project's transport package.
package gitcode

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// Provider is the GitCode implementation of provider.Provider.
type Provider struct {
	client *gitcode.Client
	logger provider.Logger
}

// New builds a GitCode Provider from the given config.
func New(cfg provider.Config) (provider.Provider, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = provider.NewNoopLogger()
	}

	transportClient := transport.NewClient(
		defaultBaseURL(cfg.BaseURL),
		transport.BearerToken{Token: cfg.Token},
	)
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

	// Build an http.Client whose transport flows through the unified
	// auth/retry/hooks pipeline, then inject it into the gitcode_api SDK
	// via SetHTTPClient.
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: chainTransport(
			httpTransport(cfg.SkipTLS),
			transportClient.NewRetryingRoundTripper(),
		),
	}

	var client *gitcode.Client
	if cfg.BaseURL == "" {
		client = gitcode.NewClient(cfg.Token)
	} else {
		client = gitcode.NewClientWithBaseURL(cfg.BaseURL, cfg.Token)
	}
	client.SetHTTPClient(httpClient)

	return &Provider{client: client, logger: logger}, nil
}

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformGitCode }

// TestConnection implements provider.Provider.
func (p *Provider) TestConnection(ctx context.Context) (*provider.TestConnectionResult, error) {
	user, err := p.client.GetCurrentUser(ctx)
	if err != nil {
		return &provider.TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &provider.TestConnectionResult{
		Connected: true,
		Platform:  string(p.Platform()),
		UserName:  user.Login,
	}
	_, err = p.client.ListRepositories(ctx, gitcode.ListRepositoriesOptions{
		ListOptions: gitcode.ListOptions{Page: 1, PerPage: 1},
	})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}

func defaultBaseURL(base string) string {
	if base == "" {
		return "https://api.gitcode.com/api/v5"
	}
	return base
}

func httpTransport(skipTLS bool) http.RoundTripper {
	if !skipTLS {
		return http.DefaultTransport
	}
	return &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // G402: user explicitly opted into SkipTLS
}

// chainTransport chains two round-trippers: outer is invoked first, then inner.
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

// convertHooks adapts the legacy provider.Hooks into transport.Hooks. Only
// the response hook is mapped today (request hooks go through buildRequest
// in the transport layer and would require rebuilding the gitcode request).
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
