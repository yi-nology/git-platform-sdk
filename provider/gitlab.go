package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type gitlabProvider struct {
	client *gitlab.Client
}

func NewGitLabProvider(baseURL, token string, skipTLS bool) *gitlabProvider {
	transport := &http.Transport{}
	if skipTLS {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	httpClient := &http.Client{Timeout: 30 * time.Second, Transport: transport}
	opts := []gitlab.ClientOptionFunc{gitlab.WithHTTPClient(httpClient)}
	if baseURL != "" {
		opts = append(opts, gitlab.WithBaseURL(baseURL))
	}
	client, err := gitlab.NewClient(token, opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to create GitLab client: %v", err))
	}
	return &gitlabProvider{client: client}
}

func (g *gitlabProvider) Platform() Platform { return PlatformGitLab }

func (g *gitlabProvider) TestConnection(ctx context.Context) (*TestConnectionResult, error) {
	user, _, err := g.client.Users.CurrentUser(gitlab.WithContext(ctx))
	if err != nil {
		return &TestConnectionResult{Connected: false, Message: err.Error()}, nil
	}
	result := &TestConnectionResult{
		Connected: true,
		Platform:  string(g.Platform()),
		UserName:  user.Username,
	}
	_, err = g.ListRepos(ctx, ListRepoOptions{Page: 1, PerPage: 1})
	result.CanListRepos = err == nil
	result.CanReadCR = result.CanListRepos
	result.CanWriteCR = result.CanListRepos
	result.CanWebhook = result.CanListRepos
	return result, nil
}

func (g *gitlabProvider) ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error) {
	page := int64(opts.Page)
	perPage := int64(opts.PerPage)
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = 20
	}
	var projects []*gitlab.Project
	var err error
	if opts.Owner != "" {
		projects, _, err = g.client.Groups.ListGroupProjects(opts.Owner, &gitlab.ListGroupProjectsOptions{ListOptions: gitlab.ListOptions{Page: page, PerPage: perPage}}, gitlab.WithContext(ctx))
	} else {
		projects, _, err = g.client.Projects.ListProjects(&gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{Page: page, PerPage: perPage}}, gitlab.WithContext(ctx))
	}
	if err != nil {
		return nil, err
	}
	repos := make([]*PlatformRepo, 0, len(projects))
	for _, p := range projects {
		repos = append(repos, convertGitlabProject(p))
	}
	return repos, nil
}

func (g *gitlabProvider) GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error) {
	p, _, err := g.client.Projects.GetProject(owner+"/"+repo, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabProject(p), nil
}

func (g *gitlabProvider) CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error) {
	pid := opts.Owner + "/" + opts.Repo
	createOpts := &gitlab.CreateMergeRequestOptions{
		SourceBranch:       gitlab.Ptr(opts.SourceBranch),
		TargetBranch:       gitlab.Ptr(opts.TargetBranch),
		Title:              gitlab.Ptr(opts.Title),
		Description:        gitlab.Ptr(opts.Description),
		RemoveSourceBranch: gitlab.Ptr(opts.RemoveSourceBranch),
	}
	if len(opts.Labels) > 0 {
		labels := gitlab.LabelOptions(opts.Labels)
		createOpts.Labels = &labels
	}
	mr, _, err := g.client.MergeRequests.CreateMergeRequest(pid, createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabMR(mr), nil
}

func (g *gitlabProvider) GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	mr, _, err := g.client.MergeRequests.GetMergeRequest(owner+"/"+repo, int64(number), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabMR(mr), nil
}

func (g *gitlabProvider) ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error) {
	pid := opts.Owner + "/" + opts.Repo
	page := int64(opts.Page)
	perPage := int64(opts.PerPage)
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = 20
	}
	listOpts := &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{Page: page, PerPage: perPage},
	}
	if opts.State != "" {
		listOpts.State = gitlab.Ptr(string(opts.State))
	}
	if opts.SourceBranch != "" {
		listOpts.SourceBranch = gitlab.Ptr(opts.SourceBranch)
	}
	if opts.TargetBranch != "" {
		listOpts.TargetBranch = gitlab.Ptr(opts.TargetBranch)
	}
	mrs, resp, err := g.client.MergeRequests.ListProjectMergeRequests(pid, listOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, 0, err
	}
	total := len(mrs)
	if resp != nil && resp.TotalItems > 0 {
		total = int(resp.TotalItems)
	}
	crs := make([]*ChangeRequest, 0, len(mrs))
	for _, mr := range mrs {
		crs = append(crs, convertGitlabBasicMR(mr))
	}
	return crs, total, nil
}

