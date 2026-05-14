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
	client   *github.Client
	baseURL  string
}

func NewGitHubProvider(baseURL, token string, skipTLS bool) *githubProvider {
	transport := &http.Transport{}
	if skipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := &http.Client{
		Transport: &oauth2.Transport{Source: src, Base: transport},
		Timeout:   30 * time.Second,
	}
	if baseURL == "" {
		return &githubProvider{
			client:  github.NewClient(httpClient),
			baseURL: "https://api.github.com",
		}
	}
	client, err := github.NewEnterpriseClient(baseURL, "", httpClient)
	if err != nil {
		return &githubProvider{
			client:  github.NewClient(httpClient),
			baseURL: "https://api.github.com",
		}
	}
	return &githubProvider{client: client, baseURL: baseURL}
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
	total := len(crs)
	if resp != nil && resp.LastPage > 0 {
		total = len(crs) * resp.LastPage
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
	_, _, err := g.client.Git.CreateRef(ctx, owner, repo, &github.Reference{
		Ref: github.String("refs/heads/" + branch),
		Object: &github.GitObject{
			SHA: github.String(ref),
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
	parts := strings.SplitN(r.GetFullName(), "/", 2)
	owner := ""
	if len(parts) == 2 {
		owner = parts[0]
	}
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
	return &ChangeRequest{
		ID:           int64(pr.GetNumber()),
		Number:       pr.GetNumber(),
		Title:        pr.GetTitle(),
		Description:  pr.GetBody(),
		State:        state,
		SourceBranch: pr.GetHead().GetRef(),
		TargetBranch: pr.GetBase().GetRef(),
		Author:       author,
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
	parts := strings.SplitN(fullName, "/", 2)
	er := &EventRepo{FullName: fullName}
	if len(parts) == 2 {
		er.Owner = parts[0]
		er.Name = parts[1]
	}
	return er
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
