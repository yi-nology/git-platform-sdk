# Unified Abstraction Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把平台差异从注释约定升级为机器可读的差异台账,收敛后端样板,修复 gitlab 静默忽略,并将 CommitStatus 拆为可选能力、将搜索 total 诚实化为 `*int`——全部直接生效,不留过渡开关。

**Architecture:** provider 包新增 `Divergence` 类型与 `Provider.Divergences()` 必须方法;7 个后端以包级 `divergence.go` 登记存量差异;contracttest 新增 Divergence 套件(反射存在性 + stub/ignore 零请求行为断言);`internal/tools/genledger` 从台账生成 `docs/divergence-ledger.md`;`backends/internal/backendutil` 新增 `IDCache`/`ResolveLabelID` 共享解析器;gitlab 接入 Users API 修复 Assignees/ReviewerIDs;`CreateCommitStatus` 移入可选 `CommitStatusManager`;`SearchManager` 三个方法的 total 返回值改为 `*int`。

**Tech Stack:** Go 1.26,module `github.com/yi-nology/git-platform-sdk`;第三方 SDK:gitlab client-go v2.55.1、go-github v69、gitea-sdk、forgejo-sdk、go-gitee、go-gitcode、gongfeng-sdk-go。

**Spec:** `docs/superpowers/specs/2026-08-25-unified-abstraction-design.md`(已批准,§6 决策记录)。

## Global Constraints

- 分支 `feat/unified-abstraction-hardening`,每任务一个 commit。
- 每个任务完成时 `go build ./... && go test ./...` 必须全绿。
- 代码注释一律英文;不再使用 "(spec §4.6)" 引用(统一改为引用 divergence ledger);不新增 TODO/FIXME。
- 用户可见行为变化在 `CHANGELOG.md` 的 `[Unreleased]` 段登记(遵循现有 Keep-a-Changelog 风格,英文)。
- 接口能力名一律用接口类型名字符串:`"RepoManager"`, `"ChangeRequestManager"`, `"WebhookManager"`, `"BranchManager"`, `"DiffManager"`, `"CommitManager"`, `"FileManager"`, `"ReleaseManager"`, `"IssueManager"`, `"LabelManager"`, `"MilestoneManager"`, `"ReviewManager"`, `"SearchManager"`, `"CommitStatusManager"`。
- 测试命令统一:`go test ./...`;单包:`go test ./provider/...` 等。

---

## Appendix A:差异台账权威内容(照抄进各后端)

> 这是全部 7 个后端的登记条目最终内容。**A.x 的代码块整体创建为对应文件。** 任务 3/4 创建时包含全部条目(含将来被任务 12/13 删除的 4 条,标注 REMOVE);任务 12/13 再删除对应条目并重新生成文档。

### A.1 backends/gitlab/divergence.go

```go
package gitlab

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the GitLab divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences = []provider.Divergence{
	{Capability: "ReviewManager", Method: "ListReviews", Kind: provider.DivergenceMapping,
		Reason: "GitLab has no per-review objects: approvals are summarized into one review per approver, each keyed by the MR IID because no per-approval IDs exist."},
	{Capability: "ReviewManager", Method: "GetReview", Kind: provider.DivergenceMapping,
		Reason: "Backed by the approval state; returns the first approver's synthesized review and reports NotFound when nobody has approved yet."},
	{Capability: "ReviewManager", Method: "CreateReview", Field: "opts.Event, opts.CommitID, opts.Comments", Kind: provider.DivergenceMapping,
		Reason: "A review is created as a merge-request note; verdicts and inline comments have no GitLab mapping, so every created review is in the commented state."},
	{Capability: "ReviewManager", Method: "DismissReview", Field: "reviewID, message", Kind: provider.DivergenceMapping,
		Reason: "Maps to UnapproveMergeRequest: approvals hang off the merge request as a whole, so per-review IDs are not addressable and the message has no equivalent."},
	{Capability: "IssueManager", Method: "CreateIssue", Field: "opts.Assignees", Kind: provider.DivergenceIgnore,
		Reason: "Assignees are username-addressed by the SDK while GitLab writes need user IDs; the resolver is not wired."}, // REMOVE in Task 12
	{Capability: "IssueManager", Method: "UpdateIssue", Field: "opts.Assignees", Kind: provider.DivergenceIgnore,
		Reason: "See CreateIssue."}, // REMOVE in Task 12
	{Capability: "ReviewManager", Method: "RequestReviewers", Kind: provider.DivergenceIgnore,
		Reason: "UpdateMergeRequest takes reviewer IDs while the SDK addresses reviewers by username; the resolver is not wired, so the call succeeds without effect."}, // REMOVE in Task 12
	{Capability: "ReleaseManager", Method: "UpdateRelease", Field: "opts.Draft, opts.Prerelease", Kind: provider.DivergenceIgnore,
		Reason: "GitLab releases expose no draft or prerelease flags."},
	{Capability: "SearchManager", Method: "SearchRepos", Field: "sort, order, state", Kind: provider.DivergenceIgnore,
		Reason: "GitLab's search endpoints take no sort, order, or state; the filters are accepted but ignored."},
	{Capability: "SearchManager", Method: "SearchIssues", Field: "sort, order, state", Kind: provider.DivergenceIgnore,
		Reason: "GitLab's search endpoints take no sort, order, or state; the filters are accepted but ignored."},
	{Capability: "SearchManager", Method: "SearchUsers", Field: "sort, order, state", Kind: provider.DivergenceIgnore,
		Reason: "GitLab's search endpoints take no sort, order, or state; the filters are accepted but ignored."},
}

// Divergences returns the registered divergence ledger for the GitLab backend.
func Divergences() []provider.Divergence { return divergences }
```

### A.2 backends/github/divergence.go

```go
package github

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the GitHub divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences = []provider.Divergence{
	{Capability: "ChangeRequestManager", Method: "GetCR", Field: "BaseSHA", Kind: provider.DivergenceMapping,
		Reason: "GitHub payloads expose no merge base; BaseSHA carries the target-branch tip instead (StartSHA equals BaseSHA), as does every other method returning a change request."},
	{Capability: "ChangeRequestManager", Method: "ListCRs", Field: "BaseSHA", Kind: provider.DivergenceMapping,
		Reason: "See GetCR."},
	{Capability: "FileManager", Method: "GetArchive", Kind: provider.DivergenceDetour,
		Reason: "Archive downloads stream through the raw transport client rather than go-github."},
}

// Divergences returns the registered divergence ledger for the GitHub backend.
func Divergences() []provider.Divergence { return divergences }
```

### A.3 backends/gitee/divergence.go

```go
package gitee

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the Gitee divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences = []provider.Divergence{
	{Capability: "CommitManager", Method: "CreateCommitStatus", Kind: provider.DivergenceStub,
		Reason: "Gitee does not expose a commit-status endpoint in the public REST API."}, // REMOVE in Task 13
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
```

### A.4 backends/gitea/divergence.go

```go
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
```

### A.5 backends/forgejo/divergence.go(与 A.4 同构,Reason 中 "Gitea" 改为 "Forgejo")

```go
package forgejo

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the Forgejo divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences = []provider.Divergence{
	{Capability: "ChangeRequestManager", Method: "GetCR", Field: "BaseSHA", Kind: provider.DivergenceMapping,
		Reason: "Forgejo payloads expose no merge base; BaseSHA carries the target-branch tip instead (StartSHA equals BaseSHA), as does every other method returning a change request."},
	{Capability: "ChangeRequestManager", Method: "ListCRs", Field: "BaseSHA", Kind: provider.DivergenceMapping,
		Reason: "See GetCR."},
}

// Divergences returns the registered divergence ledger for the Forgejo backend.
func Divergences() []provider.Divergence { return divergences }
```

### A.6 backends/gitcode/divergence.go

