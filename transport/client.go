// Package transport provides a unified HTTP transport layer for the
// git-platform-sdk. It is the single entry point through which all platform
// implementations send requests, so cross-cutting concerns (authentication
// header injection, retry/backoff, request/response hooks, structured logging,
// body capture for retry) are implemented in exactly one place.
//
// The package is designed to work in two ways:
//
//  1. Direct usage via Client. Higher-level callers build a Request and pass it
//     to Client.Do, Client.DoJSON, or Client.DoRaw. The response body is
//     captured for the caller to consume.
//
//  2. Transparent integration with third-party SDKs via RoundTripper. The
//     RoundTripper returned by Client.RoundTripper is a standard
//     http.RoundTripper that performs auth/retry/hooks on every request made by
//     any client that builds on net/http. This is how go-github, gitlab
//     client-go, gitea-sdk, forgejo-sdk and gitcode_api are wired up so they
//     benefit from the same cross-cutting behavior without code duplication.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTimeout is the per-request HTTP timeout applied when the caller does
// not specify one explicitly.
const DefaultTimeout = 30 * time.Second

// AuthStrategy applies authentication credentials to an outbound request.
// Implementations must be safe to call concurrently from multiple goroutines.
type AuthStrategy interface {
	Apply(req *http.Request)
}

// AuthFunc adapts a plain function into an AuthStrategy.
type AuthFunc func(req *http.Request)

// Apply implements AuthStrategy.
func (f AuthFunc) Apply(req *http.Request) { f(req) }

// None is the no-op auth strategy.
type None struct{}

// Apply implements AuthStrategy.
func (None) Apply(*http.Request) {}

// BearerToken sets the standard "Authorization: Bearer <token>" header.
type BearerToken struct{ Token string }

// Apply implements AuthStrategy.
func (b BearerToken) Apply(req *http.Request) {
	if b.Token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+b.Token)
}

// PrivateToken sets the GitLab-style "PRIVATE-TOKEN" header.
type PrivateToken struct{ Token string }

// Apply implements AuthStrategy.
func (p PrivateToken) Apply(req *http.Request) {
	if p.Token == "" {
		return
	}
	req.Header.Set("PRIVATE-TOKEN", p.Token)
}

// TokenHeader sets the older "Authorization: token <token>" header used by
// some GitHub-compatible APIs.
type TokenHeader struct{ Token string }

// Apply implements AuthStrategy.
func (t TokenHeader) Apply(req *http.Request) {
	if t.Token == "" {
		return
	}
	req.Header.Set("Authorization", "token "+t.Token)
}

// StaticAuth lets callers inject an arbitrary header value.
type StaticAuth struct {
	Header string
	Value  string
}

// Apply implements AuthStrategy.
func (s StaticAuth) Apply(req *http.Request) {
	if s.Header == "" {
		return
	}
	req.Header.Set(s.Header, s.Value)
}

// Request describes a single HTTP call. It is intentionally a plain value type
// so callers can build and pass it around without lock-in to a fluent API.
type Request struct {
	// Method is the HTTP method (GET, POST, PUT, PATCH, DELETE, ...).
	Method string
	// Path is the request path relative to Client.BaseURL. It may include a
	// query string; in that case Query is ignored.
	Path string
	// Query holds additional query parameters. Entries with an empty value are
	// dropped.
	Query url.Values
	// Headers carries per-request headers. The auth strategy may overwrite
	// any of these.
	Headers http.Header
	// Body is the request payload. Supported types:
	//   - nil:           no body
	//   - []byte / *bytes.Buffer / *bytes.Reader / *strings.Reader: sent verbatim
	//   - io.Reader:     streamed as-is
	//   - any other:     JSON-encoded
	Body any
	// Result is the target for JSON decoding of the response body. When nil
	// (and StatusCode is not 204), the raw body is still available via
	// Response.Body.
	Result any
}

