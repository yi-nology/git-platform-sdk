package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_ShouldRetry(t *testing.T) {
	rc := DefaultRetryConfig()
	cases := []struct {
		status int
		want   bool
	}{
		{200, false},
		{429, true},
		{500, true},
		{502, true},
		{599, true},
		{400, false},
		{404, false},
		{418, false},
	}
	for _, c := range cases {
		if got := rc.ShouldRetry(c.status); got != c.want {
			t.Errorf("status %d: got %v, want %v", c.status, got, c.want)
		}
	}
}

func TestRetry_CustomStatus(t *testing.T) {
	rc := RetryConfig{MaxAttempts: 1, Statuses: []int{418}}
	if !rc.ShouldRetry(418) {
		t.Error("expected custom 418 to be retried")
	}
	// 5xx is always retried by default
	if !rc.ShouldRetry(500) {
		t.Error("500 should still be retried (default 5xx behavior)")
	}
	if rc.ShouldRetry(404) {
		t.Error("404 should not be retried")
	}
}

func TestRetry_SucceedsOnFirstTry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Retry = &RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	resp, body, err := c.roundTripWithRetry(context.Background(), mustReq(t, srv.URL+"/x"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != "ok" {
		t.Errorf("expected ok, got %q", body)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 call, got %d", got)
	}
}

func TestRetry_RetriesOn500(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Retry = &RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	resp, body, err := c.roundTripWithRetry(context.Background(), mustReq(t, srv.URL+"/x"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 after retries, got %d", resp.StatusCode)
	}
	if string(body) != "ok" {
		t.Errorf("expected ok, got %q", body)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 calls, got %d", got)
	}
}

// TestRetry_ReplaysBody guards body replay across attempts. It uses PUT (an
// idempotent method) so the test keeps verifying the replay mechanism only:
// since the method-aware retry fix, a POST would no longer be retried on a
// retryable status (see TestRetry_PostNotRetriedOnRetryableStatus).
func TestRetry_ReplaysBody(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Retry = &RetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/x", bytes.NewReader([]byte(`{"k":"v"}`)))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = c.roundTripWithRetry(context.Background(), req)
	if len(bodies) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(bodies))
	}
	for i, b := range bodies {
		if b != `{"k":"v"}` {
			t.Errorf("attempt %d: expected body to be replayed, got %q", i, b)
		}
	}
}

// TestRetry_HonorsRetryAfterCappedByMaxDelay verifies the Retry-After cap: a
// server-specified delay takes precedence over the backoff calculation, but is
// clamped to MaxDelay so e.g. "Retry-After: 86400" cannot suspend the caller
// for a day. (Before the fix the parsed value was returned unbounded.)
func TestRetry_HonorsRetryAfterCappedByMaxDelay(t *testing.T) {
	rc := RetryConfig{MaxAttempts: 2, BaseDelay: 10 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "1")
	delay := rc.Backoff(1, resp)
	if delay != 10*time.Millisecond {
		t.Errorf("expected Retry-After capped to MaxDelay (10ms), got %v", delay)
	}
}

func TestRetry_ContextCancelAbortsBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Retry = &RetryConfig{MaxAttempts: 5, BaseDelay: 200 * time.Millisecond, MaxDelay: time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _, err := c.roundTripWithRetry(ctx, mustReq(t, srv.URL+"/x"))
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRetry_ExhaustsAttempts(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Retry = &RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	resp, _, err := c.roundTripWithRetry(context.Background(), mustReq(t, srv.URL+"/x"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected last 502, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 calls, got %d", got)
	}
}

func mustReq(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// --- Fix: Retry-After must be capped and must support HTTP-date form ---

func TestRetry_RetryAfterCappedAtMaxDelay(t *testing.T) {
	rc := DefaultRetryConfig() // MaxDelay defaults to 30s
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "86400") // one full day
	delay := rc.Backoff(1, resp)
	if delay > 30*time.Second {
		t.Errorf("expected Retry-After capped to 30s, got %v", delay)
	}
	if delay != 30*time.Second {
		t.Errorf("expected exactly the 30s cap, got %v", delay)
	}
}

func TestRetry_RetryAfterHTTPDate(t *testing.T) {
	rc := RetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Second}
	resp := &http.Response{Header: http.Header{}}
	// Far-future HTTP-date must be parsed and capped.
	resp.Header.Set("Retry-After", time.Now().Add(90*time.Second).UTC().Format(http.TimeFormat))
	if delay := rc.Backoff(1, resp); delay != 2*time.Second {
		t.Errorf("expected HTTP-date Retry-After capped to 2s, got %v", delay)
	}

	// A near-future HTTP-date is honored (not capped): formatted dates have
	// second granularity, so a date built from now+2s truncated to the second
	// is 1s..2s away.
	rc2 := RetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 30 * time.Second}
	resp2 := &http.Response{Header: http.Header{}}
	resp2.Header.Set("Retry-After", time.Now().Add(2*time.Second).Truncate(time.Second).UTC().Format(http.TimeFormat))
	delay := rc2.Backoff(1, resp2)
	if delay < 900*time.Millisecond || delay > 2*time.Second {
		t.Errorf("expected ~1-2s delay from HTTP-date, got %v", delay)
	}

	// A past HTTP-date means "no wait".
	resp.Header.Set("Retry-After", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat))
	if delay := rc.Backoff(1, resp); delay != 0 {
		t.Errorf("expected zero delay for past HTTP-date, got %v", delay)
	}

	// An unparseable value falls back to the exponential backoff (bounded).
	resp.Header.Set("Retry-After", "soon")
	if delay := rc.Backoff(1, resp); delay > rc.MaxDelay {
		t.Errorf("expected bounded exponential fallback, got %v", delay)
	}
}

