package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
)

type githubProvider struct {
	client  *github.Client
	baseURL string
	logger  Logger
}

func init() {
	Register(PlatformGitHub, func(cfg Config) (Provider, error) {
		return newGitHubProvider(cfg), nil
	})
}

func newGitHubProvider(cfg Config) *githubProvider {
	logger := cfg.Logger
	if logger == nil {
		logger = NewNoopLogger()
	}
	transport := &http.Transport{}
	if cfg.SkipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
	httpClient := &http.Client{
		Transport: &oauth2.Transport{Source: src, Base: transport},
		Timeout:   30 * time.Second,
	}
	if cfg.BaseURL == "" {
		return &githubProvider{
			client:  github.NewClient(httpClient),
			baseURL: "https://api.github.com",
			logger:  logger,
		}
	}
	client, err := github.NewEnterpriseClient(cfg.BaseURL, "", httpClient)
	if err != nil {
		return &githubProvider{
			client:  github.NewClient(httpClient),
			baseURL: "https://api.github.com",
			logger:  logger,
		}
	}
	return &githubProvider{client: client, baseURL: cfg.BaseURL, logger: logger}
}

func (g *githubProvider) Platform() Platform { return PlatformGitHub }

func (g *githubProvider) TestConnection(ctx context.Context) (*TestConnectionResult, error) {
	user, _, err := g.client.Users.Get(ctx, "")
	if err != nil {
		return &TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &TestConnectionResult{
		Connected: true,
		Platform:  string(g.Platform()),
		UserName:  user.GetLogin(),
	}
	_, err = g.ListRepos(ctx, ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}

func (g *githubProvider) ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error) {
	listOpts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
	}
	if listOpts.Page == 0 {
		listOpts.Page = 1
	}
	if listOpts.PerPage == 0 {
		listOpts.PerPage = 20
	}
	var repos []*github.Repository
	var err error
	if opts.Owner != "" {
		repos, _, err = g.client.Repositories.ListByOrg(ctx, opts.Owner, &github.RepositoryListByOrgOptions{
			ListOptions: listOpts.ListOptions,
		})
	} else {
		repos, _, err = g.client.Repositories.List(ctx, "", listOpts)
	}
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformRepo, 0, len(repos))
	for _, r := range repos {
		result = append(result, convertGithubRepo(r))
	}
	return result, nil
}

func (g *githubProvider) GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error) {
	r, _, err := g.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	return convertGithubRepo(r), nil
}

func (g *githubProvider) CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error) {
	newPR := &github.NewPullRequest{
		Title: github.String(opts.Title),
		Body:  github.String(opts.Description),
		Head:  github.String(opts.SourceBranch),
		Base:  github.String(opts.TargetBranch),
	}
	pr, _, err := g.client.PullRequests.Create(ctx, opts.Owner, opts.Repo, newPR)
	if err != nil {
		return nil, err
	}
	return convertGithubPR(pr), nil
}

func (g *githubProvider) GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	pr, _, err := g.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return convertGithubPR(pr), nil
}

func (g *githubProvider) ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error) {
	listOpts := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
	}
	if listOpts.Page == 0 {
		listOpts.Page = 1
	}
	if listOpts.PerPage == 0 {
		listOpts.PerPage = 20
	}
	if opts.State != "" {
		listOpts.State = mapCRStateToGithub(opts.State)
	}
	if opts.SourceBranch != "" {
		listOpts.Head = opts.Owner + ":" + opts.SourceBranch
	}
	if opts.TargetBranch != "" {
		listOpts.Base = opts.TargetBranch
	}
	prs, resp, err := g.client.PullRequests.List(ctx, opts.Owner, opts.Repo, listOpts)
	if err != nil {
		return nil, 0, err
	}
	crs := make([]*ChangeRequest, 0, len(prs))
	for _, pr := range prs {
		crs = append(crs, convertGithubPR(pr))
	}
	var total int
	if resp != nil && resp.LastPage > 0 {
		total = resp.LastPage * listOpts.PerPage
	} else {
		total = len(crs)
	}
	return crs, total, nil
}

