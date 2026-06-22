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

	gitea "code.gitea.io/sdk/gitea"
)

type giteaProvider struct {
	client *gitea.Client
	logger Logger
}

func init() {
	Register(PlatformGitea, func(cfg Config) (Provider, error) {
		return newGiteaProvider(cfg)
	})
}

func newGiteaProvider(cfg Config) (*giteaProvider, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = NewNoopLogger()
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://gitea.com"
	}
	baseURL = strings.TrimSuffix(strings.TrimSuffix(baseURL, "/"), "/api/v1")
	transport := &http.Transport{}
	if cfg.SkipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	httpClient := &http.Client{Timeout: 30 * time.Second, Transport: transport}
	client, err := gitea.NewClient(baseURL, gitea.SetToken(cfg.Token), gitea.SetHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gitea client: %w", err)
	}
	return &giteaProvider{client: client, logger: logger}, nil
}

func (g *giteaProvider) Platform() Platform { return PlatformGitea }

func (g *giteaProvider) TestConnection(ctx context.Context) (*TestConnectionResult, error) {
	user, _, err := g.client.GetMyUserInfo()
	if err != nil {
		return &TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &TestConnectionResult{
		Connected: true,
		Platform:  string(g.Platform()),
		UserName:  user.UserName,
	}
	_, err = g.ListRepos(ctx, ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}

func (g *giteaProvider) ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error) {
	opts.Page, opts.PerPage = NormalizePageOpts(opts.Page, opts.PerPage)
	if opts.Owner != "" {
		results, _, err := g.client.SearchRepos(gitea.SearchRepoOptions{
			ListOptions: gitea.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
		})
		if err != nil {
			return nil, err
		}
		filtered := make([]*PlatformRepo, 0)
		for _, r := range results {
			pr := convertGiteaRepo(r)
			if strings.EqualFold(pr.Owner, opts.Owner) {
				filtered = append(filtered, pr)
			}
		}
		return filtered, nil
	}
	reposRaw, _, err := g.client.ListMyRepos(gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
	})
	if err != nil {
		return nil, err
	}
	repos := make([]*PlatformRepo, 0, len(reposRaw))
	for _, r := range reposRaw {
		repos = append(repos, convertGiteaRepo(r))
	}
	return repos, nil
}

func (g *giteaProvider) GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error) {
	r, _, err := g.client.GetRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	return convertGiteaRepo(r), nil
}

func (g *giteaProvider) CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error) {
	pr, _, err := g.client.CreatePullRequest(opts.Owner, opts.Repo, gitea.CreatePullRequestOption{
		Head:  opts.SourceBranch,
		Base:  opts.TargetBranch,
		Title: opts.Title,
		Body:  opts.Description,
	})
	if err != nil {
		return nil, err
	}
	return convertGiteaPR(pr), nil
}

func (g *giteaProvider) GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	pr, _, err := g.client.GetPullRequest(owner, repo, int64(number))
	if err != nil {
		return nil, err
	}
	return convertGiteaPR(pr), nil
}

func (g *giteaProvider) ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error) {
	opts.Page, opts.PerPage = NormalizePageOpts(opts.Page, opts.PerPage)
	prs, resp, err := g.client.ListRepoPullRequests(opts.Owner, opts.Repo, gitea.ListPullRequestsOptions{
		State:       gitea.StateType(opts.State),
		ListOptions: gitea.ListOptions{Page: opts.Page, PageSize: opts.PerPage},
	})
	if err != nil {
		return nil, 0, err
	}
	crs := make([]*ChangeRequest, 0, len(prs))
	for _, pr := range prs {
		crs = append(crs, convertGiteaPR(pr))
	}
	total := parseGiteaTotalCount(resp)
	if total < len(crs) {
		total = len(crs)
	}
	return crs, total, nil
}

func (g *giteaProvider) MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error) {
	style := gitea.MergeStyleMerge
	if opts.Squash {
		style = gitea.MergeStyleSquash
	}
	deleteBranch := opts.RemoveSourceBranch
	_, resp, err := g.client.MergePullRequest(owner, repo, int64(number), gitea.MergePullRequestOption{
		Style:                  style,
		Title:                  opts.MergeCommitMessage,
		DeleteBranchAfterMerge: &deleteBranch,
	})
	if err != nil {
		if resp != nil && resp.StatusCode == 405 {
			cr, getErr := g.GetCR(ctx, owner, repo, number)
			if getErr == nil && cr.State == CRStateMerged {
				return cr, nil
			}
		}
		return nil, err
	}
	return g.GetCR(ctx, owner, repo, number)
}

