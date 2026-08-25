package forgejo

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the Forgejo divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences []provider.Divergence

// Divergences returns the registered divergence ledger for the Forgejo backend.
func Divergences() []provider.Divergence { return divergences }

// Divergences implements provider.Provider.
func (p *Provider) Divergences() []provider.Divergence { return divergences }
