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
		Body: github.Ptr(body),
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
		Body: github.Ptr(opts.Body),
		Path: github.Ptr(opts.FilePath),
	}
	if opts.NewLine > 0 {
		comment.Line = github.Ptr(opts.NewLine)
		comment.Side = github.Ptr("RIGHT")
	} else if opts.OldLine > 0 {
		comment.Line = github.Ptr(opts.OldLine)
		comment.Side = github.Ptr("LEFT")
	}
	c, _, err := p.client.PullRequests.CreateComment(ctx, owner, repo, number, comment)
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitHub, "CreateDiscussion", err)
	}
	return strconv.FormatInt(c.GetID(), 10), nil
}

// compile-time guard
var _ provider.DiffManager = (*Provider)(nil)