func (g *gitlabProvider) MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error) {
	pid := owner + "/" + repo
	existing, _, err := g.client.MergeRequests.GetMergeRequest(pid, int64(number), nil, gitlab.WithContext(ctx))
	if err == nil {
		if existing.DetailedMergeStatus != "" && existing.DetailedMergeStatus != "mergeable" && existing.DetailedMergeStatus != "checking" {
			return nil, fmt.Errorf("MR cannot be merged (status: %s). It may have conflicts or an active pipeline", existing.DetailedMergeStatus)
		}
		if existing.State != "opened" {
			return nil, fmt.Errorf("MR is not in 'opened' state (current: %s)", existing.State)
		}
	}
	acceptOpts := &gitlab.AcceptMergeRequestOptions{}
	if opts.MergeCommitMessage != "" {
		acceptOpts.MergeCommitMessage = gitlab.Ptr(opts.MergeCommitMessage)
	}
	if opts.Squash {
		acceptOpts.Squash = gitlab.Ptr(true)
	}
	if opts.RemoveSourceBranch {
		acceptOpts.ShouldRemoveSourceBranch = gitlab.Ptr(true)
	}
	mr, _, err := g.client.MergeRequests.AcceptMergeRequest(pid, int64(number), acceptOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("merge failed: %w", err)
	}
	return convertGitlabMR(mr), nil
}

func (g *gitlabProvider) CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	pid := owner + "/" + repo
	mr, _, err := g.client.MergeRequests.UpdateMergeRequest(pid, int64(number), &gitlab.UpdateMergeRequestOptions{StateEvent: gitlab.Ptr("close")}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabMR(mr), nil
}

func (g *gitlabProvider) CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	pid := opts.Owner + "/" + opts.Repo
	hookOpts := &gitlab.AddProjectHookOptions{
		URL:   gitlab.Ptr(opts.URL),
		Token: gitlab.Ptr(opts.Secret),
	}
	hookOpts.PushEvents = gitlab.Ptr(true)
	if len(opts.Events) > 0 {
		em := map[string]bool{}
		for _, e := range opts.Events {
			em[e] = true
		}
		if v, ok := em["push"]; ok {
			hookOpts.PushEvents = gitlab.Ptr(v)
		}
		hookOpts.MergeRequestsEvents = gitlab.Ptr(em["merge_request"] || em["merge_requests"] || em["pull_request"] || em["cr"])
		hookOpts.TagPushEvents = gitlab.Ptr(em["tag_push"] || em["tag"])
	}
	hook, _, err := g.client.Projects.AddProjectHook(pid, hookOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabHook(hook), nil
}

func (g *gitlabProvider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	_, err := g.client.Projects.DeleteProjectHook(owner+"/"+repo, webhookID, gitlab.WithContext(ctx))
	return err
}

func (g *gitlabProvider) ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error) {
	hooks, _, err := g.client.Projects.ListProjectHooks(owner+"/"+repo, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, convertGitlabHook(h))
	}
	return result, nil
}

func (g *gitlabProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	pid := owner + "/" + repo
	branches, _, err := g.client.Branches.ListBranches(pid, &gitlab.ListBranchesOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]*PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, convertGitlabBranch(b))
	}
	return result, nil
}

