package gitea

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the Gitea divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences = []provider.Divergence{
	{Capability: "ChangeRequestManager", Method: "GetCR", Field: "BaseSHA", Kind: provider.DivergenceMapping,
		Reason: "Gitea payloads expose no merge base; BaseSHA carries the target-branch tip instead (StartSHA equals BaseSHA), as does every other method returning a change request."},
	{Capability: "ChangeRequestManager", Method: "ListCRs", Field: "BaseSHA", Kind: provider.DivergenceMapping,
		Reason: "See GetCR."},
}

// Divergences returns the registered divergence ledger for the Gitea backend.
func Divergences() []provider.Divergence { return divergences }

// Divergences implements provider.Provider.
func (p *Provider) Divergences() []provider.Divergence { return divergences }
