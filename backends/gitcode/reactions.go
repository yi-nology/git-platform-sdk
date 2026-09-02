package gitcode

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"

	"github.com/yi-nology/git-platform-sdk/provider"
	gitcode "github.com/yi-nology/go-gitcode"
)

// ListIssueReactions implements provider.ReactionManager.
func (p *Provider) ListIssueReactions(ctx context.Context, owner, repo, number string) ([]*provider.Reaction, error) {
	n, err := backendutil.ParseIssueNumber(provider.PlatformGitCode, "ListIssueReactions", number)
	if err != nil {
		return nil, err
	}
	reactions, err := p.client.ListIssueReactions(ctx, owner, repo, n, listOpts())
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListIssueReactions", err)
	}
	return convertReactions(reactions), nil
}

// AddIssueReaction implements provider.ReactionManager.
func (p *Provider) AddIssueReaction(ctx context.Context, owner, repo, number, emoji string) (*provider.Reaction, error) {
	n, err := backendutil.ParseIssueNumber(provider.PlatformGitCode, "AddIssueReaction", number)
	if err != nil {
		return nil, err
	}
	r, err := p.client.CreateIssueReaction(ctx, owner, repo, n, emoji)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "AddIssueReaction", err)
	}
	return convertReaction(r), nil
}

// RemoveIssueReaction implements provider.ReactionManager.
func (p *Provider) RemoveIssueReaction(ctx context.Context, owner, repo, number string, reactionID int64) error {
	n, err := backendutil.ParseIssueNumber(provider.PlatformGitCode, "RemoveIssueReaction", number)
	if err != nil {
		return err
	}
	return provider.Wrap(provider.PlatformGitCode, "RemoveIssueReaction", p.client.DeleteIssueReaction(ctx, owner, repo, n, reactionID))
}

// ListIssueCommentReactions implements provider.ReactionManager.
func (p *Provider) ListIssueCommentReactions(ctx context.Context, owner, repo string, commentID int64) ([]*provider.Reaction, error) {
	reactions, err := p.client.ListIssueCommentReactions(ctx, owner, repo, commentID, listOpts())
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListIssueCommentReactions", err)
	}
	return convertReactions(reactions), nil
}

// AddIssueCommentReaction implements provider.ReactionManager.
func (p *Provider) AddIssueCommentReaction(ctx context.Context, owner, repo string, commentID int64, emoji string) (*provider.Reaction, error) {
	r, err := p.client.CreateIssueCommentReaction(ctx, owner, repo, commentID, emoji)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "AddIssueCommentReaction", err)
	}
	return convertReaction(r), nil
}

// RemoveIssueCommentReaction implements provider.ReactionManager.
func (p *Provider) RemoveIssueCommentReaction(ctx context.Context, owner, repo string, commentID, reactionID int64) error {
	return provider.Wrap(provider.PlatformGitCode, "RemoveIssueCommentReaction", p.client.DeleteIssueCommentReaction(ctx, owner, repo, commentID, reactionID))
}

// ListCRCommentReactions implements provider.ReactionManager.
func (p *Provider) ListCRCommentReactions(ctx context.Context, owner, repo string, commentID int64) ([]*provider.Reaction, error) {
	reactions, err := p.client.ListPullRequestCommentReactions(ctx, owner, repo, commentID, listOpts())
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListCRCommentReactions", err)
	}
	return convertReactions(reactions), nil
}

// AddCRCommentReaction implements provider.ReactionManager.
func (p *Provider) AddCRCommentReaction(ctx context.Context, owner, repo string, commentID int64, emoji string) (*provider.Reaction, error) {
	r, err := p.client.CreatePullRequestCommentReaction(ctx, owner, repo, commentID, emoji)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "AddCRCommentReaction", err)
	}
	return convertReaction(r), nil
}

// RemoveCRCommentReaction implements provider.ReactionManager.
func (p *Provider) RemoveCRCommentReaction(ctx context.Context, owner, repo string, commentID, reactionID int64) error {
	return provider.Wrap(provider.PlatformGitCode, "RemoveCRCommentReaction", p.client.DeletePullRequestCommentReaction(ctx, owner, repo, commentID, reactionID))
}

func listOpts() gitcode.ListOptions { return gitcode.ListOptions{PerPage: 100} }

// ListCRReactions implements provider.ReactionManager. On GitCode, PRs share
// the issue reaction API.
func (p *Provider) ListCRReactions(ctx context.Context, owner, repo, number string) ([]*provider.Reaction, error) {
	return p.ListIssueReactions(ctx, owner, repo, number)
}

var _ provider.ReactionManager = (*Provider)(nil)
