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
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*provider.MergeDiff, error) {
	pid := owner + "/" + repo
	changes, _, err := p.client.MergeRequests.GetMergeRequestChanges(ctx, pid, number)
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
func (p *Provider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*provider.ChangedFile, error) {
	diff, err := p.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

// CreateNote implements provider.DiffManager.
func (p *Provider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	pid := owner + "/" + repo
	opts := &gongfeng.CreateMergeRequestNoteOptions{
		Body: gongfeng.Ptr(body),
	}
	note, _, err := p.client.Notes.CreateMergeRequestNote(ctx, pid, number, opts)
	if err != nil {
		return "", sdkError("CreateNote", err)
	}
	return fmt.Sprintf("%d", note.ID), nil
}

// DeleteNote implements provider.DiffManager.
// The gongfeng SDK does not expose a delete-note endpoint, so we use the
// SDK client's NewRequest/Do for a raw API call.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	path := fmt.Sprintf("projects/%s/merge_requests/%d/notes/%s", owner+"/"+repo, number, noteID)
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
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts provider.DiscussionOptions) (string, error) {
	path := fmt.Sprintf("projects/%s/merge_requests/%d/discussions", owner+"/"+repo, number)
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

// CreateReview implements provider.DiffManager.
//
// Tencent 工蜂 has no native review object. We approximate one by posting
// each inline comment as a discussion and the summary body as a note.
func (p *Provider) CreateReview(ctx context.Context, owner, repo string, number int, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	pid := owner + "/" + repo
	// Fetch MR to get diff_refs via a raw API call (SDK MergeRequest type
	// does not include DiffRefs).
	var rawMR struct {
		DiffRefs struct {
			BaseSHA  string `json:"base_sha"`
			StartSHA string `json:"start_sha"`
			HeadSHA  string `json:"head_sha"`
		} `json:"diff_refs"`
	}
	req, err := p.client.NewRequest(ctx, http.MethodGet,
		fmt.Sprintf("projects/%s/merge_requests/%d", pid, number), nil)
	if err == nil {
		_, _ = p.client.Do(req, &rawMR)
	}

	baseSHA := rawMR.DiffRefs.BaseSHA
	startSHA := rawMR.DiffRefs.StartSHA
	headSHA := rawMR.DiffRefs.HeadSHA
	if opts.CommitID != "" {
		headSHA = opts.CommitID
	}

	var lastErr error
	for _, c := range opts.Comments {
		discOpts := provider.DiscussionOptions{
			Body: c.Body, FilePath: c.Path,
			BaseSHA: baseSHA, StartSHA: startSHA, HeadSHA: headSHA,
		}
		if c.Side == "LEFT" && c.Line > 0 {
			discOpts.OldLine = c.Line
		} else if c.Line > 0 {
			discOpts.NewLine = c.Line
		}
		if _, err := p.CreateDiscussion(ctx, owner, repo, number, discOpts); err != nil {
			lastErr = err
		}
	}

	if opts.Body != "" {
		noteID, err := p.CreateNote(ctx, owner, repo, number, opts.Body)
		if err != nil {
			lastErr = err
		} else {
			return &provider.ReviewResult{ID: noteID}, nil
		}
	}

	if lastErr != nil && len(opts.Comments) == 0 {
		return nil, lastErr
	}
	return &provider.ReviewResult{}, nil
}

var _ provider.DiffManager = (*Provider)(nil)
