package gitea

import (
	"context"
	"strconv"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*provider.MergeDiff, error) {
	diffBytes, _, err := p.client.GetPullRequestDiff(owner, repo, int64(number), gitea.PullRequestDiffOptions{})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetCRDiff", err)
	}
	rawDiff := string(diffBytes)

	files, err := p.GetCRFiles(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	totalAdd, totalDel := 0, 0
	for _, f := range files {
		totalAdd += f.Additions
		totalDel += f.Deletions
	}
	return &provider.MergeDiff{Files: files, TotalAdd: totalAdd, TotalDel: totalDel, RawDiff: rawDiff}, nil
}

// GetCRFiles implements provider.DiffManager.
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*provider.ChangedFile, error) {
	changedFiles, _, err := p.client.ListPullRequestFiles(owner, repo, int64(number), gitea.ListPullRequestFilesOptions{
		ListOptions: gitea.ListOptions{PageSize: 100},
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "GetCRFiles", err)
	}
	result := make([]*provider.ChangedFile, 0, len(changedFiles))
	for _, f := range changedFiles {
		cf := &provider.ChangedFile{
			OldPath:   f.PreviousFilename,
			NewPath:   f.Filename,
			Additions: f.Additions,
			Deletions: f.Deletions,
			IsNew:     f.Status == "added",
			IsDeleted: f.Status == "removed",
			IsRenamed: f.Status == "renamed",
		}
		if cf.OldPath == "" {
			cf.OldPath = cf.NewPath
		}
		result = append(result, cf)
	}
	return result, nil
}

// CreateNote implements provider.DiffManager.
func (p *Provider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	comment, _, err := p.client.CreateIssueComment(owner, repo, int64(number), gitea.CreateIssueCommentOption{Body: body})
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitea, "CreateNote", err)
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

// DeleteNote implements provider.DiffManager.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	id, err := strconv.ParseInt(noteID, 10, 64)
	if err != nil {
		return provider.Wrap(provider.PlatformGitea, "DeleteNote", err)
	}
	resp, err := p.client.DeleteIssueComment(owner, repo, id)
	if err != nil {
		// Gitea returns 404 when the comment is already gone; tolerate it.
		if resp != nil && resp.StatusCode == 404 {
			return nil
		}
		return provider.Wrap(provider.PlatformGitea, "DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
//
// Gitea has no separate discussion endpoint; we approximate one by posting
// an issue comment (the same backing store Gitea itself uses for PR
// discussions).
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts provider.DiscussionOptions) (string, error) {
	comment, _, err := p.client.CreateIssueComment(owner, repo, int64(number), gitea.CreateIssueCommentOption{Body: opts.Body})
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitea, "CreateDiscussion", err)
	}
	return strconv.FormatInt(comment.ID, 10), nil
}

// CreateReview implements provider.DiffManager.
func (p *Provider) CreateReview(ctx context.Context, owner, repo string, number int, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	reviewOpts := gitea.CreatePullReviewOptions{
		CommitID: opts.CommitID,
		Body:     opts.Body,
	}
	switch opts.Event {
	case "APPROVE":
		reviewOpts.State = gitea.ReviewStateApproved
	case "REQUEST_CHANGES":
		reviewOpts.State = gitea.ReviewStateRequestChanges
	default:
		reviewOpts.State = gitea.ReviewStateComment
	}

	for _, c := range opts.Comments {
		rc := gitea.CreatePullReviewComment{Path: c.Path, Body: c.Body}
		if c.Side == "LEFT" {
			rc.OldLineNum = int64(c.Line)
		} else {
			rc.NewLineNum = int64(c.Line)
		}
		if c.StartLine > 0 && c.EndLine > c.StartLine {
			if c.Side == "LEFT" {
				rc.OldLineNum = int64(c.StartLine)
			} else {
				rc.NewLineNum = int64(c.StartLine)
			}
		}
		reviewOpts.Comments = append(reviewOpts.Comments, rc)
	}

	review, _, err := p.client.CreatePullReview(owner, repo, int64(number), reviewOpts)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitea, "CreateReview", err)
	}
	result := &provider.ReviewResult{ID: strconv.FormatInt(review.ID, 10)}
	if review.HTMLURL != "" {
		result.HTMLURL = review.HTMLURL
	}
	if review.Reviewer != nil {
		result.User = &provider.CRUser{
			ID:        review.Reviewer.ID,
			Username:  review.Reviewer.UserName,
			AvatarURL: review.Reviewer.AvatarURL,
		}
	}
	return result, nil
}

var _ provider.DiffManager = (*Provider)(nil)
