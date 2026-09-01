package gitee

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the Gitee divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences = []provider.Divergence{
	{Capability: "ChangeRequestManager", Method: "UpdateCR", Field: "opts.TargetBranch", Kind: provider.DivergenceIgnore,
		Reason: "Gitee's pull-update endpoint has no base field; retargeting a pull request is not possible."},
	{Capability: "LabelManager", Method: "CreateLabel", Field: "opts.Description", Kind: provider.DivergenceIgnore,
		Reason: "Gitee's label wire has no description field."},
	{Capability: "LabelManager", Method: "UpdateLabel", Field: "opts.Description", Kind: provider.DivergenceIgnore,
		Reason: "Gitee's label wire has no description field."},
	{Capability: "ReleaseManager", Method: "CreateRelease", Field: "opts.Draft", Kind: provider.DivergenceIgnore,
		Reason: "Gitee's release create wire takes no draft flag."},
}

// Divergences returns the registered divergence ledger for the Gitee backend.
func Divergences() []provider.Divergence { return divergences }

// Divergences implements provider.Provider.
func (p *Provider) Divergences() []provider.Divergence { return divergences }
