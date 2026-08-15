# Changelog

All notable changes to this project are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
  GitCode).
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
  platforms that expose a real review API. Route with
  `p.Capabilities().Reviews` and type-assert `provider.ReviewManager`
  instead of calling `DiffManager.CreateReview`.

### Added

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
