package transport

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestRetry_ReplaysBody(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Retry = &RetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/x", bytes.NewReader([]byte(`{"k":"v"}`)))
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

func TestRetry_HonorsRetryAfter(t *testing.T) {
	rc := RetryConfig{MaxAttempts: 2, BaseDelay: 10 * time.Millisecond, MaxDelay: 10 * time.Millisecond}
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "1")
	delay := rc.Backoff(1, resp)
	if delay < 900*time.Millisecond || delay > 1100*time.Millisecond {
		t.Errorf("expected ~1s delay, got %v", delay)
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
