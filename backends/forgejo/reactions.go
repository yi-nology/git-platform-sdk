package forgejo

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListIssueReactions implements provider.ReactionManager.
func (p *Provider) ListIssueReactions(ctx context.Context, owner, repo, number string) ([]*provider.Reaction, error) {
	n, err := backendutil.ParseIssueNumber64(provider.PlatformForgejo, "ListIssueReactions", number)
	if err != nil {
		return nil, err
	}
	reactions, _, err := p.client.GetIssueReactions(owner, repo, n)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListIssueReactions", err)
	}
	return convertReactions(reactions), nil
}

// AddIssueReaction implements provider.ReactionManager.
func (p *Provider) AddIssueReaction(ctx context.Context, owner, repo, number, emoji string) (*provider.Reaction, error) {
	n, err := backendutil.ParseIssueNumber64(provider.PlatformForgejo, "AddIssueReaction", number)
	if err != nil {
		return nil, err
	}
	r, _, err := p.client.PostIssueReaction(owner, repo, n, emoji)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "AddIssueReaction", err)
	}
	return convertReaction(r), nil
}

// RemoveIssueReaction implements provider.ReactionManager.
func (p *Provider) RemoveIssueReaction(ctx context.Context, owner, repo, number string, reactionID int64) error {
	n, err := backendutil.ParseIssueNumber64(provider.PlatformForgejo, "RemoveIssueReaction", number)
	if err != nil {
		return err
	}
	reactions, _, err := p.client.GetIssueReactions(owner, repo, n)
	if err != nil {
		return provider.Wrap(provider.PlatformForgejo, "RemoveIssueReaction", err)
	}
	for _, r := range reactions {
		if r.User != nil && r.User.ID == reactionID {
			_, err = p.client.DeleteIssueReaction(owner, repo, n, r.Reaction)
			return provider.Wrap(provider.PlatformForgejo, "RemoveIssueReaction", err)
		}
	}
	return provider.Wrapf(provider.PlatformForgejo, "RemoveIssueReaction", "reaction %d not found", reactionID)
}

// ListIssueCommentReactions implements provider.ReactionManager.
func (p *Provider) ListIssueCommentReactions(ctx context.Context, owner, repo string, commentID int64) ([]*provider.Reaction, error) {
	reactions, _, err := p.client.GetIssueCommentReactions(owner, repo, commentID)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "ListIssueCommentReactions", err)
	}
	return convertReactions(reactions), nil
}

// AddIssueCommentReaction implements provider.ReactionManager.
func (p *Provider) AddIssueCommentReaction(ctx context.Context, owner, repo string, commentID int64, emoji string) (*provider.Reaction, error) {
	r, _, err := p.client.PostIssueCommentReaction(owner, repo, commentID, emoji)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformForgejo, "AddIssueCommentReaction", err)
	}
	return convertReaction(r), nil
}

// RemoveIssueCommentReaction implements provider.ReactionManager.
func (p *Provider) RemoveIssueCommentReaction(ctx context.Context, owner, repo string, commentID, reactionID int64) error {
	reactions, _, err := p.client.GetIssueCommentReactions(owner, repo, commentID)
	if err != nil {
		return provider.Wrap(provider.PlatformForgejo, "RemoveIssueCommentReaction", err)
	}
	for _, r := range reactions {
		if r.User != nil && r.User.ID == reactionID {
			_, err = p.client.DeleteIssueCommentReaction(owner, repo, commentID, r.Reaction)
			return provider.Wrap(provider.PlatformForgejo, "RemoveIssueCommentReaction", err)
		}
	}
	return provider.Wrapf(provider.PlatformForgejo, "RemoveIssueCommentReaction", "reaction %d not found", reactionID)
}

// ListCRCommentReactions implements provider.ReactionManager.
func (p *Provider) ListCRCommentReactions(ctx context.Context, owner, repo string, commentID int64) ([]*provider.Reaction, error) {
	return nil, nil
}

// AddCRCommentReaction implements provider.ReactionManager.
func (p *Provider) AddCRCommentReaction(ctx context.Context, owner, repo string, commentID int64, emoji string) (*provider.Reaction, error) {
	return nil, provider.Wrapf(provider.PlatformForgejo, "AddCRCommentReaction", "forgejo does not support reactions on pull-request review comments")
}

// RemoveCRCommentReaction implements provider.ReactionManager.
func (p *Provider) RemoveCRCommentReaction(ctx context.Context, owner, repo string, commentID, reactionID int64) error {
	return provider.Wrapf(provider.PlatformForgejo, "RemoveCRCommentReaction", "forgejo does not support reactions on pull-request review comments")
}

func convertReactions(in []*forgejo.Reaction) []*provider.Reaction {
	out := make([]*provider.Reaction, 0, len(in))
	for _, r := range in {
		out = append(out, convertReaction(r))
	}
	return out
}

func convertReaction(r *forgejo.Reaction) *provider.Reaction {
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
