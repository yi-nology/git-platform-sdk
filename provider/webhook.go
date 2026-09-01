package provider

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
	"sync"
)

// WebhookValidator verifies the authenticity of an incoming webhook request.
// Implementations are stateless and safe for concurrent use.
//
// The signature scheme varies per platform:
//
//   - GitHub:    HMAC-SHA256 of the body, sent in X-Hub-Signature-256.
//   - GitLab:    static token compared in constant time against X-Gitlab-Token.
//   - Gitea /
//     Forgejo:   HMAC-SHA256 of the body, sent in X-Gitea-Signature.
//   - Gitee /
//     GitCode /
//     Tencent:   HMAC-SHA256 of the body, sent in X-Gitee-Token / X-Token.
type WebhookValidator interface {
	Name() string
	Validate(r *http.Request, body []byte, secret string) error
}

// ValidatorFunc adapts a plain function into a WebhookValidator.
type ValidatorFunc struct {
	N  string
	Fn func(r *http.Request, body []byte, secret string) error
}

// Name implements WebhookValidator.
func (v ValidatorFunc) Name() string { return v.N }

// Validate implements WebhookValidator.
func (v ValidatorFunc) Validate(r *http.Request, body []byte, secret string) error {
	return v.Fn(r, body, secret)
}

// WebhookValidatorRegistry indexes WebhookValidator implementations by
// platform. It is safe for concurrent use; registrations happen during
// package init and reads happen on the request path.
type WebhookValidatorRegistry struct {
	mu         sync.RWMutex
	validators map[Platform]WebhookValidator
}

// NewWebhookValidatorRegistry builds an empty registry.
func NewWebhookValidatorRegistry() *WebhookValidatorRegistry {
	return &WebhookValidatorRegistry{validators: map[Platform]WebhookValidator{}}
}

// Register associates a validator with a platform.
func (r *WebhookValidatorRegistry) Register(p Platform, v WebhookValidator) {
	if v == nil {
		return
	}
	r.mu.Lock()
	r.validators[p] = v
	r.mu.Unlock()
}

// Get returns the validator for the given platform, or nil if none is
// registered.
func (r *WebhookValidatorRegistry) Get(p Platform) WebhookValidator {
	r.mu.RLock()
	v := r.validators[p]
	r.mu.RUnlock()
	return v
}

// Validate looks up the validator for the given platform and runs it. A nil
// validator or missing platform produces ErrNotImplemented.
func (r *WebhookValidatorRegistry) Validate(p Platform, req *http.Request, body []byte, secret string) error {
	v := r.Get(p)
	if v == nil {
		return fmt.Errorf("%w: no webhook validator for %s", ErrNotImplemented, p)
	}
	return v.Validate(req, body, secret)
}

// defaultWebhookRegistry is populated by init functions in webhook_*.go.
var defaultWebhookRegistry = NewWebhookValidatorRegistry()

// DefaultWebhookRegistry returns the process-wide registry.
func DefaultWebhookRegistry() *WebhookValidatorRegistry { return defaultWebhookRegistry }

// HMACSHA256Validator verifies a "Header: sha256=<hex>" style signature
// using HMAC-SHA256 over the request body. The expected header is configurable
// so it works for GitHub (X-Hub-Signature-256), Gitea (X-Gitea-Signature),
// Gitee (X-Gitee-Token) and others.
type HMACSHA256Validator struct {
	Header string
}

// Name implements WebhookValidator.
func (HMACSHA256Validator) Name() string { return "hmac-sha256" }

// Validate implements WebhookValidator. The signature must be exactly
// "sha256=<hex>" or just "<hex>". Comparison is done in constant time.
func (h HMACSHA256Validator) Validate(r *http.Request, body []byte, secret string) error {
	if h.Header == "" {
		return errors.New("hmac-sha256 validator: empty header")
	}
	header := r.Header.Get(h.Header)
	if header == "" {
		return fmt.Errorf("%w: missing %s header", ErrWebhookValidation, h.Header)
	}
	if secret == "" {
		// An empty key makes HMAC verification trivially forgeable for
		// predictable payloads (e.g. push events on public repos), so refuse
		// it up front, mirroring StaticTokenValidator.
		return fmt.Errorf("%w: empty secret", ErrWebhookValidation)
	}
	expected, err := decodeSignature(header, "sha256")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrWebhookValidation, err)
	}
	actual := signBody(secret, body)
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return fmt.Errorf("%w: signature mismatch", ErrWebhookValidation)
	}
	return nil
}