func (g *githubProvider) MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error) {
	mergeOpts := &github.PullRequestOptions{}
	if opts.Squash {
		mergeOpts.MergeMethod = "squash"
	}
	_, _, err := g.client.PullRequests.Merge(ctx, owner, repo, number, opts.MergeCommitMessage, mergeOpts)
	if err != nil {
		return nil, err
	}
	return g.GetCR(ctx, owner, repo, number)
}

func (g *githubProvider) CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	pr, _, err := g.client.PullRequests.Edit(ctx, owner, repo, number, &github.PullRequest{
		State: github.String("closed"),
	})
	if err != nil {
		return nil, err
	}
	return convertGithubPR(pr), nil
}

func (g *githubProvider) CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	events := opts.Events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}
	hook := &github.Hook{
		Name:   github.String("web"),
		Events: events,
		Config: &github.HookConfig{
			URL:    github.String(opts.URL),
			Secret: github.String(opts.Secret),
		},
		Active: github.Bool(true),
	}
	h, _, err := g.client.Repositories.CreateHook(ctx, opts.Owner, opts.Repo, hook)
	if err != nil {
		return nil, err
	}
	return convertGithubHook(h), nil
}

func (g *githubProvider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	_, err := g.client.Repositories.DeleteHook(ctx, owner, repo, webhookID)
	return err
}

func (g *githubProvider) ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error) {
	hooks, _, err := g.client.Repositories.ListHooks(ctx, owner, repo, nil)
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, convertGithubHook(h))
	}
	return result, nil
}

func (g *githubProvider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	_, err := github.ValidatePayload(r, []byte(secret))
	return err
}

func (g *githubProvider) ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error) {
	payload, err := github.ValidatePayload(r, []byte(secret))
	if err != nil {
		return nil, err
	}
	eventType := github.WebHookType(r)
	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		return nil, err
	}

	ne := &NormalizedEvent{
		Source:     g.Platform(),
		Timestamp:  time.Now(),
		RawPayload: json.RawMessage(payload),
	}

	switch e := event.(type) {
	case *github.PullRequestEvent:
		ne.Type = "cr." + mapGithubAction(e.GetAction(), e.GetPullRequest().GetMerged())
		if e.GetSender() != nil {
			ne.Actor = &CRUser{
				ID:        e.GetSender().GetID(),
				Username:  e.GetSender().GetLogin(),
				AvatarURL: e.GetSender().GetAvatarURL(),
			}
		}
		if e.GetRepo() != nil {
			ne.Repo = convertGithubEventRepo(e.GetRepo().GetFullName())
		}
		if e.GetPullRequest() != nil {
			ne.CR = convertGithubPR(e.GetPullRequest())
			ne.CommitSHA = e.GetPullRequest().GetHead().GetSHA()
		}
	case *github.PushEvent:
		ne.Type = "push"
		ne.Branch = strings.TrimPrefix(e.GetRef(), "refs/heads/")
		ne.CommitSHA = e.GetAfter()
		if e.GetSender() != nil {
			ne.Actor = &CRUser{
				ID:        e.GetSender().GetID(),
				Username:  e.GetSender().GetLogin(),
				AvatarURL: e.GetSender().GetAvatarURL(),
			}
		}
		if e.GetRepo() != nil {
			ne.Repo = convertGithubEventRepo(e.GetRepo().GetFullName())
		}
	case *github.CreateEvent:
		ne.Type = "branch.created"
		ne.Branch = e.GetRef()
		if e.GetSender() != nil {
			ne.Actor = &CRUser{
				ID:        e.GetSender().GetID(),
				Username:  e.GetSender().GetLogin(),
				AvatarURL: e.GetSender().GetAvatarURL(),
			}
		}
	case *github.DeleteEvent:
		ne.Type = "branch.deleted"
		ne.Branch = e.GetRef()
		if e.GetSender() != nil {
			ne.Actor = &CRUser{
				ID:        e.GetSender().GetID(),
				Username:  e.GetSender().GetLogin(),
				AvatarURL: e.GetSender().GetAvatarURL(),
			}
		}
	}

	return ne, nil
}

func (g *githubProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	branches, _, err := g.client.Repositories.ListBranches(ctx, owner, repo, &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, convertGithubBranch(b))
	}
	return result, nil
}

