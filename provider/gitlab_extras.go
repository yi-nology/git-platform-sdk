package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

func (g *gitlabProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	path := fmt.Sprintf("/projects/%s/repository/branches?per_page=100", encoded)
	var branches []struct {
		Name string `json:"name"`
	}
	if err := g.doRequest(ctx, "GET", path, nil, &branches); err != nil {
		return nil, err
	}
	result := make([]*PlatformBranch, 0, len(branches))
	for _, b := range branches {
		result = append(result, &PlatformBranch{Name: b.Name})
	}
	return result, nil
}

func (g *gitlabProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	body := map[string]string{"branch": branch, "ref": ref}
	var res struct {
		Name string `json:"name"`
	}
	if err := g.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/repository/branches", encoded), body, &res); err != nil {
		return nil, err
	}
	return &PlatformBranch{Name: res.Name}, nil
}

func (g *gitlabProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	return g.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/repository/branches/%s", encoded, branch), nil, nil)
}

func (g *gitlabProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	var resp struct {
		Changes []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			Diff        string `json:"diff"`
			NewFile     bool   `json:"new_file"`
			DeletedFile bool   `json:"deleted_file"`
			RenamedFile bool   `json:"renamed_file"`
			Binary      bool   `json:"binary"`
		} `json:"changes"`
	}
	if err := g.doRequest(ctx, "GET", fmt.Sprintf("/projects/%s/merge_requests/%d/changes", encoded, number), nil, &resp); err != nil {
		return nil, err
	}
	diff := &MergeDiff{}
	for _, c := range resp.Changes {
		additions, deletions := countDiffLines(c.Diff)
		cf := &ChangedFile{
			OldPath: c.OldPath, NewPath: c.NewPath, Diff: c.Diff,
			Additions: additions, Deletions: deletions,
			IsNew: c.NewFile, IsDeleted: c.DeletedFile, IsRenamed: c.RenamedFile, IsBinary: c.Binary,
		}
		diff.Files = append(diff.Files, cf)
		diff.TotalAdd += additions
		diff.TotalDel += deletions
		diff.RawDiff += fmt.Sprintf("diff --git a/%s b/%s\n", c.OldPath, c.NewPath)
		if c.NewFile {
			diff.RawDiff += fmt.Sprintf("new file mode 100644\n")
		}
		if c.DeletedFile {
			diff.RawDiff += fmt.Sprintf("deleted file mode 100644\n")
		}
		if c.RenamedFile {
			diff.RawDiff += fmt.Sprintf("rename from %s\nrename to %s\n", c.OldPath, c.NewPath)
		}
		if !c.NewFile {
			diff.RawDiff += fmt.Sprintf("--- a/%s\n", c.OldPath)
		}
		if !c.DeletedFile {
			diff.RawDiff += fmt.Sprintf("+++ b/%s\n", c.NewPath)
		}
		diff.RawDiff += c.Diff + "\n"
	}
	return diff, nil
}

func (g *gitlabProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	diff, err := g.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

func (g *gitlabProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	payload := map[string]string{"body": body}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := g.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/merge_requests/%d/notes", encoded, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

func (g *gitlabProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	return g.doRequest(ctx, "DELETE", fmt.Sprintf("/projects/%s/merge_requests/%d/notes/%s", encoded, number, noteID), nil, nil)
}

func (g *gitlabProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	payload := map[string]interface{}{"body": opts.Body}
	if opts.FilePath != "" {
		position := map[string]interface{}{
			"base_sha":      "head",
			"start_sha":     "head",
			"head_sha":      "head",
			"position_type": "text",
			"new_path":      opts.FilePath,
			"new_line":      opts.NewLine,
		}
		if opts.OldLine > 0 {
			position["old_path"] = opts.FilePath
			position["old_line"] = opts.OldLine
		}
		payload["position"] = position
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := g.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", encoded, number), payload, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (g *gitlabProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	payload := map[string]string{
		"state":       opts.State,
		"context":     opts.Context,
		"description": opts.Description,
		"target_url":  opts.TargetURL,
	}
	return g.doRequest(ctx, "POST", fmt.Sprintf("/projects/%s/statuses/%s", encoded, sha), payload, nil)
}

func (g *gitlabProvider) GetFileContent(ctx context.Context, owner, repo, filePath, ref string) (string, error) {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	apiPath := fmt.Sprintf("/projects/%s/repository/files/%s?ref=%s", encoded, filePath, ref)
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := g.doRequest(ctx, "GET", apiPath, nil, &resp); err != nil {
		return "", err
	}
	if resp.Encoding == "base64" {
		decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(resp.Content)))
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}
	return resp.Content, nil
}

func (g *gitlabProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	encoded := fmt.Sprintf("%s%%2F%s", owner, repo)
	payload := map[string]interface{}{"labels": strings.Join(labels, ",")}
	return g.doRequest(ctx, "PUT", fmt.Sprintf("/projects/%s/merge_requests/%d", encoded, number), payload, nil)
}
