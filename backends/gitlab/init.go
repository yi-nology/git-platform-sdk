package gitlab

import "github.com/yi-nology/go-git-platform/provider"

func init() {
	provider.Register(provider.PlatformGitLab, func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}
