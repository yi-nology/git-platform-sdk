package gitee

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the Gitee divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences = []provider.Divergence{
	{Capability: "ChangeRequestManager", Method: "UpdateCR", Field: "opts.TargetBranch", Kind: provider.DivergenceIgnore,
		Reason: "Gitee's pull-update endpoint has no base field; retargeting a pull request is not possible."},
	{Capability: "ChangeRequestManager", Method: "GetCR", Field: "Draft", Kind: provider.DivergenceMapping,
		Reason: "The go-gitee pull model carries no draft field, so Draft is always false on every returned change request."},
	{Capability: "ChangeRequestManager", Method: "ListCRs", Field: "Draft", Kind: provider.DivergenceMapping,
		Reason: "See GetCR."},
	{Capability: "LabelManager", Method: "CreateLabel", Field: "opts.Description", Kind: provider.DivergenceIgnore,
		Reason: "Gitee's label wire has no description field."},
	{Capability: "LabelManager", Method: "UpdateLabel", Field: "opts.Description", Kind: provider.DivergenceIgnore,
		Reason: "Gitee's label wire has no description field."},
	{Capability: "ReleaseManager", Method: "CreateRelease", Field: "opts.Draft", Kind: provider.DivergenceIgnore,
		Reason: "Gitee's release create wire takes no draft flag."},
	// Registered detours: the go-gitee SDK is unusable for these surfaces
	// (broken signatures or missing methods), so they are routed through the
	// raw transport client. See backends/gitee/gitee.go's package comment.
	{Capability: "RepoManager", Method: "ListRepos", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the user-repos list method decodes into a single Project instead of an array."},
	{Capability: "RepoManager", Method: "CreateRepo", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: RepositoryPostParam has no default_branch field."},
	{Capability: "BranchManager", Method: "DeleteBranch", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: no DeleteV5ReposOwnerRepoBranches method exists."},
	{Capability: "CommitManager", Method: "GetCommit", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the RepoCommit model types author/committer/stats objects as strings."},
	{Capability: "CommitManager", Method: "ListCommits", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the RepoCommit model types author/committer/stats objects as strings."},
	{Capability: "CommitManager", Method: "CompareCommits", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the Compare model types the commits/files arrays as strings."},
	{Capability: "FileManager", Method: "GetFileContent", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the Content model types size/_links as strings."},
	{Capability: "FileManager", Method: "CreateFile", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: NewFileParam serializes bracketed JSON keys and the response model is defective."},
	{Capability: "FileManager", Method: "UpdateFile", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the update method posts multipart labeled application/json and the response model is defective."},
	{Capability: "FileManager", Method: "DeleteFile", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the delete method puts body parameters into the query string."},
	{Capability: "ReleaseManager", Method: "ListTags", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the tags method returns a single Tag for an array endpoint."},
	{Capability: "ReleaseManager", Method: "ListReleases", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the Release model stringifies several fields."},
	{Capability: "ReleaseManager", Method: "CreateRelease", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the create method posts multipart labeled application/json and the model is defective."},
	{Capability: "ReleaseManager", Method: "GetReleaseByTag", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the response decodes into a defective Release model."},
	{Capability: "ReleaseManager", Method: "UpdateRelease", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the update method posts multipart labeled application/json and the model is defective."},
	{Capability: "ReleaseManager", Method: "DeleteRelease", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the delete method's model is defective."},
	{Capability: "ReleaseManager", Method: "GetArchive", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the SDK exposes no archive-download endpoint."},
	{Capability: "LabelManager", Method: "ListLabels", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the list options carry no pagination parameters."},
	{Capability: "IssueManager", Method: "ListIssueLabels", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: GetV5ReposOwnerRepoLabelsOpts has no pagination fields; the list is driven through the raw client with explicit page/per_page."},
	{Capability: "LabelManager", Method: "UpdateLabel", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the patch method posts multipart labeled application/json."},
	{Capability: "IssueManager", Method: "CreateIssue", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the create method posts multipart labeled application/json."},
	{Capability: "MilestoneManager", Method: "CreateMilestone", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the create method posts form values labeled application/json."},
	{Capability: "MilestoneManager", Method: "UpdateMilestone", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the update method posts form values labeled application/json."},
	{Capability: "WebhookManager", Method: "CreateWebhook", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the create method posts multipart labeled application/json."},
	{Capability: "WebhookManager", Method: "ListWebhooks", Kind: provider.DivergenceDetour,
		Reason: "go-gitee: the Hook model stringifies numeric/boolean fields and list decode errors are swallowed."},
}

// Divergences returns the registered divergence ledger for the Gitee backend.
func Divergences() []provider.Divergence { return divergences }

// Divergences implements provider.Provider.
func (p *Provider) Divergences() []provider.Divergence { return divergences }
