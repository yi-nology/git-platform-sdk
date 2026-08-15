package gitee

import (
	"context"
	"strconv"

	gitee "gitee.com/openeuler/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo, number string) (*provider.MergeDiff, error) {
	n, err := prNumber("GetCRDiff", number)
	if err != nil {
		return nil, err
	}
	files, resp, err := p.client.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberFiles(ctx, esc(owner), esc(repo), n, &gitee.GetV5ReposOwnerRepoPullsNumberFilesOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return nil, p.sdkErr("GetCRDiff", resp, err)
	}
	diff := &provider.MergeDiff{}
	for i := range files {
		diff.Files = append(diff.Files, convertPRFile(files[i]))
	}
	diff.TotalAdd, diff.TotalDel = provider.SumDiffStats(diff.Files)
	diff.RawDiff = provider.BuildRawDiff(diff.Files)
	return diff, nil
}

// GetCRFiles implements provider.DiffManager.
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo, number string) ([]*provider.ChangedFile, error) {
	diff, err := p.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

// CreateNote implements provider.DiffManager.
func (p *Provider) CreateNote(ctx context.Context, owner, repo, number, body string) (string, error) {
	n, err := prNumber("CreateNote", number)
	if err != nil {
		return "", err
	}
	comment, resp, err := p.client.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberComments(ctx, esc(owner), esc(repo), n, gitee.PullRequestCommentPostParam{
		AccessToken: p.token,
		Body:        body,
	})
	if err != nil {
		return "", p.sdkErr("CreateNote", resp, err)
	}
	return strconv.FormatInt(int64(comment.Id), 10), nil
}

// DeleteNote implements provider.DiffManager. The note itself is addressed
// by its numeric platform ID; the change-request number is only validated,
// as Gitee's delete-comment endpoint does not take it.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo, number, noteID string) error {
	if _, err := prNumber("DeleteNote", number); err != nil {
		return err
	}
	id, err := strconv.Atoi(noteID)
	if err != nil {
		return provider.Wrapf(provider.PlatformGitee, "DeleteNote", "invalid note id %q", noteID)
	}
	resp, err := p.client.PullRequestsApi.DeleteV5ReposOwnerRepoPullsCommentsId(ctx, esc(owner), esc(repo), toInt32(id), &gitee.DeleteV5ReposOwnerRepoPullsCommentsIdOpts{
		AccessToken: p.accessToken(),
	})
	if err != nil {
		return p.sdkErr("DeleteNote", resp, err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
//
// Gitee has no separate discussion endpoint; PR comments are the only
// option. We post the body as a regular PR comment.
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo, number string, opts provider.DiscussionOptions) (string, error) {
	return p.CreateNote(ctx, owner, repo, number, opts.Body)
}

var _ provider.DiffManager = (*Provider)(nil)
