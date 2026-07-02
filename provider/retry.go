package provider

import "time"

// RetryConfig controls automatic retry behavior for HTTP requests issued by
// the transport layer. It is the public-facing configuration type passed
// via provider.Config; the transport package has its own internal
// transport.RetryConfig that this is mapped into.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (not counting the
	// initial request). <= 0 disables retry.
	MaxRetries int
	// BaseDelay is the initial backoff delay before the first retry.
	BaseDelay time.Duration
	// MaxDelay caps the backoff delay. Zero means 30s (the transport
	// default).
	MaxDelay time.Duration
	// RetryOn lists extra HTTP status codes to retry on, in addition to the
	// transport's default set (429 and 5xx).
	RetryOn []int
}

// DefaultRetryConfig returns a sensible default retry configuration: 3
// retries, 500ms base delay, 30s cap, no extra status codes.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  500 * time.Millisecond,
		MaxDelay:   30 * time.Second,
		RetryOn:    []int{},
	}
}
