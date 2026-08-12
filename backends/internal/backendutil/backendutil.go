// Package backendutil holds the shared plumbing used by every platform backend
// under backends/. It exists to eliminate the copy-pasted HTTP-client, hook,
// retry, and base-URL construction that was previously duplicated (often
// byte-for-byte) across the seven backend packages.
//
// These helpers are intentionally internal: they are an implementation detail
// of the backends and must not become part of the SDK's public API.
package backendutil

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	"github.com/yi-nology/git-platform-sdk/provider"
	"github.com/yi-nology/git-platform-sdk/transport"
)

// ConvertHooks adapts provider.Hooks into transport.Hooks. The request-hook
// signatures differ across the two packages (provider.RequestHook returns a
// context; transport.RequestHook returns an error), so each request hook is
// wrapped to discard the returned context. Identical across all backends.
func ConvertHooks(h *provider.Hooks) *transport.Hooks {
	if h == nil {
		return nil
	}
	out := &transport.Hooks{}
	for _, rh := range h.Request {
		if rh == nil {
			continue
		}
		rhCopy := rh
		out.AddRequest(func(ctx context.Context, req *http.Request) error {
			_ = rhCopy(ctx, req)
			return nil
		})
	}
	for _, rh := range h.Response {
		if rh == nil {
			continue
		}
		rhCopy := rh
		out.AddResponse(func(ctx context.Context, req *http.Request, resp *http.Response, d time.Duration, err error) {
			rhCopy(ctx, req, resp, d, err)
		})
	}
	return out
}

// MapRetryConfig converts a provider.RetryConfig (which counts retries, i.e.
// attempts beyond the first) into a transport.RetryConfig (which counts total
// attempts), hence the +1. Returns nil for a nil input so callers can assign
// the result directly.
func MapRetryConfig(rc *provider.RetryConfig) *transport.RetryConfig {
	if rc == nil {
		return nil
	}
	return &transport.RetryConfig{
		MaxAttempts: rc.MaxRetries + 1,
		BaseDelay:   rc.BaseDelay,
		MaxDelay:    rc.MaxDelay,
		Statuses:    rc.RetryOn,
	}
}

// HTTPTransport returns an http.RoundTripper honouring the skipTLS flag: the
// default transport when verification is enabled, or a transport that disables
// certificate verification when the caller has explicitly opted into SkipTLS.
//
//nolint:gosec // G402: InsecureSkipVerify is only set when the caller opts in via SkipTLS.
func HTTPTransport(skipTLS bool) http.RoundTripper {
	if !skipTLS {
		return http.DefaultTransport
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

// ChainTransport chains two round trippers. The outer tripper is invoked first;
// if it returns a non-nil response or a non-nil error that result is returned,
// otherwise the inner transport (the underlying connection) handles the
// request.
func ChainTransport(inner, outer http.RoundTripper) http.RoundTripper {
	return &chainedTransport{inner: inner, outer: outer}
}

type chainedTransport struct {
	inner http.RoundTripper
	outer http.RoundTripper
}

func (c *chainedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if c.outer != nil {
		if resp, err := c.outer.RoundTrip(req); resp != nil || err != nil {
			return resp, err
		}
	}
	return c.inner.RoundTrip(req)
}

// ToTransportLogger adapts a provider.Logger to a transport.Logger.
func ToTransportLogger(l provider.Logger) transport.Logger {
	if l == nil {
		return nil
	}
	return transport.LoggerFunc{
		DebugFunc: func(msg string, kv ...any) { l.Debug(msg, kv...) },
		InfoFunc:  func(msg string, kv ...any) { l.Info(msg, kv...) },
		WarnFunc:  func(msg string, kv ...any) { l.Warn(msg, kv...) },
		ErrorFunc: func(msg string, kv ...any) { l.Error(msg, kv...) },
	}
}

// NormalizeBaseURL returns base with a single trailing slash removed and a
// trailing "/api/v1" suffix removed. Used by Gitea/Forgejo-style backends
// whose SDKs add the version prefix themselves.
func NormalizeBaseURL(base string) string {
	base = strings.TrimSuffix(base, "/")
	base = strings.TrimSuffix(base, "/api/v1")
	return base
}

// DefaultBaseURL returns base if non-empty, otherwise def.
func DefaultBaseURL(base, def string) string {
	if base == "" {
		return def
	}
	return base
}
