package gitcode

import (
	"context"
	"fmt"
	"strconv"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo, number string) (*provider.MergeDiff, error) {
	n, err := prNumber("GetCRDiff", number)
	if err != nil {
		return nil, err
	}
	files, err := p.client.ListPullRequestFiles(ctx, owner, repo, n)
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
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo, number string) ([]*provider.ChangedFile, error) {
	n, err := prNumber("GetCRFiles", number)
	if err != nil {
		return nil, err
	}
	files, err := p.client.ListPullRequestFiles(ctx, owner, repo, n)
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
func (p *Provider) CreateNote(ctx context.Context, owner, repo, number, body string) (string, error) {
	n, err := prNumber("CreateNote", number)
	if err != nil {
		return "", err
	}
	comment, err := p.client.CreatePullRequestComment(ctx, owner, repo, n, body, "", "", "")
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitCode, "CreateNote", err)
	}
	return string(comment.ID), nil
}

// DeleteNote implements provider.DiffManager. The note itself is addressed
// by its numeric platform ID; the change-request number is only validated,
// as GitCode's delete-comment endpoint does not take it.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo, number, noteID string) error {
	if _, err := prNumber("DeleteNote", number); err != nil {
		return err
	}
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
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo, number string, opts provider.DiscussionOptions) (string, error) {
	n, err := prNumber("CreateDiscussion", number)
	if err != nil {
		return "", err
	}
	comment, err := p.client.CreatePullRequestComment(ctx, owner, repo, n, opts.Body, "", "", "")
	if err != nil {
		return "", provider.Wrap(provider.PlatformGitCode, "CreateDiscussion", err)
	}
	return string(comment.ID), nil
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
