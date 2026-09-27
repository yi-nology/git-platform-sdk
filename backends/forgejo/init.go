package forgejo

import "github.com/yi-nology/go-git-platform/provider"

func init() {
	provider.Register(provider.PlatformForgejo, func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}
