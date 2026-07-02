package tencentcode

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// ListRepos implements provider.RepoManager.
func (p *Provider) ListRepos(ctx context.Context, opts provider.ListRepoOptions) ([]*provider.PlatformRepo, error) {
	path := "/projects"
	if opts.Owner != "" {
		path = fmt.Sprintf("/groups/%s/projects", opts.Owner)
	}
	opts.Page, opts.PerPage = provider.NormalizePageOpts(opts.Page, opts.PerPage)
	path = fmt.Sprintf("%s?page=%d&per_page=%d", path, opts.Page, opts.PerPage)

	var projects []struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		DefaultBranch string `json:"default_branch"`
		Visibility    int    `json:"visibility_level"`
	}
	if err := p.doRequest(ctx, "GET", path, nil, &projects); err != nil {
		return nil, err
	}
	repos := make([]*provider.PlatformRepo, 0, len(projects))
	for _, proj := range projects {
		repos = append(repos, projectToRepo(proj.ID, proj.Name, proj.PathWithNS, proj.Description,
			proj.HTTPURL, proj.SSHURL, proj.DefaultBranch, proj.Visibility))
	}
	return repos, nil
}

// GetRepo implements provider.RepoManager.
func (p *Provider) GetRepo(ctx context.Context, owner, repo string) (*provider.PlatformRepo, error) {
	encoded := encodeProjectPath(owner, repo)
	var proj struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		DefaultBranch string `json:"default_branch"`
		Visibility    int    `json:"visibility_level"`
	}
	if err := p.doRequest(ctx, "GET", "/projects/"+encoded, nil, &proj); err != nil {
		return nil, err
	}
	return projectToRepo(proj.ID, proj.Name, proj.PathWithNS, proj.Description,
		proj.HTTPURL, proj.SSHURL, proj.DefaultBranch, proj.Visibility), nil
}

// ForkRepo implements provider.RepoManager.
func (p *Provider) ForkRepo(ctx context.Context, owner, repo string, opts provider.ForkRepoOptions) (*provider.PlatformRepo, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{}
	if opts.Organization != "" {
		body["namespace"] = opts.Organization
	}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	var proj struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := p.doRequest(ctx, "POST", "/projects/"+encoded+"/fork", body, &proj); err != nil {
		return nil, err
	}
	return projectToRepo(proj.ID, proj.Name, proj.PathWithNS, proj.Description,
		proj.HTTPURL, proj.SSHURL, proj.DefaultBranch, 20), nil
}

// DeleteRepo implements provider.RepoManager.
func (p *Provider) DeleteRepo(ctx context.Context, owner, repo string) error {
	encoded := encodeProjectPath(owner, repo)
	return p.doRequest(ctx, "DELETE", "/projects/"+encoded, nil, nil)
}

// UpdateRepo implements provider.RepoManager.
func (p *Provider) UpdateRepo(ctx context.Context, owner, repo string, opts provider.UpdateRepoOptions) (*provider.PlatformRepo, error) {
	encoded := encodeProjectPath(owner, repo)
	body := map[string]any{}
	if opts.Name != "" {
		body["name"] = opts.Name
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.DefaultBranch != "" {
		body["default_branch"] = opts.DefaultBranch
	}
	if opts.Private != nil {
		if *opts.Private {
			body["visibility_level"] = 0
		} else {
			body["visibility_level"] = 20
		}
	}
	var proj struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		PathWithNS    string `json:"path_with_namespace"`
		Description   string `json:"description"`
		HTTPURL       string `json:"http_url_to_repo"`
		SSHURL        string `json:"ssh_url_to_repo"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := p.doRequest(ctx, "PUT", "/projects/"+encoded, body, &proj); err != nil {
		return nil, err
	}
	return projectToRepo(proj.ID, proj.Name, proj.PathWithNS, proj.Description,
		proj.HTTPURL, proj.SSHURL, proj.DefaultBranch, 20), nil
}

// projectToRepo is a tiny helper that splits the owner out of "owner/repo"
// and packs the remaining fields into a provider.PlatformRepo.
func projectToRepo(id int, name, pathWithNS, desc, httpURL, sshURL, defaultBranch string, visibility int) *provider.PlatformRepo {
	parts := strings.SplitN(pathWithNS, "/", 2)
	owner := ""
	if len(parts) == 2 {
		owner = parts[0]
	}
	return &provider.PlatformRepo{
		ID:            int64(id),
		FullName:      pathWithNS,
		Name:          name,
		Owner:         owner,
		Description:   desc,
		CloneURL:      httpURL,
		SSHURL:        sshURL,
		DefaultBranch: defaultBranch,
		Private:       visibility == 0,
		Platform:      provider.PlatformTencentCode,
	}
}

var _ provider.RepoManager = (*Provider)(nil)