# Changelog

All notable changes to this project are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **`UpdateLabel`/`DeleteLabel` no longer falsely report NotFound for labels
  beyond the first page.** The GitLab, Gitea, and Forgejo name→ID resolution
  now scans labels with server-side pagination (100 per page, bounded to 50
  pages) instead of stopping after the first 100 labels.

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
