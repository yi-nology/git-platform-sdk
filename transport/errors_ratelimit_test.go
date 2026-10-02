package transport

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestNewStatusErrorWithHeadersParsesRateLimitHints(t *testing.T) {
	reset := time.Now().Add(90 * time.Second).Unix()
	h := http.Header{}
	h.Set("Retry-After", "30")
	h.Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

	err := NewStatusErrorWithHeaders("GET", "/x", http.StatusTooManyRequests, []byte(`{}`), h).(*Error)
	if !err.IsRateLimited() {
		t.Fatal("429 must be rate-limited")
	}
	if err.RetryAfter != 30*time.Second {
		t.Fatalf("RetryAfter = %v, want 30s", err.RetryAfter)
	}
	if err.ResetAt.Unix() != reset {
		t.Fatalf("ResetAt = %v, want unix %d", err.ResetAt, reset)
	}
	ra, at := err.RateLimitInfo()
	if ra != 30*time.Second || at.Unix() != reset {
		t.Fatalf("RateLimitInfo = %v/%v", ra, at)
	}
}

func TestNewStatusError403WithExhaustedQuotaIsRateLimited(t *testing.T) {
	h := http.Header{}
	h.Set("X-RateLimit-Remaining", "0")
	err := NewStatusErrorWithHeaders("GET", "/x", http.StatusForbidden, nil, h).(*Error)
	if !err.IsRateLimited() {
		t.Fatal("403 + X-RateLimit-Remaining: 0 must be rate-limited")
	}

	// Secondary limit / abuse detection: Retry-After alone, no quota header.
	h2 := http.Header{}
	h2.Set("Retry-After", "60")
	secondary := NewStatusErrorWithHeaders("GET", "/x", http.StatusForbidden, nil, h2).(*Error)
	if !secondary.IsRateLimited() {
		t.Fatal("403 + Retry-After (secondary limit) must be rate-limited")
	}
	if secondary.RetryAfter != 60*time.Second {
		t.Fatalf("RetryAfter = %v, want 60s", secondary.RetryAfter)
	}

	h.Set("X-RateLimit-Remaining", "12")
	if err := NewStatusErrorWithHeaders("GET", "/x", http.StatusForbidden, nil, h).(*Error); err.IsRateLimited() {
		t.Fatal("403 with remaining quota must not be rate-limited")
	}
	if err := NewStatusErrorWithHeaders("GET", "/x", http.StatusForbidden, nil, nil).(*Error); err.IsRateLimited() {
		t.Fatal("plain 403 must not be rate-limited")
	}
}

func TestNewStatusErrorPlainKeepsZeroHints(t *testing.T) {
	err := NewStatusError("GET", "/x", http.StatusInternalServerError, nil).(*Error)
	if err.RetryAfter != 0 || !err.ResetAt.IsZero() {
		t.Fatalf("hints = %v/%v, want zero", err.RetryAfter, err.ResetAt)
	}
	if err.IsRateLimited() {
		t.Fatal("500 must not be rate-limited")
	}
}

func TestClient429CarriesHintsThroughDo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"too fast"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, None{})
	_, err := c.Do(t.Context(), &Request{Method: "GET", Path: "/x"})
	if err == nil {
		t.Fatal("expected error")
	}
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("err type = %T", err)
	}
	if !e.IsRateLimited() || e.RetryAfter != 7*time.Second {
		t.Fatalf("err = %+v, want rate-limited with 7s RetryAfter", e)
	}
}
