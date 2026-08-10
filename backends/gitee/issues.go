package gitee

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/provider"
)

func (p *Provider) ListIssues(_ context.Context, _ provider.ListIssuesOptions) ([]*provider.Issue, int, error) {
	return nil, 0, provider.Wrap(provider.PlatformGitee, "ListIssues", provider.ErrNotImplemented)
}
func (p *Provider) GetIssue(_ context.Context, _, _ string, _ int) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformGitee, "GetIssue", provider.ErrNotImplemented)
}
func (p *Provider) CreateIssue(_ context.Context, _ provider.CreateIssueOptions) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformGitee, "CreateIssue", provider.ErrNotImplemented)
}
func (p *Provider) UpdateIssue(_ context.Context, _, _ string, _ int, _ provider.UpdateIssueOptions) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformGitee, "UpdateIssue", provider.ErrNotImplemented)
}
func (p *Provider) CloseIssue(_ context.Context, _, _ string, _ int) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformGitee, "CloseIssue", provider.ErrNotImplemented)
}
func (p *Provider) ReopenIssue(_ context.Context, _, _ string, _ int) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformGitee, "ReopenIssue", provider.ErrNotImplemented)
}
func (p *Provider) ListIssueComments(_ context.Context, _, _ string, _ int) ([]*provider.IssueComment, error) {
	return nil, provider.Wrap(provider.PlatformGitee, "ListIssueComments", provider.ErrNotImplemented)
}
func (p *Provider) CreateIssueComment(_ context.Context, _, _ string, _ int, _ string) (*provider.IssueComment, error) {
	return nil, provider.Wrap(provider.PlatformGitee, "CreateIssueComment", provider.ErrNotImplemented)
}
func (p *Provider) ListIssueLabels(_ context.Context, _, _ string) ([]*provider.IssueLabel, error) {
	return nil, provider.Wrap(provider.PlatformGitee, "ListIssueLabels", provider.ErrNotImplemented)
}
func (p *Provider) AddIssueLabels(_ context.Context, _, _ string, _ int, _ []string) error {
	return provider.Wrap(provider.PlatformGitee, "AddIssueLabels", provider.ErrNotImplemented)
}
func (p *Provider) RemoveIssueLabel(_ context.Context, _, _ string, _ int, _ string) error {
	return provider.Wrap(provider.PlatformGitee, "RemoveIssueLabel", provider.ErrNotImplemented)
}

var _ provider.IssueManager = (*Provider)(nil)