// --- Fix: retries must be method-aware ---

func TestRetry_IdempotentMethodSet(t *testing.T) {
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions} {
		if !isIdempotentMethod(m) {
			t.Errorf("expected %s to be idempotent", m)
		}
		if !isIdempotentMethod(strings.ToLower(m)) {
			t.Errorf("expected method match to be case-insensitive for %s", m)
		}
	}
	for _, m := range []string{http.MethodPost, http.MethodPatch, http.MethodConnect, http.MethodTrace, ""} {
		if isIdempotentMethod(m) {
			t.Errorf("expected %q NOT to be idempotent", m)
		}
	}
}

func TestRetry_RequestNotSentClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("boom"), false},
		{"dial error", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, true},
		{"wrapped dial error", fmt.Errorf("Get %q: %w", "http://x", &net.OpError{Op: "dial", Err: errors.New("refused")}), true},
		{"dns error", &net.DNSError{Err: "no such host", Name: "api.example.com"}, true},
		// A read/write error means bytes already went out: ambiguous for writes.
		{"read error after send", &net.OpError{Op: "read", Err: errors.New("connection reset by peer")}, false},
	}
	for _, tc := range cases {
		if got := requestNotSent(tc.err); got != tc.want {
			t.Errorf("%s: requestNotSent = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestRetry_PostNotRetriedOnRetryableStatus(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Retry = &RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/items", strings.NewReader(`{"k":"v"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, _, err := c.roundTripWithRetry(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("POST executed-but-502 must not be replayed: expected 1 attempt, got %d", got)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected the 502 response to be returned, got %d", resp.StatusCode)
	}
}

func TestRetry_GetRetriedOnRetryableStatus(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Retry = &RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	_, _, err := c.roundTripWithRetry(context.Background(), mustReq(t, srv.URL+"/x"))
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("GET is idempotent: expected 3 attempts, got %d", got)
	}
}

func TestRetry_RetryWriteOptsStatusRetriesForPost(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Retry = &RetryConfig{
		MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond,
		RetryWrite: true, // explicit opt-in to write retries
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/items", strings.NewReader(`{"k":"v"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.roundTripWithRetry(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("RetryWrite=true: expected POST to be retried 3 times, got %d attempts", got)
	}
}

func TestRetry_PostRetriedOnlyWhenRequestNotSent(t *testing.T) {
	dialErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	ambiguousErr := errors.New("read: connection reset by peer")

	run := func(method string, err error) int {
		var calls int32
		inner := roundTripFunc(func(*http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			return nil, err
		})
		rt := &retryingRoundTripper{
			inner:  inner,
			cfg:    &RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond},
			logger: NoopLogger(),
		}
		req := httptest.NewRequest(method, "http://example.com/x", nil)
		_, _ = rt.RoundTrip(req)
		return int(atomic.LoadInt32(&calls))
	}

	if got := run(http.MethodPost, dialErr); got != 3 {
		t.Errorf("POST with provably-unsent error should retry 3x, got %d attempts", got)
	}
	if got := run(http.MethodPost, ambiguousErr); got != 1 {
		t.Errorf("POST with ambiguous error must not retry, got %d attempts", got)
	}
	if got := run(http.MethodGet, ambiguousErr); got != 3 {
		t.Errorf("GET with ambiguous error should retry 3x, got %d attempts", got)
	}
	if got := run(http.MethodDelete, ambiguousErr); got != 3 {
		t.Errorf("DELETE is idempotent: expected 3 attempts, got %d", got)
	}
}

func TestRetryingRoundTripper_PostNotRetriedOn502(t *testing.T) {
	var calls int32
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader(`{"message":"bad gateway"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})
	rt := &retryingRoundTripper{
		inner:  inner,
		cfg:    &RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond},
		logger: NoopLogger(),
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.com/items", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("a received response is not a transport error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("POST must not be replayed on 502: expected 1 attempt, got %d", got)
	}
	if resp == nil || resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected the 502 response, got %+v", resp)
	}
}

func TestRetryingRoundTripper_RetryWriteRetriesPost(t *testing.T) {
	var calls int32
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("unavailable")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})
	rt := &retryingRoundTripper{
		inner:  inner,
		cfg:    &RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, RetryWrite: true},
		logger: NoopLogger(),
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.com/items", nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("RetryWrite=true: expected 3 attempts, got %d", got)
	}
}

func TestRetry_DefaultConfigDisablesWriteRetries(t *testing.T) {
	rc := DefaultRetryConfig()
	if rc.RetryWrite {
		t.Error("RetryWrite must default to false: replaying writes is unsafe by default")
	}
}