func (g *githubProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	sha := ref
	if !isCommitSHA(ref) {
		commits, err := g.ListCommits(ctx, owner, repo, ListCommitsOptions{Branch: ref, PerPage: 1})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve ref %q to commit SHA: %w", ref, err)
		}
		if len(commits) == 0 {
			return nil, fmt.Errorf("no commits found on ref %q", ref)
		}
		sha = commits[0].SHA
	}
	_, _, err := g.client.Git.CreateRef(ctx, owner, repo, &github.Reference{
		Ref: github.String("refs/heads/" + branch),
		Object: &github.GitObject{
			SHA: github.String(sha),
		},
	})
	if err != nil {
		return nil, err
	}
	return &PlatformBranch{Name: branch}, nil
}

func (g *githubProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	_, err := g.client.Git.DeleteRef(ctx, owner, repo, "heads/"+branch)
	return err
}

func (g *githubProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	diff := &MergeDiff{}
	page := 1
	for {
		files, _, err := g.client.PullRequests.ListFiles(ctx, owner, repo, number, &github.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			cf := &ChangedFile{
				OldPath:   f.GetPreviousFilename(),
				NewPath:   f.GetFilename(),
				Diff:      f.GetPatch(),
				Additions: f.GetAdditions(),
				Deletions: f.GetDeletions(),
				IsNew:     f.GetStatus() == "added",
				IsDeleted: f.GetStatus() == "removed",
				IsRenamed: f.GetStatus() == "renamed",
			}
			if cf.OldPath == "" {
				cf.OldPath = cf.NewPath
			}
			diff.Files = append(diff.Files, cf)
			diff.TotalAdd += cf.Additions
			diff.TotalDel += cf.Deletions
			diff.RawDiff += fmt.Sprintf("diff --git a/%s b/%s\n", cf.OldPath, cf.NewPath)
			if cf.IsNew {
				diff.RawDiff += "new file mode 100644\n"
			}
			if cf.IsDeleted {
				diff.RawDiff += "deleted file mode 100644\n"
			}
			if cf.IsRenamed {
				diff.RawDiff += fmt.Sprintf("rename from %s\nrename to %s\n", cf.OldPath, cf.NewPath)
			}
			if !cf.IsNew {
				diff.RawDiff += fmt.Sprintf("--- a/%s\n", cf.OldPath)
			}
			if !cf.IsDeleted {
				diff.RawDiff += fmt.Sprintf("+++ b/%s\n", cf.NewPath)
			}
			diff.RawDiff += f.GetPatch() + "\n"
		}
		if len(files) < 100 {
			break
		}
		page++
	}
	return diff, nil
}

func (g *githubProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	diff, err := g.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

func (g *githubProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	comment, _, err := g.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
		Body: github.String(body),
	})
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(comment.GetID(), 10), nil
}

func (g *githubProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	id, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid note ID: %w", err)
	}
	_, err = g.client.Issues.DeleteComment(ctx, owner, repo, id)
	return err
}

func (g *githubProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	comment := &github.PullRequestComment{
		Body: github.String(opts.Body),
		Path: github.String(opts.FilePath),
	}
	if opts.NewLine > 0 {
		comment.Line = github.Int(opts.NewLine)
		comment.Side = github.String("RIGHT")
	} else if opts.OldLine > 0 {
		comment.Line = github.Int(opts.OldLine)
		comment.Side = github.String("LEFT")
	}
	c, _, err := g.client.PullRequests.CreateComment(ctx, owner, repo, number, comment)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(c.GetID(), 10), nil
}