// Response is the result of a single HTTP call. The body has been read in
// full and is available to the caller. It is the caller's responsibility to
// cap the body size upstream (e.g. via a wrapping RoundTripper) when feeding
// untrusted responses.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// Client is the unified transport. It is safe for concurrent use after
// construction; do not mutate it after handing it out.
type Client struct {
	// BaseURL is the API root used to resolve Request.Path. It is trimmed of
	// trailing slashes on construction.
	BaseURL string
	// Auth is applied to every request. May be nil/None for unauthenticated
	// calls.
	Auth AuthStrategy
	// Retry controls exponential-backoff retry on transient failures. nil
	// disables retry entirely.
	Retry *RetryConfig
	// Hooks receives every request/response for observability. nil is fine.
	Hooks *Hooks
	// Logger receives structured log entries. nil falls back to noopLogger.
	Logger Logger
	// Timeout is the per-request timeout applied when ctx has no deadline. A
	// non-positive value falls back to DefaultTimeout.
	Timeout time.Duration
	// Transport is the underlying http.RoundTripper. nil falls back to
	// http.DefaultTransport.
	Transport http.RoundTripper
	// Limiter provides proactive rate limiting. nil disables rate limiting.
	Limiter *RateLimiter
}

// NewClient builds a Client with the given base URL and auth strategy. It is
// the only constructor that is expected to be used in production code; the
// zero value works but skips the baseURL trimming.
func NewClient(baseURL string, auth AuthStrategy) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Auth:    auth,
		Timeout: DefaultTimeout,
	}
}

// NewClientWithTransport builds a Client with a custom underlying transport.
// Useful for tests with httptest.Server backed transport, or for callers that
// need to inject TLS / connection-pool tuning.
func NewClientWithTransport(baseURL string, auth AuthStrategy, rt http.RoundTripper) *Client {
	c := NewClient(baseURL, auth)
	c.Transport = rt
	return c
}

// Do executes req and returns the captured response. It does NOT decode JSON
// into req.Result; use DoJSON for that. Use Do to inspect the raw body or
// status code manually.
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
	return c.do(ctx, req, false)
}

// DoJSON executes req and, when req.Result is non-nil and the response has a
// body, decodes the body as JSON into req.Result. It returns ErrEmptyResponse
// when the response has no body and req.Result is non-nil.
func (c *Client) DoJSON(ctx context.Context, req *Request) (*Response, error) {
	return c.do(ctx, req, true)
}

// DoRaw executes req and returns the response body as raw bytes. It is a
// convenience over Do for endpoints that return non-JSON payloads (archives,
// tarballs, plain text, ...).
func (c *Client) DoRaw(ctx context.Context, req *Request) ([]byte, error) {
	resp, err := c.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c *Client) do(ctx context.Context, req *Request, decode bool) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("transport: nil request")
	}
	if req.Method == "" {
		return nil, fmt.Errorf("transport: empty method")
	}

	// Proactive rate limiting: wait before sending the request.
	if c.Limiter != nil {
		c.Limiter.Wait()
	}

	httpReq, err := c.buildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	resp, body, err := c.roundTripWithRetry(ctx, httpReq)
	duration := time.Since(start)
	if err != nil {
		c.log().Error("transport request failed",
			"method", req.Method,
			"path", req.Path,
			"duration", duration,
			"err", err,
		)
		c.Hooks.ExecuteResponse(ctx, httpReq, nil, duration, err)
		return nil, err
	}

	c.log().Debug("transport request ok",
		"method", req.Method,
		"path", req.Path,
		"status", resp.StatusCode,
		"duration", duration,
		"body_size", len(body),
	)
	if len(body) > 0 {
		c.log().Debug("transport response body",
			"method", req.Method,
			"path", req.Path,
			"body", truncateForLog(body, 2048),
		)
	}

	if resp.StatusCode >= 400 {
		c.log().Warn("transport request error",
			"method", req.Method,
			"path", req.Path,
			"status", resp.StatusCode,
			"duration", duration,
		)
		c.Hooks.ExecuteResponse(ctx, httpReq, resp, duration, nil)
		return nil, NewStatusError(req.Method, req.Path, resp.StatusCode, body)
	}

	c.Hooks.ExecuteResponse(ctx, httpReq, resp, duration, nil)

	out := &Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
	}
	if !decode || req.Result == nil {
		return out, nil
	}
	if len(body) == 0 || resp.StatusCode == http.StatusNoContent {
		return out, ErrEmptyResponse
	}
	if err := json.Unmarshal(body, req.Result); err != nil {
		return out, fmt.Errorf("transport: decode response: %w", err)
	}
	return out, nil
}

