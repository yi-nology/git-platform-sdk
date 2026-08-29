package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v72/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateWebhook implements provider.WebhookManager.
func (p *Provider) CreateWebhook(ctx context.Context, opts provider.CreateWebhookOptions) (*provider.PlatformWebhook, error) {
	events := opts.Events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}
	hook := &github.Hook{
		Name:   github.Ptr("web"),
		Events: events,
		Config: &github.HookConfig{
			URL:    github.Ptr(opts.URL),
			Secret: github.Ptr(opts.Secret),
		},
		Active: github.Ptr(true),
	}
	h, _, err := p.client.Repositories.CreateHook(ctx, opts.Owner, opts.Repo, hook)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateWebhook", err)
	}
	return convertHook(h), nil
}

// DeleteWebhook implements provider.WebhookManager.
func (p *Provider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	_, err := p.client.Repositories.DeleteHook(ctx, owner, repo, webhookID)
	if err != nil {
		return provider.Wrap(provider.PlatformGitHub, "DeleteWebhook", err)
	}
	return nil
}

// ListWebhooks implements provider.WebhookManager.
func (p *Provider) ListWebhooks(ctx context.Context, owner, repo string) ([]*provider.PlatformWebhook, error) {
	hooks, _, err := p.client.Repositories.ListHooks(ctx, owner, repo, nil)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ListWebhooks", err)
	}
	result := make([]*provider.PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, convertHook(h))
	}
	return result, nil
}

// ValidateWebhookSignature implements provider.WebhookManager.
//
// GitHub uses two signatures (X-Hub-Signature for SHA-1 and
// X-Hub-Signature-256 for SHA-256). Both are HMACs over the raw body. We
// verify either; preference given to the modern SHA-256 form.
func (p *Provider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	_, err := github.ValidatePayload(r, []byte(secret))
	if err != nil {
		return provider.Wrap(provider.PlatformGitHub, "ValidateWebhookSignature", err)
	}
	return nil
}

// ParseWebhookEvent implements provider.WebhookManager.
func (p *Provider) ParseWebhookEvent(r *http.Request, secret string) (*provider.NormalizedEvent, error) {
	payload, err := github.ValidatePayload(r, []byte(secret))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ParseWebhookEvent", err)
	}
	eventType := github.WebHookType(r)
	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "ParseWebhookEvent", err)
	}

	ne := &provider.NormalizedEvent{
		Source:     p.Platform(),
		Timestamp:  time.Now(),
		RawPayload: json.RawMessage(payload),
	}

	switch e := event.(type) {
	case *github.PullRequestEvent:
		ne.Type = "cr." + mapWebhookAction(e.GetAction(), e.GetPullRequest().GetMerged())
		ne.Actor = convertUser(e.GetSender())
		if e.GetRepo() != nil {
			ne.Repo = provider.BuildEventRepo(e.GetRepo().GetFullName())
			ne.Repo.ID = e.GetRepo().GetID()
		}
		if e.GetPullRequest() != nil {
			ne.CR = convertPR(e.GetPullRequest())
			ne.CommitSHA = e.GetPullRequest().GetHead().GetSHA()
		}
	case *github.PushEvent:
		ne.Type = "push"
		ne.Branch = strings.TrimPrefix(e.GetRef(), "refs/heads/")
		ne.CommitSHA = e.GetAfter()
		ne.Actor = convertUser(e.GetSender())
		if e.GetRepo() != nil {
			ne.Repo = provider.BuildEventRepo(e.GetRepo().GetFullName())
			ne.Repo.ID = e.GetRepo().GetID()
		}
	case *github.CreateEvent:
		ne.Type = "branch.created"
		ne.Branch = e.GetRef()
		ne.Actor = convertUser(e.GetSender())
	case *github.DeleteEvent:
		ne.Type = "branch.deleted"
		ne.Branch = e.GetRef()
		ne.Actor = convertUser(e.GetSender())
	default:
		ne.Type = eventType
	}
	return ne, nil
}

// compile-time guard
var _ provider.WebhookManager = (*Provider)(nil)