func (g *githubProvider) CreateReview(ctx context.Context, owner, repo string, number int, opts CreateReviewOptions) (*ReviewResult, error) {
	reviewRequest := &github.PullRequestReviewRequest{
		CommitID: github.String(opts.CommitID),
		Body:     github.String(opts.Body),
		Event:    github.String(opts.Event),
	}
	for _, c := range opts.Comments {
		rc := &github.DraftReviewComment{
			Path: github.String(c.Path),
			Body: github.String(c.Body),
		}
		if c.StartLine > 0 && c.EndLine > c.StartLine {
			rc.StartLine = github.Int(c.StartLine)
			rc.Line = github.Int(c.EndLine)
			if c.Side != "" {
				rc.Side = github.String(c.Side)
			} else {
				rc.Side = github.String("RIGHT")
			}
			if c.StartLine != c.EndLine {
				rc.StartSide = github.String("RIGHT")
			}
		} else if c.Line > 0 {
			rc.Line = github.Int(c.Line)
			if c.Side != "" {
				rc.Side = github.String(c.Side)
			} else {
				rc.Side = github.String("RIGHT")
			}
		}
		reviewRequest.Comments = append(reviewRequest.Comments, rc)
	}
	review, _, err := g.client.PullRequests.CreateReview(ctx, owner, repo, number, reviewRequest)
	if err != nil {
		return nil, err
	}
	result := &ReviewResult{
		ID: strconv.FormatInt(review.GetID(), 10),
	}
	if review.GetHTMLURL() != "" {
		result.HTMLURL = review.GetHTMLURL()
	}
	if review.GetUser() != nil {
		result.User = &CRUser{
			ID:        review.GetUser().GetID(),
			Username:  review.GetUser().GetLogin(),
			AvatarURL: review.GetUser().GetAvatarURL(),
		}
	}
	return result, nil
}

func (g *githubProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	status := &github.RepoStatus{
		State:       github.String(opts.State),
		Context:     github.String(opts.Context),
		Description: github.String(opts.Description),
		TargetURL:   github.String(opts.TargetURL),
	}
	_, _, err := g.client.Repositories.CreateStatus(ctx, owner, repo, sha, status)
	return err
}

func (g *githubProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	opts := &github.RepositoryContentGetOptions{Ref: ref}
	rc, _, err := g.client.Repositories.DownloadContents(ctx, owner, repo, path, opts)
	if err != nil {
		return "", err
	}
	if rc == nil {
		return "", fmt.Errorf("file not found: %s", path)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (g *githubProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	_, _, err := g.client.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels)
	return err
}

func convertGithubRepo(r *github.Repository) *PlatformRepo {
	if r == nil {
		return nil
	}
	owner, _ := SplitFullName(r.GetFullName())
	return &PlatformRepo{
		ID:            r.GetID(),
		FullName:      r.GetFullName(),
		Name:          r.GetName(),
		Owner:         owner,
		Description:   r.GetDescription(),
		CloneURL:      r.GetCloneURL(),
		SSHURL:        r.GetSSHURL(),
		DefaultBranch: r.GetDefaultBranch(),
		Private:       r.GetPrivate(),
		Platform:      PlatformGitHub,
	}
}

func convertGithubPR(pr *github.PullRequest) *ChangeRequest {
	if pr == nil {
		return nil
	}
	state := CRStateOpened
	if pr.GetState() == "closed" {
		if pr.GetMerged() {
			state = CRStateMerged
		} else {
			state = CRStateClosed
		}
	}
	mergeStatus := "unknown"
	if pr.Mergeable != nil {
		if *pr.Mergeable {
			mergeStatus = "mergeable"
		} else {
			mergeStatus = "conflicting"
		}
	}
	author := &CRUser{}
	if pr.GetUser() != nil {
		author = &CRUser{
			ID:        pr.GetUser().GetID(),
			Username:  pr.GetUser().GetLogin(),
			AvatarURL: pr.GetUser().GetAvatarURL(),
		}
	}
	var reviewers []*CRUser
	for _, r := range pr.RequestedReviewers {
		reviewers = append(reviewers, &CRUser{
			ID:        r.GetID(),
			Username:  r.GetLogin(),
			AvatarURL: r.GetAvatarURL(),
		})
	}
	var labels []string
	for _, l := range pr.Labels {
		if l != nil {
			labels = append(labels, l.GetName())
		}
	}
	return &ChangeRequest{
		ID:           int64(pr.GetNumber()),
		Number:       pr.GetNumber(),
		Title:        pr.GetTitle(),
		Description:  pr.GetBody(),
		State:        state,
		SourceBranch: pr.GetHead().GetRef(),
		TargetBranch: pr.GetBase().GetRef(),
		HeadSHA:      pr.GetHead().GetSHA(),
		BaseSHA:      pr.GetBase().GetSHA(),
		Author:       author,
		Reviewers:    reviewers,
		Labels:       labels,
		MergeStatus:  mergeStatus,
		WebURL:       pr.GetHTMLURL(),
		CreatedAt:    pr.GetCreatedAt().Time,
		UpdatedAt:    pr.GetUpdatedAt().Time,
	}
}

func convertGithubBranch(b *github.Branch) *PlatformBranch {
	if b == nil {
		return nil
	}
	return &PlatformBranch{Name: b.GetName()}
}

func convertGithubHook(h *github.Hook) *PlatformWebhook {
	if h == nil {
		return nil
	}
	return &PlatformWebhook{
		ID:     h.GetID(),
		URL:    h.GetURL(),
		Events: h.Events,
	}
}

func convertGithubEventRepo(fullName string) *EventRepo {
	return BuildEventRepo(fullName)
}

func mapGithubAction(action string, merged bool) string {
	if action == "closed" && merged {
		return "merged"
	}
	return action
}

func mapCRStateToGithub(state CRState) string {
	switch state {
	case CRStateOpened:
		return "open"
	case CRStateClosed:
		return "closed"
	case CRStateMerged:
		return "closed"
	default:
		return "all"
	}
}

func (g *githubProvider) UpdateCR(ctx context.Context, owner, repo string, number int, opts UpdateCROptions) (*ChangeRequest, error) {
	pr := &github.PullRequest{}
	if opts.Title != "" {
		pr.Title = github.String(opts.Title)
	}
	if opts.Description != "" {
		pr.Body = github.String(opts.Description)
	}
	if opts.TargetBranch != "" {
		pr.Base = &github.PullRequestBranch{Ref: github.String(opts.TargetBranch)}
	}
	result, _, err := g.client.PullRequests.Edit(ctx, owner, repo, number, pr)
	if err != nil {
		return nil, err
	}
	return convertGithubPR(result), nil
}

func (g *githubProvider) ReopenCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	result, _, err := g.client.PullRequests.Edit(ctx, owner, repo, number, &github.PullRequest{
		State: github.String("open"),
	})
	if err != nil {
		return nil, err
	}
	return convertGithubPR(result), nil
}

