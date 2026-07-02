package gitlab

import (
	"context"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetFileContent implements provider.FileManager.
func (p *Provider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	content, _, err := p.client.RepositoryFiles.GetRawFile(pidOf(owner, repo), path,
		&gitlab.GetRawFileOptions{Ref: gitlab.Ptr(ref)}, gitlab.WithContext(ctx))
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitLab, "GetFileContent", err)
	}
	return string(content), nil
}

// CreateFile implements provider.FileManager.
func (p *Provider) CreateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	createOpts := &gitlab.CreateFileOptions{
		Content:       gitlab.Ptr(opts.Content),
		CommitMessage: gitlab.Ptr(opts.Message),
	}
	if opts.Branch != "" {
		createOpts.Branch = gitlab.Ptr(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		createOpts.AuthorName = gitlab.Ptr(opts.Author)
		createOpts.AuthorEmail = gitlab.Ptr(opts.Email)
	}
	_, resp, err := p.client.RepositoryFiles.CreateFile(pidOf(owner, repo), opts.Path, createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "CreateFile", err)
	}
	return &provider.FileResult{CommitSHA: resp.Header.Get("X-Gitlab-Commit-Id")}, nil
}

// UpdateFile implements provider.FileManager.
func (p *Provider) UpdateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	updateOpts := &gitlab.UpdateFileOptions{
		Content:       gitlab.Ptr(opts.Content),
		CommitMessage: gitlab.Ptr(opts.Message),
	}
	if opts.Branch != "" {
		updateOpts.Branch = gitlab.Ptr(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		updateOpts.AuthorName = gitlab.Ptr(opts.Author)
		updateOpts.AuthorEmail = gitlab.Ptr(opts.Email)
	}
	_, resp, err := p.client.RepositoryFiles.UpdateFile(pidOf(owner, repo), opts.Path, updateOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "UpdateFile", err)
	}
	return &provider.FileResult{CommitSHA: resp.Header.Get("X-Gitlab-Commit-Id")}, nil
}

// DeleteFile implements provider.FileManager.
func (p *Provider) DeleteFile(ctx context.Context, owner, repo string, opts provider.FileDeleteOptions) (*provider.FileResult, error) {
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
	resp, err := p.client.RepositoryFiles.DeleteFile(pidOf(owner, repo), opts.Path, deleteOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "DeleteFile", err)
	}
	return &provider.FileResult{CommitSHA: resp.Header.Get("X-Gitlab-Commit-Id")}, nil
}

var _ provider.FileManager = (*Provider)(nil)
