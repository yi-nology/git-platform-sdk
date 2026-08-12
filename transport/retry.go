package transport

import (
	"bytes"
	"context"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
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
// delay.
type RetryConfig struct {
	// MaxAttempts is the total number of attempts (including the first).
	// <= 0 disables retry.
	MaxAttempts int
	// BaseDelay is the initial backoff delay. Defaults to 500ms when <= 0.
	BaseDelay time.Duration
	// MaxDelay caps the backoff delay. Defaults to 30s when <= 0.
	MaxDelay time.Duration
	// Statuses lists extra status codes that should trigger a retry, in
	// addition to the default 429 and 5xx.
	Statuses []int
}

// DefaultRetryConfig returns a sane default: 3 attempts, 500ms base, 30s cap.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}
}

// ShouldRetry reports whether the given status code should trigger a retry.
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

// Backoff returns the delay to wait before the given attempt (1-indexed, so
// Backoff(1) is the delay before the first retry). The Retry-After header on
// resp, if set, takes precedence.
func (rc *RetryConfig) Backoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if d, err := time.ParseDuration(ra + "s"); err == nil {
				return d
			}
			if secs, err := strconv.Atoi(ra); err == nil {
				return time.Duration(secs) * time.Second
			}
		}
	}

	base := rc.BaseDelay
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	maxd := rc.MaxDelay
	if maxd <= 0 {
		maxd = 30 * time.Second
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
				"url", req.URL.String(),
				"attempt", attempt,
				"err", err,
			)
			continue
		}
		body, readErr := readAndClose(resp, maxBodySize)
		if readErr != nil {
			return nil, nil, readErr
		}
		lastResp = resp
		lastBody = body
		if !rc.ShouldRetry(resp.StatusCode) {
			resp.Body = io.NopCloser(bytes.NewReader(body))
			return resp, body, nil
		}
		logger.Warn("transport retry: retryable status",
			"method", req.Method,
			"url", req.URL.String(),
			"status", resp.StatusCode,
			"attempt", attempt,
		)
	}

	if lastResp != nil {
		lastResp.Body = io.NopCloser(bytes.NewReader(lastBody))
	}
	return lastResp, lastBody, lastErr
}
