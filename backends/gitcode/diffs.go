package gitcode

import (
	"context"
	"fmt"
	"strconv"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*provider.MergeDiff, error) {
	files, err := p.client.ListPullRequestFiles(ctx, owner, repo, number)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetCRDiff", err)
	}
	diff := &provider.MergeDiff{}
	for _, f := range files {
		cf := convertChangedFile(f)
		diff.Files = append(diff.Files, cf)
		diff.TotalAdd += cf.Additions
		diff.TotalDel += cf.Deletions
	}
	diff.RawDiff = provider.BuildRawDiff(diff.Files)
	return diff, nil
}

// GetCRFiles implements provider.DiffManager.
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*provider.ChangedFile, error) {
	files, err := p.client.ListPullRequestFiles(ctx, owner, repo, number)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetCRFiles", err)
	}
	result := make([]*provider.ChangedFile, 0, len(files))
	for _, f := range files {
		result = append(result, convertChangedFile(f))
	}
	return result, nil
}

// CreateNote implements provider.DiffManager.
func (p *Provider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	comment, err := p.client.CreatePullRequestComment(ctx, owner, repo, number, body, "", "", "")
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitCode, "CreateNote", err)
	}
	return fmt.Sprintf("%s", comment.ID), nil
}

// DeleteNote implements provider.DiffManager.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	id, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteNote", err)
	}
	err = p.client.DeleteIssueComment(ctx, owner, repo, id)
	if err != nil {
		return provider.Wrap(provider.PlatformGitCode, "DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts provider.DiscussionOptions) (string, error) {
	comment, err := p.client.CreatePullRequestComment(ctx, owner, repo, number, opts.Body, "", "", "")
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitCode, "CreateDiscussion", err)
	}
	return fmt.Sprintf("%s", comment.ID), nil
}

// CreateReview implements provider.DiffManager.
func (p *Provider) CreateReview(ctx context.Context, owner, repo string, number int, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	review, err := p.client.CreatePullRequestReview(ctx, owner, repo, number, opts.Body, opts.Event)
	if err != nil {
		return p.createReviewFallback(ctx, owner, repo, number, opts)
	}
	result := &provider.ReviewResult{ID: fmt.Sprintf("%d", review.ID)}
	user := review.User
	if user == nil {
		user = review.Author
	}
	if user != nil {
		authorID, _ := strconv.ParseInt(string(user.ID), 10, 64)
		result.User = &provider.CRUser{
			ID: authorID, Username: user.Login, AvatarURL: user.AvatarURL,
		}
	}
	for _, c := range opts.Comments {
		if err := p.createInlineComment(ctx, owner, repo, number, c, opts.CommitID); err != nil && p.logger != nil {
			p.logger.Warn("inline comment failed", "path", c.Path, "line", c.Line, "error", err)
		}
	}
	return result, nil
}

func (p *Provider) createReviewFallback(ctx context.Context, owner, repo string, number int, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	var lastErr error
	for _, c := range opts.Comments {
		if err := p.createInlineComment(ctx, owner, repo, number, c, opts.CommitID); err != nil {
			lastErr = err
		}
	}
	if opts.Body != "" {
		if _, err := p.CreateNote(ctx, owner, repo, number, opts.Body); err != nil {
			lastErr = err
		}
	}
	if lastErr != nil && len(opts.Comments) == 0 {
		return nil, lastErr
	}
	return &provider.ReviewResult{}, nil
}

func (p *Provider) createInlineComment(ctx context.Context, owner, repo string, number int, comment provider.ReviewComment, commitID string) error {
	side := comment.Side
	if side == "" {
		side = "RIGHT"
	}
	_, err := p.client.CreatePullRequestInlineComment(ctx, owner, repo, number, gitcode.CreatePullRequestInlineCommentOptions{
		Body: comment.Body, Path: comment.Path, Line: comment.Line, Side: side, CommitID: commitID,
	})
	return err
}

func convertChangedFile(f *gitcode.PullRequestFile) *provider.ChangedFile {
	patch := ""
	if f.Patch != nil {
		patch = fmt.Sprint(f.Patch)
	}
	cf := &provider.ChangedFile{
		OldPath:   f.PreviousFilename,
		NewPath:   f.Filename,
		Diff:      patch,
		Additions: f.Additions,
		Deletions: f.Deletions,
		IsNew:     f.Status == "added",
		IsDeleted: f.Status == "removed",
		IsRenamed: f.Status == "renamed",
	}
	if cf.OldPath == "" {
		cf.OldPath = cf.NewPath
	}
	return cf
}

var _ provider.DiffManager = (*Provider)(nil)