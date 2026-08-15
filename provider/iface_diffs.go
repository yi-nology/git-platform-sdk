package provider

import "context"

// DiffManager handles diff and discussion operations. Review operations
// (CreateReview and friends) live on the optional ReviewManager capability
// interface; DiffManager itself carries five methods. Change request
// numbers are strings (same addressing scheme as IssueManager); numeric
// platforms parse with strconv and fail with a wrapped "invalid pull
// request number" error.
type DiffManager interface {
	GetCRDiff(ctx context.Context, owner, repo, number string) (*MergeDiff, error)
	GetCRFiles(ctx context.Context, owner, repo, number string) ([]*ChangedFile, error)
	CreateNote(ctx context.Context, owner, repo, number, body string) (string, error)
	DeleteNote(ctx context.Context, owner, repo, number string, noteID string) error
	CreateDiscussion(ctx context.Context, owner, repo, number string, opts DiscussionOptions) (string, error)
}
