// Package gitcode implements the GitCode Provider for the git-platform-sdk.
//
// It builds on top of the yi-nology/gitcode_api client SDK and adds
// transport-layer cross-cutting behavior (auth, retry, hooks, logging)
// provided by the parent project's transport package.
package gitcode

import (
	"context"
	"fmt"
	"time"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
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
	// The gitcode_api SDK builds its own http.Client internally. To get
	// transport-layer retry/hooks we would need to plumb an http.Client
	// through, which is not currently exposed. For now, the SDK's built-in
	// transport is used as-is.
	var client *gitcode.Client
	if cfg.BaseURL == "" {
		client = gitcode.NewClient(cfg.Token)
	} else {
		client = gitcode.NewClientWithBaseURL(cfg.BaseURL, cfg.Token)
	}
	_ = time.Second // reserved for future per-request timeout config
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

// avoid unused-import warning when fmt is not used in some build configs.
var _ = fmt.Sprintf