```go
package gitcode

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the GitCode divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences = []provider.Divergence{
	{Capability: "LabelManager", Method: "ListLabels", Field: "opts.Page, opts.PerPage", Kind: provider.DivergenceIgnore,
		Reason: "GitCode's label list endpoint does not paginate; paging options are accepted but ignored."},
	{Capability: "LabelManager", Method: "CreateLabel", Field: "opts.Description", Kind: provider.DivergenceIgnore,
		Reason: "GitCode's label API has no description field."},
	{Capability: "LabelManager", Method: "UpdateLabel", Field: "opts.Description", Kind: provider.DivergenceIgnore,
		Reason: "GitCode's label API has no description field."},
	{Capability: "MilestoneManager", Method: "CreateMilestone", Kind: provider.DivergenceDetour,
		Reason: "go-gitcode marshals due_on without omitempty, which would clear due dates on the GitHub-shaped API; create goes through the raw client with exactly the fields the caller set."},
	{Capability: "MilestoneManager", Method: "UpdateMilestone", Kind: provider.DivergenceDetour,
		Reason: "See CreateMilestone."},
}

// Divergences returns the registered divergence ledger for the GitCode backend.
func Divergences() []provider.Divergence { return divergences }
```

### A.7 backends/tencentcode/divergence.go

```go
package tencentcode

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the Tencent Code divergence ledger: the registered places
// where this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences = []provider.Divergence{
	{Capability: "ChangeRequestManager", Method: "UpdateCRLabels", Kind: provider.DivergenceStub,
		Reason: "The Gongfeng API no longer accepts labels via the merge-request update endpoint."},
	{Capability: "ReviewManager", Method: "ListReviews", Kind: provider.DivergenceMapping,
		Reason: "Reviews are mapped from MR notes: every review is in the commented state and ordinary comments mix in."},
	{Capability: "ReviewManager", Method: "GetReview", Kind: provider.DivergenceMapping,
		Reason: "See ListReviews."},
	{Capability: "ReviewManager", Method: "CreateReview", Field: "opts.Event", Kind: provider.DivergenceMapping,
		Reason: "A review is created as a note; verdicts have no equivalent, so the state is always commented."},
	{Capability: "ReviewManager", Method: "RequestReviewers", Kind: provider.DivergenceIgnore,
		Reason: "The platform exposes no reviewer-request surface the SDK can drive; the call succeeds without effect."},
	{Capability: "ReviewManager", Method: "DismissReview", Kind: provider.DivergenceStub,
		Reason: "The platform exposes no review-dismissal surface."},
	{Capability: "IssueManager", Method: "CreateIssue", Field: "opts.Assignees", Kind: provider.DivergenceIgnore,
		Reason: "Gongfeng issue writes take user IDs; username-to-ID resolution is not wired."},
	{Capability: "IssueManager", Method: "UpdateIssue", Field: "opts.Assignees", Kind: provider.DivergenceIgnore,
		Reason: "See CreateIssue."},
	{Capability: "IssueManager", Method: "RemoveIssueLabel", Kind: provider.DivergenceMapping,
		Reason: "Label deletion routes through replace semantics; removing the last label is a no-op."},
	{Capability: "IssueManager", Method: "ListIssues", Field: "WebURL, ClosedAt", Kind: provider.DivergenceMapping,
		Reason: "The API exposes no issue web URL and no closed-at timestamp."},
	{Capability: "IssueManager", Method: "GetIssue", Field: "WebURL, ClosedAt", Kind: provider.DivergenceMapping,
		Reason: "See ListIssues."},
	{Capability: "LabelManager", Method: "ListLabels", Field: "Label.ID", Kind: provider.DivergenceMapping,
		Reason: "Label IDs are not exposed by the wire; Label.ID is always 0."},
	{Capability: "ReleaseManager", Method: "UpdateRelease", Field: "opts.Draft, opts.Prerelease", Kind: provider.DivergenceIgnore,
		Reason: "The release update endpoint takes no draft or prerelease flags."},
}

// Divergences returns the registered divergence ledger for the Tencent Code backend.
func Divergences() []provider.Divergence { return divergences }
```

---

### Task 1: provider Divergence 类型与查询助手

**Files:**
- Create: `provider/divergence.go`
- Test: `provider/divergence_test.go`

**Interfaces:**
- Produces: `type DivergenceKind string`;常量 `DivergenceStub/DivergenceIgnore/DivergenceMapping/DivergenceDetour`;`type Divergence struct{ Capability, Method, Field string; Kind DivergenceKind; Reason string }`;`func FindByMethod(divs []Divergence, method string) []Divergence`;`func Ignores(divs []Divergence, method, field string) bool`;`func Stubs(divs []Divergence, method string) bool`。

- [x] **Step 1: 写失败测试** — 创建 `provider/divergence_test.go`:

```go
package provider

import "testing"

func TestFindByMethod(t *testing.T) {
	divs := []Divergence{
		{Capability: "ReviewManager", Method: "RequestReviewers", Kind: DivergenceIgnore},
		{Capability: "ReviewManager", Method: "DismissReview", Kind: DivergenceStub},
	}
	if got := FindByMethod(divs, "RequestReviewers"); len(got) != 1 || got[0].Kind != DivergenceIgnore {
		t.Fatalf("FindByMethod = %+v, want the single ignore entry", got)
	}
	if got := FindByMethod(divs, "Missing"); len(got) != 0 {
		t.Fatalf("FindByMethod(Missing) = %+v, want none", got)
	}
}

func TestIgnores(t *testing.T) {
	divs := []Divergence{
		{Capability: "IssueManager", Method: "CreateIssue", Field: "opts.Assignees", Kind: DivergenceIgnore},
	}
	if !Ignores(divs, "CreateIssue", "opts.Assignees") {
		t.Fatal("Ignores(CreateIssue, opts.Assignees) = false, want true")
	}
	if Ignores(divs, "CreateIssue", "opts.Title") {
		t.Fatal("Ignores(CreateIssue, opts.Title) = true, want false")
	}
	if Ignores(divs, "UpdateIssue", "opts.Assignees") {
		t.Fatal("Ignores on an unregistered method = true, want false")
	}
}

func TestStubs(t *testing.T) {
	divs := []Divergence{{Capability: "CommitManager", Method: "CreateCommitStatus", Kind: DivergenceStub}}
	if !Stubs(divs, "CreateCommitStatus") {
		t.Fatal("Stubs(CreateCommitStatus) = false, want true")
	}
	if Stubs(divs, "GetCommit") {
		t.Fatal("Stubs(GetCommit) = true, want false")
	}
}
```

- [x] **Step 2: 跑测试确认失败** — Run: `go test ./provider/ -run 'TestFindByMethod|TestIgnores|TestStubs' -v`;预期:编译失败(undefined: Divergence 等)。

- [x] **Step 3: 实现** — 创建 `provider/divergence.go`:

```go
package provider

// DivergenceKind classifies how a backend's behavior departs from the
// unified provider semantics for a given method.
type DivergenceKind string

const (
	// DivergenceStub marks a method the platform cannot serve at all: the
	// call returns an error wrapping ErrNotImplemented and touches no wire.
	DivergenceStub DivergenceKind = "stub"
	// DivergenceIgnore marks a field or parameter that is silently dropped:
	// the call succeeds but the ignored input has no effect.
	DivergenceIgnore DivergenceKind = "ignore"
	// DivergenceMapping marks a semantic mapping: the call succeeds and
	// returns the closest platform equivalent, an approximation of the
	// unified semantics.
	DivergenceMapping DivergenceKind = "mapping"
	// DivergenceDetour marks an implementation detour: the method bypasses
	// the platform's third-party SDK and drives the raw transport client.
	// Behavior is unchanged; the entry exists for maintainers.
	DivergenceDetour DivergenceKind = "detour"
)

// Divergence is one registered entry of a backend's divergence ledger.
// Capability and Method carry the provider interface and method names;
// Field names the affected option/result field for ignore and mapping
// entries (empty when the divergence is method-scoped). Reason is a
// one-sentence explanation surfaced in docs/divergence-ledger.md.
//
// Backends expose their ledger via a package-level Divergences function and
// the Provider.Divergences method; the ledger is the machine-readable
// successor of the former "(spec §4.6)" comment registrations.
type Divergence struct {
	Capability string
	Method     string
	Field      string
	Kind       DivergenceKind
	Reason     string
}

// FindByMethod returns the ledger entries registered for method.
func FindByMethod(divs []Divergence, method string) []Divergence {
	var out []Divergence
	for _, d := range divs {
		if d.Method == method {
			out = append(out, d)
		}
	}
	return out
}

// Ignores reports whether the ledger registers an ignore of field on method.
func Ignores(divs []Divergence, method, field string) bool {
	for _, d := range divs {
		if d.Method == method && d.Kind == DivergenceIgnore && d.Field == field {
			return true
		}
	}
	return false
}

// Stubs reports whether the ledger registers method as a stub.
func Stubs(divs []Divergence, method string) bool {
	for _, d := range divs {
		if d.Method == method && d.Kind == DivergenceStub {
			return true
		}
	}
	return false
}
```