// StaticTokenValidator compares a static token header against the configured
// secret in constant time. Used for GitLab's X-Gitlab-Token.
type StaticTokenValidator struct {
	Header string
}

// Name implements WebhookValidator.
func (StaticTokenValidator) Name() string { return "static-token" }

// Validate implements WebhookValidator.
func (s StaticTokenValidator) Validate(r *http.Request, body []byte, secret string) error {
	if s.Header == "" {
		return errors.New("static-token validator: empty header")
	}
	got := r.Header.Get(s.Header)
	if got == "" {
		return fmt.Errorf("%w: missing %s header", ErrWebhookValidation, s.Header)
	}
	if secret == "" {
		return fmt.Errorf("%w: empty secret", ErrWebhookValidation)
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
		return fmt.Errorf("%w: token mismatch", ErrWebhookValidation)
	}
	return nil
}

// HmacValidator is a lower-level helper that supports arbitrary hash
// algorithms. It is intended for use by platform implementations that need
// something other than SHA-256.
type HmacValidator struct {
	Header    string
	Algorithm string // "sha1", "sha256", "sha512"
	Prefix    string // expected signature prefix ("sha256=")
}

// Name implements WebhookValidator.
func (h *HmacValidator) Name() string { return "hmac-" + h.Algorithm }

// Validate implements WebhookValidator.
func (h *HmacValidator) Validate(r *http.Request, body []byte, secret string) error {
	header := r.Header.Get(h.Header)
	if header == "" {
		return fmt.Errorf("%w: missing %s header", ErrWebhookValidation, h.Header)
	}
	if secret == "" {
		return fmt.Errorf("%w: empty secret", ErrWebhookValidation)
	}
	algo := h.Algorithm
	if algo == "" {
		algo = "sha256"
	}
	expected, err := decodeSignature(header, h.Prefix)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrWebhookValidation, err)
	}
	actual := signBodyHash(secret, body, algo)
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return fmt.Errorf("%w: signature mismatch", ErrWebhookValidation)
	}
	return nil
}

// signBody returns the HMAC-SHA256 of body keyed by secret, as raw bytes.
func signBody(secret string, body []byte) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return mac.Sum(nil)
}

// signBodyHash is the generic form. It returns the raw digest so callers can
// compare with decoded signatures in constant time.
func signBodyHash(secret string, body []byte, algo string) []byte {
	var h hash.Hash
	switch strings.ToLower(algo) {
	case "sha1":
		h = hmac.New(sha1.New, []byte(secret))
	case "sha256":
		h = hmac.New(sha256.New, []byte(secret))
	case "sha512":
		h = hmac.New(sha512.New, []byte(secret))
	default:
		h = hmac.New(sha256.New, []byte(secret))
	}
	h.Write(body)
	return h.Sum(nil)
}

// decodeSignature strips an optional "<algo>=" prefix and returns the raw
// signature bytes. Both standard hex encoding and the GitHub-style prefixed
// form are supported.
func decodeSignature(header, expectedPrefix string) ([]byte, error) {
	header = strings.TrimSpace(header)
	if expectedPrefix != "" && strings.HasPrefix(header, expectedPrefix+"=") {
		header = strings.TrimPrefix(header, expectedPrefix+"=")
	}
	if i := strings.IndexByte(header, '='); i >= 0 {
		// Allow the header to carry a "algo=hex" prefix; ignore the algo
		// portion and decode the hex part.
		header = header[i+1:]
	}
	return hex.DecodeString(header)
}

