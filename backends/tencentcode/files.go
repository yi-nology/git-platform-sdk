package tencentcode

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/git-platform-sdk/pkg/encoding"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetFileContent implements provider.FileManager.
func (p *Provider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	encoded := encodeProjectPath(owner, repo)
	params := ""
	if ref != "" {
		params = "?ref=" + ref
	}
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/files/%s%s", encoded, path, params), nil, &resp); err != nil {
		return "", err
	}
	if resp.Encoding == "base64" {
		content := strings.ReplaceAll(resp.Content, "\n", "")
		decoded, err := encoding.Base64Decode(content)
		if err != nil {
			return "", err
		}
		return decoded, nil
	}
	return resp.Content, nil
}

// CreateFile implements provider.FileManager.
func (p *Provider) CreateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	return p.mutateFile(ctx, "POST", owner, repo, opts)
}

// UpdateFile implements provider.FileManager.
func (p *Provider) UpdateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	return p.mutateFile(ctx, "PUT", owner, repo, opts)
}

// DeleteFile implements provider.FileManager.
func (p *Provider) DeleteFile(ctx context.Context, owner, repo string, opts provider.FileDeleteOptions) (*provider.FileResult, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{
		"file_path":      opts.Path,
		"commit_message": opts.Message,
	}
	if opts.Branch != "" {
		body["branch"] = opts.Branch
	}
	var resp struct {
		CommitID string `json:"commit_id"`
	}
	if err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/repository/files", encoded), body, &resp); err != nil {
		return nil, err
	}
	return &provider.FileResult{CommitSHA: resp.CommitID}, nil
}

// mutateFile is the shared body for CreateFile and UpdateFile since they
// only differ by HTTP method.
func (p *Provider) mutateFile(ctx context.Context, method, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{
		"file_path":      opts.Path,
		"content":        opts.Content,
		"commit_message": opts.Message,
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
	if err := p.doRequest(ctx, method, fmt.Sprintf("/projects/%s/repository/files", encoded), body, &resp); err != nil {
		return nil, err
	}
	return &provider.FileResult{CommitSHA: resp.CommitID}, nil
}

var _ provider.FileManager = (*Provider)(nil)