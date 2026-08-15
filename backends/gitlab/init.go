package gitlab

import "github.com/yi-nology/git-platform-sdk/provider"

func init() {
	provider.Register(provider.PlatformGitLab, func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}