// ReadAndRestoreBody reads the full body of r and replaces it with a fresh
// NopCloser so that downstream readers (signature verification, JSON
// decoding) can still consume it. Use it at the entry point of webhook
// handlers that need to inspect the raw bytes.
func ReadAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// Pre-registered validators for the platforms the SDK ships with. These are
// registered in init() below.
func init() {
	defaultWebhookRegistry.Register(PlatformGitHub, HMACSHA256Validator{Header: "X-Hub-Signature-256"})
	defaultWebhookRegistry.Register(PlatformGitea, HMACSHA256Validator{Header: "X-Gitea-Signature"})
	defaultWebhookRegistry.Register(PlatformForgejo, HMACSHA256Validator{Header: "X-Gitea-Signature"})
	defaultWebhookRegistry.Register(PlatformGitee, HMACSHA256Validator{Header: "X-Gitee-Token"})
	defaultWebhookRegistry.Register(PlatformGitCode, HMACSHA256Validator{Header: "X-Token"})
	defaultWebhookRegistry.Register(PlatformTencentCode, StaticTokenValidator{Header: "X-Token"})
	defaultWebhookRegistry.Register(PlatformGitLab, StaticTokenValidator{Header: "X-Gitlab-Token"})
}

// --- Canonical webhook event types ---
//
// These constants define the normalized event type vocabulary. Each backend's
// ParseWebhookEvent must produce these values so consumers can switch on event
// types without knowing the source platform.

const (
	// CR (change request / pull request / merge request) actions.
	CRActionOpened   = "opened"
	CRActionClosed   = "closed"
	CRActionMerged   = "merged"
	CRActionReopened = "reopened"
	CRActionUpdated  = "updated"

	// Event type prefixes.
	EventTypeCR      = "cr."
	EventTypePush    = "push"
	EventTypeTag     = "tag."
	EventTypeBranch  = "branch."
	EventTypeIssue   = "issue."
	EventTypeComment = "comment."
)

// NormalizeCRAction maps platform-specific PR/MR action strings to the
// canonical vocabulary. Each backend should call this instead of ad-hoc
// string mapping.
//
// Mappings:
//
//	GitHub:   "opened" → opened, "closed" (+merged) → merged/closed,
//	          "reopened" → reopened, "synchronize"/"edited" → updated
//	GitLab:   "open" → opened, "close" → closed, "merge" → merged,
//	          "reopen" → reopened, "update" → updated
//	Gitee:    "open" → opened, "close" (+merged check) → merged/closed
//	GitCode:  "opened" → opened, "closed" (+merged check) → merged/closed
//	Gitea:    "opened" → opened, "closed" (+merged check) → merged/closed
func NormalizeCRAction(action string, merged bool) string {
	switch strings.ToLower(action) {
	case "opened", "open":
		return CRActionOpened
	case "closed", "close":
		if merged {
			return CRActionMerged
		}
		return CRActionClosed
	case "merged", "merge":
		return CRActionMerged
	case "reopened", "reopen":
		return CRActionReopened
	case "synchronize", "sync", "edited", "edit", "update":
		return CRActionUpdated
	default:
		return action
	}
}

// NormalizeTagAction maps platform-specific tag event actions to the
// canonical vocabulary.
func NormalizeTagAction(action string) string {
	switch strings.ToLower(action) {
	case "push", "pushed", "created", "create":
		return "push"
	default:
		return action
	}
}

// NormalizeBranchAction maps platform-specific branch event actions.
func NormalizeBranchAction(action string) string {
	switch strings.ToLower(action) {
	case "created", "create":
		return "created"
	case "deleted", "delete":
		return "deleted"
	default:
		return action
	}
}

// NormalizeIssueAction maps platform-specific issue event actions.
func NormalizeIssueAction(action string) string {
	switch strings.ToLower(action) {
	case "opened", "open":
		return "opened"
	case "closed", "close":
		return "closed"
	case "reopened", "reopen":
		return "reopened"
	case "edited", "edit", "updated", "update":
		return "updated"
	default:
		return action
	}
}
