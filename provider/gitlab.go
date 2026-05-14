package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type gitlabProvider struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewGitLabProvider(baseURL, token string, skipTLS bool) *gitlabProvider {
	if baseURL == "" {
		baseURL = "https://gitlab.com/api/v4"
	}
	transport := &http.Transport{}
	if skipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &gitlabProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}
}

func (g *gitlabProvider) Platform() Platform { return PlatformGitLab }

func (g *gitlabProvider) TestConnection(ctx context.Context) (*TestConnectionResult, error) {
	var user struct {
		Username string `json:"username"`
	}
	if err := g.doRequest(ctx, "GET", "/user", nil, &user); err != nil {
		return &TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}

	result := &TestConnectionResult{
		Connected: true,
		Platform:  string(g.Platform()),
		UserName:  user.Username,
	}

	// 权限探测
	// 1. 尝试列出项目
	_, err := g.ListRepos(ctx, ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil

	// 2. 其他权限简化处理（基于 list repos 成功推断）
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos

	return result, nil
}

func (g *gitlabProvider) ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error) {
	path := "/projects"
	if opts.Owner != "" {
		path = fmt.Sprintf("/groups/%s/projects", opts.Owner)
	}
	if opts.Page == 0 {
		opts.Page = 1
	}
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	path = fmt.Sprintf("%s?page=%d&per_page=%d", path, opts.Page, opts.PerPage)
	var projects []struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		DefaultBranch string `json:"default_branch"`
		Visibility    string `json:"visibility"`
	}
	if err := g.doRequest(ctx, "GET", path, nil, &projects); err != nil {
		return nil, err
	}
	repos := make([]*PlatformRepo, 0, len(projects))
	for _, p := range projects {
		parts := strings.SplitN(p.PathWithNS, "/", 2)
		owner := ""
		if len(parts) == 2 {
			owner = parts[0]
		}
		repos = append(repos, &PlatformRepo{
			ID: int64(p.ID), FullName: p.PathWithNS, Name: p.Name, Owner: owner,
			Description: p.Description, CloneURL: p.HTTPURL, SSHURL: p.SSHURL,
			DefaultBranch: p.DefaultBranch, Private: p.Visibility != "public", Platform: g.Platform(),
		})
	}
	return repos, nil
}

func (g *gitlabProvider) GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	var p struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		DefaultBranch string `json:"default_branch"`
		Visibility    string `json:"visibility"`
	}
	if err := g.doRequest(ctx, "GET", "/projects/"+encoded, nil, &p); err != nil {
		return nil, err
	}
	parts := strings.SplitN(p.PathWithNS, "/", 2)
	ownerR := ""
	if len(parts) == 2 {
		ownerR = parts[0]
	}
	return &PlatformRepo{
		ID: int64(p.ID), FullName: p.PathWithNS, Name: p.Name, Owner: ownerR,
		Description: p.Description, CloneURL: p.HTTPURL, SSHURL: p.SSHURL,
		DefaultBranch: p.DefaultBranch, Private: p.Visibility != "public", Platform: g.Platform(),
	}, nil
}

func (g *gitlabProvider) CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error) {
	encoded := fmt.Sprintf("%s%%2F%s", opts.Owner, opts.Repo)
	body := map[string]interface{}{
		"source_branch": opts.SourceBranch, "target_branch": opts.TargetBranch,
		"title": opts.Title, "description": opts.Description,
		"remove_source_branch": opts.RemoveSourceBranch,
	}
	if len(opts.Labels) > 0 {
		body["labels"] = strings.Join(opts.Labels, ",")
	}
	var mr gitlabMR
	if err := g.doRequest(ctx, "POST", "/projects/"+encoded+"/merge_requests", body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

func (g *gitlabProvider) GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	var mr gitlabMR
	if err := g.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), nil, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

func (g *gitlabProvider) ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error) {
	encoded := fmt.Sprintf("%s%%2F%s", opts.Owner, opts.Repo)
	if opts.Page == 0 {
		opts.Page = 1
	}
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	path := fmt.Sprintf("/projects/%s/merge_requests?page=%d&per_page=%d", encoded, opts.Page, opts.PerPage)
	if opts.State != "" {
		path += "&state=" + string(opts.State)
	}
	if opts.SourceBranch != "" {
		path += "&source_branch=" + opts.SourceBranch
	}
	if opts.TargetBranch != "" {
		path += "&target_branch=" + opts.TargetBranch
	}
	var mrs []gitlabMR
	if err := g.doRequest(ctx, "GET", path, nil, &mrs); err != nil {
		return nil, 0, err
	}
	crs := make([]*ChangeRequest, 0, len(mrs))
	for i := range mrs {
		crs = append(crs, mrs[i].toCR())
	}
	return crs, len(crs), nil
}