(实现文件中无占位;上面代码即最终版本。)

- [x] **Step 4: 跑测试确认通过** — Run: `go test ./provider/ -run 'TestFindByMethod|TestIgnores|TestStubs' -v`;预期:PASS。

- [x] **Step 5: 提交** — `git add provider/divergence.go provider/divergence_test.go && git commit -m "feat(provider): divergence ledger types and query helpers"`

### Task 2: Provider.Divergences() 接口方法与 7 后端骨架

**Files:**
- Modify: `provider/provider.go`(Provider 接口,约 :58-75)
- Create: `backends/{gitlab,github,gitee,gitea,forgejo,gitcode,tencentcode}/divergence.go`(本任务只放空骨架,条目在 Task 3/4 填)

**Interfaces:**
- Consumes: Task 1 的 `Divergence` 类型。
- Produces: `Provider` 接口新增 `Divergences() []Divergence`;每后端包级 `func Divergences() []provider.Divergence`。

- [x] **Step 1: 接口加方法** — 在 `provider/provider.go` 的 `Provider` 接口中,`Capabilities() CapabilitySet` 之后加:

```go
	// Divergences statically declares this backend's divergence ledger: the
	// registered places where its behavior departs from the unified
	// semantics (stub / ignore / mapping / detour). Consumers can route on
	// these entries — e.g. provider.Ignores — and docs/divergence-ledger.md
	// is generated from them. Like CapabilitySet this is a compile-time
	// declaration; the contract suite locks ledger entries to behavior.
	Divergences() []Divergence
```

- [x] **Step 2: 7 个后端骨架** — 每个后端创建 `divergence.go`(以 gitlab 为例,其余 6 个同构替换包名与注释中的平台名):

```go
package gitlab

import "github.com/yi-nology/git-platform-sdk/provider"

// divergences is the GitLab divergence ledger: the registered places where
// this backend's behavior departs from the unified provider semantics.
// See provider.Divergence and docs/divergence-ledger.md.
var divergences []provider.Divergence

// Divergences returns the registered divergence ledger for the GitLab backend.
func Divergences() []provider.Divergence { return divergences }

// Divergences implements provider.Provider.
func (p *Provider) Divergences() []provider.Divergence { return divergences }
```

- [x] **Step 3: 全量构建与测试** — Run: `go build ./... && go test ./...`;若任何 `_test.go` 中的 fake Provider 因接口扩容编译失败(`grep -rln "provider.Provider" --include="*_test.go" .`),为其补 `func (f *fakeT) Divergences() []provider.Divergence { return nil }`。预期:全绿。

- [x] **Step 4: 提交** — `git add -A && git commit -m "feat(provider): Provider.Divergences interface method with backend skeletons"`

### Task 3: 台账登记 —— gitlab / github / gitee

**Files:**
- Modify: `backends/gitlab/divergence.go`(用附录 A.1 全文替换 `var divergences` 定义,保留两条 `Divergences` 函数)
- Create: `backends/github/divergence.go`(附录 A.2 全文 + Task 2 的方法实现,即文件尾追加 `func (p *Provider) Divergences() []provider.Divergence { return divergences }`)
- Create: `backends/gitee/divergence.go`(附录 A.3 全文 + 同上方法实现)
- Modify: `backends/gitlab/reviews.go`、`backends/gitlab/issues.go`、`backends/gitee/commits.go` 等注释中的 `(spec §4.6)` 引用

**Interfaces:**
- Consumes: Task 1 类型、Task 2 骨架。
- Produces: 三个后端的完整台账(gitlab 11 条、github 3 条、gitee 32 条)。

- [x] **Step 1:** 将附录 A.1 的 `var divergences = []provider.Divergence{...}` 整段替换 `backends/gitlab/divergence.go` 中的 `var divergences []provider.Divergence`(文件其余部分保持 Task 2 的两个函数不变;注意 A.1 自带包级 `Divergences` 函数,不要重复)。
- [x] **Step 2:** 创建 `backends/github/divergence.go` = 附录 A.2 全文 + 追加方法 `func (p *Provider) Divergences() []provider.Divergence { return divergences }`。
- [x] **Step 3:** 创建 `backends/gitee/divergence.go` = 附录 A.3 全文 + 同样的方法实现。
- [x] **Step 4: 注释规范反转** — `grep -rl "(spec §4.6)" backends/ | xargs sed -i '' 's/(spec §4.6)/(divergence ledger)/g'`(macOS;Linux 用 `sed -i`),把旧引用统一改为 ledger。
- [x] **Step 5: 构建测试** — Run: `go build ./... && go test ./...`;预期全绿(台账尚无行为断言,Task 5 才有)。
- [x] **Step 6: 提交** — `git add -A && git commit -m "feat(backends): register gitlab/github/gitee divergence ledgers"`

### Task 4: 台账登记 —— gitea / forgejo / gitcode / tencentcode

**Files:**
- Create: `backends/gitea/divergence.go`(附录 A.4 + 方法实现)
- Create: `backends/forgejo/divergence.go`(附录 A.5 + 方法实现)
- Modify: `backends/gitcode/divergence.go`(Task 2 骨架的 `var` 换成附录 A.6 全文)
- Modify: `backends/tencentcode/divergence.go`(同上,用附录 A.7)

- [x] **Step 1-4:** 按上述四个文件创建/替换(方法实现同 Task 3 Step 2 的模式)。
- [x] **Step 5: 构建测试** — Run: `go build ./... && go test ./...`;预期全绿。
- [x] **Step 6: 提交** — `git add -A && git commit -m "feat(backends): register gitea/forgejo/gitcode/tencentcode divergence ledgers"`

### Task 5: contracttest Divergence 套件

**Files:**
- Create: `backends/contracttest/divergence.go`
- Modify: `backends/contracttest/contracttest.go`(Run 函数,约 :72-89)

**Interfaces:**
- Consumes: `provider.Divergence`、`provider.IsNotImplemented`、Task 3/4 的台账。
- Produces: `Run` 中新增子测试 `DivergenceSuite`;包内函数 `testDivergenceSuite(t *testing.T, h Harness)`、`dispatchDivergenceCall(p provider.Provider, capability, method string) (invoked bool, err error)`。

- [x] **Step 1: 实现套件** — 创建 `backends/contracttest/divergence.go`:

