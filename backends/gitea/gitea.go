// Package gitea implements the Gitea Provider for the git-platform-sdk.
//
// It builds on top of the official code.gitea.io/sdk/gitea SDK and adds
// transport-layer cross-cutting behavior (auth, retry, hooks, logging)
// provided by the parent project's transport package. All Provider methods
// are split across the per-responsibility files in this package:
//
//   - gitea.go:   constructor + identity (Platform, TestConnection, Capabilities)
//   - init.go:    provider registration with the global registry
//   - repos.go:   ListRepos, GetRepo, CreateRepo, DeleteRepo, UpdateRepo, ForkRepo
//   - crs.go:     Change requests (PRs): Create/Get/List/Merge/Close/Reopen/Update/UpdateLabels/Comments/Commits
//   - webhooks.go: webhook CRUD + signature validation + event parsing
//   - branches.go: ListBranches, CreateBranch, DeleteBranch
//   - diffs.go:   GetCRDiff, GetCRFiles, CreateNote/DeleteNote, CreateDiscussion
//   - commits.go: GetCommit, ListCommits, CompareCommits, CreateCommitStatus
//   - files.go:   GetFileContent, CreateFile, UpdateFile, DeleteFile
//   - releases.go: ListTags, ListReleases, CreateRelease, GetArchive
//   - labels.go:  repository label CRUD (LabelManager)
//   - issues.go:  issue CRUD, comments, and issue labels (IssueManager)
//   - reviews.go: pull-request code reviews (ReviewManager)
//   - milestones.go: repository milestone CRUD (MilestoneManager)
//   - search.go:    global repo/issue/user search (SearchManager)
//   - types.go:   internal Gitea-API types and conversion helpers
package gitea

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// Provider is the Gitea implementation of provider.Provider.
type Provider struct {
	client   *gitea.Client
	logger   provider.Logger
	labelIDs *backendutil.IDCache
}

// New builds a Gitea Provider from the given config.
func New(cfg provider.Config) (provider.Provider, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = provider.NewNoopLogger()
	}

	baseURL := backendutil.NormalizeBaseURL(backendutil.DefaultBaseURL(cfg.BaseURL, "https://gitea.com"))

	transportClient := transport.NewClient(baseURL, transport.TokenHeader{Token: cfg.Token})
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

	client, err := gitea.NewClient(baseURL, gitea.SetToken(cfg.Token), gitea.SetHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("gitea: failed to create client: %w", err)
	}
	return &Provider{client: client, logger: logger, labelIDs: backendutil.NewIDCache(5 * time.Minute)}, nil
}

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformGitea }

// Capabilities implements provider.Provider. Gitea implements the optional
// LabelManager (see labels.go), IssueManager (see issues.go),
// ReviewManager (see reviews.go), and SearchManager (see search.go)
// interfaces.
func (p *Provider) Capabilities() provider.CapabilitySet {
	return provider.CapabilitySet{Labels: true, Issues: true, Reviews: true, Milestones: true, Search: true, CommitStatuses: true}
}

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
