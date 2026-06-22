package provider

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

	"github.com/yi-nology/git-platform-sdk/pkg/encoding"
)

type giteeProvider struct {
	base *baseProvider
}

func init() {
	Register(PlatformGitee, func(cfg Config) (Provider, error) {
		return newGiteeProvider(cfg)
	})
}

func newGiteeProvider(cfg Config) (Provider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://gitee.com/api/v5"
	}
	if !strings.Contains(baseURL, "/api/v5") {
		baseURL = strings.TrimSuffix(baseURL, "/") + "/api/v5"
	}
	opts := configBaseOptions(cfg)
	return &giteeProvider{
		base: newBaseProvider(baseURL, cfg.Token, cfg.SkipTLS, authHeaderBearer, "Gitee", opts...),
	}, nil
}

func (g *giteeProvider) Platform() Platform { return PlatformGitee }

func (g *giteeProvider) TestConnection(ctx context.Context) (*TestConnectionResult, error) {
	var user struct {
		Login string `json:"login"`
	}
	if err := g.base.doRequest(ctx, "GET", "/user", nil, &user); err != nil {
		return &TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &TestConnectionResult{
		Connected: true,
		Platform:  string(g.Platform()),
		UserName:  user.Login,
	}
	_, err := g.ListRepos(ctx, ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}

// --- Repos ---

func (g *giteeProvider) ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error) {
	page, perPage := NormalizePageOpts(opts.Page, opts.PerPage)
	var path string
	if opts.Owner != "" {
		path = fmt.Sprintf("/users/%s/repos?page=%d&per_page=%d", opts.Owner, page, perPage)
	} else {
		path = fmt.Sprintf("/user/repos?page=%d&per_page=%d", page, perPage)
	}
	var repos []giteeRepo
	if err := g.base.doRequest(ctx, "GET", path, nil, &repos); err != nil {
		return nil, err
	}
	result := make([]*PlatformRepo, 0, len(repos))
	for _, r := range repos {
		result = append(result, r.toPlatformRepo())
	}
	return result, nil
}

func (g *giteeProvider) GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error) {
	var r giteeRepo
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s", owner, repo), nil, &r); err != nil {
		return nil, err
	}
	return r.toPlatformRepo(), nil
}

func (g *giteeProvider) DeleteRepo(ctx context.Context, owner, repo string) error {
	return g.base.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s", owner, repo), nil, nil)
}

func (g *giteeProvider) UpdateRepo(ctx context.Context, owner, repo string, opts UpdateRepoOptions) (*PlatformRepo, error) {
	body := map[string]interface{}{}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.DefaultBranch != "" {
		body["default_branch"] = opts.DefaultBranch
	}
	if opts.Private != nil {
		body["private"] = *opts.Private
	}
	var r giteeRepo
	if err := g.base.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s", owner, repo), body, &r); err != nil {
		return nil, err
	}
	return r.toPlatformRepo(), nil
}

func (g *giteeProvider) ForkRepo(ctx context.Context, owner, repo string, opts ForkRepoOptions) (*PlatformRepo, error) {
	body := map[string]interface{}{}
	if opts.Organization != "" {
		body["organization"] = opts.Organization
	}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	var r giteeRepo
	if err := g.base.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/forks", owner, repo), body, &r); err != nil {
		return nil, err
	}
	return r.toPlatformRepo(), nil
}

// --- Change Requests ---

func (g *giteeProvider) CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error) {
	body := map[string]interface{}{
		"source_branch": opts.SourceBranch,
		"target_branch": opts.TargetBranch,
		"title":         opts.Title,
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if len(opts.Labels) > 0 {
		body["labels"] = strings.Join(opts.Labels, ",")
	}
	var pr giteePR
	if err := g.base.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls", opts.Owner, opts.Repo), body, &pr); err != nil {
		return nil, err
	}
	return pr.toChangeRequest(), nil
}

func (g *giteeProvider) GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	var pr giteePR
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), nil, &pr); err != nil {
		return nil, err
	}
	return pr.toChangeRequest(), nil
}

