package gitee

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*provider.MergeDiff, error) {
	var files []giteePRFile
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/repos/%s/%s/pulls/%d/files", esc(owner), esc(repo), number), nil, &files); err != nil {
		return nil, provider.Wrap(provider.PlatformGitee, "GetCRDiff", err)
	}
	diff := &provider.MergeDiff{}
	for _, f := range files {
		diff.Files = append(diff.Files, f.toChangedFile())
	}
	diff.TotalAdd, diff.TotalDel = provider.SumDiffStats(diff.Files)
	diff.RawDiff = provider.BuildRawDiff(diff.Files)
	return diff, nil
}

// GetCRFiles implements provider.DiffManager.
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*provider.ChangedFile, error) {
	diff, err := p.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

// CreateNote implements provider.DiffManager.
func (p *Provider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	reqBody := map[string]any{"body": body}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", esc(owner), esc(repo), number), reqBody, &resp); err != nil {
		return "", provider.Wrap(provider.PlatformGitee, "CreateNote", err)
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

// DeleteNote implements provider.DiffManager.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	err := p.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/pulls/comments/%s", esc(owner), esc(repo), esc(noteID)), nil, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformGitee, "DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
//
// Gitee has no separate discussion endpoint; PR comments are the only
// option. We post the body as a regular PR comment.
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts provider.DiscussionOptions) (string, error) {
	return p.CreateNote(ctx, owner, repo, number, opts.Body)
}

// CreateReview implements provider.DiffManager.
//
// Gitee does not expose a review endpoint in its public REST API, so we
// approximate one by posting the summary body as a note and each inline
// comment as a separate PR comment. The returned ID is a synthetic
// timestamp-based identifier.
func (p *Provider) CreateReview(ctx context.Context, owner, repo string, number int, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	if opts.Body != "" {
		if _, err := p.CreateNote(ctx, owner, repo, number, opts.Body); err != nil {
			return nil, err
		}
	}
	results := make([]provider.ReviewCommentResult, 0, len(opts.Comments))
	for _, c := range opts.Comments {
		body := c.Body
		if c.Path != "" {
			body = fmt.Sprintf("**%s**\n\n%s", c.Path, c.Body)
		}
		id, err := p.CreateNote(ctx, owner, repo, number, body)
		results = append(results, provider.ReviewCommentResult{
			Path:       c.Path,
			Line:       c.Line,
			ExternalID: id,
			Error: func() string {
				if err != nil {
					return err.Error()
				}
				return ""
			}(),
		})
	}
	return &provider.ReviewResult{
		ID:       fmt.Sprintf("ge-review-%d-%d", number, time.Now().UnixNano()),
		Comments: results,
	}, nil
}

var _ provider.DiffManager = (*Provider)(nil)
