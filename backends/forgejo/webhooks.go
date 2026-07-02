package forgejo

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateWebhook implements provider.WebhookManager.
func (p *Provider) CreateWebhook(ctx context.Context, opts provider.CreateWebhookOptions) (*provider.PlatformWebhook, error) {
	events := opts.Events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}
	hook, _, err := p.client.CreateRepoHook(opts.Owner, opts.Repo, forgejo.CreateHookOption{
		Type:   forgejo.HookTypeForgejo,
		Config: map[string]string{"url": opts.URL, "content_type": "json", "secret": opts.Secret},
		Events: events,
		Active: true,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "CreateWebhook", err)
	}
	return convertHook(hook), nil
}

// DeleteWebhook implements provider.WebhookManager.
func (p *Provider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	_, err := p.client.DeleteRepoHook(owner, repo, webhookID)
	if err != nil {
		return provider.Wrap(provider.PlatformForgejo, "DeleteWebhook", err)
	}
	return nil
}

// ListWebhooks implements provider.WebhookManager.
func (p *Provider) ListWebhooks(ctx context.Context, owner, repo string) ([]*provider.PlatformWebhook, error) {
	hooks, _, err := p.client.ListRepoHooks(owner, repo, forgejo.ListHooksOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListWebhooks", err)
	}
	result := make([]*provider.PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, convertHook(h))
	}
	return result, nil
}

// ValidateWebhookSignature implements provider.WebhookManager. Forgejo uses
// HMAC-SHA256 over the raw body, sent in the X-Forgejo-Signature header.
func (p *Provider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	sig := r.Header.Get("X-Forgejo-Signature")
	if sig == "" {
		return provider.Wrapf(provider.PlatformForgejo, "ValidateWebhookSignature", "missing X-Forgejo-Signature header")
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return provider.Wrapf(provider.PlatformForgejo, "ValidateWebhookSignature", "invalid webhook signature")
	}
	return nil
}

// ParseWebhookEvent implements provider.WebhookManager.
func (p *Provider) ParseWebhookEvent(r *http.Request, secret string) (*provider.NormalizedEvent, error) {
	if err := p.ValidateWebhookSignature(r, secret); err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))

	eventType := r.Header.Get("X-Forgejo-Event")
	var pl struct {
		Action string `json:"action"`
		Sender struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
		} `json:"sender"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		PullRequest *struct {
			ID     int    `json:"id"`
			Number int    `json:"number"`
			Title  string `json:"title"`
			Body   string `json:"body"`
			State  string `json:"state"`
			Head   struct {
				Ref string `json:"ref"`
				SHA string `json:"sha"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
			Merged   bool      `json:"merged"`
			HTMLURL  string    `json:"html_url"`
			User     struct {
				ID    int    `json:"id"`
				Login string `json:"login"`
			} `json:"user"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"pull_request"`
		Number int    `json:"number"`
		Ref    string `json:"ref"`
		After  string `json:"after"`
	}
	if err := json.Unmarshal(body, &pl); err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ParseWebhookEvent", err)
	}

	er := provider.BuildEventRepo(pl.Repository.FullName)
	actor := &provider.CRUser{ID: int64(pl.Sender.ID), Username: pl.Sender.Login}

	event := &provider.NormalizedEvent{
		ID:        fmt.Sprintf("gt-%d-%d", time.Now().UnixNano(), pl.Number),
		Source:    p.Platform(),
		Timestamp: time.Now(),
		Actor:     actor,
		Repo:      er,
		RawPayload: json.RawMessage(body),
	}

	switch eventType {
	case "pull_request":
		action := pl.Action
		if action == "closed" && pl.PullRequest != nil && pl.PullRequest.Merged {
			action = "merged"
		}
		event.Type = "cr." + action
		if pl.PullRequest != nil {
			event.CommitSHA = pl.PullRequest.Head.SHA
			event.CR = &provider.ChangeRequest{
				ID:           int64(pl.PullRequest.Number),
				Number:       pl.PullRequest.Number,
				Title:        pl.PullRequest.Title,
				Description:  pl.PullRequest.Body,
				State:        mapState(pl.PullRequest.State, pl.PullRequest.Merged),
				SourceBranch: pl.PullRequest.Head.Ref,
				TargetBranch: pl.PullRequest.Base.Ref,
				WebURL:       pl.PullRequest.HTMLURL,
				Author:       &provider.CRUser{ID: int64(pl.PullRequest.User.ID), Username: pl.PullRequest.User.Login},
				CreatedAt:    pl.PullRequest.CreatedAt,
				UpdatedAt:    pl.PullRequest.UpdatedAt,
			}
		}
	case "push":
		event.Type = "push"
		event.Branch = strings.TrimPrefix(pl.Ref, "refs/heads/")
		event.CommitSHA = pl.After
	case "create":
		event.Type = "branch.created"
		event.Branch = pl.Ref
	case "delete":
		event.Type = "branch.deleted"
		event.Branch = pl.Ref
	}
	return event, nil
}

var _ provider.WebhookManager = (*Provider)(nil)