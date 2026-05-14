package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

type forgejoProvider struct {
	client *forgejo.Client
}

func NewForgejoProvider(baseURL, token string, skipTLS bool) *forgejoProvider {
	if baseURL == "" {
		baseURL = "https://codeberg.org"
	}
	baseURL = strings.TrimSuffix(strings.TrimSuffix(baseURL, "/"), "/api/v1")
	transport := &http.Transport{}
	if skipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	httpClient := &http.Client{Timeout: 30 * time.Second, Transport: transport}
	client, err := forgejo.NewClient(baseURL, forgejo.SetToken(token), forgejo.SetHTTPClient(httpClient))
	if err != nil {
		return &forgejoProvider{}
	}
	return &forgejoProvider{client: client}
}

func (f *forgejoProvider) Platform() Platform { return PlatformForgejo }

func (f *forgejoProvider) TestConnection(ctx context.Context) (*TestConnectionResult, error) {
	user, _, err := f.client.GetMyUserInfo()
	if err != nil {
		return &TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &TestConnectionResult{
		Connected: true,
		Platform:  string(f.Platform()),
		UserName:  user.UserName,
	}
	_, err = f.ListRepos(ctx, ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}

func (f *forgejoProvider) ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error) {
	if opts.Page == 0 {
		opts.Page = 1
	}
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	if opts.Owner != "" {
		results, _, err := f.client.SearchRepos(forgejo.SearchRepoOptions{
			ListOptions: forgejo.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
		})
		if err != nil {
			return nil, err
		}
		filtered := make([]*PlatformRepo, 0)
		for _, r := range results {
			pr := convertForgejoRepo(r)
			if strings.EqualFold(pr.Owner, opts.Owner) {
				filtered = append(filtered, pr)
			}
		}
		return filtered, nil
	}
	reposRaw, _, err := f.client.ListMyRepos(forgejo.ListReposOptions{
		ListOptions: forgejo.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
	})
	if err != nil {
		return nil, err
	}
	repos := make([]*PlatformRepo, 0, len(reposRaw))
	for _, r := range reposRaw {
		repos = append(repos, convertForgejoRepo(r))
	}
	return repos, nil
}

func (f *forgejoProvider) GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error) {
	r, _, err := f.client.GetRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	return convertForgejoRepo(r), nil
}

func (f *forgejoProvider) CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error) {
	pr, _, err := f.client.CreatePullRequest(opts.Owner, opts.Repo, forgejo.CreatePullRequestOption{
		Head:  opts.SourceBranch,
		Base:  opts.TargetBranch,
		Title: opts.Title,
		Body:  opts.Description,
	})
	if err != nil {
		return nil, err
	}
	return convertForgejoPR(pr), nil
}

func (f *forgejoProvider) GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	pr, _, err := f.client.GetPullRequest(owner, repo, int64(number))
	if err != nil {
		return nil, err
	}
	return convertForgejoPR(pr), nil
}

func (f *forgejoProvider) ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error) {
	if opts.Page == 0 {
		opts.Page = 1
	}
	if opts.PerPage == 0 {
		opts.PerPage = 20
	}
	prs, resp, err := f.client.ListRepoPullRequests(opts.Owner, opts.Repo, forgejo.ListPullRequestsOptions{
		State:       forgejo.StateType(opts.State),
		ListOptions: forgejo.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
	})
	if err != nil {
		return nil, 0, err
	}
	crs := make([]*ChangeRequest, 0, len(prs))
	for _, pr := range prs {
		crs = append(crs, convertForgejoPR(pr))
	}
	total := parseForgejoTotalCount(resp)
	if total < len(crs) {
		total = len(crs)
	}
	return crs, total, nil
}

func (f *forgejoProvider) MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error) {
	style := forgejo.MergeStyleMerge
	if opts.Squash {
		style = forgejo.MergeStyleSquash
	}
	_, resp, err := f.client.MergePullRequest(owner, repo, int64(number), forgejo.MergePullRequestOption{
		Style:                  style,
		Title:                  opts.MergeCommitMessage,
		DeleteBranchAfterMerge: opts.RemoveSourceBranch,
	})
	if err != nil {
		if resp != nil && resp.StatusCode == 405 {
			cr, getErr := f.GetCR(ctx, owner, repo, number)
			if getErr == nil && cr.State == CRStateMerged {
				return cr, nil
			}
		}
		return nil, err
	}
	return f.GetCR(ctx, owner, repo, number)
}

