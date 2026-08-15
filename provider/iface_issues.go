package provider

import "context"

// IssueManager provides issue CRUD, comments, and label management. Issue
// numbers are strings: every platform address is representable as a string,
// and Gitee natively uses alphanumeric identifiers (e.g. "IAINVA"). Backends
// on numeric platforms parse with strconv.Atoi and fail with a wrapped
// "invalid issue number" error.
type IssueManager interface {
	ListIssues(ctx context.Context, opts ListIssuesOptions) ([]*Issue, int, error)
	GetIssue(ctx context.Context, owner, repo, number string) (*Issue, error)
	CreateIssue(ctx context.Context, opts CreateIssueOptions) (*Issue, error)
	UpdateIssue(ctx context.Context, owner, repo, number string, opts UpdateIssueOptions) (*Issue, error)
	CloseIssue(ctx context.Context, owner, repo, number string) (*Issue, error)
	ReopenIssue(ctx context.Context, owner, repo, number string) (*Issue, error)
	ListIssueComments(ctx context.Context, owner, repo, number string) ([]*IssueComment, error)
	CreateIssueComment(ctx context.Context, owner, repo, number, body string) (*IssueComment, error)
	ListIssueLabels(ctx context.Context, owner, repo string) ([]*IssueLabel, error)
	AddIssueLabels(ctx context.Context, owner, repo, number string, labels []string) error
	RemoveIssueLabel(ctx context.Context, owner, repo, number, name string) error
}