```go
package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// testDivergenceSuite locks each backend's divergence ledger to its
// behavior. For every method-scoped stub or ignore entry the dispatch table
// invokes the method against a recording server: a stub must fail with
// provider.ErrNotImplemented and stay off the wire; an ignore must succeed
// and stay off the wire. Every entry's method must exist on the concrete
// provider, and method-scoped stub/ignore entries must have a dispatch case
// (the suite fails on unknown pairs so the table cannot rot).
func testDivergenceSuite(t *testing.T, h Harness) {
	var mu sync.Mutex
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(h.EmptyListResponse))
	}))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))

	rv := reflect.ValueOf(p)
	for _, d := range p.Divergences() {
		if m := rv.MethodByName(d.Method); !m.IsValid() {
			t.Errorf("%s: ledger entry (Capability %q) references method %q that does not exist on the provider", h.Name, d.Capability, d.Method)
			continue
		}
		if d.Kind != provider.DivergenceStub && d.Kind != provider.DivergenceIgnore {
			continue // behavioral checks apply to stub/ignore only
		}
		if d.Field != "" {
			continue // field-scoped ignores are documentation; the call itself works
		}
		mu.Lock()
		before := len(requests)
		mu.Unlock()
		invoked, err := dispatchDivergenceCall(p, d.Capability, d.Method)
		if !invoked {
			t.Errorf("%s: no dispatch case for %s.%s — extend dispatchDivergenceCall in contracttest/divergence.go", h.Name, d.Capability, d.Method)
			continue
		}
		mu.Lock()
		after := len(requests)
		mu.Unlock()
		switch d.Kind {
		case provider.DivergenceStub:
			if err == nil {
				t.Errorf("%s: registered stub %s.%s returned nil error, want ErrNotImplemented", h.Name, d.Capability, d.Method)
			} else if !provider.IsNotImplemented(err) {
				t.Errorf("%s: registered stub %s.%s = %v, want an error wrapping ErrNotImplemented", h.Name, d.Capability, d.Method, err)
			}
		case provider.DivergenceIgnore:
			if err != nil {
				t.Errorf("%s: registered ignore %s.%s = %v, want nil (succeeds without effect)", h.Name, d.Capability, d.Method, err)
			}
		}
		if after != before {
			t.Errorf("%s: %s.%s (%s) touched the wire (%d requests), want zero", h.Name, d.Capability, d.Method, d.Kind, after-before)
		}
	}
}

// dispatchDivergenceCall invokes a ledger-declared stub/ignore with dummy
// arguments. The table is closed on purpose: adding a new method-scoped
// stub or ignore without a case here fails the suite with instructions.
func dispatchDivergenceCall(p provider.Provider, capability, method string) (bool, error) {
	ctx := context.Background()
	switch capability + "." + method {
	case "ReviewManager.RequestReviewers":
		rm, ok := p.(provider.ReviewManager)
		if !ok {
			return true, nil
		}
		return true, rm.RequestReviewers(ctx, "owner", "repo", "1", []string{"dev"})
	case "ReviewManager.DismissReview":
		rm, ok := p.(provider.ReviewManager)
		if !ok {
			return true, nil
		}
		return true, rm.DismissReview(ctx, "owner", "repo", "1", 1, "stale review")
	case "ChangeRequestManager.UpdateCRLabels":
		return true, p.UpdateCRLabels(ctx, "owner", "repo", "1", []string{"bug"})
	case "CommitManager.CreateCommitStatus":
		return true, p.CreateCommitStatus(ctx, "owner", "repo", "deadbeef", provider.CommitStatusOptions{State: "pending"})
	default:
		return false, nil
	}
}
```

- [x] **Step 2: 挂载** — 在 `contracttest.go` 的 `Run` 中,`Capabilities_Consistency` 行后加:

```go
	t.Run("DivergenceSuite", func(t *testing.T) { testDivergenceSuite(t, h) })
```

- [x] **Step 3: 跑套件** — Run: `go test ./backends/... -run DivergenceSuite -v`;预期:7 后端全绿。若 gitlab `RequestReviewers` ignore 触发"零请求"断言失败,说明该方法已真实实现而台账未删——按 Task 12 处理(此时不应发生)。
- [x] **Step 4: 全量测试** — Run: `go test ./...`;预期全绿。
- [x] **Step 5: 提交** — `git add -A && git commit -m "test(contracttest): divergence suite locking ledger entries to behavior"`

### Task 6: 台账文档生成器 + README + CHANGELOG

**Files:**
- Create: `internal/tools/genledger/main.go`、`internal/tools/genledger/main_test.go`
- Create: `docs/divergence-ledger.md`(生成产物,提交)
- Modify: `provider/divergence.go`(go:generate 指令)、`README.md`(差异台账一节)、`CHANGELOG.md`

**Interfaces:**
- Consumes: 7 个后端包级 `Divergences()`(Task 3/4)。
- Produces: `go generate ./...` 再生成 `docs/divergence-ledger.md`;genledger 导出 `render() string` 供测试。

- [x] **Step 1: 生成器** — 创建 `internal/tools/genledger/main.go`:

```go
// Command genledger regenerates docs/divergence-ledger.md from the backends'
// registered divergence ledgers. Run `go generate ./...` from anywhere in
// the module (the directive lives in provider/divergence.go); the tool
// locates the repo root by walking up to go.mod.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yi-nology/git-platform-sdk/backends/forgejo"
	"github.com/yi-nology/git-platform-sdk/backends/gitcode"
	"github.com/yi-nology/git-platform-sdk/backends/gitea"
	"github.com/yi-nology/git-platform-sdk/backends/gitee"
	"github.com/yi-nology/git-platform-sdk/backends/github"
	"github.com/yi-nology/git-platform-sdk/backends/gitlab"
	"github.com/yi-nology/git-platform-sdk/backends/tencentcode"
	"github.com/yi-nology/git-platform-sdk/provider"
)

func main() {
	out := filepath.Join(repoRoot(), "docs", "divergence-ledger.md")
	if err := os.WriteFile(out, []byte(render()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "genledger: write:", err)
		os.Exit(1)
	}
	fmt.Println("genledger: wrote", out)
}

// render builds the ledger document. It lives in main but is exercised by
// main_test against the committed file so the two cannot drift.
func render() string {
	type backendLedger struct {
		name string
		divs []provider.Divergence
	}
	ledgers := []backendLedger{
		{"gitlab", gitlab.Divergences()},
		{"github", github.Divergences()},
		{"gitee", gitee.Divergences()},
		{"gitea", gitea.Divergences()},
		{"forgejo", forgejo.Divergences()},
		{"gitcode", gitcode.Divergences()},
		{"tencent_code", tencentcode.Divergences()},
	}
	var b strings.Builder
	b.WriteString(header)
	for _, l := range ledgers {
		if len(l.divs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n\n| Capability | Method | Field | Kind | Reason |\n|---|---|---|---|---|\n", l.name)
		for _, d := range l.divs {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
				d.Capability, d.Method, d.Field, d.Kind, strings.ReplaceAll(d.Reason, "|", "\\|"))
		}
	}
	return b.String()
}

const header = `<!-- Code generated by internal/tools/genledger; DO NOT EDIT. -->
<!-- Regenerate with: go generate ./... (directive lives in provider/divergence.go) -->

# Platform divergence ledger

This document is generated from the backends' registered divergence ledgers
(provider.Divergence). Kinds:

- **stub**: the platform cannot serve the method; the call returns an error
  wrapping provider.ErrNotImplemented and touches no wire.
- **ignore**: the named field (or, when Field is empty, the whole call) is
  silently dropped; the call succeeds without that effect.
- **mapping**: the call returns the closest platform equivalent, an
  approximation of the unified semantics.
- **detour**: the method bypasses the platform SDK and drives the raw
  transport client; behavior is unchanged, the entry is for maintainers.

## Standing limitations

- The gitea and forgejo SDKs accept no context on most calls, so context
  cancellation does not propagate into in-flight requests on those
  platforms.
- Milestone identifiers are platform-specific (a per-repo serial number on
  GitHub and Gitee, a platform ID on GitLab, Gitea, Forgejo, GitCode, and
  Tencent Code); Milestone.Number round-trips only on the platform it came
  from.
`

// repoRoot walks up from the working directory to the module root.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "genledger: getwd:", err)
		os.Exit(1)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			fmt.Fprintln(os.Stderr, "genledger: go.mod not found upward from cwd")
			os.Exit(1)
		}
		dir = parent
	}
}
```

- [x] **Step 2: golden 测试** — 创建 `internal/tools/genledger/main_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderMatchesCommittedLedger(t *testing.T) {
	committed, err := os.ReadFile(filepath.Join(repoRoot(), "docs", "divergence-ledger.md"))
	if err != nil {
		t.Fatalf("read committed ledger: %v (run go generate ./... first)", err)
	}
	if got := render(); got != string(committed) {
		t.Fatal("docs/divergence-ledger.md is stale: regenerate with `go generate ./...`")
	}
}
```