func (g *giteeProvider) ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error) {
	page, perPage := NormalizePageOpts(opts.Page, opts.PerPage)
	state := string(opts.State)
	if state == "" {
		state = "open"
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls?page=%d&per_page=%d&state=%s", opts.Owner, opts.Repo, page, perPage, state)
	if opts.SourceBranch != "" {
		path += "&source_branch=" + opts.SourceBranch
	}
	if opts.TargetBranch != "" {
		path += "&target_branch=" + opts.TargetBranch
	}
	var prs []giteePR
	headers, err := g.base.doRequestWithHeaders(ctx, "GET", path, nil, &prs)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*ChangeRequest, 0, len(prs))
	for i := range prs {
		result = append(result, prs[i].toChangeRequest())
	}
	return result, ParseTotalCountHeader(headers, len(result)), nil
}

func (g *giteeProvider) MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error) {
	body := map[string]interface{}{}
	if opts.MergeCommitMessage != "" {
		body["merge_message"] = opts.MergeCommitMessage
	}
	if opts.Squash {
		body["squash"] = true
	}
	var pr giteePR
	if err := g.base.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, number), body, &pr); err != nil {
		return nil, err
	}
	return pr.toChangeRequest(), nil
}

func (g *giteeProvider) CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	body := map[string]interface{}{"state": "closed"}
	var pr giteePR
	if err := g.base.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), body, &pr); err != nil {
		return nil, err
	}
	return pr.toChangeRequest(), nil
}

func (g *giteeProvider) ReopenCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	body := map[string]interface{}{"state": "open"}
	var pr giteePR
	if err := g.base.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), body, &pr); err != nil {
		return nil, err
	}
	return pr.toChangeRequest(), nil
}

func (g *giteeProvider) UpdateCR(ctx context.Context, owner, repo string, number int, opts UpdateCROptions) (*ChangeRequest, error) {
	body := map[string]interface{}{}
	if opts.Title != "" {
		body["title"] = opts.Title
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.TargetBranch != "" {
		body["target_branch"] = opts.TargetBranch
	}
	var pr giteePR
	if err := g.base.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), body, &pr); err != nil {
		return nil, err
	}
	return pr.toChangeRequest(), nil
}

func (g *giteeProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	body := map[string]interface{}{
		"labels": strings.Join(labels, ","),
	}
	return g.base.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), body, nil)
}

func (g *giteeProvider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*CRComment, error) {
	var comments []giteeComment
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number), nil, &comments); err != nil {
		return nil, err
	}
	result := make([]*CRComment, 0, len(comments))
	for _, c := range comments {
		result = append(result, c.toCRComment())
	}
	return result, nil
}

func (g *giteeProvider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*CRCommit, error) {
	var commits []giteeCommit
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/commits", owner, repo, number), nil, &commits); err != nil {
		return nil, err
	}
	result := make([]*CRCommit, 0, len(commits))
	for _, c := range commits {
		result = append(result, c.toCRCommit())
	}
	return result, nil
}

// --- Webhooks ---

func (g *giteeProvider) CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	body := map[string]interface{}{
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
	if err := g.base.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/hooks", opts.Owner, opts.Repo), body, &hook); err != nil {
		return nil, err
	}
	return hook.toPlatformWebhook(), nil
}

func (g *giteeProvider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	return g.base.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/hooks/%d", owner, repo, webhookID), nil, nil)
}

func (g *giteeProvider) ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error) {
	var hooks []giteeWebhook
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/hooks", owner, repo), nil, &hooks); err != nil {
		return nil, err
	}
	result := make([]*PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, h.toPlatformWebhook())
	}
	return result, nil
}

