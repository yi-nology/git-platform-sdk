package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	gitcode "github.com/yi-nology/gitcode_api"
)

type gitcodeProvider struct {
	client *gitcode.Client
	logger Logger
}

func init() {
	Register(PlatformGitCode, func(cfg Config) (Provider, error) {
		return newGitCodeProvider(cfg), nil
	})
}

func newGitCodeProvider(cfg Config) *gitcodeProvider {
	logger := cfg.Logger
	if logger == nil {
		logger = NewNoopLogger()
	}
	var client *gitcode.Client
	if cfg.BaseURL == "" {
		client = gitcode.NewClient(cfg.Token)
	} else {
		client = gitcode.NewClientWithBaseURL(cfg.BaseURL, cfg.Token)
	}
	return &gitcodeProvider{client: client, logger: logger}
}

func (g *gitcodeProvider) Platform() Platform { return PlatformGitCode }

func (g *gitcodeProvider) TestConnection(ctx context.Context) (*TestConnectionResult, error) {
	user, err := g.client.GetCurrentUser(ctx)
	if err != nil {
		return &TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &TestConnectionResult{
		Connected: true,
		Platform:  string(g.Platform()),
		UserName:  user.Login,
	}
	_, err = g.client.ListRepositories(ctx, gitcode.ListRepositoriesOptions{
		ListOptions: gitcode.ListOptions{Page: 1, PerPage: 1},
	})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}

func (g *gitcodeProvider) ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error) {
	repos, err := g.client.ListRepositories(ctx, gitcode.ListRepositoriesOptions{
		ListOptions: gitcode.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
		Owner:       opts.Owner,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformRepo, 0, len(repos))
	for _, r := range repos {
		owner := ""
		if r.Owner != nil {
			owner = r.Owner.Login
		}
		result = append(result, &PlatformRepo{
			ID:            r.ID,
			FullName:      r.FullName,
			Name:          r.Name,
			Owner:         owner,
			Description:   r.Description,
			CloneURL:      r.CloneURL,
			SSHURL:        r.SSHURL,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
			Platform:      PlatformGitCode,
		})
	}
	return result, nil
}

func (g *gitcodeProvider) GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error) {
	r, err := g.client.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	ownerName := ""
	if r.Owner != nil {
		ownerName = r.Owner.Login
	}
	return &PlatformRepo{
		ID:            r.ID,
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         ownerName,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      PlatformGitCode,
	}, nil
}

func (g *gitcodeProvider) CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error) {
	pr, err := g.client.CreatePullRequest(ctx, opts.Owner, opts.Repo, gitcode.CreatePullRequestOptions{
		Title: opts.Title,
		Body:  opts.Description,
		Head:  opts.SourceBranch,
		Base:  opts.TargetBranch,
	})
	if err != nil {
		return nil, err
	}
	return convertGitCodePullRequest(pr), nil
}

func (g *gitcodeProvider) GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	pr, err := g.client.GetPullRequest(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return convertGitCodePullRequest(pr), nil
}

func (g *gitcodeProvider) ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error) {
	state := gitcode.PullRequestStateOpen
	switch opts.State {
	case CRStateClosed:
		state = gitcode.PullRequestStateClosed
	case CRStateMerged:
		state = gitcode.PullRequestStateClosed
	}
	prs, err := g.client.ListPullRequests(ctx, opts.Owner, opts.Repo, gitcode.ListPullRequestsOptions{
		ListOptions: gitcode.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
		State:       state,
	})
	if err != nil {
		return nil, 0, err
	}
	result := make([]*ChangeRequest, 0, len(prs))
	for _, pr := range prs {
		result = append(result, convertGitCodePullRequest(pr))
	}
	return result, len(result), nil
}

func (g *gitcodeProvider) MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error) {
	err := g.client.MergePullRequest(ctx, owner, repo, number, &gitcode.MergePullRequestOptions{
		CommitMessage: opts.MergeCommitMessage,
		Squash:        opts.Squash,
	})
	if err != nil {
		return nil, err
	}
	return g.GetCR(ctx, owner, repo, number)
}

