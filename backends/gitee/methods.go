package gitee

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
	"strings"
	"time"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// doRequest is a tiny convenience wrapper for JSON-in / JSON-out calls. It
// adapts the transport.Client signature to the legacy "method/path/body/result"
// shape used throughout the gitee implementation.
func (p *Provider) doRequest(ctx context.Context, method, path string, body, result any) error {
	_, err := p.client.DoJSON(ctx, &transport.Request{
		Method: method,
		Path:   path,
		Body:   body,
		Result: result,
	})
	return err
}

// doRequestWithHeaders is the same as doRequest but returns the response
// headers. Used for paginated endpoints that expose X-Total-Count.
func (p *Provider) doRequestWithHeaders(ctx context.Context, method, path string, body, result any) (http.Header, error) {
	resp, err := p.client.DoJSON(ctx, &transport.Request{
		Method: method,
		Path:   path,
		Body:   body,
		Result: result,
	})
	if err != nil {
		return nil, err
	}
	return resp.Header, nil
}

// --- Repos ---

// ListRepos implements provider.RepoManager.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
	var path string
	if opts.Owner != "" {
		path = fmt.Sprintf("/users/%s/repos?page=%d&per_page=%d", opts.Owner, page, perPage)
	} else {
		path = fmt.Sprintf("/user/repos?page=%d&per_page=%d", page, perPage)
	}
	var repos []giteeRepo
	if err := p.doRequest(ctx, "GET", path, nil, &repos); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListRepos", err)
	}
	result := make([]*provider.PlatformRepo, 0, len(repos))
	for i := range repos {
		result = append(result, repos[i].toPlatformRepo())
	}
	return result, nil
}

// GetRepo implements provider.RepoManager.
func (p *Provider) GetRepo(ctx context.Context, owner, repo string) (*provider.PlatformRepo, error) {
	var r giteeRepo
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s", owner, repo), nil, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetRepo", err)
	}
	return r.toPlatformRepo(), nil
}

// DeleteRepo implements provider.RepoManager.
func (p *Provider) DeleteRepo(ctx context.Context, owner, repo string) error {
	err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s", owner, repo), nil, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteRepo", err)
	}
	return nil
}

// UpdateRepo implements provider.RepoManager.
func (p *Provider) UpdateRepo(ctx context.Context, owner, repo string, opts provider.UpdateRepoOptions) (*provider.PlatformRepo, error) {
	body := map[string]any{}
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
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s", owner, repo), body, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateRepo", err)
	}
	return r.toPlatformRepo(), nil
}

// ForkRepo implements provider.RepoManager.
func (p *Provider) ForkRepo(ctx context.Context, owner, repo string, opts provider.ForkRepoOptions) (*provider.PlatformRepo, error) {
	body := map[string]any{}
	if opts.Organization != "" {
		body["organization"] = opts.Organization
	}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	var r giteeRepo
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/forks", owner, repo), body, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ForkRepo", err)
	}
	return r.toPlatformRepo(), nil
}

var _ provider.RepoManager = (*Provider)(nil)

// --- Change Requests ---

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	body := map[string]any{
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
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls", opts.Owner, opts.Repo), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateCR", err)
	}
	return pr.toChangeRequest(), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	var pr giteePR
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), nil, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetCR", err)
	}
	return pr.toChangeRequest(), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
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
	headers, err := p.doRequestWithHeaders(ctx, "GET", path, nil, &prs)
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitee, "ListCRs", err)
	}
	result := make([]*provider.ChangeRequest, 0, len(prs))
	for i := range prs {
		result = append(result, prs[i].toChangeRequest())
	}
	return result, provider.ParseTotalCountHeader(headers, len(result)), nil
}

// MergeCR implements provider.ChangeRequestManager.
func (p *Provider) MergeCR(ctx context.Context, owner, repo string, number int, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	body := map[string]any{}
	if opts.MergeCommitMessage != "" {
		body["merge_message"] = opts.MergeCommitMessage
	}
	if opts.Squash {
		body["squash"] = true
	}
	var pr giteePR
	if err := p.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, number), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "MergeCR", err)
	}
	return pr.toChangeRequest(), nil
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	body := map[string]any{"state": "closed"}
	var pr giteePR
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CloseCR", err)
	}
	return pr.toChangeRequest(), nil
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	body := map[string]any{"state": "open"}
	var pr giteePR
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ReopenCR", err)
	}
	return pr.toChangeRequest(), nil
}

