package tencentcode

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	gongfeng "github.com/studyzy/gongfeng-sdk-go"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo, number string) (*provider.MergeDiff, error) {
	n, err := prNumber("GetCRDiff", number)
	if err != nil {
		return nil, err
	}
	pid := owner + "/" + repo
	changes, _, err := p.client.MergeRequests.GetMergeRequestChanges(ctx, pid, n)
	if err != nil {
		return nil, sdkError("GetCRDiff", err)
	}
	files := make([]*provider.ChangedFile, 0, len(changes.Files))
	totalAdd, totalDel := 0, 0
	for _, d := range changes.Files {
		cf := convertDiff(d)
		totalAdd += cf.Additions
		totalDel += cf.Deletions
		files = append(files, cf)
	}
	return &provider.MergeDiff{Files: files, TotalAdd: totalAdd, TotalDel: totalDel}, nil
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
	pid := owner + "/" + repo
	opts := &gongfeng.CreateMergeRequestNoteOptions{
		Body: gongfeng.Ptr(body),
	}
	note, _, err := p.client.Notes.CreateMergeRequestNote(ctx, pid, n, opts)
	if err != nil {
		return "", sdkError("CreateNote", err)
	}
	return fmt.Sprintf("%d", note.ID), nil
}

// DeleteNote implements provider.DiffManager.
// The gongfeng SDK does not expose a delete-note endpoint, so we use the
// SDK client's NewRequest/Do for a raw API call.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo, number, noteID string) error {
	n, err := prNumber("DeleteNote", number)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("projects/%s/merge_requests/%d/notes/%s", owner+"/"+repo, n, noteID)
	req, err := p.client.NewRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return provider.Wrap(provider.PlatformTencentCode, "DeleteNote", err)
	}
	if _, err := p.client.Do(req, nil); err != nil {
		return sdkError("DeleteNote", err)
	}
	return nil
}

// CreateDiscussion implements provider.DiffManager.
// The gongfeng SDK does not expose a discussions endpoint, so we use the
// SDK client's NewRequest/Do for a raw API call.
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo, number string, opts provider.DiscussionOptions) (string, error) {
	n, err := prNumber("CreateDiscussion", number)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("projects/%s/merge_requests/%d/discussions", owner+"/"+repo, n)
	payload := map[string]any{"body": opts.Body}
	if opts.FilePath != "" {
		position := map[string]any{
			"position_type": "text",
			"new_path":      opts.FilePath,
		}
		if opts.BaseSHA != "" {
			position["base_sha"] = opts.BaseSHA
		}
		if opts.StartSHA != "" {
			position["start_sha"] = opts.StartSHA
		}
		if opts.HeadSHA != "" {
			position["head_sha"] = opts.HeadSHA
		}
		if opts.OldLine > 0 {
			position["old_path"] = opts.FilePath
			position["old_line"] = opts.OldLine
		}
		if opts.NewLine > 0 {
			position["new_line"] = opts.NewLine
		}
		payload["position"] = position
	}
	req, err := p.client.NewRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return "", provider.Wrap(provider.PlatformTencentCode, "CreateDiscussion", err)
	}
	var resp struct {
		ID int64 `json:"id"`
	}
	if _, err := p.client.Do(req, &resp); err != nil {
		return "", sdkError("CreateDiscussion", err)
	}
	return strconv.FormatInt(resp.ID, 10), nil
}

var _ provider.DiffManager = (*Provider)(nil)
