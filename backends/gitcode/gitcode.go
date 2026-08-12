// Package gitcode implements the GitCode Provider for the git-platform-sdk.
//
// It builds on top of the yi-nology/gitcode_api client SDK and adds
// transport-layer cross-cutting behavior (auth, retry, hooks, logging)
// provided by the parent project's transport package.
package gitcode

import (
	"context"
	"net/http"
	"time"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
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
		backendutil.DefaultBaseURL(cfg.BaseURL, "https://api.gitcode.com/api/v5"),
		transport.BearerToken{Token: cfg.Token},
	)
	transportClient.Logger = logger
	// Set TLS-skipping transport on the transport client so that all
	// HTTP requests (including retries) honour SkipTLS.
	if cfg.SkipTLS {
		transportClient.Transport = backendutil.HTTPTransport(cfg.SkipTLS)
	}
	transportClient.Retry = backendutil.MapRetryConfig(cfg.RetryConfig)
	if cfg.Hooks != nil {
		transportClient.Hooks = backendutil.ConvertHooks(cfg.Hooks)
	}

	// Build an http.Client whose transport flows through the unified
	// auth/retry/hooks pipeline, then inject it into the gitcode_api SDK
	// via SetHTTPClient.
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: backendutil.ChainTransport(
			backendutil.HTTPTransport(cfg.SkipTLS),
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
