package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (g *gitlabProvider) ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error) {
	if err := g.ValidateWebhookSignature(r, secret); err != nil {
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
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"object_attributes"`
		Ref   string `json:"ref"`
		After string `json:"after"`
	}
	if err := json.Unmarshal(body, &pl); err != nil {
		return nil, err
	}

	parts := strings.SplitN(pl.Project.PathWithNS, "/", 2)
	er := &EventRepo{FullName: pl.Project.PathWithNS}
	if len(parts) == 2 {
		er.Owner = parts[0]
		er.Name = parts[1]
	}
	actor := &CRUser{ID: int64(pl.User.ID), Username: pl.User.Username, Name: pl.User.Name}

	event := &NormalizedEvent{
		ID:     fmt.Sprintf("gl-%d-%d", time.Now().UnixNano(), pl.ObjectAttributes.IID),
		Source: g.Platform(), Timestamp: time.Now(), Actor: actor, Repo: er,
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
		event.CR = &ChangeRequest{
			ID: int64(pl.ObjectAttributes.IID), Number: pl.ObjectAttributes.IID,
			Title: pl.ObjectAttributes.Title, Description: pl.ObjectAttributes.Description,
			State: state, SourceBranch: pl.ObjectAttributes.SourceBranch,
			TargetBranch: pl.ObjectAttributes.TargetBranch, MergeStatus: pl.ObjectAttributes.MergeStatus,
			WebURL: pl.ObjectAttributes.URL, Author: actor,
			CreatedAt: pl.ObjectAttributes.CreatedAt, UpdatedAt: pl.ObjectAttributes.UpdatedAt,
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
			event.CR = &ChangeRequest{
				ID: int64(pl.ObjectAttributes.IID), Number: pl.ObjectAttributes.IID,
			}
		}
	}
	return event, nil
}

func (g *gitlabProvider) ValidateWebhookSignature(r *http.Request, secret string) error {
	token := r.Header.Get("X-Gitlab-Token")
	if token == "" || token != secret {
		return fmt.Errorf("invalid GitLab webhook token")
	}
	return nil
}
