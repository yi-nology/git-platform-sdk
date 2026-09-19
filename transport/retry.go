package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryConfig controls exponential-backoff retry on transient HTTP failures.
//
// A request is retried when the response status is in {429, 5xx} or matches
// one of the Statuses entries. The delay between attempts is
//
//	delay = min(BaseDelay * 2^(attempt-1), MaxDelay) * jitter
//
// where jitter is uniform in [0.75, 1.25]. A Retry-After header on the
// response, if present and parseable, takes precedence over the calculated
// delay; it is clamped to MaxDelay so a hostile header cannot suspend the
// caller for an unbounded time.
//
// Retries are method-aware. Only idempotent methods
// {GET, HEAD, PUT, DELETE, OPTIONS} are retried on retryable statuses or
// ambiguous network errors. A non-idempotent request (e.g. POST) that already
// reached the server must not be replayed: a 502 after a successful create
// would duplicate the resource. Non-idempotent requests are therefore only
// retried when the error proves the request never went out on the wire (DNS
// resolution or dial failure). Set RetryWrite to opt in to status-based
// retries for writes.
type RetryConfig struct {
	// MaxAttempts is the total number of attempts (including the first).
	// <= 0 disables retry.
	MaxAttempts int
	// BaseDelay is the initial backoff delay. Defaults to 500ms when <= 0.
	BaseDelay time.Duration
	// MaxDelay caps the backoff delay. Defaults to 30s when <= 0. It also
	// caps any Retry-After header sent by the server.
	MaxDelay time.Duration
	// Statuses lists extra status codes that should trigger a retry, in
	// addition to the default 429 and 5xx.
	Statuses []int
	// RetryWrite re-enables status-based retries for non-idempotent methods
	// (POST, PATCH, ...). It is false by default because replaying a write
	// after "executed but failed" (e.g. 502 on resource creation) can
	// duplicate server-side effects. Set it only when the target APIs are
	// known to deduplicate writes or tolerate duplicates.
	RetryWrite bool
}

// DefaultRetryConfig returns a sane default: 3 attempts, 500ms base, 30s cap.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}
}

// idempotentMethods is the set of HTTP methods whose replay cannot duplicate
// a server-side effect, so they are always retry-safe.
var idempotentMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodPut:     true,
	http.MethodDelete:  true,
	http.MethodOptions: true,
}

// isIdempotentMethod reports whether method belongs to the retry-safe set.
func isIdempotentMethod(method string) bool {
	return idempotentMethods[strings.ToUpper(method)]
}

