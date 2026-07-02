package tencentcode

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCRDiff implements provider.DiffManager.
func (p *Provider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*provider.MergeDiff, error) {
	encoded := encodeProjectPath(owner, repo)
	var changes struct {
		Changes []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			Diff        string `json:"diff"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
		} `json:"changes"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d/changes", encoded, number), nil, &changes); err != nil {
		return nil, err
	}
	files := make([]*provider.ChangedFile, 0, len(changes.Changes))
	totalAdd, totalDel := 0, 0
	for _, c := range changes.Changes {
		add, del := provider.CountDiffLines(c.Diff)
		totalAdd += add
		totalDel += del
		files = append(files, &provider.ChangedFile{
			OldPath: c.OldPath, NewPath: c.NewPath, Diff: c.Diff,
			Additions: add, Deletions: del,
			IsNew: c.NewFile, IsDeleted: c.DeletedFile, IsRenamed: c.RenamedFile,
		})
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
	encoded := encodeProjectPath(owner, repo)
	payload := map[string]any{"body": body}
	var resp struct {
		ID int `json:"id"`
	}
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/merge_requests/%d/notes", encoded, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

// DeleteNote implements provider.DiffManager.
func (p *Provider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	encoded := encodeProjectPath(owner, repo)
	return p.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/merge_requests/%d/notes/%s", encoded, number, noteID), nil, nil)
}

// CreateDiscussion implements provider.DiffManager.
func (p *Provider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts provider.DiscussionOptions) (string, error) {
	encoded := encodeProjectPath(owner, repo)
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
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := p.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", encoded, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

// CreateReview implements provider.DiffManager.
//
// Tencent 工蜂 has no native review object. We approximate one by posting
// each inline comment as a discussion and the summary body as a note.
func (p *Provider) CreateReview(ctx context.Context, owner, repo string, number int, opts provider.CreateReviewOptions) (*provider.ReviewResult, error) {
	encoded := encodeProjectPath(owner, repo)
	var mr tcMR
	_ = p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), nil, &mr)

	var baseSHA, startSHA, headSHA string
	if mr.DiffRefs.BaseSHA != "" {
		baseSHA = mr.DiffRefs.BaseSHA
		startSHA = mr.DiffRefs.StartSHA
		headSHA = mr.DiffRefs.HeadSHA
	}
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