// Package github implements the GitHub Provider for the git-platform-sdk.
//
// It builds on top of the official google/go-github SDK and adds the
// transport-layer cross-cutting behavior (auth, retry, hooks, logging)
// provided by the parent project's transport package. All Provider methods
// are split across the per-responsibility files in this package:
//
//   - github.go:  constructor, registration, identity (Platform, TestConnection)
//   - repos.go:   ListRepos, GetRepo, CreateRepo, DeleteRepo, UpdateRepo, ForkRepo
//   - crs.go:     Change requests (PRs): Create/Get/List/Close/Merge/Reopen/Update/Comments/Commits
//   - webhooks.go: webhook CRUD + signature validation + event parsing
//   - branches.go: ListBranches, CreateBranch, DeleteBranch
//   - diffs.go:    GetCRDiff, GetCRFiles, CreateNote/DeleteNote, CreateDiscussion, CreateReview
//   - commits.go:  GetCommit, ListCommits, CompareCommits, CreateCommitStatus
//   - files.go:    GetFileContent, CreateFile, UpdateFile, DeleteFile
//   - releases.go: ListTags, ListReleases, CreateRelease, GetArchive
//   - types.go:    internal GitHub-API types and conversion helpers
package github

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// Provider is the GitHub implementation of provider.Provider. It embeds the
// official go-github client and an auxiliary transport.Client used for
// endpoints not covered by the SDK (e.g. archive downloads).
type Provider struct {
	client *github.Client
	logger provider.Logger
}

// New builds a GitHub Provider from the given config. It registers itself
// with provider.Register so provider.NewProvider(PlatformGitHub, ...) returns
// an instance of this Provider.
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

	// Underlying http.Client used by go-github. We wrap its transport with
	// transport.NewRetryingRoundTripper so all SDK-issued requests flow
	// through the auth/retry/hooks pipeline.
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: chainTransport(
			httpTransport(cfg.SkipTLS),
			transportClient.NewRetryingRoundTripper(),
		),
	}

	var ghClient *github.Client
	if cfg.BaseURL == "" {
		ghClient = github.NewClient(httpClient)
	} else {
		base := cfg.BaseURL
		//nolint:staticcheck // NewEnterpriseClient is deprecated but the replacement API (WithEnterpriseURLs) is not available in all go-github versions
		ec, err := github.NewEnterpriseClient(base, "", httpClient)
		if err != nil {
			return nil, fmt.Errorf("github: failed to create enterprise client for %s: %w", base, err)
		}
		ghClient = ec
	}

	return &Provider{client: ghClient, logger: logger}, nil
}

func defaultBaseURL(base string) string {
	if base == "" {
		return "https://api.github.com"
	}
	return base
}

func httpTransport(skipTLS bool) http.RoundTripper {
	if !skipTLS {
		return http.DefaultTransport
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
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
		// outer runs first; if it returns a response (real or wrapped), we
		// hand it back. If it short-circuits with nil/nil, we fall back to
		// the inner transport (the underlying connection).
		if resp, err := c.outer.RoundTrip(req); resp != nil || err != nil {
			return resp, err
		}
	}
	return c.inner.RoundTrip(req)
}

// convertHooks adapts the legacy provider.Hooks into transport.Hooks. Only
// the response hook is mapped today (request hooks go through buildRequest
// in the transport layer and would require rebuilding the go-github request).
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

// oauthToken is preserved for tests that need to build a go-github client
// directly. It is not used by New above (the transport layer injects the
// bearer token instead).
//
//nolint:unused // kept for test helpers
func oauthToken(token string) *http.Client {
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return &http.Client{Transport: &oauth2.Transport{Source: src}}
}

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformGitHub }

// TestConnection implements provider.Provider.
func (p *Provider) TestConnection(ctx context.Context) (*provider.TestConnectionResult, error) {
	user, _, err := p.client.Users.Get(ctx, "")
	if err != nil {
		return &provider.TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &provider.TestConnectionResult{
		Connected: true,
		Platform:  string(p.Platform()),
		UserName:  user.GetLogin(),
	}
	_, err = p.ListRepos(ctx, provider.ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}