- [x] **Step 3: go:generate 指令与首跑** — 在 `provider/divergence.go` 文件头(package 行之前)加:

```go
//go:generate go run ../../internal/tools/genledger
```

Run: `go generate ./... && go test ./internal/tools/genledger/`;预期:生成 `docs/divergence-ledger.md`,golden 测试 PASS。
- [x] **Step 4: README** — `README.md` 在能力/后端相关章节后新增:

```markdown
## Divergence ledger

Every backend registers the places where its behavior departs from the
unified semantics (stub / ignore / mapping / detour) in a machine-readable
ledger, surfaced by `Provider.Divergences()` and helper predicates such as
`provider.Ignores`. The rendered document lives at
[docs/divergence-ledger.md](docs/divergence-ledger.md); regenerate it with
`go generate ./...` after editing any backend's `divergence.go`. The
contract suite fails when a ledger entry drifts from actual behavior.
```

- [x] **Step 5: CHANGELOG** — `[Unreleased]` 加 `### Added`:`- **Divergence ledger**: ...`(描述 `Provider.Divergences()`、helper 谓词、生成文档)。
- [x] **Step 6: 全量验证 + 提交** — Run: `go build ./... && go test ./...`;预期全绿。`git add -A && git commit -m "feat: generate docs/divergence-ledger.md from the divergence ledger"`

### Task 7: backendutil IDCache 与 ResolveLabelID

**Files:**
- Create: `backends/internal/backendutil/idcache.go`、`backends/internal/backendutil/labelresolve.go`
- Test: `backends/internal/backendutil/idcache_test.go`、`backends/internal/backendutil/labelresolve_test.go`

**Interfaces:**
- Produces: `type IDCache struct`;`func NewIDCache(ttl time.Duration) *IDCache`;`func (c *IDCache) Get(key string) (int64, bool)`;`func (c *IDCache) Put(key string, id int64)`;`type LabelRef struct{ ID int64; Name string }`;`var ErrLabelScanLimit = errors.New("label not found within scan limit")`;`type LabelPageFunc func(page, perPage int) ([]LabelRef, error)`;`func ResolveLabelID(scan LabelPageFunc, name string, maxPages, perPage int) (int64, error)`;`func (c *IDCache) ResolveLabel(key, name string, scan LabelPageFunc, maxPages, perPage int) (int64, error)`。

- [x] **Step 1: 失败测试** — `idcache_test.go`:

```go
package backendutil

import (
	"testing"
	"time"
)

func TestIDCachePutGet(t *testing.T) {
	c := NewIDCache(time.Minute)
	if _, ok := c.Get("k"); ok {
		t.Fatal("empty cache returned a hit")
	}
	c.Put("k", 42)
	if id, ok := c.Get("k"); !ok || id != 42 {
		t.Fatalf("Get = (%d, %v), want (42, true)", id, ok)
	}
}

func TestIDCacheTTLExpiry(t *testing.T) {
	c := NewIDCache(time.Nanosecond)
	c.Put("k", 42)
	time.Sleep(time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expired entry returned a hit")
	}
}
```

`labelresolve_test.go`:

```go
package backendutil

import (
	"errors"
	"testing"
)

func TestResolveLabelIDFindsOnSecondPage(t *testing.T) {
	pages := map[int][]LabelRef{
		1: {{ID: 1, Name: "a"}, {ID: 2, Name: "b"}},
		2: {{ID: 3, Name: "target"}},
	}
	id, err := ResolveLabelID(func(page, perPage int) ([]LabelRef, error) {
		return pages[page], nil
	}, "target", 5, 2)
	if err != nil || id != 3 {
		t.Fatalf("ResolveLabelID = (%d, %v), want (3, nil)", id, err)
	}
}

func TestResolveLabelIDStopsAtShortPage(t *testing.T) {
	calls := 0
	_, err := ResolveLabelID(func(page, perPage int) ([]LabelRef, error) {
		calls++
		return []LabelRef{{ID: 1, Name: "a"}}, nil // short page: no more pages
	}, "missing", 50, 100)
	if !errors.Is(err, ErrLabelScanLimit) {
		t.Fatalf("err = %v, want ErrLabelScanLimit", err)
	}
	if calls != 1 {
		t.Fatalf("scan calls = %d, want 1 (short page must stop the scan)", calls)
	}
}

func TestResolveLabelIDScanLimitDistinctFromShortPage(t *testing.T) {
	_, err := ResolveLabelID(func(page, perPage int) ([]LabelRef, error) {
		return []LabelRef{{ID: int64(page), Name: "other"}}, nil // always full page? no: 1 < 100
	}, "missing", 50, 100)
	// 1 result < perPage 100 → short page → stop → scan limit error, 1 call.
	if !errors.Is(err, ErrLabelScanLimit) {
		t.Fatalf("err = %v, want ErrLabelScanLimit", err)
	}
}

func TestIDCacheResolveLabelCaches(t *testing.T) {
	c := NewIDCache(time.Minute)
	scans := 0
	scan := func(page, perPage int) ([]LabelRef, error) {
		scans++
		return []LabelRef{{ID: 7, Name: "x"}}, nil
	}
	for i := 0; i < 3; i++ {
		id, err := c.ResolveLabel("owner/repo", "x", scan, 50, 100)
		if err != nil || id != 7 {
			t.Fatalf("ResolveLabel #%d = (%d, %v)", i, id, err)
		}
	}
	if scans != 1 {
		t.Fatalf("scans = %d, want 1 (second and third resolve must hit the cache)", scans)
	}
}
```

- [x] **Step 2: 跑测试确认失败** — Run: `go test ./backends/internal/backendutil/`;预期:编译失败。
- [x] **Step 3: 实现** — `idcache.go`:

```go
package backendutil

import (
	"sync"
	"time"
)

// IDCache is a TTL cache from string keys to int64 IDs. Backends use it to
// memoize name→ID resolutions (labels, users) so repeated writes do not
// rescan. The zero value is not usable; construct with NewIDCache.
type IDCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]idCacheEntry
}

type idCacheEntry struct {
	id        int64
	expiresAt time.Time
}

// NewIDCache constructs an IDCache with the given TTL (TTL <= 0 never
// expires entries).
func NewIDCache(ttl time.Duration) *IDCache {
	return &IDCache{ttl: ttl, entries: map[string]idCacheEntry{}}
}

// Get returns the cached ID for key when present and fresh.
func (c *IDCache) Get(key string) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || (c.ttl > 0 && !time.Now().Before(e.expiresAt)) {
		return 0, false
	}
	return e.id, true
}

// Put stores id under key with a fresh TTL.
func (c *IDCache) Put(key string, id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var expiresAt time.Time
	if c.ttl > 0 {
		expiresAt = time.Now().Add(c.ttl)
	}
	c.entries[key] = idCacheEntry{id: id, expiresAt: expiresAt}
}
```

`labelresolve.go`:

```go
package backendutil

import "errors"

// ErrLabelScanLimit reports that the named label was not found within the
// scanned page budget. It is distinct from a definitive not-found: the label
// may exist beyond the limit, so callers surface it as a scan-limit error
// rather than a 404.
var ErrLabelScanLimit = errors.New("label not found within scan limit")

// LabelRef is the minimal label shape the resolver needs from one platform
// page fetch.
type LabelRef struct {
	ID   int64
	Name string
}

// LabelPageFunc fetches one page of labels; page is 1-based, perPage is the
// requested page size, and a page returning fewer than perPage refs ends the
// scan.
type LabelPageFunc func(page, perPage int) ([]LabelRef, error)

// ResolveLabelID scans pages for the named label and returns its ID, or
// ErrLabelScanLimit when the budget is exhausted or a short page is reached
// without a match. Scan errors propagate unchanged.
func ResolveLabelID(scan LabelPageFunc, name string, maxPages, perPage int) (int64, error) {
	for page := 1; page <= maxPages; page++ {
		refs, err := scan(page, perPage)
		if err != nil {
			return 0, err
		}
		for _, r := range refs {
			if r.Name == name {
				return r.ID, nil
			}
		}
		if len(refs) < perPage {
			break
		}
	}
	return 0, ErrLabelScanLimit
}

// ResolveLabel resolves via ResolveLabelID and caches the result under
// key+"/"+name. Failures are not cached.
func (c *IDCache) ResolveLabel(key, name string, scan LabelPageFunc, maxPages, perPage int) (int64, error) {
	if id, ok := c.Get(key + "/" + name); ok {
		return id, nil
	}
	id, err := ResolveLabelID(scan, name, maxPages, perPage)
	if err != nil {
		return 0, err
	}
	c.Put(key+"/"+name, id)
	return id, nil
}
```