func (c *Client) buildRequest(ctx context.Context, req *Request) (*http.Request, error) {
	bodyReader, contentType, err := encodeBody(req.Body)
	if err != nil {
		return nil, fmt.Errorf("transport: encode body: %w", err)
	}

	full := c.BaseURL + req.Path
	if i := strings.IndexByte(req.Path, '?'); i < 0 && len(req.Query) > 0 {
		full += "?" + req.Query.Encode()
	}
	httpReq, err := http.NewRequestWithContext(ctx, strings.ToUpper(req.Method), full, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("transport: build request: %w", err)
	}

	for k, vs := range req.Headers {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	if contentType != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", contentType)
	}
	if httpReq.Header.Get("Accept") == "" {
		httpReq.Header.Set("Accept", "application/json")
	}
	if c.Auth != nil {
		c.Auth.Apply(httpReq)
	}

	if err := c.Hooks.ExecuteRequest(ctx, httpReq); err != nil {
		return nil, err
	}
	return httpReq, nil
}

// roundTripRequest applies hooks/auth on a request that was not built by
// buildRequest. It is used by the round-tripper path so that third-party SDK
// requests still receive auth/hooks without going through buildRequest.
func (c *Client) roundTripRequest(req *http.Request) {
	if c.Auth != nil {
		c.Auth.Apply(req)
	}
	_ = c.Hooks.ExecuteRequest(req.Context(), req)
}

// encodeBody returns the io.Reader for the request body, the content-type
// header to set, and any encoding error. A nil body yields a nil reader and
// empty content-type.
func encodeBody(body any) (io.Reader, string, error) {
	switch b := body.(type) {
	case nil:
		return nil, "", nil
	case []byte:
		return bytes.NewReader(b), "application/octet-stream", nil
	case *bytes.Buffer:
		return b, "application/octet-stream", nil
	case *bytes.Reader:
		return b, "application/octet-stream", nil
	case *strings.Reader:
		return b, "application/octet-stream", nil
	case string:
		return strings.NewReader(b), "text/plain; charset=utf-8", nil
	case io.Reader:
		return b, "", nil
	default:
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}
		return bytes.NewReader(raw), "application/json", nil
	}
}

// Logger returns the configured logger or a noop logger when none was set.
func (c *Client) log() Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return NoopLogger()
}

