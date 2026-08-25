# Contributing to git-platform-sdk

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
