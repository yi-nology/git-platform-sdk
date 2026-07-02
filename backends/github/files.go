package github

import (
	"context"
	"fmt"
	"io"

	"github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetFileContent implements provider.FileManager.
func (p *Provider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	opts := &github.RepositoryContentGetOptions{Ref: ref}
	rc, _, err := p.client.Repositories.DownloadContents(ctx, owner, repo, path, opts)
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitHub, "GetFileContent", err)
	}
	if rc == nil {
		return "", fmt.Errorf("github: file not found: %s", path)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitHub, "GetFileContent", err)
	}
	return string(data), nil
}

// CreateFile implements provider.FileManager.
func (p *Provider) CreateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	optsReq := &github.RepositoryContentFileOptions{
		Message: github.String(opts.Message),
		Content: []byte(opts.Content),
	}
	if opts.Branch != "" {
		optsReq.Branch = github.String(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		optsReq.Author = &github.CommitAuthor{Name: github.String(opts.Author), Email: github.String(opts.Email)}
	}
	resp, _, err := p.client.Repositories.CreateFile(ctx, owner, repo, opts.Path, optsReq)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateFile", err)
	}
	return &provider.FileResult{CommitSHA: resp.GetSHA()}, nil
}

// UpdateFile implements provider.FileManager.
func (p *Provider) UpdateFile(ctx context.Context, owner, repo string, opts provider.FileOptions) (*provider.FileResult, error) {
	optsReq := &github.RepositoryContentFileOptions{
		Message: github.String(opts.Message),
		Content: []byte(opts.Content),
	}
	if opts.SHA != "" {
		optsReq.SHA = github.String(opts.SHA)
	}
	if opts.Branch != "" {
		optsReq.Branch = github.String(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		optsReq.Author = &github.CommitAuthor{Name: github.String(opts.Author), Email: github.String(opts.Email)}
	}
	resp, _, err := p.client.Repositories.UpdateFile(ctx, owner, repo, opts.Path, optsReq)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "UpdateFile", err)
	}
	return &provider.FileResult{CommitSHA: resp.GetSHA()}, nil
}

// DeleteFile implements provider.FileManager.
func (p *Provider) DeleteFile(ctx context.Context, owner, repo string, opts provider.FileDeleteOptions) (*provider.FileResult, error) {
	optsReq := &github.RepositoryContentFileOptions{
		Message: github.String(opts.Message),
	}
	if opts.SHA != "" {
		optsReq.SHA = github.String(opts.SHA)
	}
	if opts.Branch != "" {
		optsReq.Branch = github.String(opts.Branch)
	}
	if opts.Author != "" || opts.Email != "" {
		optsReq.Author = &github.CommitAuthor{Name: github.String(opts.Author), Email: github.String(opts.Email)}
	}
	resp, _, err := p.client.Repositories.DeleteFile(ctx, owner, repo, opts.Path, optsReq)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "DeleteFile", err)
	}
	return &provider.FileResult{CommitSHA: resp.GetSHA()}, nil
}

// compile-time guard
var _ provider.FileManager = (*Provider)(nil)