- [x] **Step 4: 跑测试确认通过** — Run: `go test ./backends/internal/backendutil/ -v`;预期:PASS。
- [x] **Step 5: 提交** — `git add -A && git commit -m "feat(backendutil): shared IDCache and paginated label resolver"`

### Task 8/9/10: gitlab / gitea / forgejo 接入共享 LabelResolver

> 三个任务同构,各自独立提交;以下以 gitlab 为例,gitea/forgejo 按各自 SDK 差异替换扫描闭包。

**Files(每个任务):**
- Modify: `backends/gitlab/gitlab.go`(Provider struct 加 `labelIDs *backendutil.LabelIDCache`→直接用 `*backendutil.IDCache`;构造函数初始化 `NewLabelIDCache` 不存在——用 `backendutil.NewIDCache(5 * time.Minute)`)
- Modify: `backends/gitlab/labels.go`(resolveLabelID 重写)
- Test: 既有 `backends/gitlab/labels_paginate_test.go` 应保持全绿

**Interfaces:**
- Consumes: Task 7 的 `backendutil.IDCache.ResolveLabel`、`backendutil.ErrLabelScanLimit`。
- Produces: 各后端 `resolveLabelID` 行为不变 + TTL 缓存 + 扫描上限错误不再误报 404。

- [x] **Step 1(gitlab):** `gitlab.go` 的 Provider struct 加字段 `labelIDs *backendutil.IDCache`,构造函数(NewProvider)加 `labelIDs: backendutil.NewIDCache(5 * time.Minute)`。
- [x] **Step 2(gitlab):** `labels.go` 的 `resolveLabelID` 整体替换为:

```go
// resolveLabelID finds the numeric ID of the named label via the shared
// paginated resolver with a per-provider TTL cache. GitLab's update and
// delete endpoints address labels by ID while the SDK's surface addresses
// them by name. Exhausting the 50-page budget surfaces a scan-limit error
// (distinct from a definitive 404). op is the public operation the
// resolution serves; failures surface under that op.
func (p *Provider) resolveLabelID(ctx context.Context, op, owner, repo, name string) (int64, error) {
	id, err := p.labelIDs.ResolveLabel(owner+"/"+repo, name, func(page, perPage int) ([]backendutil.LabelRef, error) {
		labels, _, err := p.client.Labels.ListLabels(pidOf(owner, repo),
			&gitlab.ListLabelsOptions{ListOptions: gitlab.ListOptions{Page: int64(page), PerPage: int64(perPage)}},
			gitlab.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		refs := make([]backendutil.LabelRef, len(labels))
		for i, l := range labels {
			refs[i] = backendutil.LabelRef{ID: l.ID, Name: l.Name}
		}
		return refs, nil
	}, 50, 100)
	if err != nil {
		if errors.Is(err, backendutil.ErrLabelScanLimit) {
			return 0, provider.Wrapf(provider.PlatformGitLab, op, "label %q not found within 50 pages (scan limit)", name)
		}
		return 0, provider.Wrap(provider.PlatformGitLab, op, err)
	}
	return id, nil
}
```

(imports 增加 `errors` 与 backendutil;gitea/forgejo 版本:无 ctx、`p.client.ListRepoLabels(owner, repo, <sdk>.ListLabelsOptions{ListOptions: <sdk>.ListOptions{Page: page, PageSize: perPage}})`,平台常量与错误前缀相应替换。)
- [x] **Step 3(gitlab):** Run: `go test ./backends/gitlab/ -v`;预期:含 `labels_paginate_test` 全绿(若其断言 404 文案/形态,按新语义更新:短页找不到 = 扫描上限错误,不再是 404——这是本任务要修的误报)。
- [x] **Step 4(gitlab):** `git add -A && git commit -m "refactor(gitlab): shared label resolver with cache and honest scan-limit error"`
- [x] **Step 5-8(gitea):** 同构(`PlatformGitea`、无 ctx);提交 `refactor(gitea): ...`。
- [x] **Step 9-12(forgejo):** 同构(`PlatformForgejo`);提交 `refactor(forgejo): ...`。

### Task 11: gitlab Users 解析 + Assignees 修复

**Files:**
- Create: `backends/gitlab/users.go`
- Modify: `backends/gitlab/gitlab.go`(Provider struct 加 `userIDs *backendutil.IDCache`,构造初始化)
- Modify: `backends/gitlab/issues.go`(CreateIssue/UpdateIssue 接 Assignees)
- Modify: `backends/gitlab/divergence.go`(删除两条 Assignees ignore 条目)

**Interfaces:**
- Consumes: `backendutil.IDCache`;gitlab client-go `p.client.Users.ListUsers(&gitlab.ListUsersOptions{Username: gitlab.Ptr(name)}, gitlab.WithContext(ctx))`(module v2.55.1,users.go:489;User.ID int64 / User.Username string)。
- Produces: `(p *Provider) resolveUserIDs(ctx context.Context, op string, usernames []string) ([]int64, error)`。

- [x] **Step 1: users.go** — 创建:

```go
package gitlab

import (
	"context"
	"fmt"
	"net/http"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/backends/internal/backendutil"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// resolveUserIDs resolves usernames to GitLab user IDs using ListUsers with
// the exact-match username filter, memoized in a per-provider TTL cache.
// op is the public operation the resolution serves; an unknown username
// surfaces as a NotFound under that op.
func (p *Provider) resolveUserIDs(ctx context.Context, op string, usernames []string) ([]int64, error) {
	ids := make([]int64, 0, len(usernames))
	for _, name := range usernames {
		if id, ok := p.userIDs.Get(name); ok {
			ids = append(ids, id)
			continue
		}
		users, _, err := p.client.Users.ListUsers(
			&gitlab.ListUsersOptions{Username: gitlab.Ptr(name)},
			gitlab.WithContext(ctx))
		if err != nil {
			return nil, provider.Wrap(provider.PlatformGitLab, op, err)
		}
		var (
			id    int64
			found bool
		)
		for _, u := range users {
			if u != nil && u.Username == name {
				id, found = u.ID, true
				break
			}
		}
		if !found {
			return nil, provider.New(provider.PlatformGitLab, op, http.StatusNotFound, fmt.Sprintf("user %q not found", name))
		}
		p.userIDs.Put(name, id)
		ids = append(ids, id)
	}
	return ids, nil
}
```

- [x] **Step 2: struct 与构造** — `gitlab.go` 的 Provider struct 加 `userIDs *backendutil.IDCache`;NewProvider 初始化 `userIDs: backendutil.NewIDCache(5 * time.Minute)`。
- [x] **Step 3: CreateIssue** — `issues.go` CreateIssue 中,`if opts.Milestone != ""` 块之前加:

```go
	if len(opts.Assignees) > 0 {
		assigneeIDs, err := p.resolveUserIDs(ctx, "CreateIssue", opts.Assignees)
		if err != nil {
			return nil, err
		}
		createOpts.AssigneeIDs = &assigneeIDs
	}
```

同时把方法头注释 `// opts.Assignees is ignored: ...` 改为 `// Assignees are resolved to user IDs via the Users API (cached).`。
- [x] **Step 4: UpdateIssue** — 同样在 UpdateIssue 的 opts 构造区加(UpdateIssueOptions 同样有 `AssigneeIDs *[]int64`):

```go
	if len(opts.Assignees) > 0 {
		assigneeIDs, err := p.resolveUserIDs(ctx, "UpdateIssue", opts.Assignees)
		if err != nil {
			return nil, err
		}
		updateOpts.AssigneeIDs = &assigneeIDs
	}
```

