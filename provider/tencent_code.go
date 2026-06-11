package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func encodeProjectPath(owner, repo string) string {
	return url.PathEscape(owner + "/" + repo)
}

type tencentCodeProvider struct {
	*baseProvider
	skipTLS bool
}

func NewTencentCodeProvider(baseURL, token string, skipTLS bool) *tencentCodeProvider {
	if baseURL == "" {
		baseURL = "https://git.code.tencent.com/api/v3"
	}
	bp := newBaseProvider(baseURL, token, skipTLS, authHeaderPrivateToken, "Tencent Code")
	if skipTLS {
		bp.client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
				MaxVersion:         tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				},
			},
		}
	}
	return &tencentCodeProvider{baseProvider: bp, skipTLS: skipTLS}
}

func (t *tencentCodeProvider) Platform() Platform { return PlatformTencentCode }

func (t *tencentCodeProvider) TestConnection(ctx context.Context) (*TestConnectionResult, error) {
	var user struct {
		Username string `json:"username"`
	}
	if err := t.doRequest(ctx, "GET", "/user", nil, &user); err != nil {
		return &TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}

	result := &TestConnectionResult{
		Connected: true,
		Platform:  string(t.Platform()),
		UserName:  user.Username,
	}

	_, listErr := t.ListRepos(ctx, ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = listErr == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos

	return result, nil
}

func (t *tencentCodeProvider) ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error) {
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
		Visibility    int    `json:"visibility_level"`
	}
	if err := t.doRequest(ctx, "GET", path, nil, &projects); err != nil {
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
			DefaultBranch: p.DefaultBranch, Private: p.Visibility == 0, Platform: t.Platform(),
		})
	}
	return repos, nil
}

func (t *tencentCodeProvider) GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error) {
	encoded := encodeProjectPath(owner, repo)
	var p struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		DefaultBranch string `json:"default_branch"`
		Visibility    int    `json:"visibility_level"`
	}
	if err := t.doRequest(ctx, "GET", "/projects/"+encoded, nil, &p); err != nil {
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
		DefaultBranch: p.DefaultBranch, Private: p.Visibility == 0, Platform: t.Platform(),
	}, nil
}

func (t *tencentCodeProvider) CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error) {
	encoded := encodeProjectPath(opts.Owner, opts.Repo)
	body := map[string]interface{}{
		"source_branch": opts.SourceBranch, "target_branch": opts.TargetBranch,
		"title": opts.Title, "description": opts.Description,
		"remove_source_branch": opts.RemoveSourceBranch,
	}
	if len(opts.Labels) > 0 {
		body["labels"] = strings.Join(opts.Labels, ",")
	}
	var mr tcMR
	if err := t.doRequest(ctx, "POST", "/projects/"+encoded+"/merge_requests", body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

func (t *tencentCodeProvider) GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
	var mr tcMR
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), nil, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

func (t *tencentCodeProvider) ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error) {
	encoded := encodeProjectPath(opts.Owner, opts.Repo)
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
	var mrs []tcMR
	headers, err := t.doRequestWithHeaders(ctx, "GET", path, nil, &mrs)
	if err != nil {
		return nil, 0, err
	}
	crs := make([]*ChangeRequest, 0, len(mrs))
	for i := range mrs {
		crs = append(crs, mrs[i].toCR())
	}
	return crs, parseTotalCount(headers, len(crs)), nil
}

func (t *tencentCodeProvider) MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
	var existingMR tcMR
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), nil, &existingMR); err == nil {
		if existingMR.MergeStatus != "" && existingMR.MergeStatus != "can_be_merged" && existingMR.MergeStatus != "checking" {
			return nil, fmt.Errorf("MR cannot be merged (status: %s). It may have conflicts or an active pipeline", existingMR.MergeStatus)
		}
		if mapTCState(existingMR.State) != CRStateOpened {
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
	var mr tcMR
	if err := t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d/merge", encoded, number), body, &mr); err != nil {
		return nil, fmt.Errorf("merge failed: %w", err)
	}
	return mr.toCR(), nil
}

func (t *tencentCodeProvider) CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]interface{}{"state_event": "close"}
	var mr tcMR
	if err := t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