func (g *gitcodeProvider) CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	pr, err := g.client.ClosePullRequest(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return convertGitCodePullRequest(pr), nil
}

func (g *gitcodeProvider) CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	events := opts.Events
	if len(events) == 0 {
		events = []string{"push", "pull_request"}
	}
	hook, err := g.client.CreateWebhook(ctx, opts.Owner, opts.Repo, gitcode.CreateWebhookOptions{
		URL:    opts.URL,
		Secret: opts.Secret,
		Events: events,
	})
	if err != nil {
		return nil, err
	}
	return &PlatformWebhook{
		ID:     hook.ID,
		URL:    hook.URL,
		Events: hook.Events,
	}, nil
}

func (g *gitcodeProvider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	return g.client.DeleteWebhook(ctx, owner, repo, webhookID)
}

func (g *gitcodeProvider) ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error) {
	hooks, err := g.client.ListWebhooks(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, &PlatformWebhook{
			ID:     h.ID,
			URL:    h.URL,
			Events: h.Events,
		})
	}
	return result, nil
}

func (g *gitcodeProvider) ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error) {
	if err := g.ValidateWebhookSignature(r, secret); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	eventType := r.Header.Get("X-Gitea-Event")
	if eventType == "" {
		eventType = r.Header.Get("X-GitCode-Event")
	}

	ne := &NormalizedEvent{
		Source:     g.Platform(),
		Timestamp:  time.Now(),
		RawPayload: json.RawMessage(body),
	}

	switch eventType {
	case "pull_request":
		prEvent, err := g.client.ParsePullRequestEvent(body)
		if err != nil {
			return nil, err
		}
		action := prEvent.Action
		if action == "closed" && prEvent.PullRequest != nil && prEvent.PullRequest.Merged {
			action = "merged"
		}
		ne.Type = "cr." + action
		ne.Action = action
		if prEvent.Sender != nil {
			senderID, _ := strconv.ParseInt(string(prEvent.Sender.ID), 10, 64)
			ne.Actor = &CRUser{
				ID:        senderID,
				Username:  prEvent.Sender.Login,
				AvatarURL: prEvent.Sender.AvatarURL,
			}
		}
		if prEvent.Repository != nil {
			ne.Repo = BuildEventRepo(prEvent.Repository.FullName)
		}
		if prEvent.PullRequest != nil {
			ne.CR = convertGitCodePullRequest(prEvent.PullRequest)
			if prEvent.PullRequest.Head != nil {
				ne.CommitSHA = prEvent.PullRequest.Head.SHA
			}
		}
	case "push":
		pushEvent, err := g.client.ParsePushEvent(body)
		if err != nil {
			return nil, err
		}
		ne.Type = "push"
		ne.Branch = strings.TrimPrefix(pushEvent.Ref, "refs/heads/")
		ne.CommitSHA = pushEvent.After
		if pushEvent.Sender != nil {
			senderID, _ := strconv.ParseInt(string(pushEvent.Sender.ID), 10, 64)
			ne.Actor = &CRUser{
				ID:        senderID,
				Username:  pushEvent.Sender.Login,
				AvatarURL: pushEvent.Sender.AvatarURL,
			}
		}
		if pushEvent.Repository != nil {
			ne.Repo = BuildEventRepo(pushEvent.Repository.FullName)
		}
	default:
		ne.Type = eventType
	}

	return ne, nil
}

func (g *gitcodeProvider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	sig := r.Header.Get("X-Gitea-Signature")
	if sig == "" {
		sig = r.Header.Get("X-GitCode-Signature")
	}
	if sig == "" {
		return fmt.Errorf("%w: missing webhook signature header", ErrWebhookValidation)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("%w: invalid webhook signature", ErrWebhookValidation)
	}
	return nil
}

func (g *gitcodeProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	branches, err := g.client.ListBranches(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, &PlatformBranch{Name: b.Name})
	}
	return result, nil
}