// requestNotSent reports whether err proves the request never reached the
// network. DNS resolution and dialing both happen before any request bytes
// are written, so failures there cannot have executed the operation
// server-side, and a replay is safe even for non-idempotent requests.
func requestNotSent(err error) bool {
	if err == nil {
		return false
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// methodRetryable reports whether requests with req's method are eligible for
// unconditional retries (idempotent methods, or writes when RetryWrite is on).
func (rc *RetryConfig) methodRetryable(req *http.Request) bool {
	if req == nil {
		return false
	}
	return isIdempotentMethod(req.Method) || rc.RetryWrite
}

// canRetryStatus reports whether a received response with the given status
// may trigger a retry for req. Both the status and the method must qualify:
// ShouldRetry alone is not sufficient, because a non-idempotent request that
// was already executed (it produced a response) must not be replayed.
func (rc *RetryConfig) canRetryStatus(req *http.Request, status int) bool {
	if !rc.ShouldRetry(status) {
		return false
	}
	return rc.methodRetryable(req)
}

// canRetryNetworkError reports whether a transport-level error may trigger a
// retry for req. Idempotent methods (or writes with RetryWrite) always retry;
// non-idempotent methods retry only when the error proves the request was
// never sent (DNS/dial failure), so the operation cannot have run remotely.
func (rc *RetryConfig) canRetryNetworkError(req *http.Request, err error) bool {
	if rc.methodRetryable(req) {
		return true
	}
	return requestNotSent(err)
}

// ShouldRetry reports whether the given status code should trigger a retry.
// This is the status-only predicate; call sites that know the request must
// additionally gate on the method via canRetryStatus.
func (rc *RetryConfig) ShouldRetry(status int) bool {
	if status == http.StatusTooManyRequests {
		return true
	}
	if status >= 500 && status <= 599 {
		return true
	}
	for _, s := range rc.Statuses {
		if s == status {
			return true
		}
	}
	return false
}

// resolvedMaxDelay returns the effective backoff cap: MaxDelay, or the 30s
// default when unset or non-positive.
func (rc *RetryConfig) resolvedMaxDelay() time.Duration {
	if rc.MaxDelay <= 0 {
		return 30 * time.Second
	}
	return rc.MaxDelay
}

// parseRetryAfter parses a Retry-After header value per RFC 9110 §10.2.3:
// either a non-negative integer number of seconds or an HTTP-date. It returns
// the delay measured from now and whether the value was parseable.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := t.Sub(now)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}

// Backoff returns the delay to wait before the given attempt (1-indexed, so
// Backoff(1) is the delay before the first retry). The Retry-After header on
// resp, if set and parseable, takes precedence — clamped to the resolved
// MaxDelay so an unbounded server-provided delay cannot hang the caller.
func (rc *RetryConfig) Backoff(attempt int, resp *http.Response) time.Duration {
	maxd := rc.resolvedMaxDelay()
	if resp != nil && resp.Header != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if d, ok := parseRetryAfter(ra, time.Now()); ok {
				return min(d, maxd)
			}
		}
	}

	base := rc.BaseDelay
	if base <= 0 {
		base = 500 * time.Millisecond
	}

	exp := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(base) * exp)
	if delay > maxd || delay < 0 {
		delay = maxd
	}
	// Jitter: uniform in [0.75, 1.25]
	jitter := 0.75 + rand.Float64()*0.5
	return time.Duration(float64(delay) * jitter)
}

// Do executes a single request through the underlying client, applying the
// configured retry policy. The response body is fully read so it can be
// replayed across attempts. The returned http.Response has a fresh body
// reader attached so the caller can read it once and then receive io.EOF.
//
// Retries respect the request method: see RetryConfig for the exact rules.
func (rc *RetryConfig) Do(ctx context.Context, client *http.Client, req *http.Request, logger Logger, maxBodySize int64) (*http.Response, []byte, error) {
	if rc == nil || rc.MaxAttempts <= 0 {
		resp, err := client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		body, readErr := readAndClose(resp, maxBodySize)
		return resp, body, readErr
	}

	// Buffer the body so retries replay the same payload.
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, nil, err
		}
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	var lastErr error
	var lastResp *http.Response
	var lastBody []byte

	for attempt := 1; attempt <= rc.MaxAttempts; attempt++ {
		if attempt > 1 {
			delay := rc.Backoff(attempt-1, lastResp)
			select {
			case <-ctx.Done():
				if lastResp != nil {
					_ = lastResp.Body.Close()
				}
				return lastResp, lastBody, ctx.Err()
			case <-time.After(delay):
			}
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			logger.Warn("transport retry: network error",
				"method", req.Method,
				"url", redactURL(*req.URL),
				"attempt", attempt,
				"err", err,
			)
			if !rc.canRetryNetworkError(req, err) {
				// Non-idempotent request with an ambiguous or already-executed
				// outcome: replaying could duplicate a server-side effect.
				return nil, nil, err
			}
			continue
		}
		body, readErr := readAndClose(resp, maxBodySize)
		if readErr != nil {
			return nil, nil, readErr
		}
		lastResp = resp
		lastBody = body
		if !rc.canRetryStatus(req, resp.StatusCode) {
			resp.Body = io.NopCloser(bytes.NewReader(body))
			return resp, body, nil
		}
		logger.Warn("transport retry: retryable status",
			"method", req.Method,
			"url", redactURL(*req.URL),
			"status", resp.StatusCode,
			"attempt", attempt,
		)
	}

	if lastResp != nil {
		lastResp.Body = io.NopCloser(bytes.NewReader(lastBody))
	}
	return lastResp, lastBody, lastErr
}
