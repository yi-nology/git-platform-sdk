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

func (g *giteaProvider) ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error) {
	if err := g.ValidateWebhookSignature(r, secret); err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))

	eventType := r.Header.Get("X-Gitea-Event")
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
			Merged  bool   `json:"merged"`
			HTMLURL string `json:"html_url"`
			User    struct {
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
		return nil, err
	}

	parts := strings.SplitN(pl.Repository.FullName, "/", 2)
	er := &EventRepo{FullName: pl.Repository.FullName}
	if len(parts) == 2 {
		er.Owner = parts[0]
		er.Name = parts[1]
	}
	actor := &CRUser{ID: int64(pl.Sender.ID), Username: pl.Sender.Login}

	event := &NormalizedEvent{
		ID:     fmt.Sprintf("gt-%d-%d", time.Now().UnixNano(), pl.Number),
		Source: g.Platform(), Timestamp: time.Now(), Actor: actor, Repo: er,
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
			event.CR = &ChangeRequest{
				ID: int64(pl.PullRequest.Number), Number: pl.PullRequest.Number,
				Title: pl.PullRequest.Title, Description: pl.PullRequest.Body,
				State:        mapGiteaState(pl.PullRequest.State, pl.PullRequest.Merged),
				SourceBranch: pl.PullRequest.Head.Ref, TargetBranch: pl.PullRequest.Base.Ref,
				WebURL:    pl.PullRequest.HTMLURL,
				Author:    &CRUser{ID: int64(pl.PullRequest.User.ID), Username: pl.PullRequest.User.Login},
				CreatedAt: pl.PullRequest.CreatedAt, UpdatedAt: pl.PullRequest.UpdatedAt,
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

func (g *giteaProvider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	sig := r.Header.Get("X-Gitea-Signature")
	if sig == "" {
		return fmt.Errorf("missing X-Gitea-Signature header")
	}
	return nil
}