func (t *tencentCodeProvider) CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	encoded := encodeProjectPath(opts.Owner, opts.Repo)
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
	if err := t.doRequest(ctx, "POST", "/projects/"+encoded+"/hooks", body, &wh); err != nil {
		return nil, err
	}
	return &PlatformWebhook{ID: int64(wh.ID), URL: wh.URL}, nil
}

func (t *tencentCodeProvider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/hooks/%d", encoded, webhookID), nil, nil)
}

func (t *tencentCodeProvider) ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error) {
	encoded := encodeProjectPath(owner, repo)
	var whs []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	}
	if err := t.doRequest(ctx, "GET", "/projects/"+encoded+"/hooks", nil, &whs); err != nil {
		return nil, err
	}
	result := make([]*PlatformWebhook, 0, len(whs))
	for _, wh := range whs {
		result = append(result, &PlatformWebhook{ID: int64(wh.ID), URL: wh.URL})
	}
	return result, nil
}

func (t *tencentCodeProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	encoded := encodeProjectPath(owner, repo)
	var branches []struct {
		Name string `json:"name"`
	}
	if err := t.doRequest(ctx, "GET", "/projects/"+encoded+"/repository/branches", nil, &branches); err != nil {
		return nil, err
	}
	result := make([]*PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, &PlatformBranch{Name: b.Name})
	}
	return result, nil
}

func (t *tencentCodeProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]interface{}{
		"branch": branch,
		"ref":    ref,
	}
	var b struct {
		Name string `json:"name"`
	}
	if err := t.doRequest(ctx, "POST", "/projects/"+encoded+"/repository/branches", body, &b); err != nil {
		return nil, err
	}
	return &PlatformBranch{Name: b.Name}, nil
}

func (t *tencentCodeProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/repository/branches/%s", encoded, branch), nil, nil)
}

