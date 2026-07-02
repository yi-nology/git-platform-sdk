package tencentcode

import (
	"bytes"
	"context"
	"crypto/subtle"
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
	encoded := encodeProjectPath(opts.Owner, opts.Repo)
	body := map[string]any{
		"url":          opts.URL,
		"token":        opts.Secret,
		"push_events":  true,
	}
	if len(opts.Events) > 0 {
		em := map[string]bool{}
		for _, e := range opts.Events {
			em[e] = true
		}
		if v, ok := em["push"]; ok {
			body["push_events"] = v
		}
		body["merge_requests_events"] = em["merge_request"] || em["merge_requests"] || em["pull_request"] || em["cr"]
		body["tag_push_events"] = em["tag_push"] || em["tag"]
	}
	var wh struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	}
	if err := p.doRequest(ctx, "POST", "/projects/"+encoded+"/hooks", body, &wh); err != nil {
		return nil, err
	}
	return &provider.PlatformWebhook{ID: int64(wh.ID), URL: wh.URL}, nil
}

// DeleteWebhook implements provider.WebhookManager.
func (p *Provider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	encoded := encodeProjectPath(owner, repo)
	return p.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/hooks/%d", encoded, webhookID), nil, nil)
}

// ListWebhooks implements provider.WebhookManager.
func (p *Provider) ListWebhooks(ctx context.Context, owner, repo string) ([]*provider.PlatformWebhook, error) {
	encoded := encodeProjectPath(owner, repo)
	var whs []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	}
	if err := p.doRequest(ctx, "GET", "/projects/"+encoded+"/hooks", nil, &whs); err != nil {
		return nil, err
	}
	result := make([]*provider.PlatformWebhook, 0, len(whs))
	for _, wh := range whs {
		result = append(result, &provider.PlatformWebhook{ID: int64(wh.ID), URL: wh.URL})
	}
	return result, nil
}

// ValidateWebhookSignature implements provider.WebhookManager. Tencent 工蜂
// uses a static token comparison against the X-Token header.
func (p *Provider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	token := r.Header.Get("X-Token")
	if token == "" {
		return provider.Wrapf(provider.PlatformTencentCode, "ValidateWebhookSignature",
			"%w: missing X-Token header", provider.ErrWebhookValidation)
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
		return provider.Wrapf(provider.PlatformTencentCode, "ValidateWebhookSignature",
			"%w: invalid Tencent Code webhook token", provider.ErrWebhookValidation)
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

	var pl struct {
		ObjectKind string `json:"object_kind"`
		User       struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"user"`
		Repository struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"repository"`
		Project struct {
			PathWithNS string `json:"path_with_namespace"`
		} `json:"project"`
		ObjectAttributes struct {
			IID          int    `json:"iid"`
			Title        string `json:"title"`
			Description  string `json:"description"`
			State        string `json:"state"`
			SourceBranch string `json:"source_branch"`
			TargetBranch string `json:"target_branch"`
			Action       string `json:"action"`
			MergeStatus  string `json:"merge_status"`
			URL          string `json:"url"`
			LastCommit   struct {
				ID string `json:"id"`
			} `json:"last_commit"`
			CreatedAt tcTime `json:"created_at"`
			UpdatedAt tcTime `json:"updated_at"`
		} `json:"object_attributes"`
		Ref        string `json:"ref"`
		Before     string `json:"before"`
		After      string `json:"after"`
		TotalCount int    `json:"total_commits_count"`
		Commits    []struct {
			ID string `json:"id"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(body, &pl); err != nil {
		return nil, provider.Wrap(provider.PlatformTencentCode, "ParseWebhookEvent", err)
	}

	repoName := pl.Project.PathWithNS
	if repoName == "" {
		repoName = pl.Repository.Name
	}
	er := provider.BuildEventRepo(repoName)
	actor := &provider.CRUser{ID: int64(pl.User.ID), Username: pl.User.Username, Name: pl.User.Name}

	event := &provider.NormalizedEvent{
		ID:        fmt.Sprintf("tc-%d-%d", time.Now().UnixNano(), pl.ObjectAttributes.IID),
		Source:    p.Platform(),
		Timestamp: time.Now(),
		Actor:     actor,
		Repo:      er,
		RawPayload: json.RawMessage(body),
	}

	switch pl.ObjectKind {
	case "merge_request":
		state := mapState(pl.ObjectAttributes.State)
		action := pl.ObjectAttributes.Action
		if action == "merge" {
			action = "merged"
		}
		event.Type = "cr." + action
		event.CommitSHA = pl.ObjectAttributes.LastCommit.ID
		event.CR = &provider.ChangeRequest{
			ID:           int64(pl.ObjectAttributes.IID),
			Number:       pl.ObjectAttributes.IID,
			Title:        pl.ObjectAttributes.Title,
			Description:  pl.ObjectAttributes.Description,
			State:        state,
			SourceBranch: pl.ObjectAttributes.SourceBranch,
			TargetBranch: pl.ObjectAttributes.TargetBranch,
			MergeStatus:  pl.ObjectAttributes.MergeStatus,
			WebURL:       pl.ObjectAttributes.URL,
			Author:       actor,
			CreatedAt:    pl.ObjectAttributes.CreatedAt.Time,
			UpdatedAt:    pl.ObjectAttributes.UpdatedAt.Time,
		}
	case "push":
		event.Type = "push"
		event.Branch = strings.TrimPrefix(pl.Ref, "refs/heads/")
		event.CommitSHA = pl.After
	case "tag_push":
		event.Type = "tag.created"
		event.Tag = strings.TrimPrefix(pl.Ref, "refs/tags/")
	case "issue":
		event.Type = "issue"
	case "note":
		event.Type = "comment"
	}
	return event, nil
}

var _ provider.WebhookManager = (*Provider)(nil)