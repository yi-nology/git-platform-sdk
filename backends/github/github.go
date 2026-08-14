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
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
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
		backendutil.DefaultBaseURL(cfg.BaseURL, "https://api.github.com"),
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

	// Underlying http.Client used by go-github. We wrap its transport with
	// transport.NewRetryingRoundTripper so all SDK-issued requests flow
	// through the auth/retry/hooks pipeline.
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: backendutil.ChainTransport(
			backendutil.HTTPTransport(cfg.SkipTLS),
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

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformGitHub }

// Capabilities implements provider.Provider. This backend does not yet
// implement any optional capability interface; flip fields here as
// capability backends land.
func (p *Provider) Capabilities() provider.CapabilitySet {
	return provider.CapabilitySet{}
}

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
