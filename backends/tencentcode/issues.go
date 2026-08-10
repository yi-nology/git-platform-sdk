package tencentcode

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/provider"
)

func (p *Provider) ListIssues(_ context.Context, _ provider.ListIssuesOptions) ([]*provider.Issue, int, error) {
	return nil, 0, provider.Wrap(provider.PlatformTencentCode, "ListIssues", provider.ErrNotImplemented)
}
func (p *Provider) GetIssue(_ context.Context, _, _ string, _ int) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformTencentCode, "GetIssue", provider.ErrNotImplemented)
}
func (p *Provider) CreateIssue(_ context.Context, _ provider.CreateIssueOptions) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformTencentCode, "CreateIssue", provider.ErrNotImplemented)
}
func (p *Provider) UpdateIssue(_ context.Context, _, _ string, _ int, _ provider.UpdateIssueOptions) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformTencentCode, "UpdateIssue", provider.ErrNotImplemented)
}
func (p *Provider) CloseIssue(_ context.Context, _, _ string, _ int) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformTencentCode, "CloseIssue", provider.ErrNotImplemented)
}
func (p *Provider) ReopenIssue(_ context.Context, _, _ string, _ int) (*provider.Issue, error) {
	return nil, provider.Wrap(provider.PlatformTencentCode, "ReopenIssue", provider.ErrNotImplemented)
}
func (p *Provider) ListIssueComments(_ context.Context, _, _ string, _ int) ([]*provider.IssueComment, error) {
	return nil, provider.Wrap(provider.PlatformTencentCode, "ListIssueComments", provider.ErrNotImplemented)
}
func (p *Provider) CreateIssueComment(_ context.Context, _, _ string, _ int, _ string) (*provider.IssueComment, error) {
	return nil, provider.Wrap(provider.PlatformTencentCode, "CreateIssueComment", provider.ErrNotImplemented)
}
func (p *Provider) ListIssueLabels(_ context.Context, _, _ string) ([]*provider.IssueLabel, error) {
	return nil, provider.Wrap(provider.PlatformTencentCode, "ListIssueLabels", provider.ErrNotImplemented)
}
func (p *Provider) AddIssueLabels(_ context.Context, _, _ string, _ int, _ []string) error {
	return provider.Wrap(provider.PlatformTencentCode, "AddIssueLabels", provider.ErrNotImplemented)
}
func (p *Provider) RemoveIssueLabel(_ context.Context, _, _ string, _ int, _ string) error {
	return provider.Wrap(provider.PlatformTencentCode, "RemoveIssueLabel", provider.ErrNotImplemented)
}

var _ provider.IssueManager = (*Provider)(nil)