func (g *giteeProvider) ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error) {
	if err := g.ValidateWebhookSignature(r, secret); err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))

	var pl struct {
		Action      string `json:"action"`
		ActionDesc  string `json:"action_desc"`
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		State       string `json:"state"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		HTMLURL     string `json:"html_url"`
		User        struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
			Name  string `json:"name"`
		} `json:"user"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Ref         string `json:"ref"`
		After       string `json:"after"`
		Target      *struct {
			FullName string `json:"full_name"`
		} `json:"target"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &pl); err != nil {
		return nil, err
	}

	hookName := r.Header.Get("X-Gitea-Event")
	er := BuildEventRepo(pl.Repository.FullName)
	actor := &CRUser{ID: int64(pl.User.ID), Username: pl.User.Login, Name: pl.User.Name}

	event := &NormalizedEvent{
		ID:         fmt.Sprintf("ge-%d-%d", time.Now().UnixNano(), pl.Number),
		Source:     g.Platform(),
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
		event.CR = &ChangeRequest{
			Number:       pl.Number,
			Title:        pl.Title,
			Description:  pl.Body,
			State:        MapBoolStateToCR(pl.State, pl.State == "merged"),
			SourceBranch: pl.SourceBranch,
			TargetBranch: pl.TargetBranch,
			WebURL:       pl.HTMLURL,
			Author:       actor,
			CreatedAt:    pl.CreatedAt,
			UpdatedAt:    pl.UpdatedAt,
		}
	case "Push Hook":
		event.Type = "push"
		event.Branch = strings.TrimPrefix(pl.Ref, "refs/heads/")
		event.CommitSHA = pl.After
	case "Tag Push Hook":
		event.Type = "tag.created"
		event.Tag = strings.TrimPrefix(pl.Ref, "refs/tags/")
	case "note":
		event.Type = "comment"
	}
	return event, nil
}

func (g *giteeProvider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	sig := r.Header.Get("X-Gitea-Token")
	if sig == "" {
		sig = r.Header.Get("X-Gitee-Token")
	}
	if sig == "" {
		return fmt.Errorf("%w: missing webhook token header", ErrWebhookValidation)
	}
	if subtle.ConstantTimeCompare([]byte(sig), []byte(secret)) != 1 {
		return fmt.Errorf("%w: invalid webhook token", ErrWebhookValidation)
	}
	return nil
}

// --- Branches ---

func (g *giteeProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	var branches []struct {
		Name string `json:"name"`
	}
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/branches", owner, repo), nil, &branches); err != nil {
		return nil, err
	}
	result := make([]*PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, &PlatformBranch{Name: b.Name})
	}
	return result, nil
}

func (g *giteeProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	body := map[string]interface{}{
		"branch_name": branch,
		"refs":        ref,
	}
	if err := g.base.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/branches", owner, repo), body, nil); err != nil {
		return nil, err
	}
	return &PlatformBranch{Name: branch}, nil
}

func (g *giteeProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	return g.base.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/branches/%s", owner, repo, branch), nil, nil)
}

// --- Diffs ---

func (g *giteeProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	var files []giteePRFile
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/files", owner, repo, number), nil, &files); err != nil {
		return nil, err
	}
	diff := &MergeDiff{}
	for _, f := range files {
		cf := f.toChangedFile()
		diff.Files = append(diff.Files, cf)
		diff.TotalAdd += cf.Additions
		diff.TotalDel += cf.Deletions
	}
	diff.RawDiff = BuildRawDiff(diff.Files)
	return diff, nil
}

func (g *giteeProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	diff, err := g.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

func (g *giteeProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	payload := map[string]interface{}{"body": body}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := g.base.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

func (g *giteeProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	return g.base.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments/%s", owner, repo, number, noteID), nil, nil)
}

func (g *giteeProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	payload := map[string]interface{}{
		"body":      opts.Body,
		"path":      opts.FilePath,
		"new_line":  opts.NewLine,
		"old_line":  opts.OldLine,
	}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := g.base.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

func (g *giteeProvider) CreateReview(ctx context.Context, owner, repo string, number int, opts CreateReviewOptions) (*ReviewResult, error) {
	// Gitee doesn't have a native review API, use commit status + comments
	if opts.Body != "" {
		if _, err := g.CreateNote(ctx, owner, repo, number, opts.Body); err != nil {
			return nil, err
		}
	}
	for _, c := range opts.Comments {
		discOpts := DiscussionOptions{
			Body:     c.Body,
			FilePath: c.Path,
			NewLine:  c.Line,
		}
		if c.StartLine > 0 {
			discOpts.NewLine = c.EndLine
			discOpts.OldLine = c.StartLine
		}
		if c.Side == "LEFT" {
			discOpts.OldLine = c.Line
			discOpts.NewLine = 0
		}
		if _, err := g.CreateDiscussion(ctx, owner, repo, number, discOpts); err != nil {
			return nil, err
		}
	}
	return &ReviewResult{ID: fmt.Sprintf("ge-review-%d-%d", number, time.Now().UnixNano())}, nil
}

func (g *giteeProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	body := map[string]interface{}{
		"state":       opts.State,
		"context":     opts.Context,
		"description": opts.Description,
	}
	if opts.TargetURL != "" {
		// Gitee uses "target_url" in the statuses endpoint
		body["target_url"] = opts.TargetURL
	}
	return g.base.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/commits/%s/statuses", owner, repo, sha), body, nil)
}

// --- Files ---

func (g *giteeProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	params := ""
	if ref != "" {
		params = "?ref=" + ref
	}
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/contents/%s%s", owner, repo, path, params), nil, &resp); err != nil {
		return "", err
	}
	if resp.Encoding == "base64" {
		decoded, err := encoding.Base64URLDecode(resp.Content)
		if err != nil {
			return "", err
		}
		return decoded, nil
	}
	return resp.Content, nil
}

func (g *giteeProvider) CreateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	body := map[string]interface{}{
		"content":         encoding.Base64URLEncode(opts.Content),
		"commit_message":  opts.Message,
	}
	if opts.Branch != "" {
		body["branch"] = opts.Branch
	}
	if opts.Author != "" {
		body["author_name"] = opts.Author
	}
	if opts.Email != "" {
		body["author_email"] = opts.Email
	}
	var resp struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
		Content struct {
			SHA string `json:"sha"`
		} `json:"content"`
	}
	if err := g.base.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, opts.Path), body, &resp); err != nil {
		return nil, err
	}
	return &FileResult{SHA: resp.Content.SHA, CommitSHA: resp.Commit.SHA}, nil
}

func (g *giteeProvider) UpdateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	body := map[string]interface{}{
		"content":         encoding.Base64URLEncode(opts.Content),
		"commit_message":  opts.Message,
	}
	if opts.SHA != "" {
		body["sha"] = opts.SHA
	}
	if opts.Branch != "" {
		body["branch"] = opts.Branch
	}
	if opts.Author != "" {
		body["author_name"] = opts.Author
	}
	if opts.Email != "" {
		body["author_email"] = opts.Email
	}
	var resp struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
		Content struct {
			SHA string `json:"sha"`
		} `json:"content"`
	}
	if err := g.base.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, opts.Path), body, &resp); err != nil {
		return nil, err
	}
	return &FileResult{SHA: resp.Content.SHA, CommitSHA: resp.Commit.SHA}, nil
}

func (g *giteeProvider) DeleteFile(ctx context.Context, owner, repo string, opts FileDeleteOptions) (*FileResult, error) {
	body := map[string]interface{}{
		"commit_message": opts.Message,
	}
	if opts.SHA != "" {
		body["sha"] = opts.SHA
	}
	if opts.Branch != "" {
		body["branch"] = opts.Branch
	}
	if opts.Author != "" {
		body["author_name"] = opts.Author
	}
	if opts.Email != "" {
		body["author_email"] = opts.Email
	}
	var resp struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := g.base.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, opts.Path), body, &resp); err != nil {
		return nil, err
	}
	return &FileResult{CommitSHA: resp.Commit.SHA}, nil
}

// --- Commits ---

func (g *giteeProvider) GetCommit(ctx context.Context, owner, repo, sha string) (*CommitInfo, error) {
	var c giteeCommitDetail
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/commits/%s", owner, repo, sha), nil, &c); err != nil {
		return nil, err
	}
	return c.toCommitInfo(), nil
}

func (g *giteeProvider) ListCommits(ctx context.Context, owner, repo string, opts ListCommitsOptions) ([]*CommitInfo, error) {
	page, perPage := NormalizePageOpts(opts.Page, opts.PerPage)
	path := fmt.Sprintf("/repos/%s/%s/commits?page=%d&per_page=%d", owner, repo, page, perPage)
	if opts.Branch != "" {
		path += "&sha=" + opts.Branch
	}
	if opts.Since != "" {
		path += "&since=" + opts.Since
	}
	if opts.Until != "" {
		path += "&until=" + opts.Until
	}
	var commits []giteeCommitDetail
	if err := g.base.doRequest(ctx, "GET", path, nil, &commits); err != nil {
		return nil, err
	}
	result := make([]*CommitInfo, 0, len(commits))
	for i := range commits {
		result = append(result, commits[i].toCommitInfo())
	}
	return result, nil
}

func (g *giteeProvider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error) {
	var cmp struct {
		Commits []giteeCommitDetail `json:"commits"`
		Files   []giteePRFile       `json:"files"`
	}
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/compare/%s...%s", owner, repo, base, head), nil, &cmp); err != nil {
		return nil, err
	}
	result := &CompareResult{TotalCommits: len(cmp.Commits)}
	for i := range cmp.Commits {
		result.Commits = append(result.Commits, cmp.Commits[i].toCommitInfo())
	}
	for _, f := range cmp.Files {
		result.Files = append(result.Files, f.toChangedFile())
	}
	return result, nil
}

// --- Tags & Releases ---

func (g *giteeProvider) ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error) {
	var tags []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/tags", owner, repo), nil, &tags); err != nil {
		return nil, err
	}
	result := make([]*TagInfo, 0, len(tags))
	for _, t := range tags {
		result = append(result, &TagInfo{Name: t.Name, Commit: t.Commit.SHA})
	}
	return result, nil
}

func (g *giteeProvider) ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error) {
	var releases []giteeRelease
	if err := g.base.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/releases", owner, repo), nil, &releases); err != nil {
		return nil, err
	}
	result := make([]*ReleaseInfo, 0, len(releases))
	for i := range releases {
		result = append(result, releases[i].toReleaseInfo())
	}
	return result, nil
}

func (g *giteeProvider) CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error) {
	body := map[string]interface{}{
		"tag_name": opts.TagName,
		"name":     opts.Title,
	}
	if opts.Target != "" {
		body["target_commitish"] = opts.Target
	}
	if opts.Body != "" {
		body["body"] = opts.Body
	}
	if opts.Draft {
		body["draft"] = true
	}
	if opts.Prerelease {
		body["prerelease"] = true
	}
	var r giteeRelease
	if err := g.base.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/releases", owner, repo), body, &r); err != nil {
		return nil, err
	}
	return r.toReleaseInfo(), nil
}

func (g *giteeProvider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	archiveFormat := "zipball"
	if format == "tar.gz" {
		archiveFormat = "tarball"
	}
	return g.base.doRawRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/%s/%s", owner, repo, archiveFormat, ref))
}

// --- Gitee API types ---

type giteeRepo struct {
	ID            int    `json:"id"`
	FullName      string `json:"full_name"`
	Name          string `json:"name"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
	Description   string `json:"description"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
}

func (r *giteeRepo) toPlatformRepo() *PlatformRepo {
	return &PlatformRepo{
		ID:            int64(r.ID),
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         r.Owner.Login,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      PlatformGitee,
	}
}

type giteePR struct {
	ID            int    `json:"id"`
	Number        int    `json:"number"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	State         string `json:"state"`
	SourceBranch  string `json:"source_branch"`
	TargetBranch  string `json:"target_branch"`
	HTMLURL       string `json:"html_url"`
	Head          struct {
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		SHA string `json:"sha"`
	} `json:"base"`
	User struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignees []struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	} `json:"assignees"`
	Mergeable    bool      `json:"mergeable"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MergedAt     *time.Time `json:"merged_at"`
}

func (pr *giteePR) toChangeRequest() *ChangeRequest {
	state := MapBoolStateToCR(pr.State, pr.MergedAt != nil)
	var labels []string
	for _, l := range pr.Labels {
		labels = append(labels, l.Name)
	}
	var reviewers []*CRUser
	for _, a := range pr.Assignees {
		reviewers = append(reviewers, &CRUser{ID: int64(a.ID), Username: a.Login})
	}
	mergeStatus := "unknown"
	if pr.Mergeable {
		mergeStatus = "mergeable"
	} else {
		mergeStatus = "conflicting"
	}
	return &ChangeRequest{
		ID:           int64(pr.ID),
		Number:       pr.Number,
		Title:        pr.Title,
		Description:  pr.Body,
		State:        state,
		SourceBranch: pr.SourceBranch,
		TargetBranch: pr.TargetBranch,
		HeadSHA:      pr.Head.SHA,
		BaseSHA:      pr.Base.SHA,
		Author:       &CRUser{ID: int64(pr.User.ID), Username: pr.User.Login, Name: pr.User.Name},
		Reviewers:    reviewers,
		Labels:       labels,
		MergeStatus:  mergeStatus,
		WebURL:       pr.HTMLURL,
		CreatedAt:    pr.CreatedAt,
		UpdatedAt:    pr.UpdatedAt,
	}
}

type giteeComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	User      struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	} `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *giteeComment) toCRComment() *CRComment {
	return &CRComment{
		ID:        c.ID,
		Body:      c.Body,
		Author:    &CRUser{ID: int64(c.User.ID), Username: c.User.Login, Name: c.User.Name},
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

type giteeCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Author struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	} `json:"author"`
}