func (f *forgejoProvider) CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	state := forgejo.StateClosed
	_, _, err := f.client.EditPullRequest(owner, repo, int64(number), forgejo.EditPullRequestOption{
		State: &state,
	})
	if err != nil {
		return nil, err
	}
	return f.GetCR(ctx, owner, repo, number)
}

func (f *forgejoProvider) CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	events := opts.Events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}
	hook, _, err := f.client.CreateRepoHook(opts.Owner, opts.Repo, forgejo.CreateHookOption{
		Type:   forgejo.HookTypeForgejo,
		Config: map[string]string{"url": opts.URL, "content_type": "json", "secret": opts.Secret},
		Events: events,
		Active: true,
	})
	if err != nil {
		return nil, err
	}
	return convertForgejoHook(hook), nil
}

func (f *forgejoProvider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	_, err := f.client.DeleteRepoHook(owner, repo, webhookID)
	return err
}

func (f *forgejoProvider) ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error) {
	hooks, _, err := f.client.ListRepoHooks(owner, repo, forgejo.ListHooksOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, convertForgejoHook(h))
	}
	return result, nil
}

func (f *forgejoProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	branches, _, err := f.client.ListRepoBranches(owner, repo, forgejo.ListRepoBranchesOptions{
		ListOptions: forgejo.ListOptions{PageSize: 100},
	})
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, convertForgejoBranch(b))
	}
	return result, nil
}

func (f *forgejoProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	b, _, err := f.client.CreateBranch(owner, repo, forgejo.CreateBranchOption{
		BranchName:    branch,
		OldBranchName: ref,
	})
	if err != nil {
		return nil, err
	}
	return convertForgejoBranch(b), nil
}

func (f *forgejoProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	_, _, err := f.client.DeleteRepoBranch(owner, repo, branch)
	return err
}

func (f *forgejoProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	diffBytes, _, err := f.client.GetPullRequestDiff(owner, repo, int64(number), forgejo.PullRequestDiffOptions{})
	if err != nil {
		return nil, err
	}
	rawDiff := string(diffBytes)

	files, err := f.GetCRFiles(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	totalAdd, totalDel := 0, 0
	for _, fi := range files {
		totalAdd += fi.Additions
		totalDel += fi.Deletions
	}
	return &MergeDiff{Files: files, TotalAdd: totalAdd, TotalDel: totalDel, RawDiff: rawDiff}, nil
}

func (f *forgejoProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	changedFiles, _, err := f.client.ListPullRequestFiles(owner, repo, int64(number), forgejo.ListPullRequestFilesOptions{
		ListOptions: forgejo.ListOptions{PageSize: 100},
	})
	if err != nil {
		return nil, err
	}
	result := make([]*ChangedFile, 0, len(changedFiles))
	for _, fi := range changedFiles {
		cf := &ChangedFile{
			OldPath:   fi.PreviousFilename,
			NewPath:   fi.Filename,
			Additions: fi.Additions,
			Deletions: fi.Deletions,
			IsNew:     fi.Status == "added",
			IsDeleted: fi.Status == "removed",
			IsRenamed: fi.Status == "renamed",
		}
		if cf.OldPath == "" {
			cf.OldPath = cf.NewPath
		}
		result = append(result, cf)
	}
	return result, nil
}

func (f *forgejoProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	comment, _, err := f.client.CreateIssueComment(owner, repo, int64(number), forgejo.CreateIssueCommentOption{
		Body: body,
	})
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

func (f *forgejoProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	id, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return err
	}
	resp, err := f.client.DeleteIssueComment(owner, repo, id)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil
		}
	}
	return err
}

