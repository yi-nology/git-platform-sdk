// Package gitee implements the Gitee Provider for the git-platform-sdk.
//
// The backend builds on the community go-gitee SDK
// (gitee.com/openeuler/go-gitee) and wires the SDK's http.Client through the
// unified transport pipeline (auth, retry, hooks, logging) so SDK-issued
// requests behave like every other backend's traffic. A handful of endpoints
// are not usable through the SDK — either missing entirely (e.g.
// DELETE /repos/{owner}/{repo}/branches/{branch}), generated with a broken
// signature (the user-repos list methods decode into a single Project instead
// of an array; the commit, compare, contents, and release models type live
// objects/arrays/booleans as plain strings, as does the webhook Hook model
// whose list decode is then silently swallowed; the tags method returns a
// single Tag for an array endpoint; the labels list opts carry no pagination;
// the labels patch, releases create, webhook create, and issue create methods
// post multipart bodies labeled application/json) — and keep using the
// retained transport.Client via Provider.raw(). Each such detour is
// registered in a doc comment on the method that takes it. All Provider
// methods are split across the per-responsibility files in this package:
//
//   - gitee.go:   constructor + identity (Platform, TestConnection, Capabilities)
//   - init.go:    provider registration with the global registry
//   - methods.go: doRequest JSON helper + esc/escPath path escaping (raw client)
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
//   - types.go:   SDK model conversions and raw-wire types/helpers
package gitee

import (
	"context"
	"net/http"
	"strings"
	"time"

	gitee "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// Provider is the Gitee implementation of provider.Provider. It embeds the
// go-gitee SDK client plus an auxiliary transport.Client used for endpoints
// the SDK does not cover (or covers with a broken generated signature).
type Provider struct {
	client    *gitee.APIClient
	rawClient *transport.Client
	token     string
	logger    provider.Logger
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
	baseURL = strings.TrimSuffix(baseURL, "/")
	if !strings.Contains(baseURL, "/api/v5") {
		baseURL += "/api/v5"
	}
	// The SDK builds request paths as BasePath + "/v5/..." (its default
	// BasePath is https://gitee.com/api), so the SDK BasePath must stop one
	// segment earlier than the raw client's API root. Stripping the "/v5"
	// suffix rather than re-appending keeps a cfg.BaseURL that already
	// carries "/api/v5" from producing "/api/v5/v5/..." requests.
	sdkBasePath := strings.TrimSuffix(baseURL, "/v5")

	// The transport client exists primarily to provide the auth/retry/hooks
	// pipeline the SDK's http.Client is wrapped with; it is also used
	// directly for SDK-missing endpoints (see Provider.raw). The SDK-side
	// token is passed per call via the access_token option of the generated
	// *Opts structs.
	transportClient := transport.NewClient(baseURL, transport.BearerToken{Token: cfg.Token})
	transportClient.Logger = backendutil.ToTransportLogger(logger)
	transportClient.Timeout = 30 * time.Second
	// Set TLS-skipping transport on the transport client so that all HTTP
	// requests (including retries and raw calls) honour SkipTLS.
	if cfg.SkipTLS {
		transportClient.Transport = backendutil.HTTPTransport(cfg.SkipTLS)
	}
	transportClient.Retry = backendutil.MapRetryConfig(cfg.RetryConfig)
	if cfg.Hooks != nil {
		transportClient.Hooks = backendutil.ConvertHooks(cfg.Hooks)
	}

	// Underlying http.Client used by go-gitee. Its transport is wrapped with
	// transport.NewRetryingRoundTripper so all SDK-issued requests flow
	// through the auth/retry/hooks pipeline.
	//
	// Caveat: go-gitee's callAPI also retries 502 internally (up to 3 attempts
	// with fixed 1s/2s sleeps), which multiplies with this pipeline on a
	// persistent 502, and its re-Do of an already-consumed request body can
	// degrade that internal retry to a client-side error for body-bearing
	// calls (upstream codegen bug, client.go callAPI).
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: backendutil.ChainTransport(
			backendutil.HTTPTransport(cfg.SkipTLS),
			transportClient.NewRetryingRoundTripper(),
		),
	}

	cfgGitee := gitee.NewConfiguration()
	cfgGitee.BasePath = sdkBasePath
	cfgGitee.HTTPClient = httpClient

	return &Provider{
		client:    gitee.NewAPIClient(cfgGitee),
		rawClient: transportClient,
		token:     cfg.Token,
		logger:    logger,
	}, nil
}

// raw returns the raw transport client, used for the endpoints go-gitee does
// not cover (e.g. DELETE branch) or covers with a broken generated signature
// (the user-repos list endpoints decode into a single Project and cannot
// represent the array response).
func (p *Provider) raw() *transport.Client { return p.rawClient }

// accessToken returns the provider token as a per-call optional.String for
// the generated *Opts structs. Unset when no token was configured.
func (p *Provider) accessToken() optional.String {
	if p.token == "" {
		return optional.String{}
	}
	return optional.NewString(p.token)
}

// sdkErr converts a go-gitee call error into a provider error, preserving the
// HTTP status from the returned *http.Response. The SDK's GenericSwaggerError
// carries only a status string, so without this provider.IsNotFound and
// friends would not classify SDK failures.
func (p *Provider) sdkErr(op string, resp *http.Response, err error) error {
	if resp != nil {
		return provider.Wrap(p.Platform(), op,
			provider.New(p.Platform(), op, resp.StatusCode, err.Error()))
	}
	return provider.Wrap(p.Platform(), op, err)
}

// Platform implements provider.Provider.
func (p *Provider) Platform() provider.Platform { return provider.PlatformGitee }

// Capabilities implements provider.Provider. Gitee implements the optional
// LabelManager (see labels.go) and IssueManager (see issues.go) interfaces.
// Gitee issue numbers are alphanumeric strings (e.g. "IAINVA"), addressed
// natively by the string-typed IssueManager.
func (p *Provider) Capabilities() provider.CapabilitySet {
	return provider.CapabilitySet{Labels: true, Issues: true}
}

// TestConnection implements provider.Provider.
func (p *Provider) TestConnection(ctx context.Context) (*provider.TestConnectionResult, error) {
	user, _, err := p.client.UsersApi.GetV5User(ctx, &gitee.GetV5UserOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return &provider.TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &provider.TestConnectionResult{
		Connected: true,
		Platform:  string(p.Platform()),
		UserName:  user.Login,
	}
	_, err = p.ListRepos(ctx, provider.ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}