func (g *githubProvider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*CRComment, error) {
	comments, _, err := g.client.PullRequests.ListComments(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, err
	}
	result := make([]*CRComment, 0, len(comments))
	for _, c := range comments {
		cc := &CRComment{
			ID:        c.GetID(),
			Body:      c.GetBody(),
			CreatedAt: c.GetCreatedAt().Time,
			UpdatedAt: c.GetUpdatedAt().Time,
		}
		if c.GetUser() != nil {
			cc.Author = &CRUser{ID: c.GetUser().GetID(), Username: c.GetUser().GetLogin(), AvatarURL: c.GetUser().GetAvatarURL()}
		}
		result = append(result, cc)
	}
	return result, nil
}

func (g *githubProvider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*CRCommit, error) {
	commits, _, err := g.client.PullRequests.ListCommits(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, err
	}
	result := make([]*CRCommit, 0, len(commits))
	for _, c := range commits {
		cc := &CRCommit{SHA: c.GetSHA(), Message: c.GetCommit().GetMessage(), CreatedAt: c.GetCommit().GetAuthor().GetDate().Time}
		if c.GetAuthor() != nil {
			cc.Author = &CRUser{ID: c.GetAuthor().GetID(), Username: c.GetAuthor().GetLogin(), AvatarURL: c.GetAuthor().GetAvatarURL()}
		}
		result = append(result, cc)
	}
	return result, nil
}

func (g *githubProvider) ForkRepo(ctx context.Context, owner, repo string, opts ForkRepoOptions) (*PlatformRepo, error) {
	forkOpts := &github.RepositoryCreateForkOptions{}
	if opts.Organization != "" {
		forkOpts.Organization = opts.Organization
	}
	if opts.Name != "" {
		forkOpts.Name = opts.Name
	}
	r, _, err := g.client.Repositories.CreateFork(ctx, owner, repo, forkOpts)
	if err != nil {
		return nil, err
	}
	return convertGithubRepo(r), nil
}

