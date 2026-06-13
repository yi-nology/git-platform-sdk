package provider

import "context"

// FileManager handles file CRUD operations on repositories.
type FileManager interface {
	GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error)
	CreateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error)
	UpdateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error)
	DeleteFile(ctx context.Context, owner, repo string, opts FileDeleteOptions) (*FileResult, error)
}
