package gitcode

import "github.com/yi-nology/git-platform-sdk/provider"

func init() {
	provider.Register(provider.PlatformGitCode, func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}