func (f *forgejoProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	comment, _, err := f.client.CreateIssueComment(owner, repo, int64(number), forgejo.CreateIssueCommentOption{
		Body: opts.Body,
	})
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

func (f *forgejoProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	stateMap := map[string]forgejo.StatusState{
		"success": forgejo.StatusSuccess,
		"failed":  forgejo.StatusFailure,
		"pending": forgejo.StatusPending,
		"error":   forgejo.StatusError,
	}
	state := stateMap[opts.State]
	if state == "" {
		state = forgejo.StatusPending
	}
	_, _, err := f.client.CreateStatus(owner, repo, sha, forgejo.CreateStatusOption{
		State:       state,
		Context:     opts.Context,
		Description: opts.Description,
		TargetURL:   opts.TargetURL,
	})
	return err
}

func (f *forgejoProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	data, _, err := f.client.GetFile(owner, repo, ref, path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *forgejoProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	labelIDs := make([]int64, 0, len(labels))
	for _, l := range labels {
		if id, err := strconv.ParseInt(l, 10, 64); err == nil {
			labelIDs = append(labelIDs, id)
		}
	}
	_, _, err := f.client.AddIssueLabels(owner, repo, int64(number), forgejo.IssueLabelsOption{
		Labels: labelIDs,
	})
	return err
}

func (f *forgejoProvider) ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error) {
	if err := f.ValidateWebhookSignature(r, secret); err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))

	eventType := r.Header.Get("X-Forgejo-Event")
	if eventType == "" {
		eventType = r.Header.Get("X-Gitea-Event")
	}
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
			Merged   bool      `json:"merged"`
			HTMLURL  string    `json:"html_url"`
			User     struct {
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
		ID:        fmt.Sprintf("fj-%d-%d", time.Now().UnixNano(), pl.Number),
		Source:    f.Platform(),
		Timestamp: time.Now(),
		Actor:     actor,
		Repo:      er,
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
				State:        mapForgejoState(pl.PullRequest.State, pl.PullRequest.Merged),
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

func (f *forgejoProvider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	sig := r.Header.Get("X-Forgejo-Signature")
	if sig == "" {
		sig = r.Header.Get("X-Gitea-Signature")
	}
	if sig == "" {
		return fmt.Errorf("missing webhook signature header")
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("invalid webhook signature")
	}
	return nil
}

func convertForgejoRepo(r *forgejo.Repository) *PlatformRepo {
	if r == nil {
		return nil
	}
	owner := ""
	if r.Owner != nil {
		owner = r.Owner.UserName
	}
	return &PlatformRepo{
		ID:            r.ID,
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         owner,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      PlatformForgejo,
	}
}

func convertForgejoPR(pr *forgejo.PullRequest) *ChangeRequest {
	if pr == nil {
		return nil
	}
	var author *CRUser
	if pr.Poster != nil {
		author = &CRUser{
			ID:        pr.Poster.ID,
			Username:  pr.Poster.UserName,
			AvatarURL: pr.Poster.AvatarURL,
		}
	}
	var labels []string
	for _, l := range pr.Labels {
		if l != nil {
			labels = append(labels, l.Name)
		}
	}
	var createdAt, updatedAt time.Time
	if pr.Created != nil {
		createdAt = *pr.Created
	}
	if pr.Updated != nil {
		updatedAt = *pr.Updated
	}
	return &ChangeRequest{
		ID:           pr.ID,
		Number:       int(pr.Index),
		Title:        pr.Title,
		Description:  pr.Body,
		State:        mapForgejoState(string(pr.State), pr.HasMerged),
		SourceBranch: pr.Head.Ref,
		TargetBranch: pr.Base.Ref,
		Author:       author,
		Labels:       labels,
		WebURL:       pr.HTMLURL,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func convertForgejoBranch(b *forgejo.Branch) *PlatformBranch {
	if b == nil {
		return nil
	}
	return &PlatformBranch{Name: b.Name}
}

func convertForgejoHook(h *forgejo.Hook) *PlatformWebhook {
	if h == nil {
		return nil
	}
	return &PlatformWebhook{
		ID:     h.ID,
		URL:    h.Config["url"],
		Events: h.Events,
	}
}

func mapForgejoState(state string, merged bool) CRState {
	if merged {
		return CRStateMerged
	}
	if state == "closed" {
		return CRStateClosed
	}
	return CRStateOpened
}

func parseForgejoTotalCount(resp *forgejo.Response) int {
	if resp == nil {
		return 0
	}
	totalStr := resp.Header.Get("X-Total-Count")
	if totalStr == "" {
		return 0
	}
	n, err := strconv.Atoi(totalStr)
	if err != nil {
		return 0
	}
	return n
}