// UpdateCR implements provider.ChangeRequestManager.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo string, number int, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	body := map[string]any{}
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
	if err := p.doRequest(ctx, "PATCH", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), body, &pr); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateCR", err)
	}
	return pr.toChangeRequest(), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	body := map[string]any{"labels": strings.Join(labels, ",")}
	err := p.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), body, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "UpdateCRLabels", err)
	}
	return nil
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*provider.CRComment, error) {
	var comments []giteeComment
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number), nil, &comments); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListCRComments", err)
	}
	result := make([]*provider.CRComment, 0, len(comments))
	for _, c := range comments {
		result = append(result, c.toCRComment())
	}
	return result, nil
}

// ListCRCommits implements provider.ChangeRequestManager.
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*provider.CRCommit, error) {
	var commits []giteeCommit
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/commits", owner, repo, number), nil, &commits); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListCRCommits", err)
	}
	result := make([]*provider.CRCommit, 0, len(commits))
	for _, c := range commits {
		result = append(result, c.toCRCommit())
	}
	return result, nil
}

var _ provider.ChangeRequestManager = (*Provider)(nil)

// --- Branches ---

// ListBranches implements provider.BranchManager.
func (p *Provider) ListBranches(ctx context.Context, owner, repo string) ([]*provider.PlatformBranch, error) {
	var branches []struct {
		Name string `json:"name"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/branches", owner, repo), nil, &branches); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListBranches", err)
	}
	result := make([]*provider.PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, &provider.PlatformBranch{Name: b.Name})
	}
	return result, nil
}

// CreateBranch implements provider.BranchManager.
func (p *Provider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*provider.PlatformBranch, error) {
	body := map[string]any{"branch_name": branch, "refs": ref}
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/branches", owner, repo), body, nil); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateBranch", err)
	}
	return &provider.PlatformBranch{Name: branch}, nil
}

// DeleteBranch implements provider.BranchManager.
func (p *Provider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/branches/%s", owner, repo, branch), nil, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteBranch", err)
	}
	return nil
}

var _ provider.BranchManager = (*Provider)(nil)

// --- Commits ---

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	var c giteeCommitDetail
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/commits/%s", owner, repo, sha), nil, &c); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetCommit", err)
	}
	return c.toCommitInfo(), nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
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
	if err := p.doRequest(ctx, "GET", path, nil, &commits); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListCommits", err)
	}
	result := make([]*provider.CommitInfo, 0, len(commits))
	for i := range commits {
		result = append(result, commits[i].toCommitInfo())
	}
	return result, nil
}

// CompareCommits implements provider.CommitManager.
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	var cmp struct {
		Commits []giteeCommitDetail `json:"commits"`
		Files   []giteePRFile       `json:"files"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/compare/%s...%s", owner, repo, base, head), nil, &cmp); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CompareCommits", err)
	}
	result := &provider.CompareResult{TotalCommits: len(cmp.Commits)}
	for i := range cmp.Commits {
		result.Commits = append(result.Commits, cmp.Commits[i].toCommitInfo())
	}
	for _, f := range cmp.Files {
		result.Files = append(result.Files, f.toChangedFile())
	}
	return result, nil
}

// CreateCommitStatus implements provider.CommitManager.
//
// Gitee does not expose a commit-status endpoint in the public REST API.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	return provider.Wrap(provider.PlatformGitee, "CreateCommitStatus", provider.ErrNotImplemented)
}

var _ provider.CommitManager = (*Provider)(nil)

// --- Files ---

