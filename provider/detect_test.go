package provider

import (
	"testing"
)

func TestDetectPlatform_SSH_GitHub(t *testing.T) {
	r, err := DetectPlatform("git@github.com:owner/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Platform != PlatformGitHub {
		t.Errorf("expected GitHub, got %s", r.Platform)
	}
	if r.Owner != "owner" {
		t.Errorf("expected owner, got %s", r.Owner)
	}
	if r.Repo != "repo" {
		t.Errorf("expected repo, got %s", r.Repo)
	}
	if r.BaseURL != "https://api.github.com" {
		t.Errorf("unexpected BaseURL: %s", r.BaseURL)
	}
}

func TestDetectPlatform_SSH_GitLab(t *testing.T) {
	r, err := DetectPlatform("git@gitlab.com:org/project.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Platform != PlatformGitLab {
		t.Errorf("expected GitLab, got %s", r.Platform)
	}
	if r.Owner != "org" || r.Repo != "project" {
		t.Errorf("owner=%s repo=%s", r.Owner, r.Repo)
	}
}

func TestDetectPlatform_SSH_Gitea(t *testing.T) {
	r, err := DetectPlatform("git@gitea.com:user/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Platform != PlatformGitea {
		t.Errorf("expected Gitea, got %s", r.Platform)
	}
}

func TestDetectPlatform_SSH_StripGitSuffix(t *testing.T) {
	r, err := DetectPlatform("git@github.com:owner/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Repo != "repo" {
		t.Errorf("expected .git stripped, got %s", r.Repo)
	}
}

func TestDetectPlatform_SSH_NoGitSuffix(t *testing.T) {
	r, err := DetectPlatform("git@github.com:owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if r.Repo != "repo" {
		t.Errorf("expected repo, got %s", r.Repo)
	}
}

func TestDetectPlatform_HTTPS_GitHub(t *testing.T) {
	r, err := DetectPlatform("https://github.com/owner/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Platform != PlatformGitHub {
		t.Errorf("expected GitHub, got %s", r.Platform)
	}
	if r.Owner != "owner" || r.Repo != "repo" {
		t.Errorf("owner=%s repo=%s", r.Owner, r.Repo)
	}
}

func TestDetectPlatform_HTTPS_GitLab(t *testing.T) {
	r, err := DetectPlatform("https://gitlab.com/org/project.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Platform != PlatformGitLab {
		t.Errorf("expected GitLab, got %s", r.Platform)
	}
}

func TestDetectPlatform_HTTP(t *testing.T) {
	r, err := DetectPlatform("http://gitlab.local/group/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Platform != PlatformGitLab {
		t.Errorf("expected GitLab (self-hosted), got %s", r.Platform)
	}
	if r.BaseURL != "https://gitlab.local/api/v4" {
		t.Errorf("unexpected BaseURL: %s", r.BaseURL)
	}
}

func TestDetectPlatform_SSHProtocol(t *testing.T) {
	r, err := DetectPlatform("ssh://git@github.com:22/owner/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Platform != PlatformGitHub {
		t.Errorf("expected GitHub, got %s", r.Platform)
	}
	if r.Owner != "owner" || r.Repo != "repo" {
		t.Errorf("owner=%s repo=%s", r.Owner, r.Repo)
	}
}

func TestDetectPlatform_EmptyURL(t *testing.T) {
	_, err := DetectPlatform("")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestDetectPlatform_UnsupportedFormat(t *testing.T) {
	_, err := DetectPlatform("ftp://example.com/repo.git")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestDetectPlatform_InvalidSSHFormat(t *testing.T) {
	_, err := DetectPlatform("git@github.com")
	if err == nil {
		t.Error("expected error for invalid SSH URL (no path)")
	}
}

func TestDetectPlatform_InvalidHTTPPath(t *testing.T) {
	_, err := DetectPlatform("https://github.com/owner")
	if err == nil {
		t.Error("expected error for HTTP URL with no repo part")
	}
}

func TestClassifyHost_DefaultSelfHosted(t *testing.T) {
	platform, baseURL := classifyHost("git.mycompany.com")
	if platform != PlatformGitLab {
		t.Errorf("expected GitLab for self-hosted, got %s", platform)
	}
	if baseURL != "https://git.mycompany.com/api/v4" {
		t.Errorf("unexpected BaseURL: %s", baseURL)
	}
}

func TestDetectPlatform_SSH_SelfHosted(t *testing.T) {
	r, err := DetectPlatform("git@git.mycompany.com:team/project.git")
	if err != nil {
		t.Fatal(err)
	}
	if r.Platform != PlatformGitLab {
		t.Errorf("expected GitLab for self-hosted, got %s", r.Platform)
	}
}
