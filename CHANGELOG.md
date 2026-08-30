# Changelog

All notable changes to this project are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.49.0] - 2026-08-30

### Added

- **`IssueManager.UpdateIssueComment`** — edit an issue comment's body on
  all seven platforms. The body is replaced wholesale (every platform's
  edit surface is a body-only PATCH/PUT), and the platform enforces
  authorship: only the comment's author — typically the token identity,
  e.g. a review bot updating its own comment — may edit. `commentID` is
  the `IssueComment.ID` from `CreateIssueComment`/`ListIssueComments`.
  `number` is ignored where the edit endpoint addresses the comment
  directly (GitHub, Gitea, Forgejo, GitCode, Gitee); GitLab and Tencent
  工蜂 route through the issue, so it must carry the issue's number there.
  Gitee's wire IDs are int32, so an out-of-range ID fails up front instead
  of truncating to a different comment. The issues contract suite now
  asserts the edit wire shape (PATCH/PUT + body) across all seven
  backends, and additionally pins the issue-routed note path
  (issues/{number}/notes/{id}) on GitLab and Tencent 工蜂 so the number
  and commentID plumbing cannot be swapped silently.

## [v0.48.0] - 2026-08-29

### Changed

- **Dependency refresh across the board** (2026-08-29): google/go-github
  v69.2.0 → v72.0.0 (import-path swap only; no used API changed), GitLab
  client-go v2.55.1 → v2.60.0, golang.org/x/crypto v0.54.0 → v0.55.0, and
  CI actions (checkout v5→v7, upload-artifact v6→v7). All direct
  dependencies are now at their latest releases.

### Security

- **Pinned `toolchain go1.26.6`** in go.mod: govulncheck reported seven
  stdlib vulnerabilities reachable from this code under go1.26.4
  (crypto/tls, net/http, net/url, encoding/xml, encoding/asn1), all fixed
  in go1.26.6. CI bootstraps the patched toolchain via go-version-file;
  govulncheck now reports zero affecting vulnerabilities.

## [v0.47.0] - 2026-08-28

### Fixed

- **Tencent Code 工蜂 `CreateIssue`/`UpdateIssue` now honor `Assignees`**
  instead of silently ignoring them. The usernames are resolved to 工蜂 user
  IDs through the Users API (`GET /users/{username}`, which accepts either an
  ID or a username), memoized in a per-provider TTL cache, and sent as the
  `assignee_ids` csv on the issue write — the same playbook as GitLab's
  v0.45.0 assignee fix. Behavior change: callers who previously passed
  `Assignees` with no effect will now see real assignments, and an unknown
  username fails the call with a `NotFound` error (`provider.IsNotFound`)
  instead of succeeding without the assignment. The two registered-ignore
  ledger entries for `opts.Assignees` are removed, and the issues contract
  suite gained a `CreateIssueAssigneesByID` harness flag (enabled for GitLab
  and 工蜂) asserting the lookup-then-write wire shape.
- **The `ListIssues` `Assignee` filter limitation on Tencent Code is now
  registered in the divergence ledger** (kind: ignore) — the Gongfeng issue
  list endpoint takes no assignee filter, so the option is accepted but
  ignored. Previously this divergence lived only in a code comment.
- **gitbackend (gogit) three-way `Merge` no longer drops head-side files.**
  The merge applied the head→branch diff to the worktree, which deleted every
  file that existed on HEAD but not on the merged branch; the merge commit
  silently lost the head side. It now applies the base→branch diff, so both
  sides survive (file-level conflict detection is unchanged).
- **gitbackend (gogit) `Fetch` with no `Remote` set no longer fails.** The
  defaulted `origin` name reached neither the fetch options nor the refspecs,
  producing `refs/remotes//<branch>` (an invalid reference) and an outright
  error; the defaulted name now flows through, and `FetchOptions.Remote`
  documents the default honestly.
- **gitbackend (gogit) `Fetch` result classification now actually
  populates.** `NewBranches`/`UpdatedBranch` were keyed off a
  `refs/remotes/<remote>/heads/` prefix that the fetch refspecs never write
  (they map onto `refs/remotes/<remote>/<name>`), so they were always empty;
  `NewTags` similarly looked under `refs/remotes/<remote>/tags/` while
  `AllTags` writes `refs/tags/*`. Classification now keys off the namespaces
  the refspecs really write.
- **gitbackend (gogit) `Rebase` no longer deletes onto-side files and no
  longer leaks the pre-rebase index into replayed commits.** Replay applied
  the new-HEAD→commit diff (deleting every onto-side file the replayed
  commit does not carry) and committed the stale pre-rebase index; it now
  hard-resets the worktree and index to the onto commit before replaying,
  and applies each commit's own diff (parent→commit).
- **gitbackend (native) `ListBranches` now classifies remote-tracking
  branches again.** Remote-ness was detected via a `remotes/` prefix on
  `%(refname:short)`, but under `--format` that prefix is stripped (the
  short name is `origin/<branch>`), so every remote branch was reported as a
  local one. The format now leads with the full `%(refname)` column and
  classifies off `refs/remotes/`.
- **gitbackend (native) conflict classification works again in
  `Merge`/`CherryPick`/`Rebase`/`RebaseContinue`.** git prints its CONFLICT
  lines on stdout while stderr stays empty on a content conflict, so the
  stderr-only check never matched and conflicts surfaced as generic errors
  instead of `ErrMergeConflict`. Both streams are now inspected.
