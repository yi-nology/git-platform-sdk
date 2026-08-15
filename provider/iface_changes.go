package provider

import "context"

// ChangeRequestManager handles pull request / merge request lifecycle.
type ChangeRequestManager interface {
	CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error)
	GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error)
	ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error)
	MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error)
	CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error)
	ReopenCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error)
	UpdateCR(ctx context.Context, owner, repo string, number int, opts UpdateCROptions) (*ChangeRequest, error)
	UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error
	ListCRComments(ctx context.Context, owner, repo string, number int) ([]*CRComment, error)
	ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*CRCommit, error)
}