func (g *giteaProvider) CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	state := gitea.StateClosed
	_, _, err := g.client.EditPullRequest(owner, repo, int64(number), gitea.EditPullRequestOption{
		State: &state,
	})
	if err != nil {
		return nil, err
	}
	return g.GetCR(ctx, owner, repo, number)
}

func (g *giteaProvider) CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	events := opts.Events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}
	hook, _, err := g.client.CreateRepoHook(opts.Owner, opts.Repo, gitea.CreateHookOption{
		Type:   gitea.HookTypeGitea,
		Config: map[string]string{"url": opts.URL, "content_type": "json", "secret": opts.Secret},
		Events: events,
		Active: true,
	})
	if err != nil {
		return nil, err
	}
	return convertGiteaHook(hook), nil
}

func (g *giteaProvider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	_, err := g.client.DeleteRepoHook(owner, repo, webhookID)
	return err
}

func (g *giteaProvider) ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error) {
	hooks, _, err := g.client.ListRepoHooks(owner, repo, gitea.ListHooksOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, convertGiteaHook(h))
	}
	return result, nil
}

func (g *giteaProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	branches, _, err := g.client.ListRepoBranches(owner, repo, gitea.ListRepoBranchesOptions{
		ListOptions: gitea.ListOptions{PageSize: 100},
	})
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, convertGiteaBranch(b))
	}
	return result, nil
}

func (g *giteaProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	b, _, err := g.client.CreateBranch(owner, repo, gitea.CreateBranchOption{
		BranchName:   branch,
		OldBranchName: ref,
	})
	if err != nil {
		return nil, err
	}
	return convertGiteaBranch(b), nil
}

func (g *giteaProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	_, _, err := g.client.DeleteRepoBranch(owner, repo, branch)
	return err
}

func (g *giteaProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	diffBytes, _, err := g.client.GetPullRequestDiff(owner, repo, int64(number), gitea.PullRequestDiffOptions{})
	if err != nil {
		return nil, err
	}
	rawDiff := string(diffBytes)

	files, err := g.GetCRFiles(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	totalAdd, totalDel := 0, 0
	for _, f := range files {
		totalAdd += f.Additions
		totalDel += f.Deletions
	}
	return &MergeDiff{Files: files, TotalAdd: totalAdd, TotalDel: totalDel, RawDiff: rawDiff}, nil
}

func (g *giteaProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	changedFiles, _, err := g.client.ListPullRequestFiles(owner, repo, int64(number), gitea.ListPullRequestFilesOptions{
		ListOptions: gitea.ListOptions{PageSize: 100},
	})
	if err != nil {
		return nil, err
	}
	result := make([]*ChangedFile, 0, len(changedFiles))
	for _, f := range changedFiles {
		cf := &ChangedFile{
			OldPath:   f.PreviousFilename,
			NewPath:   f.Filename,
			Additions: f.Additions,
			Deletions: f.Deletions,
			IsNew:     f.Status == "added",
			IsDeleted: f.Status == "removed",
			IsRenamed: f.Status == "renamed",
		}
		if cf.OldPath == "" {
			cf.OldPath = cf.NewPath
		}
		result = append(result, cf)
	}
	return result, nil
}

func (g *giteaProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	comment, _, err := g.client.CreateIssueComment(owner, repo, int64(number), gitea.CreateIssueCommentOption{
		Body: body,
	})
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

func (g *giteaProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	id, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return err
	}
	resp, err := g.client.DeleteIssueComment(owner, repo, id)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil
		}
	}
	return err
}

