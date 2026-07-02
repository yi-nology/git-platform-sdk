package tencentcode

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// GetCommit implements provider.CommitManager.
func (p *Provider) GetCommit(ctx context.Context, owner, repo, sha string) (*provider.CommitInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	var c struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
		CreatedAt tcTime `json:"created_at"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/commits/%s", encoded, sha), nil, &c); err != nil {
		return nil, err
	}
	return &provider.CommitInfo{SHA: c.ID, Message: c.Message, Author: &provider.CRUser{Name: c.Author.Name}, CreatedAt: c.CreatedAt.Time}, nil
}

// ListCommits implements provider.CommitManager.
func (p *Provider) ListCommits(ctx context.Context, owner, repo string, opts provider.ListCommitsOptions) ([]*provider.CommitInfo, error) {
	encoded := encodeProjectPath(owner, repo)
	path := fmt.Sprintf("/projects/%s/repository/commits", encoded)
	if opts.Page > 0 || opts.PerPage > 0 {
		page, perPage := provider.NormalizePageOpts(opts.Page, opts.PerPage)
		path += fmt.Sprintf("?page=%d&per_page=%d", page, perPage)
	}
	var commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
		CreatedAt tcTime `json:"created_at"`
	}
	if err := p.doRequest(ctx, "GET", path, nil, &commits); err != nil {
		return nil, err
	}
	result := make([]*provider.CommitInfo, 0, len(commits))
	for _, c := range commits {
		result = append(result, &provider.CommitInfo{
			SHA: c.ID, Message: c.Message, Author: &provider.CRUser{Name: c.Author.Name},
			CreatedAt: c.CreatedAt.Time,
		})
	}
	return result, nil
}

// CompareCommits implements provider.CommitManager.
func (p *Provider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*provider.CompareResult, error) {
	encoded := encodeProjectPath(owner, repo)
	var cmp struct {
		Commits []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"commits"`
		Diffs []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			Diff        string `json:"diff"`
			NewFile     bool   `json:"new_file"`
			DeletedFile bool   `json:"deleted_file"`
			RenamedFile bool   `json:"renamed_file"`
		} `json:"diffs"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/repository/compare?from=%s&to=%s", encoded, base, head), nil, &cmp); err != nil {
		return nil, err
	}
	result := &provider.CompareResult{TotalCommits: len(cmp.Commits)}
	for _, c := range cmp.Commits {
		result.Commits = append(result.Commits, &provider.CommitInfo{SHA: c.ID, Message: c.Message})
	}
	for _, d := range cmp.Diffs {
		add, del := provider.CountDiffLines(d.Diff)
		result.Files = append(result.Files, &provider.ChangedFile{
			OldPath: d.OldPath, NewPath: d.NewPath, Diff: d.Diff,
			Additions: add, Deletions: del, IsNew: d.NewFile, IsDeleted: d.DeletedFile, IsRenamed: d.RenamedFile,
		})
	}
	return result, nil
}

// CreateCommitStatus implements provider.CommitManager.
func (p *Provider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	encoded := encodeProjectPath(owner, repo)
	payload := map[string]any{
		"state":       opts.State,
		"context":     opts.Context,
		"description": opts.Description,
	}
	if opts.TargetURL != "" {
		payload["target_url"] = opts.TargetURL
	}
	return p.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/statuses/%s", encoded, sha), payload, nil)
}

var _ provider.CommitManager = (*Provider)(nil)