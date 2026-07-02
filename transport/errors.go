package transport

import (
	"fmt"
	"net/http"
)

// NewStatusError builds an error for an HTTP response with status >= 400. The
// returned error is an *Error with the request method, path, status code and
// response body captured for diagnostics.
func NewStatusError(method, path string, status int, body []byte) error {
	return &Error{
		Method:     method,
		Path:       path,
		StatusCode: status,
		Body:       body,
	}
}

// Error is the structured error returned by Client when a request completes
// with a non-2xx status code, or when the underlying transport fails. It
// implements errors.Is / errors.As so callers can branch on either the status
// class (via IsStatusClass) or the wrapped cause.
type Error struct {
	Method     string
	Path       string
	StatusCode int
	Body       []byte
	Cause      error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("transport: %s %s: %d: %v", e.Method, e.Path, e.StatusCode, e.Cause)
	}
	return fmt.Sprintf("transport: %s %s: %d: %s", e.Method, e.Path, e.StatusCode, truncate(e.Body, 200))
}

// Unwrap implements errors.Unwrap so callers can recover the underlying cause.
func (e *Error) Unwrap() error { return e.Cause }

// Is implements errors.Is. Two *Error values match when they share the same
// status code, which is the most useful predicate for callers that branch on
// error class. Identity comparison still works via errors.Is's default path
// (same pointer).
func (e *Error) Is(target error) bool {
	if t, ok := target.(*Error); ok {
		return e.StatusCode == t.StatusCode
	}
	return false
}

// IsStatus reports whether the error has the given status code.
func (e *Error) IsStatus(code int) bool { return e.StatusCode == code }

// IsStatusClass reports whether the error is in the given status class. For
// example IsStatusClass(http.StatusInternalServerError) reports 5xx.
func (e *Error) IsStatusClass(min int) bool { return e.StatusCode >= min && e.StatusCode < min+100 }

// IsClientError reports 4xx.
func (e *Error) IsClientError() bool { return e.IsStatusClass(http.StatusBadRequest) }

// IsServerError reports 5xx.
func (e *Error) IsServerError() bool { return e.IsStatusClass(http.StatusInternalServerError) }

// ErrEmptyResponse is returned by DoJSON when the server returned a 2xx with
// no body and the caller asked for a non-nil result.
var ErrEmptyResponse = fmt.Errorf("transport: empty response body")

// truncate returns s shortened to at most n bytes. It is used to keep error
// messages bounded when the response body is large.
func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}