func (g *giteaProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	comment, _, err := g.client.CreateIssueComment(owner, repo, int64(number), gitea.CreateIssueCommentOption{
		Body: opts.Body,
	})
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

func (g *giteaProvider) CreateReview(ctx context.Context, owner, repo string, number int, opts CreateReviewOptions) (*ReviewResult, error) {
	reviewOpts := gitea.CreatePullReviewOptions{
		CommitID: opts.CommitID,
		Body:     opts.Body,
	}
	switch opts.Event {
	case "APPROVE":
		reviewOpts.State = gitea.ReviewStateApproved
	case "REQUEST_CHANGES":
		reviewOpts.State = gitea.ReviewStateRequestChanges
	default:
		reviewOpts.State = gitea.ReviewStateComment
	}

	for _, c := range opts.Comments {
		rc := gitea.CreatePullReviewComment{
			Path: c.Path,
			Body: c.Body,
		}
		if c.Side == "LEFT" {
			rc.OldLineNum = int64(c.Line)
		} else {
			rc.NewLineNum = int64(c.Line)
		}
		if c.StartLine > 0 && c.EndLine > c.StartLine {
			if c.Side == "LEFT" {
				rc.OldLineNum = int64(c.StartLine)
			} else {
				rc.NewLineNum = int64(c.StartLine)
			}
		}
		reviewOpts.Comments = append(reviewOpts.Comments, rc)
	}

	review, _, err := g.client.CreatePullReview(owner, repo, int64(number), reviewOpts)
	if err != nil {
		return nil, err
	}
	result := &ReviewResult{
		ID: strconv.FormatInt(review.ID, 10),
	}
	if review.HTMLURL != "" {
		result.HTMLURL = review.HTMLURL
	}
	if review.Reviewer != nil {
		result.User = &CRUser{
			ID:        review.Reviewer.ID,
			Username:  review.Reviewer.UserName,
			AvatarURL: review.Reviewer.AvatarURL,
		}
	}
	return result, nil
}

func (g *giteaProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	stateMap := map[string]gitea.StatusState{
		"success": gitea.StatusSuccess,
		"failed":  gitea.StatusFailure,
		"pending": gitea.StatusPending,
		"error":   gitea.StatusError,
	}
	state := stateMap[opts.State]
	if state == "" {
		state = gitea.StatusPending
	}
	_, _, err := g.client.CreateStatus(owner, repo, sha, gitea.CreateStatusOption{
		State:       state,
		Context:     opts.Context,
		Description: opts.Description,
		TargetURL:   opts.TargetURL,
	})
	return err
}

func (g *giteaProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	data, _, err := g.client.GetFile(owner, repo, ref, path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (g *giteaProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	labelIDs := make([]int64, 0, len(labels))
	for _, l := range labels {
		if id, err := strconv.ParseInt(l, 10, 64); err == nil {
			labelIDs = append(labelIDs, id)
		}
	}
	_, _, err := g.client.AddIssueLabels(owner, repo, int64(number), gitea.IssueLabelsOption{
		Labels: labelIDs,
	})
	return err
}

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
			Merged  bool      `json:"merged"`
			HTMLURL string    `json:"html_url"`
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

	er := BuildEventRepo(pl.Repository.FullName)
	actor := &CRUser{ID: int64(pl.Sender.ID), Username: pl.Sender.Login}

	event := &NormalizedEvent{
		ID:        fmt.Sprintf("gt-%d-%d", time.Now().UnixNano(), pl.Number),
		Source:    g.Platform(),
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

func convertGiteaRepo(r *gitea.Repository) *PlatformRepo {
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
		Platform:      PlatformGitea,
	}
}

func convertGiteaPR(pr *gitea.PullRequest) *ChangeRequest {
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
	var reviewers []*CRUser
	for _, r := range pr.RequestedReviewers {
		if r != nil {
			reviewers = append(reviewers, &CRUser{
				ID:        r.ID,
				Username:  r.UserName,
				AvatarURL: r.AvatarURL,
			})
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
		State:        mapGiteaState(string(pr.State), pr.HasMerged),
		SourceBranch: pr.Head.Ref,
		TargetBranch: pr.Base.Ref,
		HeadSHA:      pr.Head.Sha,
		BaseSHA:      pr.Base.Sha,
		Author:       author,
		Reviewers:    reviewers,
		Labels:       labels,
		WebURL:       pr.HTMLURL,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func convertGiteaBranch(b *gitea.Branch) *PlatformBranch {
	if b == nil {
		return nil
	}
	return &PlatformBranch{Name: b.Name}
}

func convertGiteaHook(h *gitea.Hook) *PlatformWebhook {
	if h == nil {
		return nil
	}
	return &PlatformWebhook{
		ID:     h.ID,
		URL:    h.Config["url"],
		Events: h.Events,
	}
}

func mapGiteaState(state string, merged bool) CRState {
	return MapBoolStateToCR(state, merged)
}

func parseGiteaTotalCount(resp *gitea.Response) int {
	if resp == nil {
		return 0
	}
	return ParseTotalCountHeader(resp.Header, 0)
}

func (g *giteaProvider) UpdateCR(ctx context.Context, owner, repo string, number int, opts UpdateCROptions) (*ChangeRequest, error) {
	editOpts := gitea.EditPullRequestOption{}
	if opts.Title != "" {
		editOpts.Title = opts.Title
	}
	if opts.Description != "" {
		editOpts.Body = &opts.Description
	}
	if opts.TargetBranch != "" {
		editOpts.Base = opts.TargetBranch
	}
	pr, _, err := g.client.EditPullRequest(owner, repo, int64(number), editOpts)
	if err != nil {
		return nil, err
	}
	return convertGiteaPR(pr), nil
}

func (g *giteaProvider) ReopenCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	state := gitea.StateOpen
	pr, _, err := g.client.EditPullRequest(owner, repo, int64(number), gitea.EditPullRequestOption{State: &state})
	if err != nil {
		return nil, err
	}
	return convertGiteaPR(pr), nil
}

func (g *giteaProvider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*CRComment, error) {
	comments, _, err := g.client.ListIssueComments(owner, repo, int64(number), gitea.ListIssueCommentOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]*CRComment, 0, len(comments))
	for _, c := range comments {
		cc := &CRComment{ID: c.ID, Body: c.Body, CreatedAt: c.Created, UpdatedAt: c.Updated}
		if c.Poster != nil {
			cc.Author = &CRUser{ID: c.Poster.ID, Username: c.Poster.UserName, AvatarURL: c.Poster.AvatarURL}
		}
		result = append(result, cc)
	}
	return result, nil
}

func (g *giteaProvider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*CRCommit, error) {
	commits, _, err := g.client.ListPullRequestCommits(owner, repo, int64(number), gitea.ListPullRequestCommitsOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]*CRCommit, 0, len(commits))
	for _, c := range commits {
		sha := ""
		if c.CommitMeta != nil {
			sha = c.CommitMeta.SHA
		}
		cc := &CRCommit{SHA: sha}
		if c.RepoCommit != nil {
			cc.Message = c.RepoCommit.Message
			if c.RepoCommit.Author != nil {
				cc.Author = &CRUser{Name: c.RepoCommit.Author.Name}
			}
		}
		result = append(result, cc)
	}
	return result, nil
}

func (g *giteaProvider) ForkRepo(ctx context.Context, owner, repo string, opts ForkRepoOptions) (*PlatformRepo, error) {
	forkOpts := gitea.CreateForkOption{}
	if opts.Organization != "" {
		forkOpts.Organization = &opts.Organization
	}
	if opts.Name != "" {
		forkOpts.Name = &opts.Name
	}
	r, _, err := g.client.CreateFork(owner, repo, forkOpts)
	if err != nil {
		return nil, err
	}
	return convertGiteaRepo(r), nil
}

func (g *giteaProvider) DeleteRepo(ctx context.Context, owner, repo string) error {
	_, err := g.client.DeleteRepo(owner, repo)
	return err
}

func (g *giteaProvider) UpdateRepo(ctx context.Context, owner, repo string, opts UpdateRepoOptions) (*PlatformRepo, error) {
	editOpts := gitea.EditRepoOption{}
	if opts.Name != "" {
		editOpts.Name = &opts.Name
	}
	if opts.Description != "" {
		editOpts.Description = &opts.Description
	}
	if opts.DefaultBranch != "" {
		editOpts.DefaultBranch = &opts.DefaultBranch
	}
	if opts.Private != nil {
		editOpts.Private = opts.Private
	}
	r, _, err := g.client.EditRepo(owner, repo, editOpts)
	if err != nil {
		return nil, err
	}
	return convertGiteaRepo(r), nil
}

func (g *giteaProvider) GetCommit(ctx context.Context, owner, repo, sha string) (*CommitInfo, error) {
	c, _, err := g.client.GetSingleCommit(owner, repo, sha)
	if err != nil {
		return nil, err
	}
	return convertGiteaCommit(c), nil
}

func (g *giteaProvider) ListCommits(ctx context.Context, owner, repo string, opts ListCommitsOptions) ([]*CommitInfo, error) {
	listOpts := gitea.ListCommitOptions{ListOptions: gitea.ListOptions{Page: opts.Page, PageSize: opts.PerPage}}
	listOpts.Page, listOpts.PageSize = NormalizePageOpts(listOpts.Page, listOpts.PageSize)
	commits, _, err := g.client.ListRepoCommits(owner, repo, listOpts)
	if err != nil {
		return nil, err
	}
	result := make([]*CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, convertGiteaCommit(c))
	}
	return result, nil
}

func (g *giteaProvider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error) {
	cmp, _, err := g.client.CompareCommits(owner, repo, base, head)
	if err != nil {
		return nil, err
	}
	result := &CompareResult{TotalCommits: cmp.TotalCommits}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, convertGiteaCommit(c))
	}
	return result, nil
}

