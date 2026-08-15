// Package gitee implements the Gitee Provider for the git-platform-sdk.
//
// Gitee's REST API is similar to GitHub's, but with a different auth model
// (bearer token in the Authorization header) and some path differences. This
// package uses the unified transport.Client directly rather than wrapping
// a third-party SDK, since Gitee does not ship an official Go SDK. All
// Provider methods are split across the per-responsibility files in this
// package:
//
//   - gitee.go:   constructor + identity (Platform, TestConnection, Capabilities)
//   - init.go:    provider registration with the global registry
//   - methods.go: doRequest/doRequestWithHeaders JSON HTTP helpers
//   - repos.go:   ListRepos, GetRepo, CreateRepo, DeleteRepo, UpdateRepo, ForkRepo
//   - crs.go:     Change requests (PRs): Create/Get/List/Merge/Close/Reopen/Update/UpdateLabels/Comments/Commits
//   - webhooks.go: webhook CRUD + signature validation + event parsing
//   - branches.go: ListBranches, CreateBranch, DeleteBranch
//   - diffs.go:   GetCRDiff, GetCRFiles, CreateNote/DeleteNote, CreateDiscussion, CreateReview
//   - commits.go: GetCommit, ListCommits, CompareCommits, CreateCommitStatus
//   - files.go:   GetFileContent, CreateFile, UpdateFile, DeleteFile
//   - releases.go: ListTags, ListReleases, CreateRelease, GetArchive
//   - labels.go:  repository label CRUD (LabelManager)
//   - issues.go:  issue CRUD, comments, and issue labels (IssueManager)
//   - types.go:   internal Gitee-API types and conversion helpers
package gitee

import (
	"context"
	"strings"
	"time"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
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
	c.Logger = backendutil.ToTransportLogger(logger)
	c.Timeout = 30 * time.Second
	// Set TLS-skipping transport on the transport client so that all
	// HTTP requests (including retries) honour SkipTLS.
	if cfg.SkipTLS {
		c.Transport = backendutil.HTTPTransport(cfg.SkipTLS)
	}
	c.Retry = backendutil.MapRetryConfig(cfg.RetryConfig)
	if cfg.Hooks != nil {
		c.Hooks = backendutil.ConvertHooks(cfg.Hooks)
	}
	return &Provider{client: c, logger: logger}, nil
}

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformGitee }

// Capabilities implements provider.Provider. Gitee implements the optional
// LabelManager (see labels.go) and IssueManager (see issues.go) interfaces.
func (p *Provider) Capabilities() provider.CapabilitySet {
	return provider.CapabilitySet{Labels: true, Issues: true}
}

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
