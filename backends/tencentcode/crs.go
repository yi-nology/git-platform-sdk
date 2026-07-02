package tencentcode

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	encoded := encodeProjectPath(opts.Owner, opts.Repo)
	body := map[string]any{
		"source_branch":        opts.SourceBranch,
		"target_branch":        opts.TargetBranch,
		"title":                opts.Title,
		"description":          opts.Description,
		"remove_source_branch": opts.RemoveSourceBranch,
	}
	if len(opts.Labels) > 0 {
		body["labels"] = strings.Join(opts.Labels, ",")
	}
	var mr tcMR
	if err := p.doRequest(ctx, "POST", "/projects/"+encoded+"/merge_requests", body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
	var mr tcMR
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), nil, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	encoded := encodeProjectPath(opts.Owner, opts.Repo)
	opts.Page, opts.PerPage = provider.NormalizePageOpts(opts.Page, opts.PerPage)
	path := fmt.Sprintf("/projects/%s/merge_requests?page=%d&per_page=%d", encoded, opts.Page, opts.PerPage)
	if opts.State != "" {
		path += "&state=" + string(opts.State)
	}
	if opts.SourceBranch != "" {
		path += "&source_branch=" + opts.SourceBranch
	}
	if opts.TargetBranch != "" {
		path += "&target_branch=" + opts.TargetBranch
	}
	var mrs []tcMR
	headers, err := p.doRequestWithHeaders(ctx, "GET", path, nil, &mrs)
	if err != nil {
		return nil, 0, err
	}
	crs := make([]*provider.ChangeRequest, 0, len(mrs))
	for i := range mrs {
		crs = append(crs, mrs[i].toCR())
	}
	return crs, provider.ParseTotalCountHeader(headers, len(crs)), nil
}

// MergeCR implements provider.ChangeRequestManager.
func (p *Provider) MergeCR(ctx context.Context, owner, repo string, number int, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
	// Pre-flight: reject MRs in a non-mergeable state.
	var existingMR tcMR
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), nil, &existingMR); err == nil {
		if existingMR.MergeStatus != "" && existingMR.MergeStatus != "can_be_merged" && existingMR.MergeStatus != "checking" {
			return nil, provider.Wrapf(provider.PlatformTencentCode, "MergeCR",
				"MR cannot be merged (status: %s). It may have conflicts or an active pipeline", existingMR.MergeStatus)
		}
		if mapState(existingMR.State) != provider.CRStateOpened {
			return nil, provider.Wrapf(provider.PlatformTencentCode, "MergeCR",
				"MR is not in 'opened' state (current: %s)", existingMR.State)
		}
	}
	body := map[string]any{}
	if opts.MergeCommitMessage != "" {
		body["merge_commit_message"] = opts.MergeCommitMessage
	}
	if opts.Squash {
		body["squash"] = true
	}
	if opts.RemoveSourceBranch {
		body["should_remove_source_branch"] = true
	}
	var mr tcMR
	if err := p.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d/merge", encoded, number), body, &mr); err != nil {
		return nil, provider.Wrap(provider.PlatformTencentCode, "MergeCR", fmt.Errorf("merge failed: %w", err))
	}
	return mr.toCR(), nil
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{"state_event": "close"}
	var mr tcMR
	if err := p.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{"state_event": "reopen"}
	var mr tcMR
	if err := p.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

// UpdateCR implements provider.ChangeRequestManager.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo string, number int, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{}
	if opts.Title != "" {
		body["title"] = opts.Title
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.TargetBranch != "" {
		body["target_branch"] = opts.TargetBranch
	}
	var mr tcMR
	if err := p.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), body, &mr); err != nil {
		return nil, err
	}
	return mr.toCR(), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{"labels": strings.Join(labels, ",")}
	return p.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), body, nil)
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*provider.CRComment, error) {
	encoded := encodeProjectPath(owner, repo)
	var notes []struct {
		ID     int    `json:"id"`
		Body   string `json:"body"`
		Author struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"author"`
		CreatedAt tcTime `json:"created_at"`
		UpdatedAt tcTime `json:"updated_at"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d/notes", encoded, number), nil, &notes); err != nil {
		return nil, err
	}
	result := make([]*provider.CRComment, 0, len(notes))
	for _, n := range notes {
		result = append(result, &provider.CRComment{
			ID:   int64(n.ID),
			Body: n.Body,
			Author: &provider.CRUser{
				ID: int64(n.Author.ID), Username: n.Author.Username, Name: n.Author.Name,
			},
			CreatedAt: n.CreatedAt.Time,
			UpdatedAt: n.UpdatedAt.Time,
		})
	}
	return result, nil
}

// ListCRCommits implements provider.ChangeRequestManager.
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*provider.CRCommit, error) {
	encoded := encodeProjectPath(owner, repo)
	var commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	}
	if err := p.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d/commits", encoded, number), nil, &commits); err != nil {
		return nil, err
	}
	result := make([]*provider.CRCommit, 0, len(commits))
	for _, c := range commits {
		result = append(result, &provider.CRCommit{
			SHA: c.ID, Message: c.Message, Author: &provider.CRUser{Name: c.Author.Name},
		})
	}
	return result, nil
}

var _ provider.ChangeRequestManager = (*Provider)(nil)