func (g *giteaProvider) CreateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	createOpts := gitea.CreateFileOptions{
		FileOptions: gitea.FileOptions{Message: opts.Message, BranchName: opts.Branch},
		Content:     opts.Content,
	}
	resp, _, err := g.client.CreateFile(owner, repo, opts.Path, createOpts)
	if err != nil {
		return nil, err
	}
	sha := ""
	if resp.Commit != nil {
		sha = resp.Commit.SHA
	}
	return &FileResult{CommitSHA: sha}, nil
}

func (g *giteaProvider) UpdateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	updateOpts := gitea.UpdateFileOptions{
		FileOptions: gitea.FileOptions{Message: opts.Message, BranchName: opts.Branch},
		SHA:         opts.SHA,
		Content:     opts.Content,
	}
	resp, _, err := g.client.UpdateFile(owner, repo, opts.Path, updateOpts)
	if err != nil {
		return nil, err
	}
	sha := ""
	if resp.Commit != nil {
		sha = resp.Commit.SHA
	}
	return &FileResult{CommitSHA: sha}, nil
}

func (g *giteaProvider) DeleteFile(ctx context.Context, owner, repo string, opts FileDeleteOptions) (*FileResult, error) {
	deleteOpts := gitea.DeleteFileOptions{
		FileOptions: gitea.FileOptions{Message: opts.Message, BranchName: opts.Branch},
		SHA:         opts.SHA,
	}
	resp, err := g.client.DeleteFile(owner, repo, opts.Path, deleteOpts)
	if err != nil {
		return nil, err
	}
	sha := ""
	if resp != nil {
		sha = resp.Header.Get("X-Commit-Sha")
	}
	return &FileResult{CommitSHA: sha}, nil
}

