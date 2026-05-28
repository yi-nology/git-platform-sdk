package provider

import (
	"fmt"
	"net/url"
	"strings"
)

type DetectResult struct {
	Platform Platform
	Owner    string
	Repo     string
	BaseURL  string
}

func DetectPlatform(remoteURL string) (*DetectResult, error) {
	if remoteURL == "" {
		return nil, fmt.Errorf("empty remote URL")
	}

	if strings.HasPrefix(remoteURL, "git@") {
		return detectSSH(remoteURL)
	}
	if strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://") {
		return detectHTTP(remoteURL)
	}
	if strings.HasPrefix(remoteURL, "ssh://") {
		return detectSSHProtocol(remoteURL)
	}
	return nil, fmt.Errorf("unsupported URL format: %s", remoteURL)
}

func detectSSH(raw string) (*DetectResult, error) {
	rest := raw[4:]
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid SSH URL: %s", raw)
	}
	host := parts[0]
	path := strings.TrimSuffix(parts[1], ".git")
	pathParts := strings.SplitN(path, "/", 2)
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid SSH path: %s", path)
	}
	platform, baseURL := classifyHost(host)
	return &DetectResult{
		Platform: platform,
		Owner:    pathParts[0],
		Repo:     pathParts[1],
		BaseURL:  baseURL,
	}, nil
}

func detectSSHProtocol(raw string) (*DetectResult, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host := u.Host
	path := strings.TrimSuffix(u.Path, ".git")
	path = strings.TrimPrefix(path, "/")
	pathParts := strings.SplitN(path, "/", 2)
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid SSH path: %s", path)
	}
	platform, baseURL := classifyHost(host)
	return &DetectResult{
		Platform: platform,
		Owner:    pathParts[0],
		Repo:     pathParts[1],
		BaseURL:  baseURL,
	}, nil
}

func detectHTTP(raw string) (*DetectResult, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host := u.Host
	path := strings.TrimSuffix(u.Path, ".git")
	path = strings.TrimPrefix(path, "/")
	pathParts := strings.SplitN(path, "/", 2)
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid HTTP path: %s", path)
	}
	platform, baseURL := classifyHost(host)
	return &DetectResult{
		Platform: platform,
		Owner:    pathParts[0],
		Repo:     pathParts[1],
		BaseURL:  baseURL,
	}, nil
}

func classifyHost(host string) (Platform, string) {
	lower := strings.ToLower(host)
	switch {
	case strings.Contains(lower, "github.com"):
		return PlatformGitHub, "https://api.github.com"
	case strings.Contains(lower, "code.tencent.com"):
		return PlatformTencentCode, "https://git.code.tencent.com/api/v3"
	case strings.Contains(lower, "codeberg.org"):
		return PlatformForgejo, "https://codeberg.org"
	case strings.Contains(lower, "gitlab.com"):
		return PlatformGitLab, "https://gitlab.com/api/v4"
	case strings.Contains(lower, "gitea.com"):
		return PlatformGitea, "https://gitea.com/api/v1"
	case strings.Contains(lower, "gitee.com"):
		return PlatformGitee, "https://gitee.com/api/v5"
	case strings.Contains(lower, "gitcode.com"):
		return PlatformGitCode, "https://api.gitcode.com/api/v5"
	default:
		return PlatformGitLab, fmt.Sprintf("https://%s/api/v4", host)
	}
}