func (c *giteeCommit) toCRCommit() *CRCommit {
	cc := &CRCommit{
		SHA:       c.SHA,
		Message:   c.Commit.Message,
		CreatedAt: c.Commit.Author.Date,
	}
	if c.Author.ID > 0 {
		cc.Author = &CRUser{ID: int64(c.Author.ID), Username: c.Author.Login}
	} else if c.Commit.Author.Name != "" {
		cc.Author = &CRUser{Name: c.Commit.Author.Name}
	}
	return cc
}

type giteeCommitDetail struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"committer"`
	} `json:"commit"`
	Author    *struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	} `json:"author"`
	Committer *struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	} `json:"committer"`
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
	} `json:"stats"`
}

func (c *giteeCommitDetail) toCommitInfo() *CommitInfo {
	ci := &CommitInfo{
		SHA:       c.SHA,
		Message:   c.Commit.Message,
		CreatedAt: c.Commit.Author.Date,
		Additions: c.Stats.Additions,
		Deletions: c.Stats.Deletions,
	}
	if c.Author != nil {
		ci.Author = &CRUser{ID: int64(c.Author.ID), Username: c.Author.Login, Name: c.Commit.Author.Name}
	} else if c.Commit.Author.Name != "" {
		ci.Author = &CRUser{Name: c.Commit.Author.Name}
	}
	if c.Committer != nil {
		ci.Committer = &CRUser{ID: int64(c.Committer.ID), Username: c.Committer.Login, Name: c.Commit.Committer.Name}
	}
	return ci
}

type giteePRFile struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	Diff        string `json:"diff"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

