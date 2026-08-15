package gitlab

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateWebhook implements provider.WebhookManager.
func (p *Provider) CreateWebhook(ctx context.Context, opts provider.CreateWebhookOptions) (*provider.PlatformWebhook, error) {
	pid := pidOf(opts.Owner, opts.Repo)
	hookOpts := &gitlab.AddProjectHookOptions{
		URL:   gitlab.Ptr(opts.URL),
		Token: gitlab.Ptr(opts.Secret),
	}
	hookOpts.PushEvents = gitlab.Ptr(true)
	if len(opts.Events) > 0 {
		em := map[string]bool{}
		for _, e := range opts.Events {
			em[e] = true
		}
		if v, ok := em["push"]; ok {
			hookOpts.PushEvents = gitlab.Ptr(v)
		}
		hookOpts.MergeRequestsEvents = gitlab.Ptr(em["merge_request"] || em["merge_requests"] || em["pull_request"] || em["cr"])
		hookOpts.TagPushEvents = gitlab.Ptr(em["tag_push"] || em["tag"])
	}
	hook, _, err := p.client.Projects.AddProjectHook(pid, hookOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateWebhook", err)
	}
	return convertHook(hook), nil
}

// DeleteWebhook implements provider.WebhookManager.
func (p *Provider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	_, err := p.client.Projects.DeleteProjectHook(pidOf(owner, repo), webhookID, gitlab.WithContext(ctx))
	if err != nil {
		return provider.Wrap(provider.PlatformGitLab, "DeleteWebhook", err)
	}
	return nil
}

// ListWebhooks implements provider.WebhookManager.
func (p *Provider) ListWebhooks(ctx context.Context, owner, repo string) ([]*provider.PlatformWebhook, error) {
	hooks, _, err := p.client.Projects.ListProjectHooks(pidOf(owner, repo), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListWebhooks", err)
	}
	result := make([]*provider.PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, convertHook(h))
	}
	return result, nil
}

// ValidateWebhookSignature implements provider.WebhookManager. GitLab uses a
// static-token comparison against the X-Gitlab-Token header.
func (p *Provider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	token := r.Header.Get("X-Gitlab-Token")
	if token == "" {
		return provider.Wrapf(provider.PlatformGitLab, "ValidateWebhookSignature", "%w: missing X-Gitlab-Token header", provider.ErrWebhookValidation)
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
		return provider.Wrapf(provider.PlatformGitLab, "ValidateWebhookSignature", "%w: invalid GitLab webhook token", provider.ErrWebhookValidation)
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
		return nil, provider.Wrap(provider.PlatformGitLab, "ParseWebhookEvent", readErr)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var pl struct {
		ObjectKind string `json:"object_kind"`
		User       struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"user"`
		Project struct {
			ID         int64  `json:"id"`
			PathWithNS string `json:"path_with_namespace"`
		} `json:"project"`
		ObjectAttributes struct {
			IID            int64  `json:"iid"`
			Title          string `json:"title"`
			Description    string `json:"description"`
			State          string `json:"state"`
			SourceBranch   string `json:"source_branch"`
			TargetBranch   string `json:"target_branch"`
			Action         string `json:"action"`
			MergeStatus    string `json:"merge_status"`
			URL            string `json:"url"`
			MergeCommitSHA string `json:"merge_commit_sha"`
			WorkInProgress bool   `json:"work_in_progress"`
			LastCommit     struct {
				ID string `json:"id"`
			} `json:"last_commit"`
			DiffRefs struct {
				BaseSHA  string `json:"base_sha"`
				StartSHA string `json:"start_sha"`
				HeadSHA  string `json:"head_sha"`
			} `json:"diff_refs"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"object_attributes"`
		Ref   string `json:"ref"`
		After string `json:"after"`
	}
	if err := json.Unmarshal(body, &pl); err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ParseWebhookEvent", err)
	}

	er := provider.BuildEventRepo(pl.Project.PathWithNS)
	er.ID = pl.Project.ID
	actor := &provider.CRUser{ID: pl.User.ID, Username: pl.User.Username, Name: pl.User.Name}

	event := &provider.NormalizedEvent{
		ID:         fmt.Sprintf("gl-%d-%d", time.Now().UnixNano(), pl.ObjectAttributes.IID),
		RawPayload: json.RawMessage(body),
		Source:     p.Platform(),
		Timestamp:  time.Now(),
		Actor:      actor,
		Repo:       er,
	}

	switch pl.ObjectKind {
	case "merge_request":
		state := mapGLState(pl.ObjectAttributes.State)
		action := pl.ObjectAttributes.Action
		if action == "merge" {
			action = "merged"
		}
		event.Type = "cr." + action
		event.Action = action
		event.CommitSHA = pl.ObjectAttributes.LastCommit.ID
		headSHA, baseSHA, startSHA := provider.ResolveMRSHAs(
			pl.ObjectAttributes.DiffRefs.HeadSHA,
			pl.ObjectAttributes.DiffRefs.BaseSHA,
			pl.ObjectAttributes.DiffRefs.StartSHA,
			pl.ObjectAttributes.MergeCommitSHA,
			pl.ObjectAttributes.LastCommit.ID,
		)
		event.CR = &provider.ChangeRequest{
			ID:           pl.ObjectAttributes.IID,
			Number:       strconv.FormatInt(pl.ObjectAttributes.IID, 10),
			Title:        pl.ObjectAttributes.Title,
			Description:  pl.ObjectAttributes.Description,
			State:        state,
			Draft:        pl.ObjectAttributes.WorkInProgress,
			SourceBranch: pl.ObjectAttributes.SourceBranch,
			TargetBranch: pl.ObjectAttributes.TargetBranch,
			HeadSHA:      headSHA,
			BaseSHA:      baseSHA,
			StartSHA:     startSHA,
			MergeStatus:  pl.ObjectAttributes.MergeStatus,
			WebURL:       pl.ObjectAttributes.URL,
			Author:       actor,
			CreatedAt:    pl.ObjectAttributes.CreatedAt,
			UpdatedAt:    pl.ObjectAttributes.UpdatedAt,
		}
	case "push":
		event.Type = "push"
		event.Action = "push"
		event.Branch = strings.TrimPrefix(pl.Ref, "refs/heads/")
		event.CommitSHA = pl.After
	case "tag_push":
		event.Type = "tag.created"
		event.Tag = strings.TrimPrefix(pl.Ref, "refs/tags/")
	case "note":
		if pl.ObjectAttributes.IID != 0 {
			event.Type = "cr.note"
			event.Action = "note"
			event.CR = &provider.ChangeRequest{
				ID:     pl.ObjectAttributes.IID,
				Number: strconv.FormatInt(pl.ObjectAttributes.IID, 10),
			}
		}
	}
	return event, nil
}

var _ provider.WebhookManager = (*Provider)(nil)