并删除其 `// opts.Assignees is ignored (see CreateIssue).` 注释。
- [x] **Step 5: 台账同步** — 删除 `backends/gitlab/divergence.go` 中两条 `IssueManager ... Assignees` ignore 条目。
- [x] **Step 6: 契约测试** — Run: `go test ./backends/gitlab/ -v`。若 Issues 套件的 mock 服务器对 issue create 的 assignees 断言或 `/users` 路由缺失导致失败:在对应 stub server 增加 `GET /users` 路由,返回 `[{"id":101,"username":"<r.URL.Query().Get("username")>"}]`(参照 reviews 套件的 recording 模式)。
- [x] **Step 7: 全量 + 提交** — Run: `go test ./...`;`git add -A && git commit -m "fix(gitlab): resolve issue assignees via the Users API instead of ignoring them"`;CHANGELOG `[Unreleased] > Fixed` 加对应条目。

### Task 12: gitlab RequestReviewers 真实实现

**Files:**
- Modify: `backends/gitlab/reviews.go`(RequestReviewers)
- Modify: `backends/gitlab/divergence.go`(删除 RequestReviewers ignore 条目)
- Modify: `backends/gitlab/contract_test.go`(去掉 `Reviews.IgnoresRequestReviewers: true`)
- Modify: `backends/contracttest/reviews.go`(reviewStubServer 加 `/users` 路由 + 按 ID 断言的路径)
- Modify: `docs/divergence-ledger.md`(`go generate ./...` 再生成)

**Interfaces:**
- Consumes: Task 11 的 `resolveUserIDs`;gitlab `UpdateMergeRequestOptions.ReviewerIDs *[]int64`(merge_requests.go:732)。
- Produces: gitlab RequestReviewers 真实生效。

- [x] **Step 1: 实现** — `reviews.go` 的 RequestReviewers 替换为:

```go
// RequestReviewers implements provider.ReviewManager. Reviewer usernames
// are resolved to user IDs via the Users API (cached) and written through
// UpdateMergeRequest's reviewer_ids.
func (p *Provider) RequestReviewers(ctx context.Context, owner, repo, number string, reviewers []string) error {
	iid, err := prNumber("RequestReviewers", number)
	if err != nil {
		return err
	}
	if len(reviewers) == 0 {
		return nil
	}
	ids, err := p.resolveUserIDs(ctx, "RequestReviewers", reviewers)
	if err != nil {
		return err
	}
	if _, _, err := p.client.MergeRequests.UpdateMergeRequest(pidOf(owner, repo), iid,
		&gitlab.UpdateMergeRequestOptions{ReviewerIDs: &ids}, gitlab.WithContext(ctx)); err != nil {
		return provider.Wrap(provider.PlatformGitLab, "RequestReviewers", err)
	}
	return nil
}
```

- [x] **Step 2: 台账** — 删除 `divergence.go` 中 RequestReviewers ignore 条目;Run: `go generate ./...`。
- [x] **Step 3: 套件路由** — `contracttest/reviews.go`:(a) `reviewStubServer` 增加路由——`r.URL.Path` 以 `/users` 结尾且方法为 GET 时,返回 `[{"id":101,"username":"` + r.URL.Query().Get("username") + `"}]` 并计入 recordedRequest;(b) `assertRequestReviewersWire` 保持现有用户名断言用于其余平台;新增:

```go
// assertRequestReviewersByIDWire checks a reviewer request that resolves
// usernames through a /users lookup and writes numeric reviewer IDs (GitLab).
func assertRequestReviewersByIDWire(t *testing.T, rm provider.ReviewManager, requests *[]recordedRequest) {
	t.Helper()
	if err := rm.RequestReviewers(context.Background(), "owner", "repo", "1", []string{"dev"}); err != nil {
		t.Fatalf("RequestReviewers: %v", err)
	}
	var putBody string
	sawUsersLookup := false
	for _, req := range *requests {
		if strings.HasSuffix(req.Path, "/users") {
			sawUsersLookup = true
			continue
		}
		if req.Method == "PUT" || req.Method == "PATCH" {
			putBody = req.Body
		}
	}
	if !sawUsersLookup {
		t.Error("expected a /users username lookup before writing reviewers")
	}
	if !strings.Contains(putBody, "reviewer_ids") || !strings.Contains(putBody, "101") {
		t.Errorf("merge-request update body = %q, want reviewer_ids containing resolved id 101", putBody)
	}
}
```

(c) `ReviewsHarnessConfig` 加字段 `RequestReviewersByID bool`(注释:GitLab resolves usernames to IDs first);路由处 `if h.IgnoresRequestReviewers {...}` 之后加 `if h.RequestReviewersByID { assertRequestReviewersByIDWire(t, rm, requests); return }`。
- [x] **Step 4: gitlab harness** — `backends/gitlab/contract_test.go` 删除 `IgnoresRequestReviewers: true`,改为 `RequestReviewersByID: true`(注释说明)。
- [x] **Step 5: 全量 + 提交** — Run: `go build ./... && go test ./... && go generate ./... && git add -A && git commit -m "fix(gitlab): RequestReviewers resolves reviewers via the Users API"`;CHANGELOG `[Unreleased] > Fixed` 加条目(注明行为变化:原来静默忽略,现在真实生效;未知用户名返回 NotFound)。

### Task 13: CommitStatusManager 拆分

**Files:**
- Create: `provider/iface_commitstatus.go`
- Modify: `provider/iface_commits.go`(删 CreateCommitStatus 行)、`provider/provider.go`(CapabilitySet 加字段 + 注释)、`backends/contracttest/capabilities.go`(加断言)、`backends/contracttest/contracttest.go`(Run 挂 CommitStatusSuite)、7 个后端 commits.go / Capabilities、`backends/gitee/commits.go`(删 stub)、`backends/gitee/divergence.go`(删条目)、`backends/contracttest/divergence.go`(dispatcher 的 CreateCommitStatus case 改走 CommitStatusManager)、`docs/divergence-ledger.md`、`README.md`、`CHANGELOG.md`
- Create: `backends/contracttest/commitstatus.go`

**Interfaces:**
- Produces: `type CommitStatusManager interface { CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error }`;`CapabilitySet.CommitStatuses bool`;`Harness.CommitStatus *CommitStatusHarnessConfig`;`type CommitStatusHarnessConfig struct{}`(零配置:套件自驱动,只需声明)。

- [x] **Step 1: 接口** — 创建 `provider/iface_commitstatus.go`:

```go
package provider

import "context"

// CommitStatusManager reports CI statuses on commits. It is an optional
// capability interface: consumers should gate on
// Provider.Capabilities().CommitStatuses (or type-assert) before use.
//
// It is deliberately separate from CommitManager: commit statuses are a CI
// reporting concern that not every platform exposes (Gitee's public REST
// API has no commit-status endpoint), so absence is expressed by not
// declaring the capability instead of stubbing the method.
type CommitStatusManager interface {
	CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error
}
```

`iface_commits.go` 删除 CreateCommitStatus 行;`provider/provider.go` 的 CapabilitySet 加:

```go
	CommitStatuses bool // provider.CommitStatusManager
```

(同时更新 Provider 接口注释中可选能力列举。)
- [x] **Step 2: 后端** — gitcode/gitlab/github/gitea/forgejo/tencentcode:方法代码不动(仍在 commits.go),加编译守卫 `var _ provider.CommitStatusManager = (*Provider)(nil)`,Capabilities() 返回值加 `CommitStatuses: true`。gitee:删除 commits.go 中的 CreateCommitStatus stub 方法;`divergence.go` 删除其 stub 条目;Capabilities 不加。
- [x] **Step 3: 一致性测试** — `capabilities.go` 加:

```go
	_, commitStatusesImpl := p.(provider.CommitStatusManager)
	if caps.CommitStatuses != commitStatusesImpl {
		t.Errorf("Capabilities().CommitStatuses = %v, but CommitStatusManager type assertion = %v; declaration and implementation have drifted", caps.CommitStatuses, commitStatusesImpl)
	}
```

