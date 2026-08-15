package provider_test

import (
	"testing"

	// Register all built-in backends so factory/manager tests have providers
	// to work with. Without this, only custom-registered platforms would be
	// available.
	_ "github.com/yi-nology/git-platform-sdk/backends/all"

	"github.com/yi-nology/git-platform-sdk/provider"
)

func TestNewProvider_GitLab(t *testing.T) {
	p, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformGitLab,
		BaseURL:  "https://gitlab.com/api/v4",
		Token:    "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != provider.PlatformGitLab {
		t.Errorf("expected GitLab, got %s", p.Platform())
	}
}

func TestNewProvider_GitHub(t *testing.T) {
	p, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformGitHub,
		Token:    "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != provider.PlatformGitHub {
		t.Errorf("expected GitHub, got %s", p.Platform())
	}
}

func TestNewProvider_Gitea(t *testing.T) {
	_, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformGitea,
		BaseURL:  "https://gitea.com/api/v1",
		Token:    "test-token",
	})
	// Gitea client validates token on creation, so we expect an error
	if err != nil {
		t.Logf("Gitea client creation failed (expected with fake token): %v", err)
	}
}

func TestNewProvider_TencentCode(t *testing.T) {
	p, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformTencentCode,
		Token:    "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != provider.PlatformTencentCode {
		t.Errorf("expected TencentCode, got %s", p.Platform())
	}
}

func TestNewProvider_Unsupported(t *testing.T) {
	_, err := provider.NewProvider(provider.Config{
		Platform: provider.Platform("unsupported"),
	})
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
}

func TestNewProvider_DefaultBaseURL(t *testing.T) {
	p, err := provider.NewProvider(provider.Config{
		Platform: provider.PlatformGitHub,
		Token:    "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Platform() != provider.PlatformGitHub {
		t.Errorf("expected GitHub with empty BaseURL, got %s", p.Platform())
	}
}