func (t *tencentCodeProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	encoded := encodeProjectPath(owner, repo)
	var changes struct {
		Changes []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			Diff        string `json:"diff"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
		} `json:"changes"`
	}
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d/changes", encoded, number), nil, &changes); err != nil {
		return nil, err
	}
	files := make([]*ChangedFile, 0, len(changes.Changes))
	totalAdd, totalDel := 0, 0
	for _, c := range changes.Changes {
		add, del := countDiffLines(c.Diff)
		totalAdd += add
		totalDel += del
		files = append(files, &ChangedFile{
			OldPath: c.OldPath, NewPath: c.NewPath, Diff: c.Diff,
			Additions: add, Deletions: del,
			IsNew: c.NewFile, IsDeleted: c.DeletedFile, IsRenamed: c.RenamedFile,
		})
	}
	return &MergeDiff{Files: files, TotalAdd: totalAdd, TotalDel: totalDel}, nil
}

func (t *tencentCodeProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	diff, err := t.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

func (t *tencentCodeProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	encoded := encodeProjectPath(owner, repo)
	payload := map[string]interface{}{"body": body}
	var resp struct {
		ID int `json:"id"`
	}
	if err := t.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/merge_requests/%d/notes", encoded, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

func (t *tencentCodeProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/merge_requests/%d/notes/%s", encoded, number, noteID), nil, nil)
}

func (t *tencentCodeProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	return t.CreateNote(ctx, owner, repo, number, opts.Body)
}

func (t *tencentCodeProvider) CreateReview(ctx context.Context, owner, repo string, number int, opts CreateReviewOptions) (*ReviewResult, error) {
	noteID, err := t.CreateNote(ctx, owner, repo, number, opts.Body)
	if err != nil {
		return nil, err
	}
	return &ReviewResult{ID: noteID}, nil
}

func (t *tencentCodeProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	encoded := encodeProjectPath(owner, repo)
	payload := map[string]interface{}{
		"state":       opts.State,
		"context":     opts.Context,
		"description": opts.Description,
	}
	if opts.TargetURL != "" {
		payload["target_url"] = opts.TargetURL
	}
	return t.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/statuses/%s", encoded, sha), payload, nil)
}

func (t *tencentCodeProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	encoded := encodeProjectPath(owner, repo)
	params := ""
	if ref != "" {
		params = "?ref=" + ref
	}
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/files/%s%s", encoded, path, params), nil, &resp); err != nil {
		return "", err
	}
	if resp.Encoding == "base64" {
		content := strings.ReplaceAll(resp.Content, "\n", "")
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}
	return resp.Content, nil
}

func (t *tencentCodeProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]interface{}{
		"labels": strings.Join(labels, ","),
	}
	return t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), body, nil)
}

func (t *tencentCodeProvider) ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error) {
	if err := t.ValidateWebhookSignature(r, secret); err != nil {
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
		return nil, err
	}

	repoName := pl.Project.PathWithNS
	if repoName == "" {
		repoName = pl.Repository.Name
	}
	parts := strings.SplitN(repoName, "/", 2)
	er := &EventRepo{FullName: repoName}
	if len(parts) == 2 {
		er.Owner = parts[0]
		er.Name = parts[1]
	}
	actor := &CRUser{ID: int64(pl.User.ID), Username: pl.User.Username, Name: pl.User.Name}

	event := &NormalizedEvent{
		ID:     fmt.Sprintf("tc-%d-%d", time.Now().UnixNano(), pl.ObjectAttributes.IID),
		Source: t.Platform(), Timestamp: time.Now(), Actor: actor, Repo: er,
	}

	switch pl.ObjectKind {
	case "merge_request":
		state := mapTCState(pl.ObjectAttributes.State)
		action := pl.ObjectAttributes.Action
		if action == "merge" {
			action = "merged"
		}
		event.Type = "cr." + action
		event.CommitSHA = pl.ObjectAttributes.LastCommit.ID
		event.CR = &ChangeRequest{
			ID: int64(pl.ObjectAttributes.IID), Number: pl.ObjectAttributes.IID,
			Title: pl.ObjectAttributes.Title, Description: pl.ObjectAttributes.Description,
			State: state, SourceBranch: pl.ObjectAttributes.SourceBranch,
			TargetBranch: pl.ObjectAttributes.TargetBranch, MergeStatus: pl.ObjectAttributes.MergeStatus,
			WebURL: pl.ObjectAttributes.URL, Author: actor,
			CreatedAt: pl.ObjectAttributes.CreatedAt.Time, UpdatedAt: pl.ObjectAttributes.UpdatedAt.Time,
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

func (t *tencentCodeProvider) ValidateWebhookSignature(r *http.Request, secret string) error {
	token := r.Header.Get("X-Token")
	if token == "" || token != secret {
		return fmt.Errorf("invalid Tencent Code webhook token")
	}
	return nil
}

type tcMR struct {
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
	Labels      []string `json:"labels"`
	MergeStatus string   `json:"merge_status"`
	WebURL      string   `json:"web_url"`
	CreatedAt   tcTime   `json:"created_at"`
	UpdatedAt   tcTime   `json:"updated_at"`
}

type tcTime struct {
	time.Time
}

func (t *tcTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), "\"")
	if s == "null" || s == "" {
		return nil
	}
	formats := []string{
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if parsed, err := time.Parse(f, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("cannot parse time %q", s)
}

func (mr *tcMR) toCR() *ChangeRequest {
	return &ChangeRequest{
		ID: int64(mr.IID), Number: mr.IID, Title: mr.Title, Description: mr.Description,
		State: mapTCState(mr.State), SourceBranch: mr.SourceBranch, TargetBranch: mr.TargetBranch,
		Author: &CRUser{ID: int64(mr.Author.ID), Username: mr.Author.Username, Name: mr.Author.Name},
		Labels: mr.Labels, MergeStatus: mr.MergeStatus, WebURL: mr.WebURL,
		CreatedAt: mr.CreatedAt.Time, UpdatedAt: mr.UpdatedAt.Time,
	}
}

func mapTCState(state string) CRState {
	switch state {
	case "merged":
		return CRStateMerged
	case "closed":
		return CRStateClosed
	default:
		return CRStateOpened
	}
}

func (t *tencentCodeProvider) UpdateCR(ctx context.Context, owner, repo string, number int, opts UpdateCROptions) (*ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
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
	var mr tcMR
	if err := t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

func (t *tencentCodeProvider) ReopenCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]interface{}{"state_event": "reopen"}
	var mr tcMR
	if err := t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

func (t *tencentCodeProvider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*CRComment, error) {
	encoded := encodeProjectPath(owner, repo)
	var notes []struct {
		ID        int    `json:"id"`
		Body      string `json:"body"`
		Author    struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"author"`
		CreatedAt tcTime `json:"created_at"`
		UpdatedAt tcTime `json:"updated_at"`
	}
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d/notes", encoded, number), nil, &notes); err != nil {
		return nil, err
	}
	result := make([]*CRComment, 0, len(notes))
	for _, n := range notes {
		result = append(result, &CRComment{
			ID: int64(n.ID), Body: n.Body,
			Author:    &CRUser{ID: int64(n.Author.ID), Username: n.Author.Username, Name: n.Author.Name},
			CreatedAt: n.CreatedAt.Time, UpdatedAt: n.UpdatedAt.Time,
		})
	}
	return result, nil
}

