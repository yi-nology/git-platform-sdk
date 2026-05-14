package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

func (g *githubProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	path := fmt.Sprintf("/repos/%s/%s/branches?per_page=100", owner, repo)
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

func (g *githubProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	body := map[string]string{"ref": ref, "branch_name": branch}
	var res struct {
		Ref string `json:"ref"`
	}
	if err := g.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/git/refs", owner, repo), body, &res); err != nil {
		return nil, err
	}
	name := strings.TrimPrefix(res.Ref, "refs/heads/")
	return &PlatformBranch{Name: name}, nil
}

func (g *githubProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	return g.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/git/refs/heads/%s", owner, repo, branch), nil, nil)
}

func (g *githubProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	diff := &MergeDiff{}
	page := 1
	for {
		path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=100&page=%d", owner, repo, number, page)
		var files []githubPRFile
		if err := g.doRequest(ctx, "GET", path, nil, &files); err != nil {
			return nil, err
		}
		for _, f := range files {
			cf := &ChangedFile{
				OldPath:   f.PreviousFilename,
				NewPath:   f.Filename,
				Diff:      f.Patch,
				Additions: f.Additions,
				Deletions: f.Deletions,
				IsNew:     f.Status == "added",
				IsDeleted: f.Status == "removed",
				IsRenamed: f.Status == "renamed",
				IsBinary:  false,
			}
			if cf.OldPath == "" {
				cf.OldPath = cf.NewPath
			}
			diff.Files = append(diff.Files, cf)
			diff.TotalAdd += f.Additions
			diff.TotalDel += f.Deletions
			diff.RawDiff += fmt.Sprintf("diff --git a/%s b/%s\n", cf.OldPath, cf.NewPath)
			if cf.IsNew {
				diff.RawDiff += "new file mode 100644\n"
			}
			if cf.IsDeleted {
				diff.RawDiff += "deleted file mode 100644\n"
			}
			if cf.IsRenamed {
				diff.RawDiff += fmt.Sprintf("rename from %s\nrename to %s\n", cf.OldPath, cf.NewPath)
			}
			if !cf.IsNew {
				diff.RawDiff += fmt.Sprintf("--- a/%s\n", cf.OldPath)
			}
			if !cf.IsDeleted {
				diff.RawDiff += fmt.Sprintf("+++ b/%s\n", cf.NewPath)
			}
			diff.RawDiff += f.Patch + "\n"
		}
		if len(files) < 100 {
			break
		}
		page++
	}
	return diff, nil
}

func (g *githubProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	diff, err := g.GetCRDiff(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return diff.Files, nil
}

func (g *githubProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	payload := map[string]string{"body": body}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := g.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

func (g *githubProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	return g.doRequest(ctx, "DELETE", fmt.Sprintf("/repos/%s/%s/issues/comments/%s", owner, repo, noteID), nil, nil)
}

func (g *githubProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	payload := map[string]interface{}{
		"body": opts.Body,
		"path": opts.FilePath,
	}
	if opts.NewLine > 0 {
		payload["line"] = opts.NewLine
		payload["side"] = "RIGHT"
	} else if opts.OldLine > 0 {
		payload["line"] = opts.OldLine
		payload["side"] = "LEFT"
	}
	var resp struct {
		ID int64 `json:"id"`
	}
	if err := g.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, number), payload, &resp); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", resp.ID), nil
}

func (g *githubProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	payload := map[string]string{
		"state":       opts.State,
		"context":     opts.Context,
		"description": opts.Description,
	}
	if opts.TargetURL != "" {
		payload["target_url"] = opts.TargetURL
	}
	return g.doRequest(ctx, "POST", fmt.Sprintf("/repos/%s/%s/statuses/%s", owner, repo, sha), payload, nil)
}

func (g *githubProvider) GetFileContent(ctx context.Context, owner, repo, filePath, ref string) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, filePath)
	if ref != "" {
		path += "?ref=" + ref
	}
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := g.doRequest(ctx, "GET", path, nil, &resp); err != nil {
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

func (g *githubProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	payload := map[string]interface{}{"labels": labels}
	return g.doRequest(ctx, "PUT", fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number), payload, nil)
}

type githubPRFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Patch            string `json:"patch"`
}
