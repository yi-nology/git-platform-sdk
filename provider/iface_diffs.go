package provider

import "context"

// DiffManager handles diff and discussion operations. Review operations
// (CreateReview and friends) live on the optional ReviewManager capability
// interface; DiffManager itself carries five methods.
type DiffManager interface {
	GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error)
	GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error)
	CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error)
	DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error
	CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error)
}
