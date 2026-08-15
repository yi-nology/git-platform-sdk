package gitee

import (
	"context"
	"fmt"
	"net/url"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetFileContent implements provider.FileManager.
//
// Routed through the raw transport client rather than the SDK: the SDK's
// Content model types the live payload's numeric "size" and object "_links"
// fields as plain strings (upstream swagger generation bug — verified against
// the live v5 contents payload), so GetV5ReposOwnerRepoContentsPath fails to
// decode every real file response. The returned string is the raw payload
// "content" field (base64), matching the pre-SDK behaviour.
func (p *Provider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	var resp struct {
		Content string `json:"content"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", esc(owner), esc(repo), escPath(path), url.QueryEscape(ref)), nil, &resp); err != nil {
		return "", provider.Wrap(provider.PlatformGitee, "GetFileContent", err)
	}
	return resp.Content, nil
}

// CreateFile implements provider.FileManager.
//
// Routed through the raw transport client rather than the SDK: the generated
// NewFileParam marshals bracketed JSON keys ("author[name]",
// "committer[email]") the live API does not accept — the endpoint takes the
// flat author_name/author_email vocabulary sent here — and the response's
// CommitContent model re-types "size"/"_links" as strings while its Commit
// model lacks the wire's "sha" key, so the decoded FileResult would lose
// both SHAs even when decoding succeeded.
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
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/contents/%s", esc(owner), esc(repo), escPath(opts.Path)), body, &resp); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "CreateFile", err)
	}
	return &provider.FileResult{SHA: resp.Content.SHA, CommitSHA: resp.Commit.SHA}, nil
}

// UpdateFile implements provider.FileManager.
//
// Routed through the raw transport client rather than the SDK: besides the
// CommitContent response-model defects (see CreateFile), the generated
// PutV5ReposOwnerRepoContentsPath sends its author/committer options as a
// multipart body labeled application/json (upstream client.go bug), which
// the server cannot parse.
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
	if err := p.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/contents/%s", esc(owner), esc(repo), escPath(opts.Path)), body, &resp); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "UpdateFile", err)
	}
	return &provider.FileResult{SHA: resp.Content.SHA, CommitSHA: resp.Commit.SHA}, nil
}

// DeleteFile implements provider.FileManager.
//
// Routed through the raw transport client rather than the SDK: the generated
// DeleteV5ReposOwnerRepoContentsPath turns sha/message/branch/author into
// URL query parameters — the REST contract puts them in the JSON body — and
// shares the broken CommitContent response model.
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
	if err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/contents/%s", esc(owner), esc(repo), escPath(opts.Path)), body, &resp); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "DeleteFile", err)
	}
	return &provider.FileResult{CommitSHA: resp.Commit.SHA}, nil
}

var _ provider.FileManager = (*Provider)(nil)
