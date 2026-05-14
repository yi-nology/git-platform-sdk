package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (g *giteaProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	path := fmt.Sprintf("/repos/%s/%s/branches?limit=100", owner, repo)
	var branches []struct {
		Name string `json:"name"`
	}
	if err := g.doRequest(ctx, "GET", path, nil, &branches); err != nil {
		return nil, err
	}
	result := make([]*PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, &PlatformBranch{Name: b.Name})
	}
	return result, nil
}

func (g *giteaProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	body := map[string]string{"new_branch_name": branch, "old_branch_name": ref}
	var res struct {
		Name string `json:"name"`
	}
	if err := g.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/branches", owner, repo), body, &res); err != nil {
		return nil, err
	}
	return &PlatformBranch{Name: res.Name}, nil
}

func (g *giteaProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	return g.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/branches/%s", owner, repo, branch), nil, nil)
}

func (g *giteaProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	diffURL := fmt.Sprintf("/repos/%s/%s/pulls/%d.diff", owner, repo, number)
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL+diffURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+g.token)
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Gitea API GET %s returned %d: %s", diffURL, resp.StatusCode, string(body[:minInt(200, len(body))]))
	}

	rawDiff := string(body)
	return &MergeDiff{RawDiff: rawDiff}, nil
}

func (g *giteaProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	diff, err := g.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

func (g *giteaProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	payload := map[string]string{"body": body}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := g.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

func (g *giteaProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	err := g.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/issues/comments/%s", owner, repo, noteID), nil, nil)
	if err != nil && strings.Contains(err.Error(), "404") {
		return nil
	}
	return err
}

func (g *giteaProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	payload := map[string]interface{}{
		"body": opts.Body,
	}
	if opts.FilePath != "" {
		payload["path"] = opts.FilePath
		if opts.NewLine > 0 {
			payload["line"] = opts.NewLine
			payload["side"] = "RIGHT"
		} else if opts.OldLine > 0 {
			payload["line"] = opts.OldLine
			payload["side"] = "LEFT"
		}
	}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := g.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

func (g *giteaProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	stateMap := map[string]string{
		"success": "success",
		"failed":  "failure",
		"pending": "pending",
		"error":   "error",
	}
	state := stateMap[opts.State]
	if state == "" {
		state = "pending"
	}
	payload := map[string]interface{}{
		"state":       state,
		"target_url":  "",
		"description": opts.Description,
		"context":     opts.Context,
	}
	return g.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/statuses/%s", owner, repo, sha), payload, nil)
}

func (g *giteaProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	if ref != "" {
		apiPath += "?ref=" + ref
	}
	var resp struct {
		Content string `json:"content"`
	}
	if err := g.doRequest(ctx, "GET", apiPath, nil, &resp); err != nil {
		return "", err
	}
	decoded, err := decodeGiteaContent(resp.Content)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func (g *giteaProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	payload := map[string]interface{}{"labels": labels}
	return g.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, number), payload, nil)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