func (g *githubProvider) DeleteRepo(ctx context.Context, owner, repo string) error {
	_, err := g.client.Repositories.Delete(ctx, owner, repo)
	return err
}

func (g *githubProvider) UpdateRepo(ctx context.Context, owner, repo string, opts UpdateRepoOptions) (*PlatformRepo, error) {
	r := &github.Repository{}
	if opts.Name != "" {
		r.Name = github.String(opts.Name)
	}
	if opts.Description != "" {
		r.Description = github.String(opts.Description)
	}
	if opts.DefaultBranch != "" {
		r.DefaultBranch = github.String(opts.DefaultBranch)
	}
	if opts.Private != nil {
		r.Private = opts.Private
	}
	result, _, err := g.client.Repositories.Edit(ctx, owner, repo, r)
	if err != nil {
		return nil, err
	}
	return convertGithubRepo(result), nil
}

func (g *githubProvider) GetCommit(ctx context.Context, owner, repo, sha string) (*CommitInfo, error) {
	c, _, err := g.client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return nil, err
	}
	return convertGithubCommit(c), nil
}

func (g *githubProvider) ListCommits(ctx context.Context, owner, repo string, opts ListCommitsOptions) ([]*CommitInfo, error) {
	listOpts := &github.CommitsListOptions{
		ListOptions: github.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
	}
	if listOpts.Page == 0 {
		listOpts.Page = 1
	}
	if listOpts.PerPage == 0 {
		listOpts.PerPage = 20
	}
	if opts.Branch != "" {
		listOpts.SHA = opts.Branch
	}
	if opts.Since != "" {
		if t, err := time.Parse(time.RFC3339, opts.Since); err == nil {
			listOpts.Since = t
		}
	}
	if opts.Until != "" {
		if t, err := time.Parse(time.RFC3339, opts.Until); err == nil {
			listOpts.Until = t
		}
	}
	commits, _, err := g.client.Repositories.ListCommits(ctx, owner, repo, listOpts)
	if err != nil {
		return nil, err
	}
	result := make([]*CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, convertGithubCommit(c))
	}
	return result, nil
}

func (g *githubProvider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error) {
	cmp, _, err := g.client.Repositories.CompareCommits(ctx, owner, repo, base, head, nil)
	if err != nil {
		return nil, err
	}
	result := &CompareResult{
		TotalCommits: cmp.GetTotalCommits(),
		AheadBy:      cmp.GetAheadBy(),
		BehindBy:     cmp.GetBehindBy(),
	}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, convertGithubCommit(c))
	}
	for _, f := range cmp.Files {
		result.Files = append(result.Files, &ChangedFile{
			OldPath:   f.GetPreviousFilename(),
			NewPath:   f.GetFilename(),
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
			IsNew:     f.GetStatus() == "added",
			IsDeleted: f.GetStatus() == "removed",
			IsRenamed: f.GetStatus() == "renamed",
		})
	}
	return result, nil
}

func (g *githubProvider) CreateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	optsReq := &github.RepositoryContentFileOptions{
		Message: github.String(opts.Message),
		Content: []byte(opts.Content),
	}
	if opts.Branch != "" {
		optsReq.Branch = github.String(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		optsReq.Author = &github.CommitAuthor{Name: github.String(opts.Author), Email: github.String(opts.Email)}
	}
	resp, _, err := g.client.Repositories.CreateFile(ctx, owner, repo, opts.Path, optsReq)
	if err != nil {
		return nil, err
	}
	sha := ""
	if resp.SHA != nil {
		sha = *resp.SHA
	}
	return &FileResult{CommitSHA: sha}, nil
}