func (t *tencentCodeProvider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*CRCommit, error) {
	encoded := encodeProjectPath(owner, repo)
	var commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	}
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d/commits", encoded, number), nil, &commits); err != nil {
		return nil, err
	}
	result := make([]*CRCommit, 0, len(commits))
	for _, c := range commits {
		result = append(result, &CRCommit{SHA: c.ID, Message: c.Message, Author: &CRUser{Name: c.Author.Name}})
	}
	return result, nil
}

func (t *tencentCodeProvider) ForkRepo(ctx context.Context, owner, repo string, opts ForkRepoOptions) (*PlatformRepo, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]interface{}{}
	if opts.Organization != "" {
		body["namespace"] = opts.Organization
	}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	var p struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := t.doRequest(ctx, "POST", "/projects/"+encoded+"/fork", body, &p); err != nil {
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
		DefaultBranch: p.DefaultBranch, Platform: t.Platform(),
	}, nil
}

func (t *tencentCodeProvider) DeleteRepo(ctx context.Context, owner, repo string) error {
	encoded := encodeProjectPath(owner, repo)
	return t.doRequest(ctx, "DELETE", "/projects/"+encoded, nil, nil)
}

func (t *tencentCodeProvider) UpdateRepo(ctx context.Context, owner, repo string, opts UpdateRepoOptions) (*PlatformRepo, error) {
	encoded := encodeProjectPath(owner, repo)
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
		if *opts.Private {
			body["visibility_level"] = 0
		} else {
			body["visibility_level"] = 20
		}
	}
	var p struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := t.doRequest(ctx, "PUT", "/projects/"+encoded, body, &p); err != nil {
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
		DefaultBranch: p.DefaultBranch, Platform: t.Platform(),
	}, nil
}

func (t *tencentCodeProvider) GetCommit(ctx context.Context, owner, repo, sha string) (*CommitInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	var c struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
		CreatedAt tcTime `json:"created_at"`
	}
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/commits/%s", encoded, sha), nil, &c); err != nil {
		return nil, err
	}
	return &CommitInfo{SHA: c.ID, Message: c.Message, Author: &CRUser{Name: c.Author.Name}, CreatedAt: c.CreatedAt.Time}, nil
}

func (t *tencentCodeProvider) ListCommits(ctx context.Context, owner, repo string, opts ListCommitsOptions) ([]*CommitInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	path := fmt.Sprintf("/projects/%s/repository/commits", encoded)
	if opts.Page > 0 || opts.PerPage > 0 {
		page := opts.Page
		perPage := opts.PerPage
		if page == 0 {
			page = 1
		}
		if perPage == 0 {
			perPage = 20
		}
		path += fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	}
	var commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
		CreatedAt tcTime `json:"created_at"`
	}
	if err := t.doRequest(ctx, "GET", path, nil, &commits); err != nil {
		return nil, err
	}
	result := make([]*CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, &CommitInfo{SHA: c.ID, Message: c.Message, Author: &CRUser{Name: c.Author.Name}, CreatedAt: c.CreatedAt.Time})
	}
	return result, nil
}