func (f *giteePRFile) toChangedFile() *ChangedFile {
	add, del := CountDiffLines(f.Diff)
	if f.Additions > 0 {
		add = f.Additions
	}
	if f.Deletions > 0 {
		del = f.Deletions
	}
	return &ChangedFile{
		OldPath:   f.OldPath,
		NewPath:   f.NewPath,
		Diff:      f.Diff,
		Additions: add,
		Deletions: del,
		IsNew:     f.NewFile,
		IsDeleted: f.DeletedFile,
		IsRenamed: f.RenamedFile,
	}
}

type giteeWebhook struct {
	ID     int64    `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

func (h *giteeWebhook) toPlatformWebhook() *PlatformWebhook {
	return &PlatformWebhook{
		ID:     h.ID,
		URL:    h.URL,
		Events: h.Events,
	}
}

type giteeRelease struct {
	ID          int64      `json:"id"`
	TagName     string     `json:"tag_name"`
	Name        string     `json:"name"`
	Body        string     `json:"body"`
	HTMLURL     string     `json:"html_url"`
	Draft       bool       `json:"draft"`
	Prerelease  bool       `json:"prerelease"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at"`
}

func (r *giteeRelease) toReleaseInfo() *ReleaseInfo {
	ri := &ReleaseInfo{
		ID:         r.ID,
		TagName:    r.TagName,
		Title:      r.Name,
		Body:       r.Body,
		URL:        r.HTMLURL,
		Draft:      r.Draft,
		Prerelease: r.Prerelease,
		CreatedAt:  r.CreatedAt,
	}
	if r.PublishedAt != nil {
		ri.PublishedAt = *r.PublishedAt
	}
	return ri
}


