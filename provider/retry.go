package provider

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// RetryConfig controls automatic retry behavior for HTTP requests.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	RetryOn    []int // HTTP status codes to retry on (in addition to 429 and 5xx)
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   30 * time.Second,
		RetryOn:    []int{},
	}
}

// shouldRetry determines if a request should be retried based on the response.
func (rc *RetryConfig) shouldRetry(statusCode int) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode >= 500 {
		return true
	}
	for _, code := range rc.RetryOn {
		if statusCode == code {
			return true
		}
	}
	return false
}

// retryDelay calculates the delay for a given attempt using exponential backoff with jitter.
func (rc *RetryConfig) retryDelay(attempt int, resp *http.Response) time.Duration {
	// Check Retry-After header first
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if d, err := time.ParseDuration(ra + "s"); err == nil {
				return d
			}
		}
	}

	delay := float64(rc.BaseDelay) * math.Pow(2, float64(attempt))
	if delay > float64(rc.MaxDelay) {
		delay = float64(rc.MaxDelay)
	}
	// Add jitter: 75%-125% of calculated delay
	jitter := 0.75 + rand.Float64()*0.5
	return time.Duration(delay * jitter)
}

// isRetryableMethod checks if the HTTP method is safe to retry.
func isRetryableMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

// retryableDo executes an HTTP request with retry logic.
// It clones the request body for each attempt so POST/PUT can be retried.
func (bp *baseProvider) retryableDo(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	rc := bp.retryConfig
	if rc == nil || rc.MaxRetries <= 0 {
		resp, err := bp.client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		body, _ := readAndRestoreBody(resp)
		return resp, body, nil
	}

	var lastErr error
	var lastResp *http.Response
	var lastBody []byte

	for attempt := 0; attempt <= rc.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := rc.retryDelay(attempt-1, lastResp)
			select {
			case <-ctx.Done():
				return lastResp, lastBody, ctx.Err()
			case <-time.After(delay):
			}
			// Re-create the request for retry
			req = req.Clone(ctx)
		}

		resp, err := bp.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, _ := readAndRestoreBody(resp)
		lastResp = resp
		lastBody = body

		if !rc.shouldRetry(resp.StatusCode) || !isRetryableMethod(req.Method) {
			return resp, body, nil
		}

		lastErr = fmt.Errorf("HTTP %d (attempt %d/%d)", resp.StatusCode, attempt+1, rc.MaxRetries+1)

		if bp.logger != nil {
			bp.logger.Warn("retrying request",
				"method", req.Method,
				"url", req.URL.String(),
				"status", resp.StatusCode,
				"attempt", attempt+1,
				"max_attempts", rc.MaxRetries+1,
			)
		}
	}

	return lastResp, lastBody, lastErr
}