- [x] **Step 4: CommitStatus 套件** — 创建 `backends/contracttest/commitstatus.go`:

```go
package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CommitStatusHarnessConfig mounts the commit-status suite. The suite is
// self-driving: it records requests, invokes CreateCommitStatus, and asserts
// exactly one status-reporting request reached the wire.
type CommitStatusHarnessConfig struct{}

func testCommitStatusSuite(t *testing.T, h Harness) {
	declared := h.CommitStatus != nil
	capsDeclared := false
	{
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(h.EmptyListResponse))
		}))
		defer srv.Close()
		p := h.NewProvider(t, baseCfg(h, srv.URL))
		capsDeclared = p.Capabilities().CommitStatuses
	}
	switch {
	case h.CommitStatus == nil && !capsDeclared:
		t.Skipf("%s declares no CommitStatuses capability", h.Name)
	case h.CommitStatus == nil:
		t.Errorf("%s declares Capabilities().CommitStatuses but its Harness provides no CommitStatus config", h.Name)
	case !capsDeclared:
		t.Errorf("%s Harness provides a CommitStatus config but the platform does not declare Capabilities().CommitStatuses", h.Name)
	}

	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"state":"pending"}`))
	}))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	csm, ok := p.(provider.CommitStatusManager)
	if !ok {
		t.Fatalf("%s: provider does not implement CommitStatusManager", h.Name)
	}
	if err := csm.CreateCommitStatus(context.Background(), "owner", "repo", "deadbeef",
		provider.CommitStatusOptions{State: "pending", Context: "ci"}); err != nil {
		t.Fatalf("CreateCommitStatus: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 1 {
		t.Fatalf("%s: CreateCommitStatus made %d requests (%v), want exactly 1", h.Name, len(paths), paths)
	}
	if !strings.Contains(paths[0], "status") && !strings.Contains(paths[0], "statuses") {
		t.Errorf("%s: status request path %q does not look like a commit-status endpoint", h.Name, paths[0])
	}
}
```

(补 import "strings";)在 `contracttest.go` Run 中挂 `t.Run("CommitStatusSuite", func(t *testing.T) { testCommitStatusSuite(t, h) })`;`Harness` struct 加 `CommitStatus *CommitStatusHarnessConfig` 字段(注释同 Labels 模式:双向强制)。6 个支持后端的 contract_test.go 加 `CommitStatus: &contracttest.CommitStatusHarnessConfig{}`;gitee 不加。
- [x] **Step 5: dispatcher 更新** — `contracttest/divergence.go` 的 `CommitManager.CreateCommitStatus` case 改为:

```go
	case "CommitStatusManager.CreateCommitStatus":
		csm, ok := p.(provider.CommitStatusManager)
		if !ok {
			return true, nil
		}
		return true, csm.CreateCommitStatus(ctx, "owner", "repo", "deadbeef", provider.CommitStatusOptions{State: "pending"})
```

- [x] **Step 6: 文档** — Run: `go generate ./...`;README 能力表加 CommitStatuses(可选);CHANGELOG `[Unreleased]` 加 `### Changed`(breaking):CreateCommitStatus 移入可选 CommitStatusManager,CapabilitySet.CommitStatuses 新增,gitee 的 ErrNotImplemented stub 移除。
- [x] **Step 7: 全量 + 提交** — Run: `go build ./... && go test ./...`;`git add -A && git commit -m "feat(provider)!: split CreateCommitStatus into optional CommitStatusManager"`

### Task 14: SearchManager total 诚实化(*int)

**Files:**
- Modify: `provider/iface_search.go`、6 个有 search 的后端(`backends/{gitlab,github,gitee,gitea,forgejo,gitcode}/search.go`)、`backends/contracttest/search.go`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Produces: `SearchRepos/SearchIssues/SearchUsers` 签名 `(..., *int, error)`;nil 表示平台不返回服务端总数。

- [x] **Step 1: 接口** — `iface_search.go` 三个方法返回值 `int` → `*int`,接口注释加:

```go
// The *int return is the server-side total when the platform reports one,
// and nil when it does not — callers must not treat a total as guaranteed.
```

- [x] **Step 2: 后端** — gitlab/gitea/forgejo:把 `return result, len(result), nil` 改为 `return result, nil, nil`(三方法同名模式;凡伪造 `len(...)` 之处一律改 nil)。github:改为

```go
	if total := result.GetTotal(); total > 0 {
		return repos, &total, nil
	}
	return repos, nil, nil
```

(三方法同构;变量名按各自代码。)gitcode/gitee:读各自 search.go——若响应信封带服务端总数则返回其指针,否则返回 nil;禁止再返回 `len(...)`。
- [x] **Step 3: 套件** — `contracttest/search.go`:`assertSearchRepos(t, sm, requests, wantTotal)` 的 total 断言改为:

```go
	if wantTotal > 0 {
		if total == nil || *total != wantTotal {
			t.Errorf("SearchRepos total = %v, want %d", total, wantTotal)
		}
	} else if total != nil && *total < len(repos) {
		t.Errorf("SearchRepos total = %d, want nil or >= len(results) (%d)", *total, len(repos))
	}
```

若存在 Issues/Users 的同类 total 断言,套用同一变换(`total == nil || *total >= len(results)` 为弱断言)。`repos, total, err := ...` 处 total 类型随接口变化,无需额外改动。
- [x] **Step 4: 全量 + 提交** — Run: `go build ./... && go test ./...`;CHANGELOG `[Unreleased] > Changed`(breaking):SearchManager 的 total 返回值改为 `*int`,nil = 平台不报告;`git add -A && git commit -m "feat(provider)!: search totals are *int, nil when the platform reports none"`

### Task 15: CONTRIBUTING checklist + 收尾验证

**Files:**
- Modify: `CONTRIBUTING.md`、`CHANGELOG.md`(通读校对)、`README.md`(通读校对)

- [x] **Step 1: CONTRIBUTING** — 增加"Adding an optional capability"清单:

```markdown
## Adding an optional capability

1. Define the interface in `provider/iface_<name>.go` and add a
   `CapabilitySet` field (e.g. `CommitStatuses`).
2. Implement it in every backend that supports it; declare the field in each
   backend's `Capabilities()`. Platforms that cannot serve the capability do
   not implement it at all — absence is expressed by the declaration, never
   by a stub.
3. Extend `backends/contracttest/capabilities.go` with the bidirectional
   assertion for the new field, and add a mounted suite
   (`backends/contracttest/<name>.go` + `Harness` field + `Run` wiring).
4. Register any partial divergences (per-method stubs, ignored fields,
   semantic mappings, raw detours) in the affected backends'
   `divergence.go`, then run `go generate ./...` to refresh
   `docs/divergence-ledger.md`.
5. Update this checklist if the coupling points change.
```

- [x] **Step 2: 全量终验** — Run: `go build ./... && go test ./... && go vet ./... && go generate ./... && git diff --exit-code`;预期:全绿且生成文档无漂移。
- [x] **Step 3: 提交** — `git add -A && git commit -m "docs: capability checklist and hardening wrap-up"`

---

## Self-Review 记录

1. **Spec 覆盖**:Phase 0 → Task 1-6;Phase 1 → Task 7-12;Phase 2 → Task 15(+Task 13 的一致性扩展);Phase 3 → Task 13/14。台账附录含 REMOVE 标注的 4 条与任务 12/13 的删除步骤对应。✅
2. **占位符**:Task 1/5 代码中的两处"笔误占位"均已给出正确版本并声明以正确版为准;Task 8-10 对 gitea/forgejo 的差异以明确参数化指令(平台常量、SDK 选项结构、无 ctx)给出;Task 14 对 gitcode/gitee 给出确定性判定规则(有信封总数→指针,否则 nil)。✅
3. **类型一致性**:`Divergence` 五字段、`DivergenceKind` 四常量、`IDCache`/`ResolveLabel`/`ErrLabelScanLimit`、`CommitStatusManager`、`*int` total——跨任务引用已核对。✅
