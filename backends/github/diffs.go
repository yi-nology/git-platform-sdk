package github

import (
	"context"
	"strconv"

	"github.com/google/go-github/v69/github"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*provider.MergeDiff, error) {
	diff := &provider.MergeDiff{}
	page := 1
	for {
		files, _, err := p.client.PullRequests.ListFiles(ctx, owner, repo, number, &github.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			return nil, provider.Wrap(provider.PlatformGitHub, "GetCRDiff", err)
		}
		for _, f := range files {
			cf := &provider.ChangedFile{
				OldPath:   f.GetPreviousFilename(),
				NewPath:   f.GetFilename(),
				Diff:      f.GetPatch(),
				Additions: f.GetAdditions(),
				Deletions: f.GetDeletions(),
				IsNew:     f.GetStatus() == "added",
				IsDeleted: f.GetStatus() == "removed",
				IsRenamed: f.GetStatus() == "renamed",
			}
			if cf.OldPath == "" {
				cf.OldPath = cf.NewPath
			}
			diff.Files = append(diff.Files, cf)
		}
		if len(files) < 100 {
			break
		}
		page++
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
	comment, _, err := p.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
		Body: github.String(body),
	})
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitHub, "CreateNote", err)
	}
	return strconv.FormatInt(comment.GetID(), 10), nil
}

// DeleteNote implements provider.DiffManager.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	id, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return provider.Wrapf(provider.PlatformGitHub, "DeleteNote", "invalid note ID %q: %v", noteID, err)
	}
	_, err = p.client.Issues.DeleteComment(ctx, owner, repo, id)
	if err != nil {
		return provider.Wrap(provider.PlatformGitHub, "DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts provider.DiscussionOptions) (string, error) {
	comment := &github.PullRequestComment{
		Body: github.String(opts.Body),
		Path: github.String(opts.FilePath),
	}
	if opts.NewLine > 0 {
		comment.Line = github.Int(opts.NewLine)
		comment.Side = github.String("RIGHT")
	} else if opts.OldLine > 0 {
		comment.Line = github.Int(opts.OldLine)
		comment.Side = github.String("LEFT")
	}
	c, _, err := p.client.PullRequests.CreateComment(ctx, owner, repo, number, comment)
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitHub, "CreateDiscussion", err)
	}
	return strconv.FormatInt(c.GetID(), 10), nil
}

// CreateReview implements provider.DiffManager.
func (p *Provider) CreateReview(ctx context.Context, owner, repo string, number int, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	reviewRequest := &github.PullRequestReviewRequest{
		CommitID: github.String(opts.CommitID),
		Body:     github.String(opts.Body),
		Event:    github.String(opts.Event),
	}
	for _, c := range opts.Comments {
		rc := &github.DraftReviewComment{
			Path: github.String(c.Path),
			Body: github.String(c.Body),
		}
		if c.StartLine > 0 && c.EndLine > c.StartLine {
			rc.StartLine = github.Int(c.StartLine)
			rc.Line = github.Int(c.EndLine)
			if c.Side != "" {
				rc.Side = github.String(c.Side)
			} else {
				rc.Side = github.String("RIGHT")
			}
			if c.StartLine != c.EndLine {
				rc.StartSide = github.String("RIGHT")
			}
		} else if c.Line > 0 {
			rc.Line = github.Int(c.Line)
			if c.Side != "" {
				rc.Side = github.String(c.Side)
			} else {
				rc.Side = github.String("RIGHT")
			}
		}
		reviewRequest.Comments = append(reviewRequest.Comments, rc)
	}
	review, _, err := p.client.PullRequests.CreateReview(ctx, owner, repo, number, reviewRequest)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitHub, "CreateReview", err)
	}
	result := &provider.ReviewResult{
		ID: strconv.FormatInt(review.GetID(), 10),
	}
	if review.GetHTMLURL() != "" {
		result.HTMLURL = review.GetHTMLURL()
	}
	if review.GetUser() != nil {
		result.User = convertUser(review.GetUser())
	}
	return result, nil
}

// compile-time guard
var _ provider.DiffManager = (*Provider)(nil)