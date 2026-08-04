package gitcode

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

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateWebhook implements provider.WebhookManager.
func (p *Provider) CreateWebhook(ctx context.Context, opts provider.CreateWebhookOptions) (*provider.PlatformWebhook, error) {
	events := opts.Events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}
	hook, err := p.client.CreateWebhook(ctx, opts.Owner, opts.Repo, gitcode.CreateWebhookOptions{
		URL: opts.URL, Secret: opts.Secret, Events: events,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CreateWebhook", err)
	}
	return &provider.PlatformWebhook{ID: hook.ID, URL: hook.URL, Events: hook.Events}, nil
}

// DeleteWebhook implements provider.WebhookManager.
func (p *Provider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	err := p.client.DeleteWebhook(ctx, owner, repo, webhookID)
	if err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteWebhook", err)
	}
	return nil
}

// ListWebhooks implements provider.WebhookManager.
func (p *Provider) ListWebhooks(ctx context.Context, owner, repo string) ([]*provider.PlatformWebhook, error) {
	hooks, err := p.client.ListWebhooks(ctx, owner, repo)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListWebhooks", err)
	}
	result := make([]*provider.PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, &provider.PlatformWebhook{ID: h.ID, URL: h.URL, Events: h.Events})
	}
	return result, nil
}

// ValidateWebhookSignature implements provider.WebhookManager. GitCode uses
// HMAC-SHA256 over the body, sent in X-Gitea-Signature or X-GitCode-Signature.
func (p *Provider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	sig := r.Header.Get("X-Gitea-Signature")
	if sig == "" {
		sig = r.Header.Get("X-GitCode-Signature")
	}
	if sig == "" {
		return provider.Wrapf(provider.PlatformGitCode, "ValidateWebhookSignature",
			"%w: missing webhook signature header", provider.ErrWebhookValidation)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return provider.Wrap(provider.PlatformGitCode, "ValidateWebhookSignature", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return provider.Wrapf(provider.PlatformGitCode, "ValidateWebhookSignature",
			"%w: invalid webhook signature", provider.ErrWebhookValidation)
	}
	return nil
}

// ParseWebhookEvent implements provider.WebhookManager.
func (p *Provider) ParseWebhookEvent(r *http.Request, secret string) (*provider.NormalizedEvent, error) {
	if err := p.ValidateWebhookSignature(r, secret); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ParseWebhookEvent", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	eventType := r.Header.Get("X-Gitea-Event")
	if eventType == "" {
		eventType = r.Header.Get("X-GitCode-Event")
	}

	ne := &provider.NormalizedEvent{
		Source:     p.Platform(),
		Timestamp:  time.Now(),
		RawPayload: json.RawMessage(body),
	}

	switch eventType {
	case "pull_request":
		prEvent, err := p.client.ParsePullRequestEvent(body)
		if err != nil {
			return nil, provider.Wrap(provider.PlatformGitCode, "ParseWebhookEvent", err)
		}
		action := prEvent.Action
		if action == "closed" && prEvent.PullRequest != nil && prEvent.PullRequest.Merged {
			action = "merged"
		}
		ne.Type = "cr." + action
		ne.Action = action
		if prEvent.Sender != nil {
			senderID, _ := parseGitCodeID(prEvent.Sender.ID)
			ne.Actor = &provider.CRUser{
				ID: senderID, Username: prEvent.Sender.Login, AvatarURL: prEvent.Sender.AvatarURL,
			}
		}
		if prEvent.Repository != nil {
			ne.Repo = provider.BuildEventRepo(prEvent.Repository.FullName)
		}
		if prEvent.PullRequest != nil {
			ne.CR = convertPullRequest(prEvent.PullRequest)
			if prEvent.PullRequest.Head != nil {
				ne.CommitSHA = prEvent.PullRequest.Head.SHA
			}
		}
	case "push":
		pushEvent, err := p.client.ParsePushEvent(body)
		if err != nil {
			return nil, provider.Wrap(provider.PlatformGitCode, "ParseWebhookEvent", err)
		}
		ne.Type = "push"
		ne.Branch = strings.TrimPrefix(pushEvent.Ref, "refs/heads/")
		ne.CommitSHA = pushEvent.After
		if pushEvent.Sender != nil {
			senderID, _ := parseGitCodeID(pushEvent.Sender.ID)
			ne.Actor = &provider.CRUser{
				ID: senderID, Username: pushEvent.Sender.Login, AvatarURL: pushEvent.Sender.AvatarURL,
			}
		}
		if pushEvent.Repository != nil {
			ne.Repo = provider.BuildEventRepo(pushEvent.Repository.FullName)
		}
	case "tag_push":
		tagEvent, err := p.client.ParseTagPushEvent(body)
		if err != nil {
			return nil, provider.Wrap(provider.PlatformGitCode, "ParseWebhookEvent", err)
		}
		ne.Type = "tag.push"
		ne.Tag = strings.TrimPrefix(tagEvent.Ref, "refs/tags/")
		ne.CommitSHA = tagEvent.After
		if tagEvent.Sender != nil {
			senderID, _ := parseGitCodeID(tagEvent.Sender.ID)
			ne.Actor = &provider.CRUser{
				ID: senderID, Username: tagEvent.Sender.Login, AvatarURL: tagEvent.Sender.AvatarURL,
			}
		}
		if tagEvent.Repository != nil {
			ne.Repo = provider.BuildEventRepo(tagEvent.Repository.FullName)
		}
	case "create":
		var createEvent struct {
			Ref        string              `json:"ref"`
			Sender     *gitcode.User       `json:"sender"`
			Repository *gitcode.Repository `json:"repository"`
		}
		if err := json.Unmarshal(body, &createEvent); err != nil {
			return nil, provider.Wrap(provider.PlatformGitCode, "ParseWebhookEvent", err)
		}
		ne.Type = "branch.created"
		ne.Branch = createEvent.Ref
		if createEvent.Sender != nil {
			senderID, _ := parseGitCodeID(createEvent.Sender.ID)
			ne.Actor = &provider.CRUser{
				ID: senderID, Username: createEvent.Sender.Login, AvatarURL: createEvent.Sender.AvatarURL,
			}
		}
		if createEvent.Repository != nil {
			ne.Repo = provider.BuildEventRepo(createEvent.Repository.FullName)
		}
	case "delete":
		var deleteEvent struct {
			Ref        string              `json:"ref"`
			Sender     *gitcode.User       `json:"sender"`
			Repository *gitcode.Repository `json:"repository"`
		}
		if err := json.Unmarshal(body, &deleteEvent); err != nil {
			return nil, provider.Wrap(provider.PlatformGitCode, "ParseWebhookEvent", err)
		}
		ne.Type = "branch.deleted"
		ne.Branch = deleteEvent.Ref
		if deleteEvent.Sender != nil {
			senderID, _ := parseGitCodeID(deleteEvent.Sender.ID)
			ne.Actor = &provider.CRUser{
				ID: senderID, Username: deleteEvent.Sender.Login, AvatarURL: deleteEvent.Sender.AvatarURL,
			}
		}
		if deleteEvent.Repository != nil {
			ne.Repo = provider.BuildEventRepo(deleteEvent.Repository.FullName)
		}
	default:
		ne.Type = eventType
	}
	return ne, nil
}

// parseGitCodeID converts an SDK gitcode.FlexString (an alias for a string)
// into an int64. Returns 0 when the value is empty or non-numeric.
func parseGitCodeID(id gitcode.FlexString) (int64, error) {
	return parseInt64(string(id))
}

func parseInt64(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer %q", s)
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

var _ provider.WebhookManager = (*Provider)(nil)
