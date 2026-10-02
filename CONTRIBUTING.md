# Contributing to go-git-platform

Thanks for your interest in contributing! This document covers the basics.

## Prerequisites

- Go (see the version pinned in `go.mod`).
- [`golangci-lint`](https://golangci-lint.run/) (the CI pins v2.12.2).

## Development loop

Common tasks are wired through the `Makefile`:

```sh
make fmt      # gofmt + goimports
make vet      # go vet
make lint     # golangci-lint run ./...
make test     # go test -race -coverprofile=coverage.out ./...
make check    # vet + lint + test (the CI gate)
make cover    # print the coverage summary
```

Run `make check` before pushing — it mirrors what CI runs.

## Versioning & compatibility commitment

The project follows [SemVer](https://semver.org/spec/v2.0.0.html) with the
0.x convention: breaking changes may land in a minor release, but must be
called out in the CHANGELOG. What counts as what:

**Non-breaking (fine in any release)**

- Adding a field to an exported struct (`options.go` types, `Error`,
  `NormalizedEvent`, ...). Callers using unkeyed struct literals is not a
  supported pattern.
- Adding a new optional capability: a new `iface_*.go` interface plus a
  `CapabilitySet` field. Existing `Capabilities()` declarations compile
  unchanged because the zero value means "not implemented".
- Adding a package, function, or method (including new helper functions in
  `provider`).
- New toolsets/tools in the MCP module; new tool parameters are additive
  JSON fields.

**Breaking (minor release + CHANGELOG entry; avoid where possible)**

- Removing or renaming an exported identifier (`provider.ListAllPages`
  removal in v0.73.0 is the precedent).
- Adding a method to an interface others implement — every `Provider`
  implementor breaks. Route new surface through optional capability
  interfaces instead.
- Changing a function signature or the meaning of an existing value
  (e.g. webhook event-type vocabulary renames in v0.72.0).

When a release contains breaking changes, the CHANGELOG entry must start
with the migration note (see v0.72.0's `cr.note` → `comment.created` for
the shape).

## What we look for in changes

- **Tests.** Add or update tests for behavior changes. Backends share a
  cross-platform contract suite in `backends/contracttest/`; when you add a
  capability that should be consistent across platforms, extend that suite
  rather than only testing one backend.
- **Race safety.** The test suite runs with `-race`; new concurrency code must
  be safe under it (see `pkg/credential/sshkey.go` for the mutex + double-check
  pattern used for shared maps).
- **No secrets in code or logs.** Never pass secrets through `argv` (see the
  removed `ssh-keygen` fallback). Keep credentials out of log output.
- **Follow existing patterns.** The `provider` package defines the abstraction;
  each `backends/<platform>/` implements it. Shared plumbing lives in
  `backends/internal/backendutil` — reuse it instead of copy-pasting.

## Adding a new platform backend

1. Create `backends/<platform>/` with a `New(cfg provider.Config) (provider.Provider, error)`
   constructor. Use `backends/internal/backendutil` for HTTP client / retry /
   hooks wiring so you don't reimplement it.
2. Implement only the capability interfaces the platform actually supports.
   `IssueManager` and `SearchManager` are **optional** — leave them out unless
   the platform genuinely offers them; consumers type-assert for optional caps.
3. Register the platform with `provider.Register` in an `init()`, and add a
   blank import in `backends/all/all.go`.
4. Add a `contract_test.go` that wires `contracttest.Run` with a `Harness`
   (see any existing backend's contract test).
5. Add the platform constant to `provider/provider.go` and the README platform
   list.

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

## Breaking changes

This project follows SemVer. Breaking changes to the public API require a new
major version. When you make one, document it under a `### ⚠️ Breaking changes`
heading in `CHANGELOG.md` and explain the migration.

### What counts as breaking (v1.0 contract)

| Change | Version |
|---|---|
| Adding a field to a unified type (`ChangeRequest`, `Issue`, …) | minor (non-breaking — structs are append-only for consumers) |
| Adding a method to `provider.Provider` or a core sub-interface | **major** (breaks every implementor, in-repo and external) |
| Adding an optional capability interface (`ReviewManager`, …) | minor (additive; nothing implements it before) |
| Adding a method to an existing optional capability interface | **major** (breaks implementors of that capability) |
| Changing a sentinel error, normalizing a state vocabulary, or renaming an option field | **major** |
| Widening behavior inside an existing method (pagination completeness, tighter validation) | patch/minor — behavior locked by the contract suites, so a suite change accompanying it must explain why the old wire shape was wrong |
| Swapping a wrapped platform SDK when the unified surface is unchanged | minor (the divergence ledger + contract suites are the compatibility proof) |

Two repo invariants back this contract:

1. **The contract suites are the spec.** Every behavioral promise above is
   enforced by `backends/contracttest/`; a PR that changes suite assertions
   must justify the change as a bug fix in the commit message.
2. **The divergence ledger is the fine print.** Platform gaps are declared
   (`provider.Divergences()`), generated into `docs/divergence-ledger.md`,
   and asserted against behavior by the drift check — never documented
  -only.