func (t *tencentCodeProvider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error) {
	encoded := encodeProjectPath(owner, repo)
	var cmp struct {
		Commits []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"commits"`
		Diffs []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			Diff        string `json:"diff"`
			NewFile     bool   `json:"new_file"`
			DeletedFile bool   `json:"deleted_file"`
			RenamedFile bool   `json:"renamed_file"`
		} `json:"diffs"`
	}
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/compare?from=%s&to=%s", encoded, base, head), nil, &cmp); err != nil {
		return nil, err
	}
	result := &CompareResult{TotalCommits: len(cmp.Commits)}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, &CommitInfo{SHA: c.ID, Message: c.Message})
	}
	for _, d := range cmp.Diffs {
		add, del := countDiffLines(d.Diff)
		result.Files = append(result.Files, &ChangedFile{
			OldPath: d.OldPath, NewPath: d.NewPath, Diff: d.Diff,
			Additions: add, Deletions: del, IsNew: d.NewFile, IsDeleted: d.DeletedFile, IsRenamed: d.RenamedFile,
		})
	}
	return result, nil
}

func (t *tencentCodeProvider) CreateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]interface{}{
		"file_path": opts.Path, "content": opts.Content, "commit_message": opts.Message,
	}
	if opts.Branch != "" {
		body["branch"] = opts.Branch
	}
	if opts.Author != "" || opts.Email != "" {
		body["author_name"] = opts.Author
		body["author_email"] = opts.Email
	}
	var resp struct {
		CommitID string `json:"commit_id"`
	}
	if err := t.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/repository/files", encoded), body, &resp); err != nil {
		return nil, err
	}
	return &FileResult{CommitSHA: resp.CommitID}, nil
}

func (t *tencentCodeProvider) UpdateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]interface{}{
		"file_path": opts.Path, "content": opts.Content, "commit_message": opts.Message,
	}
	if opts.Branch != "" {
		body["branch"] = opts.Branch
	}
	if opts.Author != "" || opts.Email != "" {
		body["author_name"] = opts.Author
		body["author_email"] = opts.Email
	}
	var resp struct {
		CommitID string `json:"commit_id"`
	}
	if err := t.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/repository/files", encoded), body, &resp); err != nil {
		return nil, err
	}
	return &FileResult{CommitSHA: resp.CommitID}, nil
}

func (t *tencentCodeProvider) DeleteFile(ctx context.Context, owner, repo string, opts FileDeleteOptions) (*FileResult, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]interface{}{
		"file_path": opts.Path, "commit_message": opts.Message,
	}
	if opts.Branch != "" {
		body["branch"] = opts.Branch
	}
	var resp struct {
		CommitID string `json:"commit_id"`
	}
	if err := t.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/repository/files", encoded), body, &resp); err != nil {
		return nil, err
	}
	return &FileResult{CommitSHA: resp.CommitID}, nil
}

func (t *tencentCodeProvider) ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	var tags []struct {
		Name    string `json:"name"`
		Commit  struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/tags", encoded), nil, &tags); err != nil {
		return nil, err
	}
	result := make([]*TagInfo, 0, len(tags))
	for _, tg := range tags {
		result = append(result, &TagInfo{Name: tg.Name, Commit: tg.Commit.ID})
	}
	return result, nil
}

func (t *tencentCodeProvider) ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	var releases []struct {
		ID          int    `json:"id"`
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := t.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/releases", encoded), nil, &releases); err != nil {
		return nil, err
	}
	result := make([]*ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, &ReleaseInfo{ID: int64(r.ID), TagName: r.TagName, Title: r.Name, Body: r.Description})
	}
	return result, nil
}

func (t *tencentCodeProvider) CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]interface{}{
		"tag_name": opts.TagName, "name": opts.Title, "description": opts.Body,
	}
	if opts.Target != "" {
		body["target_commitish"] = opts.Target
	}
	var r struct {
		ID          int    `json:"id"`
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := t.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/releases", encoded), body, &r); err != nil {
		return nil, err
	}
	return &ReleaseInfo{ID: int64(r.ID), TagName: r.TagName, Title: r.Name, Body: r.Description}, nil
}

func (t *tencentCodeProvider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	encoded := encodeProjectPath(owner, repo)
	suffix := "tar.gz"
	if format == "zip" {
		suffix = "zip"
	}
	return t.doRawRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/archive.%s?sha=%s", encoded, suffix, ref))
}