func (g *gitlabProvider) MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	var existingMR gitlabMR
	if err := g.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), nil, &existingMR); err == nil {
		if existingMR.MergeStatus != "" && existingMR.MergeStatus != "can_be_merged" && existingMR.MergeStatus != "checking" {
			return nil, fmt.Errorf("MR cannot be merged (status: %s). It may have conflicts or an active pipeline", existingMR.MergeStatus)
		}
		if existingMR.State != "opened" {
			return nil, fmt.Errorf("MR is not in 'opened' state (current: %s)", existingMR.State)
		}
	}
	body := map[string]interface{}{}
	if opts.MergeCommitMessage != "" {
		body["merge_commit_message"] = opts.MergeCommitMessage
	}
	if opts.Squash {
		body["squash"] = true
	}
	if opts.RemoveSourceBranch {
		body["should_remove_source_branch"] = true
	}
	var mr gitlabMR
	if err := g.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d/merge", encoded, number), body, &mr); err != nil {
		return nil, fmt.Errorf("merge failed: %w. The MR may have conflicts, an active pipeline, or branch protection rules preventing merge", err)
	}
	return mr.toCR(), nil
}

func (g *gitlabProvider) CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	body := map[string]interface{}{"state_event": "close"}
	var mr gitlabMR
	if err := g.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

func (g *gitlabProvider) CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	encoded := fmt.Sprintf("%s%%2F%s", opts.Owner, opts.Repo)
	body := map[string]interface{}{
		"url": opts.URL, "token": opts.Secret,
		"push_events": true,
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
	if err := g.doRequest(ctx, "POST", "/projects/"+encoded+"/hooks", body, &wh); err != nil {
		return nil, err
	}
	return &PlatformWebhook{ID: int64(wh.ID), URL: wh.URL}, nil
}

func (g *gitlabProvider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	return g.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/hooks/%d", encoded, webhookID), nil, nil)
}

func (g *gitlabProvider) ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	var whs []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	}
	if err := g.doRequest(ctx, "GET", "/projects/"+encoded+"/hooks", nil, &whs); err != nil {
		return nil, err
	}
	result := make([]*PlatformWebhook, 0, len(whs))
	for _, wh := range whs {
		result = append(result, &PlatformWebhook{ID: int64(wh.ID), URL: wh.URL})
	}
	return result, nil
}

func (g *gitlabProvider) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", g.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitLab API %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	if result != nil && resp.StatusCode != http.StatusNoContent {
		return json.Unmarshal(respBody, result)
	}
	return nil
}

type gitlabMR struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Author       struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"author"`
	Labels      []string  `json:"labels"`
	MergeStatus string    `json:"merge_status"`
	WebURL      string    `json:"web_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (mr *gitlabMR) toCR() *ChangeRequest {
	return &ChangeRequest{
		ID: int64(mr.IID), Number: mr.IID, Title: mr.Title, Description: mr.Description,
		State: mapGLState(mr.State), SourceBranch: mr.SourceBranch, TargetBranch: mr.TargetBranch,
		Author: &CRUser{ID: int64(mr.Author.ID), Username: mr.Author.Username, Name: mr.Author.Name},
		Labels: mr.Labels, MergeStatus: mr.MergeStatus, WebURL: mr.WebURL,
		CreatedAt: mr.CreatedAt, UpdatedAt: mr.UpdatedAt,
	}
}

func mapGLState(state string) CRState {
	switch state {
	case "merged":
		return CRStateMerged
	case "closed":
		return CRStateClosed
	default:
		return CRStateOpened
	}
}

func countDiffLines(diff string) (additions, deletions int) {
	for _, line := range strings.Split(diff, "\n") {
		if len(line) == 0 {
			continue
		}
		if line[0] == '+' && !strings.HasPrefix(line, "+++") {
			additions++
		} else if line[0] == '-' && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}
	return
}