// GetFileContent implements provider.FileManager.
func (p *Provider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	var resp struct {
		Content string `json:"content"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref), nil, &resp); err != nil {
		return "", provider.Wrap(provider.PlatformGitee, "GetFileContent", err)
	}
	return resp.Content, nil
}

// CreateFile implements provider.FileManager.
func (p *Provider) CreateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	body := map[string]any{
		"content": opts.Content,
		"message": opts.Message,
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
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, opts.Path), body, &resp); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateFile", err)
	}
	return &provider.FileResult{SHA: resp.Content.SHA, CommitSHA: resp.Commit.SHA}, nil
}

// UpdateFile implements provider.FileManager.
func (p *Provider) UpdateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	body := map[string]any{
		"content": opts.Content,
		"message": opts.Message,
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
	if err := p.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, opts.Path), body, &resp); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateFile", err)
	}
	return &provider.FileResult{SHA: resp.Content.SHA, CommitSHA: resp.Commit.SHA}, nil
}

// DeleteFile implements provider.FileManager.
func (p *Provider) DeleteFile(ctx context.Context, owner, repo string, opts provider.FileDeleteOptions) (*provider.FileResult, error) {
	body := map[string]any{
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
	if err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, opts.Path), body, &resp); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "DeleteFile", err)
	}
	return &provider.FileResult{CommitSHA: resp.Commit.SHA}, nil
}

var _ provider.FileManager = (*Provider)(nil)

// --- Tags & Releases ---

// ListTags implements provider.ReleaseManager.
func (p *Provider) ListTags(ctx context.Context, owner, repo string) ([]*provider.TagInfo, error) {
	var tags []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/tags", owner, repo), nil, &tags); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListTags", err)
	}
	result := make([]*provider.TagInfo, 0, len(tags))
	for _, t := range tags {
		result = append(result, &provider.TagInfo{Name: t.Name, Commit: t.Commit.SHA})
	}
	return result, nil
}

// ListReleases implements provider.ReleaseManager.
func (p *Provider) ListReleases(ctx context.Context, owner, repo string) ([]*provider.ReleaseInfo, error) {
	var releases []giteeRelease
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/releases", owner, repo), nil, &releases); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListReleases", err)
	}
	result := make([]*provider.ReleaseInfo, 0, len(releases))
	for i := range releases {
		result = append(result, releases[i].toReleaseInfo())
	}
	return result, nil
}

// CreateRelease implements provider.ReleaseManager.
func (p *Provider) CreateRelease(ctx context.Context, owner, repo string, opts provider.CreateReleaseOptions) (*provider.ReleaseInfo, error) {
	body := map[string]any{
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
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/releases", owner, repo), body, &r); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateRelease", err)
	}
	return r.toReleaseInfo(), nil
}

// GetArchive implements provider.ReleaseManager.
func (p *Provider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	archiveFormat := "zipball"
	if format == "tar.gz" {
		archiveFormat = "tarball"
	}
	resp, err := p.client.Do(ctx, &transport.Request{
		Method: "GET",
		Path:   fmt.Sprintf("/repos/%s/%s/%s/%s", owner, repo, archiveFormat, ref),
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetArchive", err)
	}
	return resp.Body, nil
}

var _ provider.ReleaseManager = (*Provider)(nil)

// --- Diffs ---

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*provider.MergeDiff, error) {
	var files []giteePRFile
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/files", owner, repo, number), nil, &files); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetCRDiff", err)
	}
	diff := &provider.MergeDiff{}
	for _, f := range files {
		diff.Files = append(diff.Files, f.toChangedFile())
	}
	diff.TotalAdd, diff.TotalDel = provider.SumDiffStats(diff.Files)
	diff.RawDiff = provider.BuildRawDiff(diff.Files)
	return diff, nil
}

// GetCRFiles implements provider.DiffManager.
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*provider.ChangedFile, error) {
	diff, err := p.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

// CreateNote implements provider.DiffManager.
func (p *Provider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	reqBody := map[string]any{"body": body}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number), reqBody, &resp); err != nil {
		return "", provider.Wrap(provider.PlatformGitee, "CreateNote", err)
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

// DeleteNote implements provider.DiffManager.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/pulls/comments/%s", owner, repo, noteID), nil, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
//
// Gitee has no separate discussion endpoint; PR comments are the only
// option. We post the body as a regular PR comment.
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts provider.DiscussionOptions) (string, error) {
	return p.CreateNote(ctx, owner, repo, number, opts.Body)
}

// CreateReview implements provider.DiffManager.
//
// Gitee does not expose a review endpoint in its public REST API, so we
// approximate one by posting the summary body as a note and each inline
// comment as a separate PR comment. The returned ID is a synthetic
// timestamp-based identifier.
func (p *Provider) CreateReview(ctx context.Context, owner, repo string, number int, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	if opts.Body != "" {
		if _, err := p.CreateNote(ctx, owner, repo, number, opts.Body); err != nil {
			return nil, err
		}
	}
	results := make([]provider.ReviewCommentResult, 0, len(opts.Comments))
	for _, c := range opts.Comments {
		body := c.Body
		if c.Path != "" {
			body = fmt.Sprintf("**%s**\n\n%s", c.Path, c.Body)
		}
		id, err := p.CreateNote(ctx, owner, repo, number, body)
		results = append(results, provider.ReviewCommentResult{
			Path:       c.Path,
			Line:       c.Line,
			ExternalID: id,
			Error: func() string {
				if err != nil {
					return err.Error()
				}
				return ""
			}(),
		})
	}
	return &provider.ReviewResult{
		ID:       fmt.Sprintf("ge-review-%d-%d", number, time.Now().UnixNano()),
		Comments: results,
	}, nil
}

var _ provider.DiffManager = (*Provider)(nil)

// --- Webhooks ---

// CreateWebhook implements provider.WebhookManager.
func (p *Provider) CreateWebhook(ctx context.Context, opts provider.CreateWebhookOptions) (*provider.PlatformWebhook, error) {
	body := map[string]any{
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
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/hooks", opts.Owner, opts.Repo), body, &hook); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateWebhook", err)
	}
	return hook.toPlatformWebhook(), nil
}

// DeleteWebhook implements provider.WebhookManager.
func (p *Provider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/hooks/%d", owner, repo, webhookID), nil, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteWebhook", err)
	}
	return nil
}

// ListWebhooks implements provider.WebhookManager.
func (p *Provider) ListWebhooks(ctx context.Context, owner, repo string) ([]*provider.PlatformWebhook, error) {
	var hooks []giteeWebhook
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/hooks", owner, repo), nil, &hooks); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ListWebhooks", err)
	}
	result := make([]*provider.PlatformWebhook, 0, len(hooks))
	for _, h := range hooks {
		result = append(result, h.toPlatformWebhook())
	}
	return result, nil
}

// ValidateWebhookSignature implements provider.WebhookManager.
//
// Gitee uses HMAC-SHA256 over the body, sent in the X-Gitee-Token header.
func (p *Provider) ValidateWebhookSignature(r *http.Request, secret string) error {
	if secret == "" {
		return nil
	}
	sig := r.Header.Get("X-Gitee-Token")
	if sig == "" {
		return provider.Wrapf(provider.PlatformGitee, "ValidateWebhookSignature",
			"%w: missing X-Gitee-Token header", provider.ErrWebhookValidation)
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return provider.Wrapf(provider.PlatformGitee, "ValidateWebhookSignature",
			"%w: invalid signature", provider.ErrWebhookValidation)
	}
	return nil
}

// ParseWebhookEvent implements provider.WebhookManager.
func (p *Provider) ParseWebhookEvent(r *http.Request, secret string) (*provider.NormalizedEvent, error) {
	if err := p.ValidateWebhookSignature(r, secret); err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))

	var pl struct {
		Action       string `json:"action"`
		ActionDesc   string `json:"action_desc"`
		Number       int    `json:"number"`
		Title        string `json:"title"`
		Body         string `json:"body"`
		State        string `json:"state"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		HTMLURL      string `json:"html_url"`
		User         struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
			Name  string `json:"name"`
		} `json:"user"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Ref       string    `json:"ref"`
		After     string    `json:"after"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(body, &pl); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "ParseWebhookEvent", err)
	}

	hookName := r.Header.Get("X-Gitee-Event")
	er := provider.BuildEventRepo(pl.Repository.FullName)
	actor := &provider.CRUser{ID: int64(pl.User.ID), Username: pl.User.Login, Name: pl.User.Name}

	event := &provider.NormalizedEvent{
		ID:         fmt.Sprintf("ge-%d-%d", time.Now().UnixNano(), pl.Number),
		Source:     p.Platform(),
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
		event.CR = &provider.ChangeRequest{
			Number:       pl.Number,
			Title:        pl.Title,
			Description:  pl.Body,
			State:        provider.MapBoolStateToCR(pl.State, pl.State == "merged"),
			SourceBranch: pl.SourceBranch,
			TargetBranch: pl.TargetBranch,
			WebURL:       pl.HTMLURL,
			Author:       actor,
			CreatedAt:    pl.CreatedAt,
			UpdatedAt:    pl.UpdatedAt,
		}
	case "push":
		event.Type = "push"
		event.Action = "push"
		event.Branch = strings.TrimPrefix(pl.Ref, "refs/heads/")
		event.CommitSHA = pl.After
	}
	return event, nil
}

var _ provider.WebhookManager = (*Provider)(nil)
