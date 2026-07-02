// Package gitee implements the Gitee Provider for the git-platform-sdk.
//
// Gitee's REST API is similar to GitHub's, but with a different auth model
// (bearer token in the Authorization header) and some path differences. This
// package uses the unified transport.Client directly rather than wrapping
// a third-party SDK, since Gitee does not ship an official Go SDK.
package gitee

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// Provider is the Gitea implementation of provider.Provider.
type Provider struct {
	client *transport.Client
	logger provider.Logger
}

// New builds a Gitee Provider from the given config.
func New(cfg provider.Config) (provider.Provider, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = provider.NewNoopLogger()
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://gitee.com/api/v5"
	}
	if !strings.Contains(baseURL, "/api/v5") {
		baseURL = strings.TrimSuffix(baseURL, "/") + "/api/v5"
	}

	c := transport.NewClient(baseURL, transport.BearerToken{Token: cfg.Token})
	c.Logger = toTransportLogger(logger)
	c.Timeout = 30 * time.Second
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

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformGitee }

// TestConnection implements provider.Provider.
func (p *Provider) TestConnection(ctx context.Context) (*provider.TestConnectionResult, error) {
	var user struct {
		Login string `json:"login"`
	}
	if _, err := p.client.DoJSON(ctx, &transport.Request{Method: "GET", Path: "/user", Result: &user}); err != nil {
		return &provider.TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &provider.TestConnectionResult{
		Connected: true,
		Platform:  string(p.Platform()),
		UserName:  user.Login,
	}
	_, err := p.ListRepos(ctx, provider.ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}
