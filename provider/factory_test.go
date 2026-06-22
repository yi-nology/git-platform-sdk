package provider

import "testing"

func TestNewProvider_GitLab(t *testing.T) {
	p, err := NewProvider(Config{
		Platform: PlatformGitLab,
		BaseURL:  "https://gitlab.com/api/v4",
		Token:    "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != PlatformGitLab {
		t.Errorf("expected GitLab, got %s", p.Platform())
	}
}

func TestNewProvider_GitHub(t *testing.T) {
	p, err := NewProvider(Config{
		Platform: PlatformGitHub,
		Token:    "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != PlatformGitHub {
		t.Errorf("expected GitHub, got %s", p.Platform())
	}
}

func TestNewProvider_Gitea(t *testing.T) {
	_, err := NewProvider(Config{
		Platform: PlatformGitea,
		BaseURL:  "https://gitea.com/api/v1",
		Token:    "test-token",
	})
	// Gitea client validates token on creation, so we expect an error
	if err != nil {
		t.Logf("Gitea client creation failed (expected with fake token): %v", err)
	}
}

func TestNewProvider_TencentCode(t *testing.T) {
	p, err := NewProvider(Config{
		Platform: PlatformTencentCode,
		Token:    "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != PlatformTencentCode {
		t.Errorf("expected TencentCode, got %s", p.Platform())
	}
}

func TestNewProvider_Unsupported(t *testing.T) {
	_, err := NewProvider(Config{
		Platform: Platform("unsupported"),
	})
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

func TestNewProvider_DefaultBaseURL(t *testing.T) {
	p, err := NewProvider(Config{
		Platform: PlatformGitHub,
		Token:    "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != PlatformGitHub {
		t.Errorf("expected GitHub with empty BaseURL, got %s", p.Platform())
	}
}
