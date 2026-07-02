package gitcode

import (
	"context"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetFileContent implements provider.FileManager.
func (p *Provider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	content, err := p.client.GetRepositoryContent(ctx, owner, repo, path, ref)
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitCode, "GetFileContent", err)
	}
	return content.Content, nil
}

// CreateFile implements provider.FileManager.
func (p *Provider) CreateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	result, err := p.client.CreateFile(ctx, owner, repo, opts.Path, gitcode.CreateFileOptions{
		Message: opts.Message, Content: opts.Content, Branch: opts.Branch,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CreateFile", err)
	}
	return fileResultFrom(result), nil
}

// UpdateFile implements provider.FileManager.
func (p *Provider) UpdateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	result, err := p.client.UpdateFile(ctx, owner, repo, opts.Path, gitcode.UpdateFileOptions{
		Message: opts.Message, Content: opts.Content, SHA: opts.SHA, Branch: opts.Branch,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateFile", err)
	}
	return fileResultFrom(result), nil
}

// DeleteFile implements provider.FileManager.
func (p *Provider) DeleteFile(ctx context.Context, owner, repo string, opts provider.FileDeleteOptions) (*provider.FileResult, error) {
	result, err := p.client.DeleteFile(ctx, owner, repo, opts.Path, gitcode.DeleteFileOptions{
		Message: opts.Message, SHA: opts.SHA, Branch: opts.Branch,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "DeleteFile", err)
	}
	sha := ""
	commitSHA := ""
	if result != nil {
		if result.Commit != nil {
			commitSHA = result.Commit.SHA
		}
	}
	_ = sha
	return &provider.FileResult{CommitSHA: commitSHA}, nil
}

func fileResultFrom(result *gitcode.FileResult) *provider.FileResult {
	if result == nil {
		return &provider.FileResult{}
	}
	sha := ""
	commitSHA := ""
	if result.Content != nil {
		sha = result.Content.SHA
	}
	if result.Commit != nil {
		commitSHA = result.Commit.SHA
	}
	return &provider.FileResult{SHA: sha, CommitSHA: commitSHA}
}

var _ provider.FileManager = (*Provider)(nil)