func (g *giteaProvider) ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error) {
	tags, _, err := g.client.ListRepoTags(owner, repo, gitea.ListRepoTagsOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]*TagInfo, 0, len(tags))
	for _, t := range tags {
		ti := &TagInfo{Name: t.Name}
		if t.Commit != nil {
			ti.Commit = t.Commit.SHA
		}
		result = append(result, ti)
	}
	return result, nil
}

func (g *giteaProvider) ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error) {
	releases, _, err := g.client.ListReleases(owner, repo, gitea.ListReleasesOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]*ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, convertGiteaRelease(r))
	}
	return result, nil
}

func (g *giteaProvider) CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error) {
	r, _, err := g.client.CreateRelease(owner, repo, gitea.CreateReleaseOption{
		TagName:      opts.TagName,
		Target:       opts.Target,
		Title:        opts.Title,
		Note:         opts.Body,
		IsDraft:      opts.Draft,
		IsPrerelease: opts.Prerelease,
	})
	if err != nil {
		return nil, err
	}
	return convertGiteaRelease(r), nil
}

func (g *giteaProvider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	data, _, err := g.client.GetArchive(owner, repo, ref, gitea.TarGZArchive)
	if format == "zip" {
		data, _, err = g.client.GetArchive(owner, repo, ref, gitea.ZipArchive)
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func convertGiteaCommit(c *gitea.Commit) *CommitInfo {
	if c == nil {
		return nil
	}
	sha := ""
	if c.CommitMeta != nil {
		sha = c.CommitMeta.SHA
	}
	ci := &CommitInfo{SHA: sha}
	if c.RepoCommit != nil {
		ci.Message = c.RepoCommit.Message
		if c.RepoCommit.Author != nil {
			ci.Author = &CRUser{Name: c.RepoCommit.Author.Name}
		}
	}
	if c.CommitMeta != nil {
		ci.CreatedAt = c.CommitMeta.Created
	}
	return ci
}

func convertGiteaRelease(r *gitea.Release) *ReleaseInfo {
	if r == nil {
		return nil
	}
	return &ReleaseInfo{
		ID: r.ID, TagName: r.TagName, Title: r.Title, Body: r.Note,
		URL: r.URL, Draft: r.IsDraft, Prerelease: r.IsPrerelease,
		CreatedAt: r.CreatedAt, PublishedAt: r.PublishedAt,
	}
}
