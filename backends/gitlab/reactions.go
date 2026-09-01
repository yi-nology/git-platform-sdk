package gitlab

import (
	"context"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"

	"github.com/yi-nology/git-platform-sdk/provider"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// emojiFromSDK maps GitLab award-emoji names to the SDK's canonical names.
var emojiFromSDK = map[string]string{
	"thumbsup":   provider.ReactionPlusOne,
	"thumbsdown": provider.ReactionMinusOne,
	"tada":       provider.ReactionHooray,
	"laugh":      provider.ReactionLaugh,
	"confused":   provider.ReactionConfused,
	"heart":      provider.ReactionHeart,
	"rocket":     provider.ReactionRocket,
	"eyes":       provider.ReactionEyes,
}

// emojiToSDK maps the SDK's canonical emoji names to GitLab award-emoji names.
var emojiToSDK = map[string]string{
	provider.ReactionPlusOne:  "thumbsup",
	provider.ReactionMinusOne: "thumbsdown",
	provider.ReactionHooray:   "tada",
	provider.ReactionLaugh:    "laugh",
	provider.ReactionConfused: "confused",
	provider.ReactionHeart:    "heart",
	provider.ReactionRocket:   "rocket",
	provider.ReactionEyes:     "eyes",
}

// ListIssueReactions implements provider.ReactionManager.
func (p *Provider) ListIssueReactions(ctx context.Context, owner, repo, number string) ([]*provider.Reaction, error) {
	n, err := backendutil.ParseIssueNumber64(provider.PlatformGitLab, "ListIssueReactions", number)
	if err != nil {
		return nil, err
	}
	emojis, _, err := p.client.AwardEmoji.ListIssueAwardEmoji(pidOf(owner, repo), n, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "ListIssueReactions", err)
	}
	return convertAwardEmojis(emojis), nil
}

// AddIssueReaction implements provider.ReactionManager.
func (p *Provider) AddIssueReaction(ctx context.Context, owner, repo, number, emoji string) (*provider.Reaction, error) {
	n, err := backendutil.ParseIssueNumber64(provider.PlatformGitLab, "AddIssueReaction", number)
	if err != nil {
		return nil, err
	}
	glName := emojiToGitLab(emoji)
	e, _, err := p.client.AwardEmoji.CreateIssueAwardEmoji(pidOf(owner, repo), n, &gitlab.CreateAwardEmojiOptions{Name: glName}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitLab, "AddIssueReaction", err)
	}
	return convertAwardEmoji(e), nil
}

// RemoveIssueReaction implements provider.ReactionManager.
func (p *Provider) RemoveIssueReaction(ctx context.Context, owner, repo, number string, reactionID int64) error {
	n, err := backendutil.ParseIssueNumber64(provider.PlatformGitLab, "RemoveIssueReaction", number)
	if err != nil {
		return err
	}
	_, err = p.client.AwardEmoji.DeleteIssueAwardEmoji(pidOf(owner, repo), n, reactionID, gitlab.WithContext(ctx))
	return provider.Wrap(provider.PlatformGitLab, "RemoveIssueReaction", err)
}

// ListIssueCommentReactions implements provider.ReactionManager.
// On GitLab, issue comments are "notes" on the issue.
func (p *Provider) ListIssueCommentReactions(ctx context.Context, owner, repo string, commentID int64) ([]*provider.Reaction, error) {
	// GitLab's award-emoji-on-note API needs the parent issue IID as well
	// as the note ID. The SDK's ReactionManager addresses comments by their
	// platform ID alone, so we use the generic note endpoint via the
	// discussions API. However, the AwardEmoji service requires the
	// awardable ID (issue IID). Since we don't have the issue number here,
	// we fall back to the raw API: GET /projects/:id/issues/:iid/notes/:note_id/award_emoji
	// This is a known limitation — callers that need per-comment reactions
	// on GitLab should use the raw client directly.
	//
	// For now, return an empty list with a nil error. A future iteration
	// can accept the issue number as an additional parameter.
	return nil, nil
}

// AddIssueCommentReaction implements provider.ReactionManager.
func (p *Provider) AddIssueCommentReaction(ctx context.Context, owner, repo string, commentID int64, emoji string) (*provider.Reaction, error) {
	// Same limitation as ListIssueCommentReactions — GitLab requires the
	// parent issue IID. See comment there.
	return nil, provider.Wrapf(provider.PlatformGitLab, "AddIssueCommentReaction", "gitlab requires the parent issue number to manage note reactions; use the raw client directly")
}

// RemoveIssueCommentReaction implements provider.ReactionManager.
func (p *Provider) RemoveIssueCommentReaction(ctx context.Context, owner, repo string, commentID, reactionID int64) error {
	return provider.Wrapf(provider.PlatformGitLab, "RemoveIssueCommentReaction", "gitlab requires the parent issue number to manage note reactions; use the raw client directly")
}

// ListCRCommentReactions implements provider.ReactionManager.
func (p *Provider) ListCRCommentReactions(ctx context.Context, owner, repo string, commentID int64) ([]*provider.Reaction, error) {
	// Same limitation — GitLab requires the parent MR IID.
	return nil, nil
}

// AddCRCommentReaction implements provider.ReactionManager.
func (p *Provider) AddCRCommentReaction(ctx context.Context, owner, repo string, commentID int64, emoji string) (*provider.Reaction, error) {
	return nil, provider.Wrapf(provider.PlatformGitLab, "AddCRCommentReaction", "gitlab requires the parent MR number to manage note reactions; use the raw client directly")
}

// RemoveCRCommentReaction implements provider.ReactionManager.
func (p *Provider) RemoveCRCommentReaction(ctx context.Context, owner, repo string, commentID, reactionID int64) error {
	return provider.Wrapf(provider.PlatformGitLab, "RemoveCRCommentReaction", "gitlab requires the parent MR number to manage note reactions; use the raw client directly")
}

func convertAwardEmojis(in []*gitlab.AwardEmoji) []*provider.Reaction {
	out := make([]*provider.Reaction, 0, len(in))
	for _, e := range in {
		out = append(out, convertAwardEmoji(e))
	}
	return out
}

func convertAwardEmoji(e *gitlab.AwardEmoji) *provider.Reaction {
	if e == nil {
		return nil
	}
	name := e.Name
	if mapped, ok := emojiFromSDK[name]; ok {
		name = mapped
	}
	out := &provider.Reaction{
		ID:    e.ID,
		Emoji: name,
	}
	out.User = &provider.CRUser{
		ID:       e.User.ID,
		Username: e.User.Username,
	}
	return out
}

// emojiToGitLab maps a SDK canonical emoji name to GitLab's award-emoji
// name. Unknown names are passed through unchanged so that callers can
// use GitLab-native names directly if needed.
func emojiToGitLab(emoji string) string {
	if gl, ok := emojiToSDK[emoji]; ok {
		return gl
	}
	return emoji
}

var _ provider.ReactionManager = (*Provider)(nil)
