package gitee

import (
	"context"

	gitee "github.com/next-bin/go-gitee/gitee"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateCommitStatus implements provider.CommitStatusManager.
// Gitee uses the Checks API (check runs) for CI status reporting.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	status := mapCommitStatus(opts.State)
	_, _, err := p.client.Checks.Create(ctx, esc(owner), esc(repo), &gitee.CreateCheckRunOptions{
		Name:       gitee.String(opts.Context),
		HeadSHA:    gitee.String(sha),
		Status:     gitee.String("completed"),
		Conclusion: gitee.String(status),
		Output: &gitee.CheckRunOutput{
			Title:   gitee.String(opts.Context),
			Summary: gitee.String(opts.Description),
		},
	})
	return provider.Wrap(provider.PlatformGitee, "CreateCommitStatus", err)
}

// mapCommitStatus maps the SDK's normalized state strings to Gitee check-run
// conclusion values.
func mapCommitStatus(state string) string {
	switch state {
	case "success":
		return "success"
	case "failure", "error":
		return "failure"
	case "pending":
		return "neutral"
	default:
		return "neutral"
	}
}

var _ provider.CommitStatusManager = (*Provider)(nil)
