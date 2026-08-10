package provider

import "context"

// IssueManager provides issue CRUD, comments, and label management.
type IssueManager interface {
	ListIssues(ctx context.Context, opts ListIssuesOptions) ([]*Issue, int, error)
	GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error)
	CreateIssue(ctx context.Context, opts CreateIssueOptions) (*Issue, error)
	UpdateIssue(ctx context.Context, owner, repo string, number int, opts UpdateIssueOptions) (*Issue, error)
	CloseIssue(ctx context.Context, owner, repo string, number int) (*Issue, error)
	ReopenIssue(ctx context.Context, owner, repo string, number int) (*Issue, error)
	ListIssueComments(ctx context.Context, owner, repo string, number int) ([]*IssueComment, error)
	CreateIssueComment(ctx context.Context, owner, repo string, number int, body string) (*IssueComment, error)
	ListIssueLabels(ctx context.Context, owner, repo string) ([]*IssueLabel, error)
	AddIssueLabels(ctx context.Context, owner, repo string, number int, labels []string) error
	RemoveIssueLabel(ctx context.Context, owner, repo string, number int, name string) error
}