// httpClient builds the per-call http.Client. A new client is created on every
// call so that the per-request timeout always applies. The cost is one
// allocation per request, which is acceptable for a high-level SDK.
func (c *Client) httpClient() *http.Client {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	transport := c.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// roundTripWithRetry executes the request through the configured http.Client
// and, when a retry config is present, retries on transient failures. The
// returned body has been fully read and the response is closed.
func (c *Client) roundTripWithRetry(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	client := c.httpClient()
	if c.Retry == nil || c.Retry.MaxAttempts <= 0 {
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		// Update rate limiter state from response headers.
		if c.Limiter != nil {
			c.Limiter.UpdateFromResponse(resp)
		}
		body, readErr := readAndClose(resp)
		return resp, body, readErr
	}
	resp, body, err := c.Retry.Do(ctx, client, req, c.log())
	// Update rate limiter state from the final response.
	if resp != nil && c.Limiter != nil {
		c.Limiter.UpdateFromResponse(resp)
	}
	return resp, body, err
}

// RoundTripper exposes the auth/hooks/logging of this Client as a standard
// http.RoundTripper so third-party SDK clients built on net/http benefit from
// the same pipeline. It does NOT apply retries; retries are request-scoped
// and belong to the call site (e.g. Client.Do). Use NewRetryingRoundTripper
// to wrap this with retry, if needed.
func (c *Client) RoundTripper() http.RoundTripper {
	return &clientRoundTripper{client: c}
}

// NewRetryingRoundTripper wraps rt in retry/backoff. The returned RoundTripper
// can be plugged into any http.Client; retries fire on 429, 5xx, and the
// configured retry list.
func (c *Client) NewRetryingRoundTripper() http.RoundTripper {
	return &retryingRoundTripper{inner: c.RoundTripper(), cfg: c.Retry, logger: c.log()}
}

type clientRoundTripper struct {
	client *Client
}

// RoundTrip implements http.RoundTripper.
func (rt *clientRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	// Proactive rate limiting.
	if rt.client.Limiter != nil {
		rt.client.Limiter.Wait()
	}
	rt.client.roundTripRequest(req)
	start := time.Now()
	resp, err := rt.client.httpClient().Transport.RoundTrip(req)
	duration := time.Since(start)
	// Update rate limiter state from response headers.
	if resp != nil && rt.client.Limiter != nil {
		rt.client.Limiter.UpdateFromResponse(resp)
	}
	rt.client.Hooks.ExecuteResponse(ctx, req, resp, duration, err)
	if err != nil {
		rt.client.log().Error("transport roundtrip failed",
			"method", req.Method,
			"url", req.URL.String(),
			"duration", duration,
			"err", err,
		)
		return nil, err
	}
	if resp.StatusCode >= 400 {
		rt.client.log().Warn("transport roundtrip error",
			"method", req.Method,
			"url", req.URL.String(),
			"status", resp.StatusCode,
			"duration", duration,
		)
	}
	return resp, nil
}

type retryingRoundTripper struct {
	inner  http.RoundTripper
	cfg    *RetryConfig
	logger Logger
}

// RoundTrip implements http.RoundTripper and re-issues the request on
// transient failures. The request body is buffered so it can be replayed.
func (rt *retryingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.cfg == nil || rt.cfg.MaxAttempts <= 0 {
		return rt.inner.RoundTrip(req)
	}

	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	var lastErr error
	var lastResp *http.Response
	for attempt := 1; attempt <= rt.cfg.MaxAttempts; attempt++ {
		if attempt > 1 {
			delay := rt.cfg.Backoff(attempt-1, lastResp)
			select {
			case <-req.Context().Done():
				if lastResp != nil {
					_ = lastResp.Body.Close()
				}
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
			// Reset body for retry
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				req.Body = body
			}
		}

		resp, err := rt.inner.RoundTrip(req)
		if err != nil {
			lastErr = err
			rt.logger.Warn("transport retry: network error",
				"method", req.Method,
				"url", req.URL.String(),
				"attempt", attempt,
				"err", err,
			)
			continue
		}
		if !rt.cfg.ShouldRetry(resp.StatusCode) {
			return resp, nil
		}
		lastResp = resp
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		rt.logger.Warn("transport retry: retryable status",
			"method", req.Method,
			"url", req.URL.String(),
			"status", resp.StatusCode,
			"attempt", attempt,
		)
		_ = resp.Body.Close()
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}

// readAndClose reads the full response body and closes it. The body is
// returned as a byte slice so it can be replayed by the caller.
func readAndClose(resp *http.Response) ([]byte, error) {
	if resp == nil {
		return nil, nil
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}

// truncateForLog returns a string representation of body, capped at maxLen
// bytes. Bodies exceeding the limit are truncated with "...".
func truncateForLog(body []byte, maxLen int) string {
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "... (truncated)"
}
