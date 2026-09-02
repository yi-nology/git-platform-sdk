package gitea

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"

	"code.gitea.io/sdk/gitea"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListIssueReactions implements provider.ReactionManager.
func (p *Provider) ListIssueReactions(ctx context.Context, owner, repo, number string) ([]*provider.Reaction, error) {
	n, err := backendutil.ParseIssueNumber64(provider.PlatformGitea, "ListIssueReactions", number)
	if err != nil {
		return nil, err
	}
	reactions, _, err := p.client.ListIssueReactions(owner, repo, n, gitea.ListIssueReactionsOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListIssueReactions", err)
	}
	return convertReactions(reactions), nil
}

// AddIssueReaction implements provider.ReactionManager.
func (p *Provider) AddIssueReaction(ctx context.Context, owner, repo, number, emoji string) (*provider.Reaction, error) {
	n, err := backendutil.ParseIssueNumber64(provider.PlatformGitea, "AddIssueReaction", number)
	if err != nil {
		return nil, err
	}
	r, _, err := p.client.PostIssueReaction(owner, repo, n, emoji)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "AddIssueReaction", err)
	}
	return convertReaction(r), nil
}

// RemoveIssueReaction implements provider.ReactionManager.
//
// KNOWN N+1 COST: Gitea reactions do not have their own ID; the reactionID
// parameter is actually the reacting user's User.ID (set in convertReaction).
// Gitea's DeleteIssueReaction API only accepts the reaction content string
// (emoji), not an ID, so we must ListIssueReactions to find the matching
// user's reaction content first. This is an O(reactions) scan per call and
// cannot be avoided given Gitea's API design.
func (p *Provider) RemoveIssueReaction(ctx context.Context, owner, repo, number string, reactionID int64) error {
	n, err := backendutil.ParseIssueNumber64(provider.PlatformGitea, "RemoveIssueReaction", number)
	if err != nil {
		return err
	}
	// Gitea's DeleteIssueReaction takes the reaction content string, not the ID.
	// Since we only have the user ID, we must list all reactions to find the
	// matching content string. This is the only approach available with Gitea's API.
	reactions, _, err := p.client.ListIssueReactions(owner, repo, n, gitea.ListIssueReactionsOptions{})
	if err != nil {
		return provider.Wrap(provider.PlatformGitea, "RemoveIssueReaction", err)
	}
	for _, r := range reactions {
		if r.User != nil && r.User.ID == reactionID {
			_, err = p.client.DeleteIssueReaction(owner, repo, n, r.Reaction)
			return provider.Wrap(provider.PlatformGitea, "RemoveIssueReaction", err)
		}
	}
	return provider.Wrapf(provider.PlatformGitea, "RemoveIssueReaction", "reaction %d not found", reactionID)
}

// ListCRReactions implements provider.ReactionManager.
// Gitea does not have a dedicated PR-level reaction API; PR reactions are
// the same as issue reactions, but the SDK exposes them separately for
// consistency. This returns "not supported" for now.
func (p *Provider) ListCRReactions(ctx context.Context, owner, repo, number string) ([]*provider.Reaction, error) {
	return nil, provider.Wrapf(provider.PlatformGitea, "ListCRReactions", "gitea does not support change-request-level reactions; use ListIssueReactions instead")
}

// ListIssueCommentReactions implements provider.ReactionManager.
func (p *Provider) ListIssueCommentReactions(ctx context.Context, owner, repo string, commentID int64) ([]*provider.Reaction, error) {
	reactions, _, err := p.client.GetIssueCommentReactions(owner, repo, commentID)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "ListIssueCommentReactions", err)
	}
	return convertReactions(reactions), nil
}

// AddIssueCommentReaction implements provider.ReactionManager.
func (p *Provider) AddIssueCommentReaction(ctx context.Context, owner, repo string, commentID int64, emoji string) (*provider.Reaction, error) {
	r, _, err := p.client.PostIssueCommentReaction(owner, repo, commentID, emoji)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "AddIssueCommentReaction", err)
	}
	return convertReaction(r), nil
}

// RemoveIssueCommentReaction implements provider.ReactionManager.
//
// KNOWN N+1 COST: Same trade-off as RemoveIssueReaction — Gitea reactions
// lack their own ID, so reactionID is the user's User.ID and we must list
// all comment reactions to find the content string for deletion.
func (p *Provider) RemoveIssueCommentReaction(ctx context.Context, owner, repo string, commentID, reactionID int64) error {
	reactions, _, err := p.client.GetIssueCommentReactions(owner, repo, commentID)
	if err != nil {
		return provider.Wrap(provider.PlatformGitea, "RemoveIssueCommentReaction", err)
	}
	for _, r := range reactions {
		if r.User != nil && r.User.ID == reactionID {
			_, err = p.client.DeleteIssueCommentReaction(owner, repo, commentID, r.Reaction)
			return provider.Wrap(provider.PlatformGitea, "RemoveIssueCommentReaction", err)
		}
	}
	return provider.Wrapf(provider.PlatformGitea, "RemoveIssueCommentReaction", "reaction %d not found", reactionID)
}

// ListCRCommentReactions implements provider.ReactionManager.
// Gitea does not have a dedicated PR comment reaction API; this returns an
// empty list.
func (p *Provider) ListCRCommentReactions(ctx context.Context, owner, repo string, commentID int64) ([]*provider.Reaction, error) {
	return nil, nil
}

// AddCRCommentReaction implements provider.ReactionManager.
func (p *Provider) AddCRCommentReaction(ctx context.Context, owner, repo string, commentID int64, emoji string) (*provider.Reaction, error) {
	return nil, provider.Wrapf(provider.PlatformGitea, "AddCRCommentReaction", "gitea does not support reactions on pull-request review comments")
}

// RemoveCRCommentReaction implements provider.ReactionManager.
func (p *Provider) RemoveCRCommentReaction(ctx context.Context, owner, repo string, commentID, reactionID int64) error {
	return provider.Wrapf(provider.PlatformGitea, "RemoveCRCommentReaction", "gitea does not support reactions on pull-request review comments")
}

func convertReactions(in []*gitea.Reaction) []*provider.Reaction {
	out := make([]*provider.Reaction, 0, len(in))
	for _, r := range in {
		out = append(out, convertReaction(r))
	}
	return out
}

func convertReaction(r *gitea.Reaction) *provider.Reaction {
	if r == nil {
		return nil
	}
	out := &provider.Reaction{
		Emoji: r.Reaction,
	}
	if r.User != nil {
		out.ID = r.User.ID
		out.User = &provider.CRUser{
			ID:       r.User.ID,
			Username: r.User.UserName,
		}
	}
	return out
}

var _ provider.ReactionManager = (*Provider)(nil)
