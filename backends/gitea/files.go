package gitea

import (
	"context"
	"net/http"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetFileContent implements provider.FileManager.
func (p *Provider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	data, resp, err := p.client.GetFile(owner, repo, ref, path)
	if err != nil {
		// The gitea SDK flattens API errors to the server's message string (no
		// status info survives on the error), so classify from the *Response —
		// otherwise optional-file callers depending on provider.IsNotFound see
		// a generic failure for every missing file.
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return "", provider.New(provider.PlatformGitea, "GetFileContent", http.StatusNotFound, err.Error())
		}
		return "", provider.Wrap(provider.PlatformGitea, "GetFileContent", err)
	}
	return string(data), nil
}

// CreateFile implements provider.FileManager.
func (p *Provider) CreateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	createOpts := gitea.CreateFileOptions{
		FileOptions: gitea.FileOptions{Message: opts.Message, BranchName: opts.Branch},
		Content:     opts.Content,
	}
	resp, _, err := p.client.CreateFile(owner, repo, opts.Path, createOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateFile", err)
	}
	sha := ""
	if resp.Commit != nil {
		sha = resp.Commit.SHA
	}
	return &provider.FileResult{CommitSHA: sha}, nil
}

// UpdateFile implements provider.FileManager.
func (p *Provider) UpdateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	updateOpts := gitea.UpdateFileOptions{
		FileOptions: gitea.FileOptions{Message: opts.Message, BranchName: opts.Branch},
		SHA:         opts.SHA,
		Content:     opts.Content,
	}
	resp, _, err := p.client.UpdateFile(owner, repo, opts.Path, updateOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "UpdateFile", err)
	}
	sha := ""
	if resp.Commit != nil {
		sha = resp.Commit.SHA
	}
	return &provider.FileResult{CommitSHA: sha}, nil
}

// DeleteFile implements provider.FileManager.
func (p *Provider) DeleteFile(ctx context.Context, owner, repo string, opts provider.FileDeleteOptions) (*provider.FileResult, error) {
	deleteOpts := gitea.DeleteFileOptions{
		FileOptions: gitea.FileOptions{Message: opts.Message, BranchName: opts.Branch},
		SHA:         opts.SHA,
	}
	resp, err := p.client.DeleteFile(owner, repo, opts.Path, deleteOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "DeleteFile", err)
	}
	sha := ""
	if resp != nil {
		sha = resp.Header.Get("X-Commit-Sha")
	}
	return &provider.FileResult{CommitSHA: sha}, nil
}

var _ provider.FileManager = (*Provider)(nil)