func (g *githubProvider) UpdateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	optsReq := &github.RepositoryContentFileOptions{
		Message: github.String(opts.Message),
		Content: []byte(opts.Content),
	}
	if opts.SHA != "" {
		optsReq.SHA = github.String(opts.SHA)
	}
	if opts.Branch != "" {
		optsReq.Branch = github.String(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		optsReq.Author = &github.CommitAuthor{Name: github.String(opts.Author), Email: github.String(opts.Email)}
	}
	resp, _, err := g.client.Repositories.UpdateFile(ctx, owner, repo, opts.Path, optsReq)
	if err != nil {
		return nil, err
	}
	sha := ""
	if resp.SHA != nil {
		sha = *resp.SHA
	}
	return &FileResult{CommitSHA: sha}, nil
}

func (g *githubProvider) DeleteFile(ctx context.Context, owner, repo string, opts FileDeleteOptions) (*FileResult, error) {
	optsReq := &github.RepositoryContentFileOptions{
		Message: github.String(opts.Message),
	}
	if opts.SHA != "" {
		optsReq.SHA = github.String(opts.SHA)
	}
	if opts.Branch != "" {
		optsReq.Branch = github.String(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		optsReq.Author = &github.CommitAuthor{Name: github.String(opts.Author), Email: github.String(opts.Email)}
	}
	resp, _, err := g.client.Repositories.DeleteFile(ctx, owner, repo, opts.Path, optsReq)
	if err != nil {
		return nil, err
	}
	sha := ""
	if resp.SHA != nil {
		sha = *resp.SHA
	}
	return &FileResult{CommitSHA: sha}, nil
}

func (g *githubProvider) ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error) {
	tags, _, err := g.client.Repositories.ListTags(ctx, owner, repo, nil)
	if err != nil {
		return nil, err
	}
	result := make([]*TagInfo, 0, len(tags))
	for _, t := range tags {
		result = append(result, &TagInfo{Name: t.GetName(), Commit: t.GetCommit().GetSHA()})
	}
	return result, nil
}

func (g *githubProvider) ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error) {
	releases, _, err := g.client.Repositories.ListReleases(ctx, owner, repo, nil)
	if err != nil {
		return nil, err
	}
	result := make([]*ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, convertGithubRelease(r))
	}
	return result, nil
}

func (g *githubProvider) CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error) {
	r, _, err := g.client.Repositories.CreateRelease(ctx, owner, repo, &github.RepositoryRelease{
		TagName:         github.String(opts.TagName),
		TargetCommitish: github.String(opts.Target),
		Name:            github.String(opts.Title),
		Body:            github.String(opts.Body),
		Draft:           github.Bool(opts.Draft),
		Prerelease:      github.Bool(opts.Prerelease),
	})
	if err != nil {
		return nil, err
	}
	return convertGithubRelease(r), nil
}

func (g *githubProvider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	url, _, err := g.client.Repositories.GetArchiveLink(ctx, owner, repo, github.ArchiveFormat(format), &github.RepositoryContentGetOptions{Ref: ref}, 1)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Client().Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func convertGithubCommit(c *github.RepositoryCommit) *CommitInfo {
	if c == nil {
		return nil
	}
	ci := &CommitInfo{
		SHA:     c.GetSHA(),
		Message: c.GetCommit().GetMessage(),
	}
	if c.GetCommit() != nil && c.GetCommit().GetAuthor() != nil {
		ci.CreatedAt = c.GetCommit().GetAuthor().GetDate().Time
	}
	if c.GetAuthor() != nil {
		ci.Author = &CRUser{ID: c.GetAuthor().GetID(), Username: c.GetAuthor().GetLogin(), AvatarURL: c.GetAuthor().GetAvatarURL()}
	}
	if c.GetCommitter() != nil {
		ci.Committer = &CRUser{ID: c.GetCommitter().GetID(), Username: c.GetCommitter().GetLogin()}
	}
	return ci
}

func convertGithubRelease(r *github.RepositoryRelease) *ReleaseInfo {
	if r == nil {
		return nil
	}
	return &ReleaseInfo{
		ID:          r.GetID(),
		TagName:     r.GetTagName(),
		Title:       r.GetName(),
		Body:        r.GetBody(),
		URL:         r.GetHTMLURL(),
		Draft:       r.GetDraft(),
		Prerelease:  r.GetPrerelease(),
		CreatedAt:   r.GetCreatedAt().Time,
		PublishedAt: r.GetPublishedAt().Time,
	}
}