- **gitbackend (native) `GetFileHistory` with a limit no longer fails.**
  The `-<limit>` flag was appended after the `--` separator, turning it
  into a second pathspec that `--follow` rejects outright ("--follow
  requires exactly one pathspec"); the flag now precedes the separator.
- **gitbackend (native) `Fetch` reports the fetched refs again.**
  `parseFetchRefs` checked for the ref-line indent after `TrimSpace`, which
  no line can ever match, so `FetchResult.FetchedRefs` was always empty;
  the indent check now runs on the raw line.
- **gitbackend (gogit) `RebaseContinue` replays the original branch's
  commits.** It collected the commits between the saved orig-head and the
  onto target — the onto side's commits — while the persisted `end` counter
  refers to the branch side's commits Rebase planned, so a continued rebase
  replayed (and labeled commits with) the wrong side. It now derives the
  replay set from the merge base, matching `Rebase`.

### Added

- **gitbackend test coverage 15.8% → 76.9%** across auth constructors, error
  sentinels, branch operations (list/rename/sync-info/remote listing on
  both backends), merge variants (ff, three-way, squash, no-commit,
  ff-only, conflict), cherry-pick, rebase/abort/continue (both backends),
  the stash lifecycle, file/tree/blob queries and checkouts, the
  Repository wrapper delegation surface, the output parsers, and the
  fetch/push/pull/clone/tag/remote/config core paths on both backends —
  the new tests caught the nine gitbackend bugs above (five gogit, four
  native).
- **Dependabot** now watches the module's seven platform-SDK dependencies
  (`gomod`, weekly) and the CI workflow actions (`github-actions`, weekly,
  grouped).
- **CI enforces a total-coverage floor of 45%** (current: ~50%) on the
  ubuntu leg, so a coverage regression fails the build instead of only
  uploading an artifact.

## [v0.46.0] - 2026-08-27

### Changed

- **Provider `Manager` capacity eviction is now true LRU** (least recently
  used instead of oldest-created), powered by `hashicorp/golang-lru/v2`
  access-order tracking; the O(n) eviction scan is gone. Public Manager API
  is unchanged.
- **`transport.RateLimiter`'s RPS cap is now enforced by**
  `golang.org/x/time/rate` (burst-1 token bucket); header-adaptive
  throttling and min-delay behavior are unchanged.
- Removed the pass-through `pkg/encoding` base64 wrappers; use
  `encoding/base64` directly.

## [v0.45.0] - 2026-08-26

### Fixed

- **GitLab `RequestReviewers` now really requests reviewers instead of
  silently doing nothing.** The reviewer usernames are resolved to GitLab
  user IDs through the Users API (`GET /users?username=<name>`, exact-match
  filter, memoized in the same per-provider TTL cache as issue assignees)
  and are written as `reviewer_ids` via `UpdateMergeRequest`. Behavior
  change: the call was previously a registered ignore that succeeded
  without any effect; it now takes real effect, and an unknown reviewer
  username fails the call with a `NotFound` error (`provider.IsNotFound`)
  instead of succeeding without the assignment. The registered-ignore
  ledger entry for `RequestReviewers` is removed, and the reviews contract
  suite gained a `RequestReviewersByID` harness flag asserting the
  lookup-then-write-by-ID wire shape.
- **GitLab `CreateIssue`/`UpdateIssue` now honor `Assignees` instead of
  silently ignoring them.** The usernames are resolved to GitLab user IDs
  through the Users API (`GET /users?username=<name>`, exact-match filter)
  memoized in a per-provider TTL cache, and are sent as `assignee_ids` on the
  issue write. Behavior change: callers who previously passed `Assignees`
  with no effect will now see real assignments, and an unknown username
  fails the call with a `NotFound` error (`provider.IsNotFound`) instead of
  succeeding without the assignment. The two registered-ignore ledger
  entries for `opts.Assignees` are removed.

### Added

- **Divergence ledger**: every backend now registers the places where its
  behavior departs from the unified semantics (stub / ignore / mapping /
  detour) as machine-readable `provider.Divergence` entries, surfaced by the
  new `Provider.Divergences()` interface method and package-level
  `<backend>.Divergences()` functions, with helper predicates
  `provider.Ignores`, `provider.Stubs`, and `provider.FindByMethod` for
  querying the ledger. The rendered document is generated into
  `docs/divergence-ledger.md` by `internal/tools/genledger` — run
  `go generate ./...` after editing any backend's `divergence.go`; a golden
  test keeps the committed document in sync, and the contract-test
  divergence suite fails when a ledger entry drifts from actual behavior.

### Changed

- **BREAKING: `SearchManager` total return changed from `int` to `*int`.**
  The three methods (`SearchRepos`, `SearchIssues`, `SearchUsers`) now return
  `*int` for the server-side total: non-nil with the platform-reported count
  when available (GitHub), nil when the platform does not report a total
  (GitLab, Gitea, Forgejo, GitCode, Gitee). Callers that previously relied on
  the total being a concrete `int` must now nil-check before dereferencing.
  The old behavior of returning `len(results)` as a fake total is removed.
- **BREAKING: `CreateCommitStatus` moved out of the core `CommitManager`
  into a new optional `CommitStatusManager` capability interface**
  (`provider/iface_commitstatus.go`). Commit statuses are a CI reporting
  concern that not every platform exposes — Gitee's public REST API has no
  commit-status endpoint — so absence is now expressed by not declaring the
  capability instead of stubbing the method. `CapabilitySet` gained a
  `CommitStatuses` field (declared by GitHub, GitLab, Gitea, Forgejo,
  GitCode, and Tencent Code); consumers should gate on
  `p.Capabilities().CommitStatuses` or type-assert
  `p.(provider.CommitStatusManager)`. The Gitee backend's
  `ErrNotImplemented` stub and its divergence-ledger entry are removed.
  Migration: calls through the `provider.Provider`/`CommitManager`
  interface now fail to compile — assert `CommitStatusManager` first. The
  contract suite gained a self-driving `CommitStatusSuite` (mounted via
  `Harness.CommitStatus`) asserting that exactly one status-reporting
  request reaches the wire, plus a bidirectional
  `Capabilities().CommitStatuses` consistency check.
- **Replaced `gitcode_api` with `go-gitcode` v0.7.2** (module rename of the
  same SDK). The GitCode backend now imports
  `github.com/yi-nology/go-gitcode` instead of the deprecated
  `github.com/yi-nology/gitcode_api`. The new module's API is a strict
  superset (adds private-deployment support: `SetBaseURL`/`BaseURL`,
  `NewClientFromEnv`, OAuth with custom base URL), so no behavior changes.
  The backend constructor was also simplified to always construct the client
  via `NewClientWithBaseURL` with a single resolved base URL.

## [v0.44.0] - 2026-08-24

### Fixed

- **GitLab `ListRepos` now scopes project listings to the token user.** Without
  an `Owner`, the backend called `GET /projects` with no filters, which returns
  every project visible to the token on the instance (on public GitLab that is
  the entire instance; on self-hosted instances far more than the user's own
  repos), in unstable order. It now sends `membership=true` so only projects
  the user belongs to are listed.
- **Provider cache key now includes `SkipTLS`.** The Manager's cache key was
  `platform:baseURL:tokenHash`, so flipping a platform's skip-TLS setting kept
  hitting the cached strict-TLS client until process restart. TLS policy
  changes now take effect immediately.
- **gogit `Push` treats `already up-to-date` as idempotent success.** Re-pushing
  an unchanged ref returned go-git's `NoErrAlreadyUpToDate` sentinel, which
  surfaced as an error and made no-op syncs fail. It is now a success with an
  empty `PushedRefs`.

## [v0.43.0] - 2026-08-16

### Changed

- **Upgraded the GitCode backend to gitcode_api v0.7.0** (from v0.6.0) and
  unlocked the backend's last two registered stubs, both riding real
  endpoints the new SDK surface exposes — no behavior mappings or harness
  opt-outs involved. `GetArchive` rides
  `GetRepositoryArchive` (GET `/repos/{o}/{r}/archive/{archive}`): the
  provider's (ref, format) pair folds into GitCode's single `ref.ext` path
  segment — format `tar.gz` → `ref.tar.gz`, format `zip`/empty/anything
  else → `ref.zip` (default-zip semantics mirroring the gitee backend's
  format mapping, adapted to GitCode's extension-in-path scheme; gitee's
  zipball/tarball keyword URLs do not apply). `CreateCommitStatus` rides
  `CreateCommitStatus` (POST `/repos/{o}/{r}/statuses/{sha}`):
  `provider.CommitStatusOptions` map 1:1 onto the SDK options, with state
  passed through verbatim (GitCode expects the GitHub-shaped
  pending/success/error/failure verbs). Targeted wire tests pin both the
  request paths and the archive format mapping / status body keys.

## [v0.42.0] - 2026-08-16

### Added

- **Tencent Code 工蜂 now declares the optional `Reviews` capability.**
  `ReviewManager` is implemented over the gongfeng SDK's merge-request
  notes (`NotesService`): `CreateReview` posts an MR note carrying the
  review (`CreateMergeRequestNote`), `ListReviews` lists the MR's notes
  (`ListMergeRequestNotes`, dropping 工蜂's system bookkeeping notes), and
  `GetReview` fetches a single note by ID (`GetMergeRequestNote`) — the
  same collection end to end, so created reviews round-trip.
  Create verdicts map to the platform's `reviewer_state` verbs
  (`APPROVE`→`approved`, `REQUEST_CHANGES`→`change_required`); note IDs
  are the review identifiers end to end. Four registered limitations:
  review reads carry no verdict state — the note model has no state
  field, so `Review.State` is always `commented`; `CreateReview` ignores
  `Comments` and `CommitID` (a review note carries at most one inline
  position and is commit-less); `RequestReviewers` is a registered
  ignore — 工蜂's merge-request update surface takes no reviewers and the
  native invite endpoint addresses reviewers by numeric user IDs the
  username-based SDK cannot resolve; and `DismissReview` is a registered
  stub returning `provider.ErrNotImplemented` — 工蜂 exposes no review
  dismissal surface (the only registered stub among the optional-capability
  interfaces; the SDK's other stubs live on core interfaces).
- **The contract-test reviews suite learned two platform-declaration
  flags.** `ReviewsHarnessConfig.IgnoresDismissal` (asserts a registered
  `DismissReview` stub wraps `provider.ErrNotImplemented` and stays off
  the wire, instead of asserting a state-changing verb) and
  `ReviewsHarnessConfig.ListStateIsCommented` (asserts the registered
  commented read state of platforms whose review model carries no
  verdict). The single-review mock route also accepts note-style paths
  (`.../notes/{id}`) beside review-style ones (`.../reviews/{id}`). The
  five other review platforms are unaffected.

- **Tencent Code 工蜂 now declares the optional `Issues` capability.**
  `IssueManager` is implemented over the gongfeng SDK's GitLab-shaped
  issues and notes surfaces (all eleven methods: `ListIssues`, `GetIssue`,
  `CreateIssue`, `UpdateIssue`, `CloseIssue`, `ReopenIssue`,
  `ListIssueComments`, `CreateIssueComment`, `ListIssueLabels`,
  `AddIssueLabels`, `RemoveIssueLabel`). Issues are addressed by IID and
  carried as `Issue.Number`; state changes travel as the platform's
  `state_event` verbs (`close`/`reopen`), wire state `opened` normalizes
  to `open`, labels exchange as csv strings, and the milestone reference
  exchanges as the numeric milestone ID (`MilestoneRef.Number` on this
  platform). Four registered limitations: the `Assignees` fields on
  create/update are ignored and the `ListIssuesOptions.Assignee` filter is
  not carried — 工蜂's issue surfaces take assignee IDs, and resolving
  usernames to IDs requires the Users API; removing an issue's only label
  is a no-op — the empty label csv drops off the update body (`omitempty`),
  so the label stays; `Issue.WebURL` is always empty — the gongfeng
  issue model carries no web URL field; and `Issue.ClosedAt` is always
  nil — the model carries no closed-at field either.

- **Tencent Code 工蜂 now declares the optional `Labels` capability.**
  `LabelManager` is implemented over the gongfeng SDK's GitLab-shaped labels
  surface (all four methods: `ListLabels`, `CreateLabel`, `UpdateLabel`,
  `DeleteLabel`). 工蜂 addresses labels by name natively — the current name
  travels in the update/delete bodies — so no name→ID resolution scan is
  needed. Colors exchange in GitLab form with a leading `#` on the wire and
  normalize to bare 6-digit hex in the SDK. Registered limitation: the
  gongfeng label model carries no id field, so `Label.ID` stays zero on this
  platform (labels are name-addressed end to end).

### Changed

- **TencentCode's hand-built raw request paths now escape their variable
  segments.** Nine request paths that interpolate into the URL by hand —
  the `DiffManager` note delete and discussion create, the code-review
  changed-files read, and the six branch-protection endpoints on the
  extras surface — previously carried the owner/repo project path (and
  the note ID) raw, so `#`, `?`, `%`, or spaces in an owner, repo, or
  note ID corrupted the request URL. The whole project path now travels
  percent-encoded as `owner%2Frepo` — the same wire form the gongfeng
  SDK's typed methods have always produced (`parseID`) — and the escaping
  is pinned by a dedicated wire-shape test asserting both the encoded
  and the server-decoded path forms.

## [v0.41.0] - 2026-08-16

### ⚠️ Breaking changes

- **`ChangeRequestManager` and `DiffManager` methods now address change
  requests by string number.** The thirteen number-taking methods (`GetCR`,
  `MergeCR`, `CloseCR`, `ReopenCR`, `UpdateCR`, `UpdateCRLabels`,
  `ListCRComments`, `ListCRCommits`, `GetCRDiff`, `GetCRFiles`,
  `CreateNote`, `DeleteNote`, `CreateDiscussion`) changed `number int` →
  `number string`, closing out the entity-addressing alignment (same scheme
  as `IssueManager`/`ReviewManager`). All seven platforms parse internally
  and return a wrapped `invalid pull request number` error on non-numeric
  input. Pass `"1"` where `1` was passed before.
- **`ChangeRequest.Number` changed `int` → `string`.** The model field now
  carries the platform's change-request identifier as a string, mirroring
  `Issue.Number`; all backends' converters and webhook event constructors
  format it via `strconv`.

### Added

- **Tencent Code 工蜂 now declares the optional `Milestones` capability.**
  `MilestoneManager` is implemented over the gongfeng SDK's GitLab-shaped
  milestones surface (all five methods: `ListMilestones`, `GetMilestone`,
  `CreateMilestone`, `UpdateMilestone`, `DeleteMilestone`). Milestones are
  addressed by the gongfeng milestone ID (the same string-addressing scheme
  as the other ID-based platforms), wire state `active` normalizes to
  `open`, state changes travel as the platform's `state_event` verbs, and
  due dates exchange as date-only strings. Registered limitation:
  `ListMilestonesOptions.State` is ignored — gongfeng's list endpoint
  options expose pagination only.

### Changed

- **Tightened milestone-number error texts, GitCode raw-path escaping,
  and registered platform semantics.** The milestone number helpers on
  GitHub, Gitea, Forgejo, and GitCode now report `invalid milestone
  number` on non-numeric input (they previously reused the issue parser
  and mis-reported "invalid issue number"). GitCode's registered raw
  milestone create/update paths now escape the owner/repo path segments —
  `#`, `?`, `%`, or spaces previously corrupted the request URL (the
  transport client joins base URL and path by plain concatenation).
  TencentCode's `UpdateRelease` with nothing to carry (Body nil — the
  only field the platform's update surface accepts) short-circuits to the
  `GetReleaseByTag` result instead of PUTting an empty update body.
  Newly doc-registered platform semantics: Gitea/Forgejo release creates
  serialize `target_commitish: ""` when no target is set (the SDK field
  has no omitempty; the server reads it as the default branch); Gitee's
  `GetArchive` rides the raw transport client (the go-gitee SDK exposes
  no archive endpoint); Gitee enterprise workspaces' extra issue states
  (progressing, rejected, ...) pass through `Issue.State` as-is (an open
  string vocabulary, not a closed enum); and
  `SearchReposOptions.Sort`/`Order` are platform-vocabulary-dependent —
  gitea/forgejo reject unknown values with HTTP 422, and GitLab's search
  API exposes no sort/order at all (registered ignore).

### Tests

- **The reviews and search contract suites now pin the wire more
  tightly.** Reviews: harnesses declare `CreateEvent` — the exact wire
  value the platform's create must carry under the `event` key for the
  suite's APPROVE-verdict create ("APPROVE" on GitHub/GitCode,
  "APPROVED" on Gitea/Forgejo after their SDK translation) — and the
  suite asserts exact equality, so the two wire vocabularies can no
  longer pass for each other; GitLab's note-based create (no verdict key
  on the wire) skips only the event assertion via an empty value. Search:
  harnesses declare `ReposTotalCount`, and when a platform reports a
  server-side total (GitHub's `total_count`, set to 1 in its fixture) the
  suite asserts the returned total equals it exactly instead of the
  weaker total ≥ len(results) fallback. Harness docs caught up too: the
  milestones suite's ID-addressing platform list now includes Tencent
  Code (its seventh ID-based platform), and the issues suite's routing
  note acknowledges alphanumeric issue numbers (Gitee).

## [v0.40.0] - 2026-08-15

### ⚠️ Breaking changes

- **`IssueManager` methods now address issues by string number.** The eight
  number-taking methods (`GetIssue`, `UpdateIssue`, `CloseIssue`,
  `ReopenIssue`, `ListIssueComments`, `CreateIssueComment`, `AddIssueLabels`,
  `RemoveIssueLabel`) changed `number int` → `number string`, and
  `Issue.Number` changed `int` → `string`. Numeric platforms (GitHub, GitLab,
  Gitea, Forgejo, GitCode) parse internally and return a wrapped
  `invalid issue number` error on non-numeric input; Gitee's alphanumeric
  identifiers (e.g. "IAINVA") are now natively representable. Pass `"1"`
  where `1` was passed before.
- **`CreateIssueOptions.Milestone`/`UpdateIssueOptions.Milestone` changed
  `int` → `string`, and `MilestoneRef.Number` changed `int` → `string`.**
  The option carries the platform's milestone addressing identifier as a
  string (`""` = don't set on create / leave unchanged on update);
  `MilestoneRef.Number` is the same identifier as returned by the platform
  (milestone number on GitHub, milestone ID on GitLab, Gitea, Forgejo, and
  GitCode, milestone serial number on Gitee).
- **`SearchIssueResult.Number` changed `int` → `string`.** Search results
  now carry the platform's issue addressing identifier as a string, so
  they feed `GetIssue(number string)` directly (the same string-addressing
  scheme as `IssueManager`): numeric platforms return `"1"`, Gitee's
  alphanumeric identifiers (e.g. `"IAINVA"`) are natively representable.
  GitCode's search no longer round trips through `strconv.Atoi`.
- **`CreateReview` moved from `DiffManager` to the new optional
  `ReviewManager` capability interface, and `DiffManager` is slimmed to five
  methods** (`GetCRDiff`, `GetCRFiles`, `CreateNote`, `DeleteNote`,
  `CreateDiscussion`). `ReviewManager.CreateReview` keeps its shape except
  that the change request number is now addressed as `number string` (same
  string-addressing scheme as `IssueManager`). The synthetic
  `CreateReview` approximations on GitLab (note + discussions + commit
  status), Gitee (notes), TencentCode (discussions + note), and GitCode
  (review endpoint with inline-comment fallback) are gone with the
  migration: reviews are now a declared capability implemented against
  platforms that expose a real review API. Gitee is ruled out entirely —
  its API exposes only PR tester assignment, with no review
  list/create/dismiss endpoints — so it declares no Reviews capability
  (capability threshold, design spec §4.6). Route with
  `p.Capabilities().Reviews` and type-assert `provider.ReviewManager`
  instead of calling `DiffManager.CreateReview`.
- **`ReleaseManager` (composed into `provider.Provider`) gains three
  methods: `GetReleaseByTag`, `UpdateRelease`, and `DeleteRelease`**
  (releases are addressed by tag name across all three), plus the
  `UpdateReleaseOptions` type (`Name`/`Body`/`Draft`/`Prerelease`, nil =
  leave unchanged). The methods are implemented on all seven platform
  backends, but any external implementation of `ReleaseManager` (or of the
  full `Provider` interface) must add the three methods to keep compiling.

### Added

- **`SearchManager` implemented by GitHub, GitLab, Gitea, Forgejo, and
  Gitee**, joining GitCode: all six platforms now declare
  `Capabilities().Search` and implement the three-method interface
  (`SearchRepos`, `SearchIssues`, `SearchUsers`) against real endpoints —
  no registered stubs anywhere (3/3 methods real on every platform, well
  past the §4.6 capability threshold). Platform mappings: GitHub rides
  `SearchService` with `repo:`/`state:`/`is:issue` qualifiers built from
  the options (`is:issue` keeps pull requests out of issue results; repo
  search is global since `SearchReposOptions` carries no scoping) and
  reports the API's `total_count`; GitLab rides the typed search scopes
  (`projects`/`issues`/`users`), routing `SearchIssuesOptions.Repo` to the
  project-scoped issue search (its search API has no state/sort/order
  parameters — registered ignore); Gitea and Forgejo ride the keyword
  searches (`/repos/search`, `/repos/issues/search` restricted to real
  issues via the type filter, `/users/search`), routing
  `SearchIssuesOptions.Repo` to the SDK's `ListRepoIssues`
  (`/repos/{owner}/{repo}/issues`) so repo scoping is exact and server-side
  across pages — totals included — with a malformed repo string failing
  fast instead of silently matching nothing; Gitee rides
  `/v5/search/{repositories,issues,users}` with native repo/state
  parameters. A cross-platform search contract suite
  (`contracttest.RunSearchSuite`, auto-mounted via `Harness.Search` with
  the same bidirectional capability-drift checks as the labels/issues/
  reviews/milestones suites) verifies repo parsing (`full_name`
  `owner/repo`), issue parsing (title plus string number feeding
  `GetIssue(number string)`), user parsing (`login`), that the keyword
  reaches the wire, and that repo-scoped issue search takes a wire route
  reflecting the repo.
- **Tag-addressed release get/update/delete on every platform.** The core
  `ReleaseManager` interface gains `GetReleaseByTag`, `UpdateRelease`, and
  `DeleteRelease` plus `UpdateReleaseOptions` (`Name`/`Body`/`Draft`/
  `Prerelease`, nil = leave unchanged), implemented across all seven
  backends. GitLab and TencentCode are tag-native end to end; GitHub,
  Gitea, Forgejo, GitCode, and Gitee resolve tag→ID through the platform's
  exact single-release-by-tag endpoint before their ID-addressed
  update/delete calls (no list-scan window to bound, unlike name-keyed
  label resolution); Gitea/Forgejo/GitCode merge the current title/body
  into their non-pointer SDK update fields so nil options never clobber a
  release. Two registered platform-semantic registrations: GitLab releases
  have no draft/prerelease concepts, so `Draft`/`Prerelease` are ignored
  there (name/description carry); TencentCode's update surface accepts a
  description only, so `Name`/`Draft`/`Prerelease` are ignored there. The
  Gitee implementation rides the raw transport client throughout (same
  ledger as its other release methods — the SDK's Release model mis-types
  the live payload and its PATCH posts a mislabeled multipart body): get
  by tag via `GET /repos/{o}/{r}/releases/tags/{tag}`, then update/delete
  by the resolved ID. A cross-platform release contract suite
  (`contracttest.RunReleaseSuite`, auto-mounted via `Harness.Releases` for
  all seven platforms — mandatory, since `ReleaseManager` is a core
  interface with no capability drift to enforce) verifies by-tag parsing,
  the update wire body, and the delete verb.
- **New optional `ReviewManager` capability interface** with five methods:
  `CreateReview`, `ListReviews`, `GetReview`, `RequestReviewers`,
  `DismissReview` (change requests addressed by string number, individual
  reviews by numeric platform ID), plus the `Review` model and normalized
  `ReviewState` constants (`approved`, `changes_requested`, `commented`,
  `pending`). **The GitHub backend implements it and declares
  `Capabilities().Reviews`**, backed by go-github's
  `PullRequestsService` review endpoints; UPPERCASE wire states are
  normalized to the lowercase constants. A cross-platform reviews contract
  suite (`contracttest.RunReviewsSuite`, auto-mounted via
  `Harness.Reviews`) verifies list parsing/state normalization, single
  review fetch, create/request-reviewers wire bodies, and non-GET
  dismissal, with the same bidirectional capability-declaration drift
  checks as the labels and issues suites.

- **The GitLab backend implements `ReviewManager` and declares
  `Capabilities().Reviews`** via registered platform-semantic mappings
  (spec §4.6): GitLab has no per-review list, so `ListReviews`/`GetReview`
  ride `MergeRequestApprovalsService.GetApprovalState` and synthesize one
  summary `approved` review per approver found in `rules[].approved_by`
  (keyed by the MR IID because GitLab approvals expose no per-approval
  IDs; `GetReview` returns the first such review, `NotFound` when nobody
  has approved); `CreateReview` is comment-style — a merge-request note
  via `Notes.CreateMergeRequestNote` (`commented` state; inline comments
  and verdicts are not mapped); `DismissReview` maps to
  `UnapproveMergeRequest` (review ID and dismissal message have no GitLab
  equivalent); `RequestReviewers` is a registered ignore — GitLab's
  `reviewer_ids` need username→ID resolution the SDK surface does not
  offer (same class of limitation as issue `Assignees`), and the reviews
  contract suite gained an `IgnoresRequestReviewers` harness flag (in the
  spirit of `IgnoresListPagination`) asserting such registered ignores
  stay silent on the wire.

- **The Gitea and Forgejo backends implement `ReviewManager` and declare
  `Capabilities().Reviews`**, covering all five methods against real
  endpoints — zero registered stubs, so the spec's one-stub allowance went
  unused. `ListReviews`/`GetReview` ride `ListPullReviews`/`GetPullReview`;
  `CreateReview` is a single `CreatePullReview` call carrying the verdict in
  the `event` field (the server finalizes immediately — the two-step
  create-then-submit shape exists only for pending drafts and would double
  the round trips); `RequestReviewers` rides `CreateReviewRequests` and
  `DismissReview` rides `DismissPullReview` (both need server ≥1.14, and
  `DismissPullReview` was spike-verified real, clearing the matrix's last
  open verification cell for these platforms). One SDK-side behavior to
  know: the client-side validation rejects a review that carries neither a
  body nor inline comments unless the verdict is APPROVE, so
  `REQUEST_CHANGES`/`COMMENT` reviews need a body.

- **The GitCode backend implements `ReviewManager` and declares
  `Capabilities().Reviews`**, restoring the review capability dropped
  from `DiffManager` in this release: all five methods ride real
  gitcode_api@v0.6.0 endpoints — `ListReviews`/`GetReview` via
  `ListPullRequestReviews`/`GetPullRequestReview`,
  `CreateReview` via `CreatePullRequestReview` (body+event on the wire),
  `RequestReviewers` via the real `requested_reviewers` endpoint, and
  `DismissReview` via `DismissPullRequestReview`
  (PUT `.../reviews/{id}/dismissals`). `CreateReview` keeps the
  pre-slimming resilience behavior: inline comments post individually
  after the review itself, and if the review endpoint rejects the
  request the fallback path posts inline comments plus a plain note
  instead of failing. No registered stubs, no mappings, and no harness
  opt-outs — all five reviews contract subtests pass on the standard
  wire assertions.

- **The gitee backend now declares `Capabilities().Issues`.** The
  IssueManager implementation is fully migrated onto the go-gitee SDK —
  Gitee's alphanumeric issue numbers (e.g. "IAINVA") flow through the
  string-typed interface natively — with one registered raw detour: issue
  create keeps the raw transport client because the SDK's generated create
  call posts an unparseable multipart body (upstream codegen bug), so it
  uses Gitee's documented owner-scoped `POST /repos/{owner}/issues`
  endpoint. `MilestoneRef.Number` carries Gitee's milestone serial number,
  the identifier Gitee's issue write endpoints take. The
  `IssuesImplementedButUndeclared` contract-harness flag is gone; the
  Issues capability check is bidirectional for every platform again.
- **The gitee backend's webhook CRUD migrated to the go-gitee SDK** where
  the SDK is usable: `DeleteWebhook` rides the generated delete call.
  `CreateWebhook`/`ListWebhooks` keep registered raw detours (the SDK's
  create posts an unparseable multipart body; its Hook model mis-types the
  live wire's numeric id and boolean event flags as strings, and the
  generated list swallows the decode error into an empty result). Create
  now uses Gitee's documented vocabulary: the signing secret travels as
  `password` (the key Gitee HMACs into `X-Gitee-Token`) and event
  selections map onto Gitee's `*_events` booleans.

- **New optional `MilestoneManager` capability interface with five methods**
  (`ListMilestones`, `GetMilestone`, `CreateMilestone`, `UpdateMilestone`,
  `DeleteMilestone` — all milestone-addressed by `number string`, the same
  identifier `MilestoneRef.Number` and the new `Milestone.Number` carry:
  milestone number on GitHub, milestone ID on GitLab, Gitea, Forgejo, and
  GitCode, milestone serial number on Gitee), plus the `Milestone` model,
  normalized `MilestoneState` constants (`open`, `closed`), and
  `ListMilestonesOptions`/`CreateMilestoneOptions`/`UpdateMilestoneOptions`
  (nil = leave unchanged). A cross-platform milestones contract suite
  (`contracttest.RunMilestonesSuite`, auto-mounted via `Harness.Milestones`)
  verifies list parsing/normalization (identifier as string, state), the
  create/update wire bodies (POST/PATCH/PUT carrying the title), and a
  non-GET delete, with the same bidirectional capability-declaration drift
  checks as the labels, issues, and reviews suites. Six backends implement
  it and declare `Capabilities().Milestones`: GitHub through its SDK's
  number-addressed CRUD as-is; GitCode via the SDK's ID-addressed surface
  for list/get/delete, with a registered raw detour on create/update —
  gitcode_api's option structs marshal `due_on` without omitempty, so an
  SDK-ridden call without a due date would post `"due_on": ""` and clear
  GitCode's stored due date (the raw bodies carry exactly the fields the
  caller set); GitLab via the project
  `MilestonesService` with two registered vocabulary mappings (wire state
  `active` ↔ SDK `open`, so state changes travel as the `state_event`
  verbs `activate`/`close`; and a date-only `ISOTime` due date, so
  `DueOn`'s time-of-day is lost on GitLab's wire); Gitea and Forgejo via
  their ID-addressed milestone CRUD; and Gitee via the go-gitee SDK with
  registered raw detours on create/update — the generated Post/Patch
  milestone calls encode their parameters as form values under an
  `application/json` Content-Type (upstream prepareRequest bug, same
  family as the labels/issue/release detours), so those two methods post
  documented JSON bodies through the raw transport client while
  list/get/delete ride the SDK. TencentCode does not implement the
  interface (out of this release's designed milestone platform set);
  note for a future phase: the gongfeng SDK actually ships a complete
  GitLab-shaped `MilestonesService` (list/get/create/edit/delete), so the
  capability is implementable there if ever prioritized.

### Changed

- **The gitee backend is rebuilt on the go-gitee SDK**
  (`gitee.com/openeuler/go-gitee`): repos, branches, CRs, commits, diffs,
  files, labels, releases, webhooks, and issues now ride the generated
  client through the shared transport pipeline, replacing the hand-rolled
  REST plumbing (the backend's manual escapers `esc`/`escPath` were
  **retained**, not retired: they now also wrap every go-gitee call site,
  because the SDK interpolates path parameters into URLs without escaping
  them — `escape_test.go` pins this behavior). The SDK is pinned by
  pseudo-version (`v0.0.0-20251225091545-a0f78272dafc`, a commit-hash
  lock): upgrading it requires deliberate human review of the upstream
  changes it pulls in. Retry caveat: go-gitee internally retries 502
  responses up to 3× with fixed 1s/2s sleeps, so on a persistent 502 its
  internal retries multiply with the SDK's own transport retry pipeline
  (registered in code at the gitee client construction). Known gap:
  **Gitee's `ChangeRequest.Draft` is always
  `false`** — the live PR payload carries a `draft` boolean but the SDK's
  `PullRequest` model omits the field (upstream swagger omission); it
  returns to wire-accurate once go-gitee models the field. Where the
  generated client is unusable, methods keep doc-registered raw detours
  through the same transport pipeline. Write detours: repo create
  (`RepositoryPostParam` has no `default_branch` field), file
  create/update/delete (bracketed JSON keys, multipart posted as
  `application/json`, and query-param encodings the REST contract puts in
  the body, plus a `CommitContent` model that mis-types the response),
  issue create (unparseable multipart body), webhook create/list
  (multipart body; the `Hook` model mis-types the live wire), label
  update (multipart-as-JSON — label create and delete ride the SDK),
  milestone create/update (form values under a JSON Content-Type),
  release create/update/delete/get-by-tag (the `Release` model mis-types
  the live payload; the PATCH call posts multipart), and branch delete
  (no SDK method exists). Read detours: repo list (the generated list
  calls decode a single Project where the endpoint answers an array),
  commit get/list/compare (`RepoCommit`/`Compare` models type payload
  objects as plain strings), file content (`Content` model mis-types
  `size`/`_links`), label list (the generated list options carry no
  pagination), and tag/release lists (the same `Release` model defect).

### Security

- **Credential query parameters are now masked in transport logs.** The
  transport's round-tripper and retry logging redact `access_token`,
  `token`, and `private_token` values to `***` before logging request
  URLs. Gitee's query-string token auth is the motivating case: its
  `access_token` parameter previously appeared verbatim in logged URLs.
  Only the logged form is masked; the outgoing request keeps its real
  credential.

## [v0.39.0] - 2026-08-15

### ⚠️ Breaking changes

- **`Issue.Milestone` changed from `string` to `*MilestoneRef{Number, Title}`,
  and `CreateIssueOptions`/`UpdateIssueOptions` gained a `Milestone int`
  field (0 = unset/unchanged).** Consumers reading the milestone title now
  use `issue.Milestone.Title` with a nil check. `MilestoneRef.Number`
  carries the platform's addressing identifier (number on GitHub, ID on
  GitLab/Gitea/Forgejo/GitCode).

### Added

- **`IssueManager` implemented by GitHub, GitLab, Gitea, and Forgejo**,
  joining GitCode: all four now declare `Capabilities().Issues` and implement
  the full 11-method interface (list/get/create/update/close/reopen,
  comments, issue labels). Backends whose APIs diverge from the SDK's
  vocabulary translate internally — GitLab applies state changes via
  `state_event` verbs and label changes via the `add_labels`/`remove_labels`
  update options, while Gitea and Forgejo resolve label names to numeric IDs
  and backfill the title on issue edit (their edit API always serializes
  `Title`). GitLab ignores `Assignees` on issue create/update: its API takes
  assignee IDs, and resolving usernames to IDs requires the Users API (a
  future UserManager round). Gitee's implementation exists in code but is
  deliberately not declared — every current Gitee repo returns alphanumeric
  string issue numbers (e.g. "IAINVA"), which the int-typed `IssueManager`
  can neither decode nor address; it will be re-enabled after a planned
  string-identifier spike (see `backends/gitee/gitee.go`).
- **The issue-management contract suite is now auto-mounted from the main
  `contracttest.Harness` via the optional `Issues` field**, mirroring the
  labels suite: `Run` fails when a platform declares
  `Capabilities().Issues` without wiring the suite, or wires it without
  declaring, and the capability checks enforce that declarations match the
  implemented method set. Gitee's deliberate implemented-but-undeclared
  state is documented via the harness's `IssuesImplementedButUndeclared`
  field.

### Fixed

- **The gitee backend now maps the SDK `CRState` vocabulary to gitee's
  pull-list vocabulary and query-escapes the `state` parameter in
  `ListCRs`.** The SDK's `opened` previously went out raw where gitee
  expects `open`; an empty state defaults to `open` as before.
- **Gitea and Forgejo `ListIssues` now request only issues, not pull
  requests.** The list option's type filter was left unset, so real
  instances returned PRs mixed into issue lists.
- **GitLab `ListIssues` now maps the SDK `open` state filter to GitLab's
  `opened` vocabulary.** The SDK's `open` previously went out raw where
  GitLab's issues API expects `opened`; the inbound conversion already
  mapped back.

## [v0.38.1] - 2026-08-15

### Fixed

- **`provider.Wrap` no longer panics on non-struct error values.** The
  reflection-based status-code fallback in `httpStatusFromError` called
  `NumField` on non-struct kinds (e.g. `url.EscapeError`, a string kind) and
  panicked; it now falls through to message parsing.
- **`UpdateLabel`/`DeleteLabel` no longer falsely report NotFound for labels
  beyond the first page.** The GitLab, Gitea, and Forgejo name→ID resolution
  now scans labels with server-side pagination (100 per page, bounded to 50
  pages) instead of stopping after the first 100 labels.
- **The gitee backend now percent-encodes variable URL path segments**
  (owner, repo, branch, label name, sha, file path, ref). Names containing
  `#`, `?`, `%`, spaces, or non-ASCII characters previously corrupted or
  truncated the request URL.
- **The gitee backend now query-escapes string query values** on list
  endpoints (`source_branch`, `target_branch`, `sha`, `since`, `until`),
  matching the previously fixed `ref` parameter. Values containing `#`,
  `?`, spaces, or a `+` in timestamps previously corrupted the query string.
- **GitCode `UpdateLabel` now sends `#`-prefixed colors.** GitCode's label
  API uses `#`-prefixed colors (matching its create endpoint), but the update
  path forwarded the SDK's canonical `#`-free form, breaking color changes
  against the real API.

### Tests

- The label-management contract suite is now auto-mounted from the main
  `contracttest.Harness` via the optional `Labels` field: `Run` enforces that
  a platform declaring `Capabilities().Labels` wires the suite (and vice
  versa), replacing the six hand-written `TestX_LabelsContract` functions.

## [v0.38.0] - 2026-08-15

### ⚠️ Breaking changes

- **`IssueManager` and `SearchManager` removed from the `Provider` interface.**
  Only GitCode genuinely implemented these; the other six backends returned
  `ErrNotImplemented` stubs for all methods, which forced every platform to
  pretend it supported issues/search. These two interfaces still exist and are
  now **optional capability interfaces**: consumers type-assert against them.

  ```go
  p, _ := provider.NewProvider(cfg)
  if ism, ok := p.(provider.IssueManager); ok {
      _ = ism.ListIssues(ctx, opts)
  }
  ```

  The corresponding stub files (`issues.go` / `search.go`) were deleted from
  the github, gitlab, gitea, gitee, gitea, forgejo, and tencentcode backends.
  GitCode's real implementations are unchanged and still satisfy both
  optional interfaces.

- **Credential encryption format changed (argon2id + salt).** Ciphertexts
  produced by older builds of `pkg/credential` (single-iteration SHA-256 KDF,
  no salt) **can no longer be decrypted** and must be re-encrypted with the
  current key. New ciphertexts carry a version byte, a 16-byte random salt,
  and a 12-byte GCM nonce. See "Security" below for the rationale.

- **Contract test harness signature changed.**
  `contracttest.Harness.NewProvider` now takes a full `provider.Config`
  (`func(t *testing.T, cfg provider.Config) provider.Provider`) instead of a
  base URL, so the harness can inject a retry config. Backends' `contract_test.go`
  were updated accordingly.

- **`Provider` interface gains `Capabilities() CapabilitySet`.** Custom
  implementations of `provider.Provider` outside this module must add the
  method. All seven built-in backends declare their capabilities statically;
  the contract suite enforces that declarations match implementations.

### Security

- **Credential KDF upgraded to argon2id with a per-ciphertext random salt**
  (`pkg/credential/encrypt.go`). Previously a low-entropy passphrase was
  stretched with a single unsalted SHA-256, making offline brute force trivial
  if a ciphertext was exfiltrated. argon2id (t=3, m=64 MiB, p=2) makes such
  attacks expensive. (Clean break — see breaking changes.)
- **`transport` retry no longer returns a closed/empty response body.**
  `retryingRoundTripper` now buffers the final upstream response body and
  re-attaches a fresh reader, so SDK decoders sitting on top see the real error
  payload instead of a misleading `EOF` / "read on closed body" (and the
  `http.RoundTripper` contract — a received response returns a nil error — is
  honored).
- **`MaxBodySize` is now enforced on the retry path**, not only on the
  success path (`transport/retry.go`). A server returning a huge body on a
  retried 5xx can no longer bypass the cap.
- **HMAC webhook validators reject an empty secret** (`provider/webhook.go`),
  matching `StaticTokenValidator`. An empty HMAC key makes signature forgery
  trivial for predictable payloads (e.g. push events on public repos).
- **`BuildAuthURL` now percent-encodes credentials** via `net/url`
  (`pkg/credential/manager.go`). Previously a token or username containing
  `@`, `:`, `/`, etc. was interpolated raw, producing malformed/ambiguous URLs
  and risking the secret leaking into the wrong URL component.
- **SSH host-key helper is now concurrency-safe** (`pkg/credential/sshkey.go`):
  the `hostKeys` map is guarded by a mutex, and the trust-on-first-use insert
  is double-checked under the write lock. Also fixed: the key comparison used
  the struct marshaler (`ssh.Marshal`) which panicked on real keys; it now uses
  the canonical wire format (`key.Marshal()`).
- **Removed the `ssh-keygen` shell-out fallback** in
  `ExtractPublicKeyFromPrivateKey`. It passed the key passphrase as a command-
  line argument, leaking it to other local users via `ps` / `/proc`.

### Changed

- **`gitbackend.GitBackend` god-interface split into composed sub-interfaces**
  (`gitbackend/iface.go`): `CoreOps`, `BranchOps`, `StatusDiffOps`, `CommitOps`,
  `RemoteOps`, `TagOps`, `FileOps`, `StashOps`, `ConfigOps`, `AdvancedOps`.
  `GitBackend` is now the composition of all of them, so its method set and
  the public API are unchanged; consumers may now depend on a narrower
  sub-interface.
- **Backend plumbing de-duplicated.** The byte-identical `convertHooks`, the
  retry-config mapping (with its `MaxRetries+1` conversion), `chainTransport`,
  `httpTransport`, `toTransportLogger`, and base-URL helpers that were copied
  across all seven backends are now shared via the internal
  `backends/internal/backendutil` package.

### Tests

- The cross-platform contract suite now asserts real behavior: a configured
  retry recovers from a 503 (and the server observes >1 attempt), and each
  platform's registered webhook validator accepts a correct signature, rejects
  an empty secret, and (for HMAC validators) rejects a tampered body. The
  duplicated `newVersionProxy` helper was promoted to `contracttest.VersionProxy`.

### Added

- **Declarative capability introspection: `Provider.Capabilities()`.**
  Returns a static `CapabilitySet{Issues, Search, Labels, Milestones,
  Reviews}` so consumers can route on declared capabilities instead of
  probing with type assertions. No runtime detection is performed.
- **`LabelManager` optional interface (repository-level label CRUD).**
  `ListLabels` / `CreateLabel` / `UpdateLabel` / `DeleteLabel`, addressed by
  label name with colors canonicalized to 6-digit hex without `#`. Backends
  whose APIs address labels by numeric ID (GitLab, Gitea, Forgejo) resolve
  names internally. Implemented by GitHub, GitLab, Gitea, Forgejo, Gitee,
  and GitCode; a new cross-platform `RunLabelsSuite` contract suite enforces
  behavior. GitCode's label API has no description field, so
  `Label.Description` is always empty there.
- `LICENSE` (MIT), `CHANGELOG.md`, `CONTRIBUTING.md`, and an `examples/`
  directory.
