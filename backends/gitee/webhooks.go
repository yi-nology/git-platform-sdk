package gitee

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

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateWebhook implements provider.WebhookManager.
func (p *Provider) CreateWebhook(ctx context.Context, opts provider.CreateWebhookOptions) (*provider.PlatformWebhook, error) {
	body := map[string]any{
		"url":    opts.URL,
		"secret": opts.Secret,
	}
	if len(opts.Events) > 0 {
		body["events"] = opts.Events
	} else {
		body["events"] = []string{"push", "pull_request"}
	}
	body["push_events"] = true
	body["merge_requests_events"] = true
	var hook giteeWebhook
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/hooks", esc(opts.Owner), esc(opts.Repo)), body, &hook); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateWebhook", err)
	}
	return hook.toPlatformWebhook(), nil
}

// DeleteWebhook implements provider.WebhookManager.
func (p *Provider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/hooks/%d", esc(owner), esc(repo), webhookID), nil, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteWebhook", err)
	}
	return nil
}

// ListWebhooks implements provider.WebhookManager.
func (p *Provider) ListWebhooks(ctx context.Context, owner, repo string) ([]*provider.PlatformWebhook, error) {
	var hooks []giteeWebhook
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/hooks", esc(owner), esc(repo)), nil, &hooks); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListWebhooks", err)
	}
	result := make([]*provider.PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, h.toPlatformWebhook())
	}
	return result, nil
}

// ValidateWebhookSignature implements provider.WebhookManager.
//
// Gitee uses HMAC-SHA256 over the body, sent in the X-Gitee-Token header.
func (p *Provider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	sig := r.Header.Get("X-Gitee-Token")
	if sig == "" {
		return provider.Wrapf(provider.PlatformGitee, "ValidateWebhookSignature",
			"%w: missing X-Gitee-Token header", provider.ErrWebhookValidation)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "ValidateWebhookSignature", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return provider.Wrapf(provider.PlatformGitee, "ValidateWebhookSignature",
			"%w: invalid signature", provider.ErrWebhookValidation)
	}
	return nil
}

// ParseWebhookEvent implements provider.WebhookManager.
func (p *Provider) ParseWebhookEvent(r *http.Request, secret string) (*provider.NormalizedEvent, error) {
	if err := p.ValidateWebhookSignature(r, secret); err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ParseWebhookEvent", readErr)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var pl struct {
		Action       string `json:"action"`
		ActionDesc   string `json:"action_desc"`
		Number       int    `json:"number"`
		Title        string `json:"title"`
		Body         string `json:"body"`
		State        string `json:"state"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		HTMLURL      string `json:"html_url"`
		User         struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
			Name  string `json:"name"`
		} `json:"user"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Ref       string    `json:"ref"`
		After     string    `json:"after"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &pl); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ParseWebhookEvent", err)
	}

	hookName := r.Header.Get("X-Gitee-Event")
	er := provider.BuildEventRepo(pl.Repository.FullName)
	actor := &provider.CRUser{ID: int64(pl.User.ID), Username: pl.User.Login, Name: pl.User.Name}

	event := &provider.NormalizedEvent{
		ID:         fmt.Sprintf("ge-%d-%d", time.Now().UnixNano(), pl.Number),
		Source:     p.Platform(),
		Timestamp:  time.Now(),
		Actor:      actor,
		Repo:       er,
		RawPayload: json.RawMessage(body),
	}

	switch hookName {
	case "pull_request":
		action := pl.Action
		if action == "close" && pl.State == "merged" {
			action = "merged"
		}
		event.Type = "cr." + action
		event.Action = action
		event.CR = &provider.ChangeRequest{
			Number:       pl.Number,
			Title:        pl.Title,
			Description:  pl.Body,
			State:        provider.MapBoolStateToCR(pl.State, pl.State == "merged"),
			SourceBranch: pl.SourceBranch,
			TargetBranch: pl.TargetBranch,
			WebURL:       pl.HTMLURL,
			Author:       actor,
			CreatedAt:    pl.CreatedAt,
			UpdatedAt:    pl.UpdatedAt,
		}
	case "push":
		event.Type = "push"
		event.Action = "push"
		event.Branch = strings.TrimPrefix(pl.Ref, "refs/heads/")
		event.CommitSHA = pl.After
	case "tag_push":
		event.Type = "tag.push"
		event.Action = "tag.push"
		event.Tag = strings.TrimPrefix(pl.Ref, "refs/tags/")
		event.CommitSHA = pl.After
	case "note", "comment":
		event.Type = "comment.created"
		event.Action = "comment.created"
	case "Issue Hook":
		event.Type = "issue." + pl.Action
		event.Action = pl.Action
	}
	return event, nil
}

var _ provider.WebhookManager = (*Provider)(nil)
