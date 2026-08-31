// Package gitlab implements the GitLab Provider for the git-platform-sdk.
//
// It builds on top of the official gitlab-org/api/client-go SDK and adds
// transport-layer cross-cutting behavior (auth, retry, hooks, logging)
// provided by the parent project's transport package. All Provider methods
// are split across the per-responsibility files in this package:
//
//   - gitlab.go:  constructor + identity (Platform, TestConnection, Capabilities)
//   - init.go:    provider registration with the global registry
//   - repos.go:   ListRepos, GetRepo, CreateRepo, DeleteRepo, UpdateRepo, ForkRepo
//   - crs.go:     Change requests (MRs): Create/Get/List/Close/Merge/Reopen/Update/Comments/Commits
//   - webhooks.go: webhook CRUD + signature validation + event parsing
//   - branches.go: ListBranches, CreateBranch, DeleteBranch
//   - diffs.go:    GetCRDiff, GetCRFiles, CreateNote/DeleteNote, CreateDiscussion
//   - commits.go:  GetCommit, ListCommits, CompareCommits, CreateCommitStatus
//   - files.go:    GetFileContent, CreateFile, UpdateFile, DeleteFile
//   - releases.go: ListTags, ListReleases, CreateRelease, GetArchive
//   - labels.go:   repository label CRUD (LabelManager)
//   - issues.go:   issue CRUD, comments (notes), and issue labels (IssueManager)
//   - reviews.go:  code reviews via approvals mappings (ReviewManager)
//   - milestones.go: repository milestone CRUD (MilestoneManager)
//   - search.go:    global repo/issue/user search (SearchManager)
//   - types.go:    internal GitLab-API types and conversion helpers
package gitlab

import (
	"context"
	"fmt"

	"net/http"
	"time"

	"golang.org/x/oauth2"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// Provider is the GitLab implementation of provider.Provider.
type Provider struct {
	client   *gitlab.Client
	logger   provider.Logger
	labelIDs *backendutil.IDCache
	userIDs  *backendutil.IDCache
}

// New builds a GitLab Provider from the given config.
func New(cfg provider.Config) (provider.Provider, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = provider.NewNoopLogger()
	}

	// Select auth strategy: "bearer" → OAuth Bearer, otherwise → PRIVATE-TOKEN (GitLab default).
	var authStrategy transport.AuthStrategy = transport.PrivateToken{Token: cfg.Token}
	if cfg.TokenStyle == "bearer" {
		authStrategy = transport.BearerToken{Token: cfg.Token}
	}
	transportClient := transport.NewClient(
		backendutil.DefaultBaseURL(cfg.BaseURL, "https://gitlab.com/api/v4"),
		authStrategy,
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

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: backendutil.ChainTransport(
			backendutil.HTTPTransport(cfg.SkipTLS),
			transportClient.NewRetryingRoundTripper(),
		),
	}

	opts := []gitlab.ClientOptionFunc{gitlab.WithHTTPClient(httpClient)}
	if cfg.BaseURL != "" {
		opts = append(opts, gitlab.WithBaseURL(cfg.BaseURL))
	}
	var client *gitlab.Client
	var err error
	if cfg.TokenStyle == "bearer" {
		// Bearer auth (e.g. GitLab CI_JOB_TOKEN). The deprecated
		// NewOAuthClient is replaced by NewAuthSourceClient per the
		// client-go v2.60 guidance.
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
		client, err = gitlab.NewAuthSourceClient(gitlab.OAuthTokenSource{TokenSource: ts}, opts...)
	} else {
		client, err = gitlab.NewClient(cfg.Token, opts...)
	}
	if err != nil {
		return nil, fmt.Errorf("gitlab: failed to create client: %w", err)
	}
	return &Provider{
		client:   client,
		logger:   logger,
		labelIDs: backendutil.NewIDCache(5 * time.Minute),
		userIDs:  backendutil.NewIDCache(5 * time.Minute),
	}, nil
}

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformGitLab }

// Capabilities implements provider.Provider. GitLab implements the optional
// LabelManager (see labels.go), IssueManager (see issues.go),
// ReviewManager (see reviews.go), and SearchManager (see search.go)
// interfaces.
func (p *Provider) Capabilities() provider.CapabilitySet {
	return provider.CapabilitySet{Labels: true, Issues: true, Reviews: true, Milestones: true, Search: true, CommitStatuses: true, Notifications: true, Reactions: true}
}

// TestConnection implements provider.Provider.
func (p *Provider) TestConnection(ctx context.Context) (*provider.TestConnectionResult, error) {
	user, _, err := p.client.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		return &provider.TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &provider.TestConnectionResult{
		Connected: true,
		Platform:  string(p.Platform()),
		UserName:  user.Username,
	}
	_, err = p.ListRepos(ctx, provider.ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}