func (g *gitlabProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	pid := owner + "/" + repo
	b, _, err := g.client.Branches.CreateBranch(pid, &gitlab.CreateBranchOptions{Branch: gitlab.Ptr(branch), Ref: gitlab.Ptr(ref)}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabBranch(b), nil
}

func (g *gitlabProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	_, err := g.client.Branches.DeleteBranch(owner+"/"+repo, branch, gitlab.WithContext(ctx))
	return err
}

func (g *gitlabProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	pid := owner + "/" + repo
	diffs, _, err := g.client.MergeRequests.ListMergeRequestDiffs(pid, int64(number), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	diff := &MergeDiff{}
	for _, c := range diffs {
		additions, deletions := countDiffLines(c.Diff)
		cf := &ChangedFile{
			OldPath: c.OldPath, NewPath: c.NewPath, Diff: c.Diff,
			Additions: additions, Deletions: deletions,
			IsNew: c.NewFile, IsDeleted: c.DeletedFile, IsRenamed: c.RenamedFile,
		}
		diff.Files = append(diff.Files, cf)
		diff.TotalAdd += additions
		diff.TotalDel += deletions
		diff.RawDiff += fmt.Sprintf("diff --git a/%s b/%s\n", c.OldPath, c.NewPath)
		if c.NewFile {
			diff.RawDiff += "new file mode 100644\n"
		}
		if c.DeletedFile {
			diff.RawDiff += "deleted file mode 100644\n"
		}
		if c.RenamedFile {
			diff.RawDiff += fmt.Sprintf("rename from %s\nrename to %s\n", c.OldPath, c.NewPath)
		}
		if !c.NewFile {
			diff.RawDiff += fmt.Sprintf("--- a/%s\n", c.OldPath)
		}
		if !c.DeletedFile {
			diff.RawDiff += fmt.Sprintf("+++ b/%s\n", c.NewPath)
		}
		diff.RawDiff += c.Diff + "\n"
	}
	return diff, nil
}

func (g *gitlabProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	diff, err := g.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

func (g *gitlabProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	pid := owner + "/" + repo
	note, _, err := g.client.Notes.CreateMergeRequestNote(pid, int64(number), &gitlab.CreateMergeRequestNoteOptions{Body: gitlab.Ptr(body)}, gitlab.WithContext(ctx))
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(note.ID, 10), nil
}

func (g *gitlabProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	pid := owner + "/" + repo
	nid, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid note ID: %w", err)
	}
	_, err = g.client.Notes.DeleteMergeRequestNote(pid, int64(number), nid, gitlab.WithContext(ctx))
	return err
}

func (g *gitlabProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	pid := owner + "/" + repo
	discOpts := &gitlab.CreateMergeRequestDiscussionOptions{Body: gitlab.Ptr(opts.Body)}
	if opts.FilePath != "" {
		discOpts.Position = &gitlab.PositionOptions{
			BaseSHA:      gitlab.Ptr("head"),
			StartSHA:     gitlab.Ptr("head"),
			HeadSHA:      gitlab.Ptr("head"),
			PositionType: gitlab.Ptr("text"),
			NewPath:      gitlab.Ptr(opts.FilePath),
			NewLine:      gitlab.Ptr(int64(opts.NewLine)),
		}
		if opts.OldLine > 0 {
			discOpts.Position.OldPath = gitlab.Ptr(opts.FilePath)
			discOpts.Position.OldLine = gitlab.Ptr(int64(opts.OldLine))
		}
	}
	disc, _, err := g.client.Discussions.CreateMergeRequestDiscussion(pid, int64(number), discOpts, gitlab.WithContext(ctx))
	if err != nil {
		return "", err
	}
	return disc.ID, nil
}

func (g *gitlabProvider) CreateReview(ctx context.Context, owner, repo string, number int, opts CreateReviewOptions) (*ReviewResult, error) {
	pid := owner + "/" + repo

	var headSHA string
	mr, _, err := g.client.MergeRequests.GetMergeRequest(pid, int64(number), nil, gitlab.WithContext(ctx))
	if err == nil && mr != nil {
		headSHA = gitlabMRHeadSHA(mr)
	}

	if opts.Body != "" {
		noteID, err := g.CreateNote(ctx, owner, repo, number, opts.Body)
		if err != nil {
			return nil, fmt.Errorf("create summary note: %w", err)
		}
		_ = noteID
	}

	if opts.CommitID != "" {
		state := "success"
		if opts.Event == "REQUEST_CHANGES" {
			state = "failed"
		}
		_ = g.CreateCommitStatus(ctx, owner, repo, opts.CommitID, CommitStatusOptions{
			State:       state,
			Context:     "review-service",
			Description: opts.Body,
		})
	}

	for _, c := range opts.Comments {
		discOpts := DiscussionOptions{
			Body:     c.Body,
			FilePath: c.Path,
		}
		if c.Side == "LEFT" && c.Line > 0 {
			discOpts.OldLine = c.Line
		} else if c.Line > 0 {
			discOpts.NewLine = c.Line
		}
		if c.StartLine > 0 && c.EndLine > c.StartLine {
			discOpts.NewLine = c.EndLine
		}

		_, _ = g.CreateDiscussion(ctx, owner, repo, number, discOpts)
	}

	result := &ReviewResult{
		ID: fmt.Sprintf("gl-review-%d-%d", number, time.Now().UnixNano()),
	}
	_ = headSHA
	return result, nil
}

func (g *gitlabProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	pid := owner + "/" + repo
	statusOpts := &gitlab.SetCommitStatusOptions{
		State:       mapCommitStateToGitlab(opts.State),
		Context:     gitlab.Ptr(opts.Context),
		Description: gitlab.Ptr(opts.Description),
		TargetURL:   gitlab.Ptr(opts.TargetURL),
	}
	_, _, err := g.client.Commits.SetCommitStatus(pid, sha, statusOpts, gitlab.WithContext(ctx))
	return err
}

func (g *gitlabProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	pid := owner + "/" + repo
	content, _, err := g.client.RepositoryFiles.GetRawFile(pid, path, &gitlab.GetRawFileOptions{Ref: gitlab.Ptr(ref)}, gitlab.WithContext(ctx))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (g *gitlabProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	pid := owner + "/" + repo
	l := gitlab.LabelOptions(labels)
	_, _, err := g.client.MergeRequests.UpdateMergeRequest(pid, int64(number), &gitlab.UpdateMergeRequestOptions{Labels: &l}, gitlab.WithContext(ctx))
	return err
}

func (g *gitlabProvider) ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error) {
	if err := g.ValidateWebhookSignature(r, secret); err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))

	var pl struct {
		ObjectKind string `json:"object_kind"`
		User       struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"user"`
		Project struct {
			PathWithNS string `json:"path_with_namespace"`
		} `json:"project"`
		ObjectAttributes struct {
			IID          int64     `json:"iid"`
			Title        string    `json:"title"`
			Description  string    `json:"description"`
			State        string    `json:"state"`
			SourceBranch string    `json:"source_branch"`
			TargetBranch string    `json:"target_branch"`
			Action       string    `json:"action"`
			MergeStatus  string    `json:"merge_status"`
			URL          string    `json:"url"`
			LastCommit   struct {
				ID string `json:"id"`
			} `json:"last_commit"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"object_attributes"`
		Ref   string `json:"ref"`
		After string `json:"after"`
	}
	if err := json.Unmarshal(body, &pl); err != nil {
		return nil, err
	}

	parts := strings.SplitN(pl.Project.PathWithNS, "/", 2)
	er := &EventRepo{FullName: pl.Project.PathWithNS}
	if len(parts) == 2 {
		er.Owner = parts[0]
		er.Name = parts[1]
	}
	actor := &CRUser{ID: pl.User.ID, Username: pl.User.Username, Name: pl.User.Name}

	event := &NormalizedEvent{
		ID:         fmt.Sprintf("gl-%d-%d", time.Now().UnixNano(), pl.ObjectAttributes.IID),
		RawPayload: json.RawMessage(body),
		Source:     g.Platform(), Timestamp: time.Now(), Actor: actor, Repo: er,
	}

	switch pl.ObjectKind {
	case "merge_request":
		state := mapGLState(pl.ObjectAttributes.State)
		action := pl.ObjectAttributes.Action
		if action == "merge" {
			action = "merged"
		}
		event.Type = "cr." + action
		event.Action = action
		event.CommitSHA = pl.ObjectAttributes.LastCommit.ID
		event.CR = &ChangeRequest{
			ID: pl.ObjectAttributes.IID, Number: int(pl.ObjectAttributes.IID),
			Title: pl.ObjectAttributes.Title, Description: pl.ObjectAttributes.Description,
			State: state, SourceBranch: pl.ObjectAttributes.SourceBranch,
			TargetBranch: pl.ObjectAttributes.TargetBranch, MergeStatus: pl.ObjectAttributes.MergeStatus,
			WebURL: pl.ObjectAttributes.URL, Author: actor,
			CreatedAt: pl.ObjectAttributes.CreatedAt, UpdatedAt: pl.ObjectAttributes.UpdatedAt,
		}
	case "push":
		event.Type = "push"
		event.Action = "push"
		event.Branch = strings.TrimPrefix(pl.Ref, "refs/heads/")
		event.CommitSHA = pl.After
	case "tag_push":
		event.Type = "tag.created"
		event.Tag = strings.TrimPrefix(pl.Ref, "refs/tags/")
	case "note":
		if pl.ObjectAttributes.IID != 0 {
			event.Type = "cr.note"
			event.Action = "note"
			event.CR = &ChangeRequest{
				ID: pl.ObjectAttributes.IID, Number: int(pl.ObjectAttributes.IID),
			}
		}
	}
	return event, nil
}

func (g *gitlabProvider) ValidateWebhookSignature(r *http.Request, secret string) error {
	token := r.Header.Get("X-Gitlab-Token")
	if token == "" || token != secret {
		return fmt.Errorf("invalid GitLab webhook token")
	}
	return nil
}

func convertGitlabProject(p *gitlab.Project) *PlatformRepo {
	parts := strings.SplitN(p.PathWithNamespace, "/", 2)
	owner := ""
	if len(parts) == 2 {
		owner = parts[0]
	}
	return &PlatformRepo{
		ID: p.ID, FullName: p.PathWithNamespace, Name: p.Name, Owner: owner,
		Description: p.Description, CloneURL: p.HTTPURLToRepo, SSHURL: p.SSHURLToRepo,
		DefaultBranch: p.DefaultBranch, Private: p.Visibility != "public", Platform: PlatformGitLab,
	}
}

func convertGitlabMR(mr *gitlab.MergeRequest) *ChangeRequest {
	var author *CRUser
	if mr.Author != nil {
		author = &CRUser{ID: mr.Author.ID, Username: mr.Author.Username, Name: mr.Author.Name, AvatarURL: mr.Author.AvatarURL}
	}
	var reviewers []*CRUser
	for _, r := range mr.Reviewers {
		reviewers = append(reviewers, &CRUser{ID: r.ID, Username: r.Username, Name: r.Name, AvatarURL: r.AvatarURL})
	}
	createdAt := time.Time{}
	if mr.CreatedAt != nil {
		createdAt = *mr.CreatedAt
	}
	updatedAt := time.Time{}
	if mr.UpdatedAt != nil {
		updatedAt = *mr.UpdatedAt
	}
	return &ChangeRequest{
		ID: mr.IID, Number: int(mr.IID), Title: mr.Title, Description: mr.Description,
		State: mapGLState(mr.State), SourceBranch: mr.SourceBranch, TargetBranch: mr.TargetBranch,
		Author: author, Reviewers: reviewers, Labels: mr.Labels,
		MergeStatus: mr.DetailedMergeStatus, WebURL: mr.WebURL,
		HeadSHA: gitlabMRHeadSHA(mr), BaseSHA: gitlabMRBaseSHA(mr),
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}

func gitlabMRHeadSHA(mr *gitlab.MergeRequest) string {
	if mr.DiffRefs.HeadSha != "" {
		return mr.DiffRefs.HeadSha
	}
	return mr.SHA
}

func gitlabMRBaseSHA(mr *gitlab.MergeRequest) string {
	return mr.DiffRefs.BaseSha
}

func convertGitlabBasicMR(mr *gitlab.BasicMergeRequest) *ChangeRequest {
	var author *CRUser
	if mr.Author != nil {
		author = &CRUser{ID: mr.Author.ID, Username: mr.Author.Username, Name: mr.Author.Name, AvatarURL: mr.Author.AvatarURL}
	}
	var reviewers []*CRUser
	for _, r := range mr.Reviewers {
		reviewers = append(reviewers, &CRUser{ID: r.ID, Username: r.Username, Name: r.Name, AvatarURL: r.AvatarURL})
	}
	createdAt := time.Time{}
	if mr.CreatedAt != nil {
		createdAt = *mr.CreatedAt
	}
	updatedAt := time.Time{}
	if mr.UpdatedAt != nil {
		updatedAt = *mr.UpdatedAt
	}
	return &ChangeRequest{
		ID: mr.IID, Number: int(mr.IID), Title: mr.Title, Description: mr.Description,
		State: mapGLState(mr.State), SourceBranch: mr.SourceBranch, TargetBranch: mr.TargetBranch,
		Author: author, Reviewers: reviewers, Labels: mr.Labels,
		MergeStatus: mr.DetailedMergeStatus, WebURL: mr.WebURL,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}

func convertGitlabBranch(b *gitlab.Branch) *PlatformBranch {
	return &PlatformBranch{Name: b.Name}
}

func convertGitlabHook(h *gitlab.ProjectHook) *PlatformWebhook {
	events := []string{}
	if h.PushEvents {
		events = append(events, "push")
	}
	if h.MergeRequestsEvents {
		events = append(events, "merge_request")
	}
	if h.TagPushEvents {
		events = append(events, "tag_push")
	}
	if h.NoteEvents {
		events = append(events, "note")
	}
	return &PlatformWebhook{ID: h.ID, URL: h.URL, Events: events}
}

func mapCommitStateToGitlab(state string) gitlab.BuildStateValue {
	switch state {
	case "success":
		return gitlab.Success
	case "failed":
		return gitlab.Failed
	case "pending":
		return gitlab.Pending
	case "running":
		return gitlab.Running
	default:
		return gitlab.Pending
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

func (g *gitlabProvider) UpdateCR(ctx context.Context, owner, repo string, number int, opts UpdateCROptions) (*ChangeRequest, error) {
	pid := owner + "/" + repo
	updateOpts := &gitlab.UpdateMergeRequestOptions{}
	if opts.Title != "" {
		updateOpts.Title = gitlab.Ptr(opts.Title)
	}
	if opts.Description != "" {
		updateOpts.Description = gitlab.Ptr(opts.Description)
	}
	if opts.TargetBranch != "" {
		updateOpts.TargetBranch = gitlab.Ptr(opts.TargetBranch)
	}
	mr, _, err := g.client.MergeRequests.UpdateMergeRequest(pid, int64(number), updateOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabMR(mr), nil
}

func (g *gitlabProvider) ReopenCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	pid := owner + "/" + repo
	mr, _, err := g.client.MergeRequests.UpdateMergeRequest(pid, int64(number), &gitlab.UpdateMergeRequestOptions{StateEvent: gitlab.Ptr("reopen")}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabMR(mr), nil
}

func (g *gitlabProvider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*CRComment, error) {
	pid := owner + "/" + repo
	notes, _, err := g.client.Notes.ListMergeRequestNotes(pid, int64(number), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]*CRComment, 0, len(notes))
	for _, n := range notes {
		c := &CRComment{ID: n.ID, Body: n.Body, Author: &CRUser{ID: n.Author.ID, Username: n.Author.Username, Name: n.Author.Name, AvatarURL: n.Author.AvatarURL}}
		if n.CreatedAt != nil {
			c.CreatedAt = *n.CreatedAt
		}
		if n.UpdatedAt != nil {
			c.UpdatedAt = *n.UpdatedAt
		}
		result = append(result, c)
	}
	return result, nil
}

func (g *gitlabProvider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*CRCommit, error) {
	pid := owner + "/" + repo
	commits, _, err := g.client.MergeRequests.GetMergeRequestCommits(pid, int64(number), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]*CRCommit, 0, len(commits))
	for _, c := range commits {
		cc := &CRCommit{SHA: c.ShortID, Message: c.Title, CreatedAt: *c.CreatedAt}
		if c.AuthorName != "" {
			cc.Author = &CRUser{Name: c.AuthorName}
		}
		result = append(result, cc)
	}
	return result, nil
}

func (g *gitlabProvider) ForkRepo(ctx context.Context, owner, repo string, opts ForkRepoOptions) (*PlatformRepo, error) {
	pid := owner + "/" + repo
	forkOpts := &gitlab.ForkProjectOptions{}
	if opts.Organization != "" {
		forkOpts.Namespace = gitlab.Ptr(opts.Organization)
	}
	if opts.Name != "" {
		forkOpts.Name = gitlab.Ptr(opts.Name)
	}
	p, _, err := g.client.Projects.ForkProject(pid, forkOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabProject(p), nil
}

func (g *gitlabProvider) DeleteRepo(ctx context.Context, owner, repo string) error {
	_, err := g.client.Projects.DeleteProject(owner+"/"+repo, nil, gitlab.WithContext(ctx))
	return err
}

func (g *gitlabProvider) UpdateRepo(ctx context.Context, owner, repo string, opts UpdateRepoOptions) (*PlatformRepo, error) {
	pid := owner + "/" + repo
	updateOpts := &gitlab.EditProjectOptions{}
	if opts.Name != "" {
		updateOpts.Name = gitlab.Ptr(opts.Name)
	}
	if opts.Description != "" {
		updateOpts.Description = gitlab.Ptr(opts.Description)
	}
	if opts.DefaultBranch != "" {
		updateOpts.DefaultBranch = gitlab.Ptr(opts.DefaultBranch)
	}
	if opts.Private != nil {
		vis := "public"
		if *opts.Private {
			vis = "private"
		}
		updateOpts.Visibility = gitlab.Ptr(gitlab.VisibilityValue(vis))
	}
	p, _, err := g.client.Projects.EditProject(pid, updateOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabProject(p), nil
}

func (g *gitlabProvider) GetCommit(ctx context.Context, owner, repo, sha string) (*CommitInfo, error) {
	pid := owner + "/" + repo
	c, _, err := g.client.Commits.GetCommit(pid, sha, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return convertGitlabCommit(c), nil
}

func (g *gitlabProvider) ListCommits(ctx context.Context, owner, repo string, opts ListCommitsOptions) ([]*CommitInfo, error) {
	pid := owner + "/" + repo
	listOpts := &gitlab.ListCommitsOptions{
		ListOptions: gitlab.ListOptions{Page: int64(opts.Page), PerPage: int64(opts.PerPage)},
	}
	if listOpts.Page == 0 {
		listOpts.Page = 1
	}
	if listOpts.PerPage == 0 {
		listOpts.PerPage = 20
	}
	if opts.Branch != "" {
		listOpts.RefName = gitlab.Ptr(opts.Branch)
	}
	if opts.Since != "" {
		if t, err := time.Parse(time.RFC3339, opts.Since); err == nil {
			listOpts.Since = gitlab.Ptr(t)
		}
	}
	if opts.Until != "" {
		if t, err := time.Parse(time.RFC3339, opts.Until); err == nil {
			listOpts.Until = gitlab.Ptr(t)
		}
	}
	commits, _, err := g.client.Commits.ListCommits(pid, listOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]*CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, convertGitlabCommit(c))
	}
	return result, nil
}

func (g *gitlabProvider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error) {
	pid := owner + "/" + repo
	cmp, _, err := g.client.Repositories.Compare(pid, &gitlab.CompareOptions{From: gitlab.Ptr(base), To: gitlab.Ptr(head)}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	result := &CompareResult{}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, convertGitlabCommit(c))
		result.TotalCommits++
	}
	for _, d := range cmp.Diffs {
		add, del := countDiffLines(d.Diff)
		result.Files = append(result.Files, &ChangedFile{
			OldPath: d.OldPath, NewPath: d.NewPath, Diff: d.Diff,
			Additions: add, Deletions: del,
			IsNew: d.NewFile, IsDeleted: d.DeletedFile, IsRenamed: d.RenamedFile,
		})
	}
	return result, nil
}

func (g *gitlabProvider) CreateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	pid := owner + "/" + repo
	createOpts := &gitlab.CreateFileOptions{
		Content:  gitlab.Ptr(opts.Content),
		CommitMessage: gitlab.Ptr(opts.Message),
	}
	if opts.Branch != "" {
		createOpts.Branch = gitlab.Ptr(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		createOpts.AuthorName = gitlab.Ptr(opts.Author)
		createOpts.AuthorEmail = gitlab.Ptr(opts.Email)
	}
	_, resp, err := g.client.RepositoryFiles.CreateFile(pid, opts.Path, createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	sha := resp.Header.Get("X-Gitlab-Commit-Id")
	return &FileResult{CommitSHA: sha}, nil
}

func (g *gitlabProvider) UpdateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	pid := owner + "/" + repo
	updateOpts := &gitlab.UpdateFileOptions{
		Content:  gitlab.Ptr(opts.Content),
		CommitMessage: gitlab.Ptr(opts.Message),
	}
	if opts.Branch != "" {
		updateOpts.Branch = gitlab.Ptr(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		updateOpts.AuthorName = gitlab.Ptr(opts.Author)
		updateOpts.AuthorEmail = gitlab.Ptr(opts.Email)
	}
	_, resp, err := g.client.RepositoryFiles.UpdateFile(pid, opts.Path, updateOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	sha := resp.Header.Get("X-Gitlab-Commit-Id")
	return &FileResult{CommitSHA: sha}, nil
}

func (g *gitlabProvider) DeleteFile(ctx context.Context, owner, repo string, opts FileDeleteOptions) (*FileResult, error) {
	pid := owner + "/" + repo
	deleteOpts := &gitlab.DeleteFileOptions{
		CommitMessage: gitlab.Ptr(opts.Message),
	}
	if opts.Branch != "" {
		deleteOpts.Branch = gitlab.Ptr(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		deleteOpts.AuthorName = gitlab.Ptr(opts.Author)
		deleteOpts.AuthorEmail = gitlab.Ptr(opts.Email)
	}
	resp, err := g.client.RepositoryFiles.DeleteFile(pid, opts.Path, deleteOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	sha := resp.Header.Get("X-Gitlab-Commit-Id")
	return &FileResult{CommitSHA: sha}, nil
}

func (g *gitlabProvider) ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error) {
	pid := owner + "/" + repo
	tags, _, err := g.client.Tags.ListTags(pid, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]*TagInfo, 0, len(tags))
	for _, t := range tags {
		ti := &TagInfo{Name: t.Name}
		if t.Commit != nil {
			ti.Commit = t.Commit.ID
		}
		result = append(result, ti)
	}
	return result, nil
}

func (g *gitlabProvider) ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error) {
	pid := owner + "/" + repo
	releases, _, err := g.client.Releases.ListReleases(pid, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]*ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		ri := &ReleaseInfo{
			TagName: r.TagName, Title: r.Name, Body: r.Description,
			URL: r.Links.Self, CreatedAt: *r.CreatedAt,
		}
		if r.ReleasedAt != nil {
			ri.PublishedAt = *r.ReleasedAt
		}
		result = append(result, ri)
	}
	return result, nil
}

func (g *gitlabProvider) CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error) {
	pid := owner + "/" + repo
	r, _, err := g.client.Releases.CreateRelease(pid, &gitlab.CreateReleaseOptions{
		TagName:     gitlab.Ptr(opts.TagName),
		Ref:         gitlab.Ptr(opts.Target),
		Name:        gitlab.Ptr(opts.Title),
		Description: gitlab.Ptr(opts.Body),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	ri := &ReleaseInfo{TagName: r.TagName, Title: r.Name, Body: r.Description, URL: r.Links.Self, CreatedAt: *r.CreatedAt}
	if r.ReleasedAt != nil {
		ri.PublishedAt = *r.ReleasedAt
	}
	return ri, nil
}

func (g *gitlabProvider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	pid := owner + "/" + repo
	fmtVal := "tar.gz"
	if format == "zip" {
		fmtVal = "zip"
	}
	data, _, err := g.client.Repositories.Archive(pid, &gitlab.ArchiveOptions{Format: gitlab.Ptr(fmtVal), SHA: gitlab.Ptr(ref)}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func convertGitlabCommit(c *gitlab.Commit) *CommitInfo {
	if c == nil {
		return nil
	}
	ci := &CommitInfo{SHA: c.ID, Message: c.Message}
	if c.Stats != nil {
		ci.Additions = int(c.Stats.Additions)
		ci.Deletions = int(c.Stats.Deletions)
	}
	if c.CreatedAt != nil {
		ci.CreatedAt = *c.CreatedAt
	}
	if c.AuthorName != "" || c.AuthorEmail != "" {
		ci.Author = &CRUser{Name: c.AuthorName}
	}
	return ci
}
