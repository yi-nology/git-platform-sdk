package provider

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrNotFound           = errors.New("resource not found")
	ErrAuthentication     = errors.New("authentication failed")
	ErrRateLimited        = errors.New("rate limited")
	ErrForbidden          = errors.New("forbidden")
	ErrConflict           = errors.New("conflict")
	ErrNotImplemented     = errors.New("not implemented")
	ErrInvalidInput       = errors.New("invalid input")
	ErrWebhookValidation  = errors.New("webhook validation failed")
	ErrConnectionFailed   = errors.New("connection failed")
	ErrPlatformNotSupported = errors.New("platform not supported")
)

// ProviderError is a structured error from a provider operation.
type ProviderError struct {
	Platform   Platform
	StatusCode int
	Op         string // operation name, e.g., "ListRepos"
	Message    string
	Err        error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s %s: %v", e.Platform, e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s %s", e.Platform, e.Op, e.Message)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is matching.
func (e *ProviderError) Is(target error) bool {
	if e.Err != nil {
		return errors.Is(e.Err, target)
	}
	return false
}

// NewProviderError creates a ProviderError from an HTTP response.
func NewProviderError(platform Platform, op string, statusCode int, body string) *ProviderError {
	err := classifyStatusCode(statusCode)
	return &ProviderError{
		Platform:   platform,
		StatusCode: statusCode,
		Op:         op,
		Message:    body,
		Err:        err,
	}
}

// WrapProviderError wraps an existing error as a ProviderError.
func WrapProviderError(platform Platform, op string, err error) *ProviderError {
	return &ProviderError{
		Platform: platform,
		Op:       op,
		Message:  err.Error(),
		Err:      err,
	}
}

// classifyStatusCode maps HTTP status codes to sentinel errors.
func classifyStatusCode(statusCode int) error {
	switch {
	case statusCode == http.StatusNotFound:
		return ErrNotFound
	case statusCode == http.StatusUnauthorized:
		return ErrAuthentication
	case statusCode == http.StatusForbidden:
		return ErrForbidden
	case statusCode == http.StatusTooManyRequests:
		return ErrRateLimited
	case statusCode == http.StatusConflict:
		return ErrConflict
	case statusCode >= 500:
		return fmt.Errorf("server error (status %d)", statusCode)
	default:
		return fmt.Errorf("HTTP %d", statusCode)
	}
}

// IsNotFound checks if an error is a not-found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAuthentication checks if an error is an authentication error.
func IsAuthentication(err error) bool {
	return errors.Is(err, ErrAuthentication)
}

// IsRateLimited checks if an error is a rate-limit error.
func IsRateLimited(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// IsNotImplemented checks if an error is a not-implemented error.
func IsNotImplemented(err error) bool {
	return errors.Is(err, ErrNotImplemented)
}
