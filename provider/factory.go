package provider

import "fmt"

type Config struct {
	Platform Platform
	BaseURL  string
	Token    string
	SkipTLS  bool
}

func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Platform {
	case PlatformGitLab:
		return NewGitLabProvider(cfg.BaseURL, cfg.Token, cfg.SkipTLS), nil
	case PlatformGitHub:
		return NewGitHubProvider(cfg.BaseURL, cfg.Token, cfg.SkipTLS), nil
	case PlatformGitea:
		return NewGiteaProvider(cfg.BaseURL, cfg.Token, cfg.SkipTLS), nil
	case PlatformTencentCode:
		return NewTencentCodeProvider(cfg.BaseURL, cfg.Token, cfg.SkipTLS), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", cfg.Platform)
	}
}
