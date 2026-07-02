package gitea

import (
	"context"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetFileContent implements provider.FileManager.
func (p *Provider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	data, _, err := p.client.GetFile(owner, repo, ref, path)
	if err != nil {
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