func (g *gitcodeProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	_, err := g.client.CreateBranch(ctx, owner, repo, gitcode.CreateBranchOptions{
		BranchName: branch,
		Ref:        ref,
	})
	if err != nil {
		return nil, err
	}
	return &PlatformBranch{Name: branch}, nil
}

func (g *gitcodeProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	return g.client.DeleteBranch(ctx, owner, repo, branch)
}

func (g *gitcodeProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	files, err := g.client.ListPullRequestFiles(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	diff := &MergeDiff{}
	for _, f := range files {
		patch := ""
		if f.Patch != nil {
			patch = fmt.Sprint(f.Patch)
		}
		cf := &ChangedFile{
			OldPath:   f.PreviousFilename,
			NewPath:   f.Filename,
			Diff:      patch,
			Additions: f.Additions,
			Deletions: f.Deletions,
			IsNew:     f.Status == "added",
			IsDeleted: f.Status == "removed",
			IsRenamed: f.Status == "renamed",
		}
		if cf.OldPath == "" {
			cf.OldPath = cf.NewPath
		}
		diff.Files = append(diff.Files, cf)
		diff.TotalAdd += cf.Additions
		diff.TotalDel += cf.Deletions
	}
	return diff, nil
}

func (g *gitcodeProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	files, err := g.client.ListPullRequestFiles(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	result := make([]*ChangedFile, 0, len(files))
	for _, f := range files {
		patch := ""
		if f.Patch != nil {
			patch = fmt.Sprint(f.Patch)
		}
		cf := &ChangedFile{
			OldPath:   f.PreviousFilename,
			NewPath:   f.Filename,
			Diff:      patch,
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

func (g *gitcodeProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	comment, err := g.client.CreatePullRequestComment(ctx, owner, repo, number, body, "", "", "")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", comment.ID), nil
}

func (g *gitcodeProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	var id int64
	for _, c := range noteID {
		id = id*10 + int64(c-'0')
	}
	return g.client.DeleteIssueComment(ctx, owner, repo, id)
}

func (g *gitcodeProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	comment, err := g.client.CreatePullRequestComment(ctx, owner, repo, number, opts.Body, "", "", "")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", comment.ID), nil
}

func (g *gitcodeProvider) CreateReview(ctx context.Context, owner, repo string, number int, opts CreateReviewOptions) (*ReviewResult, error) {
	review, err := g.client.CreatePullRequestReview(ctx, owner, repo, number, opts.Body, opts.Event)
	if err != nil {
		return g.createReviewFallback(ctx, owner, repo, number, opts)
	}
	result := &ReviewResult{
		ID: fmt.Sprintf("%d", review.ID),
	}
	user := review.User
	if user == nil {
		user = review.Author
	}
	if user != nil {
		authorID, _ := strconv.ParseInt(string(user.ID), 10, 64)
		result.User = &CRUser{
			ID:        authorID,
			Username:  user.Login,
			AvatarURL: user.AvatarURL,
		}
	}
	for _, c := range opts.Comments {
		if cErr := g.createInlineComment(ctx, owner, repo, number, c, opts.CommitID); cErr != nil {
			if g.logger != nil {
				g.logger.Warn("inline comment failed", "path", c.Path, "line", c.Line, "error", cErr)
			}
		}
	}
	return result, nil
}

func (g *gitcodeProvider) createReviewFallback(ctx context.Context, owner, repo string, number int, opts CreateReviewOptions) (*ReviewResult, error) {
	var lastErr error
	for _, c := range opts.Comments {
		if err := g.createInlineComment(ctx, owner, repo, number, c, opts.CommitID); err != nil {
			lastErr = err
		}
	}
	if opts.Body != "" {
		_, err := g.CreateNote(ctx, owner, repo, number, opts.Body)
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil && len(opts.Comments) == 0 {
		return nil, lastErr
	}
	return &ReviewResult{}, nil
}

func (g *gitcodeProvider) createInlineComment(ctx context.Context, owner, repo string, number int, comment ReviewComment, commitID string) error {
	side := comment.Side
	if side == "" {
		side = "RIGHT"
	}
	_, err := g.client.CreatePullRequestInlineComment(ctx, owner, repo, number, gitcode.CreatePullRequestInlineCommentOptions{
		Body:     comment.Body,
		Path:     comment.Path,
		Line:     comment.Line,
		Side:     side,
		CommitID: commitID,
	})
	return err
}

func (g *gitcodeProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	return fmt.Errorf("%w: CreateCommitStatus for GitCode", ErrNotImplemented)
}

func (g *gitcodeProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	content, err := g.client.GetRepositoryContent(ctx, owner, repo, path, ref)
	if err != nil {
		return "", err
	}
	return content.Content, nil
}

func (g *gitcodeProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	return g.client.AddIssueLabels(ctx, owner, repo, number, labels)
}

func (g *gitcodeProvider) UpdateCR(ctx context.Context, owner, repo string, number int, opts UpdateCROptions) (*ChangeRequest, error) {
	pr, err := g.client.UpdatePullRequest(ctx, owner, repo, number, gitcode.UpdatePullRequestOptions{
		Title: opts.Title,
		Body:  opts.Description,
		Base:  opts.TargetBranch,
	})
	if err != nil {
		return nil, err
	}
	return convertGitCodePullRequest(pr), nil
}

func (g *gitcodeProvider) ReopenCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	pr, err := g.client.ReopenPullRequest(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return convertGitCodePullRequest(pr), nil
}

func (g *gitcodeProvider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*CRComment, error) {
	comments, err := g.client.ListIssueComments(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	result := make([]*CRComment, 0, len(comments))
	for _, c := range comments {
		cc := &CRComment{
			ID:   c.ID,
			Body: c.Body,
		}
		if c.Author != nil {
			authorID, _ := strconv.ParseInt(string(c.Author.ID), 10, 64)
			cc.Author = &CRUser{
				ID:        authorID,
				Username:  c.Author.Login,
				AvatarURL: c.Author.AvatarURL,
			}
		}
		cc.CreatedAt = c.CreatedAt
		cc.UpdatedAt = c.UpdatedAt
		result = append(result, cc)
	}
	return result, nil
}

func (g *gitcodeProvider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*CRCommit, error) {
	commits, err := g.client.ListPullRequestCommits(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	result := make([]*CRCommit, 0, len(commits))
	for _, c := range commits {
		cc := &CRCommit{
			SHA:     c.SHA,
			Message: c.Message,
		}
		if c.Author != nil {
			authorID, _ := strconv.ParseInt(string(c.Author.ID), 10, 64)
			cc.Author = &CRUser{
				ID:        authorID,
				Username:  c.Author.Login,
				AvatarURL: c.Author.AvatarURL,
			}
		}
		cc.CreatedAt = c.CreatedAt
		result = append(result, cc)
	}
	return result, nil
}

func (g *gitcodeProvider) ForkRepo(ctx context.Context, owner, repo string, opts ForkRepoOptions) (*PlatformRepo, error) {
	r, err := g.client.ForkRepository(ctx, owner, repo, nil)
	if err != nil {
		return nil, err
	}
	ownerName := ""
	if r.Owner != nil {
		ownerName = r.Owner.Login
	}
	return &PlatformRepo{
		ID:            r.ID,
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         ownerName,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      PlatformGitCode,
	}, nil
}

func (g *gitcodeProvider) DeleteRepo(ctx context.Context, owner, repo string) error {
	return g.client.DeleteRepository(ctx, owner, repo)
}

func (g *gitcodeProvider) UpdateRepo(ctx context.Context, owner, repo string, opts UpdateRepoOptions) (*PlatformRepo, error) {
	r, err := g.client.UpdateRepository(ctx, owner, repo, gitcode.UpdateRepositoryOptions{
		Name:          opts.Name,
		Description:   opts.Description,
		DefaultBranch: opts.DefaultBranch,
		Private:       opts.Private,
	})
	if err != nil {
		return nil, err
	}
	ownerName := ""
	if r.Owner != nil {
		ownerName = r.Owner.Login
	}
	return &PlatformRepo{
		ID:            r.ID,
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         ownerName,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      PlatformGitCode,
	}, nil
}

func (g *gitcodeProvider) GetCommit(ctx context.Context, owner, repo, sha string) (*CommitInfo, error) {
	c, err := g.client.GetCommit(ctx, owner, repo, sha)
	if err != nil {
		return nil, err
	}
	ci := &CommitInfo{
		SHA:     c.SHA,
		Message: c.Message,
	}
	if c.Author != nil {
		authorID, _ := strconv.ParseInt(string(c.Author.ID), 10, 64)
		ci.Author = &CRUser{
			ID:        authorID,
			Username:  c.Author.Login,
			AvatarURL: c.Author.AvatarURL,
		}
	}
	ci.CreatedAt = c.CreatedAt
	return ci, nil
}

func (g *gitcodeProvider) ListCommits(ctx context.Context, owner, repo string, opts ListCommitsOptions) ([]*CommitInfo, error) {
	commits, err := g.client.ListCommits(ctx, owner, repo, gitcode.ListCommitsOptions{
		ListOptions: gitcode.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
		Branch:      opts.Branch,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*CommitInfo, 0, len(commits))
	for _, c := range commits {
		ci := &CommitInfo{
			SHA:     c.SHA,
			Message: c.Message,
		}
		if c.Author != nil {
			authorID, _ := strconv.ParseInt(string(c.Author.ID), 10, 64)
			ci.Author = &CRUser{
				ID:        authorID,
				Username:  c.Author.Login,
				AvatarURL: c.Author.AvatarURL,
			}
		}
		ci.CreatedAt = c.CreatedAt
		result = append(result, ci)
	}
	return result, nil
}

func (g *gitcodeProvider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error) {
	cmp, err := g.client.CompareCommits(ctx, owner, repo, base, head)
	if err != nil {
		return nil, err
	}
	result := &CompareResult{
		TotalCommits: cmp.TotalCommits,
		AheadBy:      cmp.AheadBy,
		BehindBy:     cmp.BehindBy,
	}
	for _, c := range cmp.Commits {
		ci := &CommitInfo{
			SHA:     c.SHA,
			Message: c.Message,
		}
		if c.Author != nil {
			authorID, _ := strconv.ParseInt(string(c.Author.ID), 10, 64)
			ci.Author = &CRUser{
				ID:        authorID,
				Username:  c.Author.Login,
				AvatarURL: c.Author.AvatarURL,
			}
		}
		ci.CreatedAt = c.CreatedAt
		result.Commits = append(result.Commits, ci)
	}
	for _, f := range cmp.Files {
		result.Files = append(result.Files, &ChangedFile{
			OldPath:   f.PreviousFilename,
			NewPath:   f.Filename,
			Additions: f.Additions,
			Deletions: f.Deletions,
			IsNew:     f.Status == "added",
			IsDeleted: f.Status == "removed",
			IsRenamed: f.Status == "renamed",
		})
	}
	return result, nil
}

func (g *gitcodeProvider) CreateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	result, err := g.client.CreateFile(ctx, owner, repo, opts.Path, gitcode.CreateFileOptions{
		Message: opts.Message,
		Content: opts.Content,
		Branch:  opts.Branch,
	})
	if err != nil {
		return nil, err
	}
	sha := ""
	if result.Content != nil {
		sha = result.Content.SHA
	}
	commitSHA := ""
	if result.Commit != nil {
		commitSHA = result.Commit.SHA
	}
	return &FileResult{SHA: sha, CommitSHA: commitSHA}, nil
}

func (g *gitcodeProvider) UpdateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	result, err := g.client.UpdateFile(ctx, owner, repo, opts.Path, gitcode.UpdateFileOptions{
		Message: opts.Message,
		Content: opts.Content,
		SHA:     opts.SHA,
		Branch:  opts.Branch,
	})
	if err != nil {
		return nil, err
	}
	sha := ""
	if result.Content != nil {
		sha = result.Content.SHA
	}
	commitSHA := ""
	if result.Commit != nil {
		commitSHA = result.Commit.SHA
	}
	return &FileResult{SHA: sha, CommitSHA: commitSHA}, nil
}

func (g *gitcodeProvider) DeleteFile(ctx context.Context, owner, repo string, opts FileDeleteOptions) (*FileResult, error) {
	result, err := g.client.DeleteFile(ctx, owner, repo, opts.Path, gitcode.DeleteFileOptions{
		Message: opts.Message,
		SHA:     opts.SHA,
		Branch:  opts.Branch,
	})
	if err != nil {
		return nil, err
	}
	commitSHA := ""
	if result.Commit != nil {
		commitSHA = result.Commit.SHA
	}
	return &FileResult{CommitSHA: commitSHA}, nil
}

func (g *gitcodeProvider) ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error) {
	tags, err := g.client.ListTags(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	result := make([]*TagInfo, 0, len(tags))
	for _, t := range tags {
		result = append(result, &TagInfo{Name: t.Name, Commit: t.Commit.SHA})
	}
	return result, nil
}

func (g *gitcodeProvider) ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error) {
	releases, err := g.client.ListReleases(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	result := make([]*ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		result = append(result, &ReleaseInfo{
			ID:          r.ID,
			TagName:     r.TagName,
			Title:       r.Name,
			Body:        r.Body,
			URL:         r.HTMLURL,
			Draft:       r.Draft,
			Prerelease:  r.Prerelease,
			CreatedAt:   r.CreatedAt,
			PublishedAt: r.PublishedAt,
		})
	}
	return result, nil
}

func (g *gitcodeProvider) CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error) {
	r, err := g.client.CreateRelease(ctx, owner, repo, gitcode.CreateReleaseOptions{
		TagName:    opts.TagName,
		Target:     opts.Target,
		Title:      opts.Title,
		Body:       opts.Body,
		Draft:      opts.Draft,
		Prerelease: opts.Prerelease,
	})
	if err != nil {
		return nil, err
	}
	return &ReleaseInfo{
		ID:          r.ID,
		TagName:     r.TagName,
		Title:       r.Name,
		Body:        r.Body,
		URL:         r.HTMLURL,
		Draft:       r.Draft,
		Prerelease:  r.Prerelease,
		CreatedAt:   r.CreatedAt,
		PublishedAt: r.PublishedAt,
	}, nil
}

func (g *gitcodeProvider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	return nil, fmt.Errorf("%w: GetArchive for GitCode", ErrNotImplemented)
}

func convertGitCodePullRequest(pr *gitcode.PullRequest) *ChangeRequest {
	if pr == nil {
		return nil
	}
	state := CRStateOpened
	if pr.State == gitcode.PullRequestStateClosed {
		if pr.Merged {
			state = CRStateMerged
		} else {
			state = CRStateClosed
		}
	}
	cr := &ChangeRequest{
		ID:           pr.ID,
		Number:       pr.Number,
		Title:        pr.Title,
		Description:  pr.Body,
		State:        state,
		WebURL:       pr.HTMLURL,
		CreatedAt:    pr.CreatedAt,
		UpdatedAt:    pr.UpdatedAt,
	}
	if pr.Head != nil {
		cr.SourceBranch = pr.Head.Ref
	}
	if pr.Base != nil {
		cr.TargetBranch = pr.Base.Ref
	}
	if pr.Author != nil {
		authorID, _ := strconv.ParseInt(string(pr.Author.ID), 10, 64)
		cr.Author = &CRUser{
			ID:        authorID,
			Username:  pr.Author.Login,
			AvatarURL: pr.Author.AvatarURL,
		}
	}
	return cr
}
