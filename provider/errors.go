package provider

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
)

// Sentinel errors classify transport and provider failures so callers can
// branch on category without inspecting status codes or wrapped causes.
var (
	ErrNotFound             = errors.New("resource not found")
	ErrAuthentication       = errors.New("authentication failed")
	ErrRateLimited          = errors.New("rate limited")
	ErrForbidden            = errors.New("forbidden")
	ErrConflict             = errors.New("conflict")
	ErrNotImplemented       = errors.New("not implemented")
	ErrInvalidInput         = errors.New("invalid input")
	ErrWebhookValidation    = errors.New("webhook validation failed")
	ErrConnectionFailed     = errors.New("connection failed")
	ErrPlatformNotSupported = errors.New("platform not supported")
)

// ProviderError is a structured error from a provider operation. It carries
// enough context (platform, op, resource, status code, cause) to support
// logging, retry, and user-facing messages. It implements errors.Is so the
// sentinel errors above are matched by Cause, and errors.As for typed access.
type ProviderError struct {
	Platform   Platform
	Op         string // operation name, e.g., "ListRepos"
	Resource   string // optional resource identifier, e.g., "owner/repo"
	StatusCode int    // 0 when not applicable (e.g. configuration errors)
	Cause      error  // underlying cause; nil when constructed from raw fields
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	prefix := e.Op
	if e.Resource != "" {
		prefix = e.Op + " " + e.Resource
	}
	if e.Platform != "" {
		prefix = string(e.Platform) + " " + prefix
	}
	if e.Cause != nil {
		if e.StatusCode != 0 {
			return fmt.Sprintf("%s: HTTP %d: %v", prefix, e.StatusCode, e.Cause)
		}
		return fmt.Sprintf("%s: %v", prefix, e.Cause)
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("%s: HTTP %d", prefix, e.StatusCode)
	}
	return prefix
}

// Unwrap implements errors.Unwrap.
func (e *ProviderError) Unwrap() error { return e.Cause }

// Is implements errors.Is. It matches when the wrapped Cause equals the
// target, which lets callers write `errors.Is(err, provider.ErrNotFound)`.
func (e *ProviderError) Is(target error) bool {
	if e.Cause == nil {
		return false
	}
	return errors.Is(e.Cause, target)
}

// IsStatus reports whether the error has the given HTTP status code.
func (e *ProviderError) IsStatus(code int) bool { return e.StatusCode == code }

// IsClientError reports 4xx.
func (e *ProviderError) IsClientError() bool { return e.StatusCode >= 400 && e.StatusCode < 500 }

// IsServerError reports 5xx.
func (e *ProviderError) IsServerError() bool { return e.StatusCode >= 500 && e.StatusCode < 600 }

// Wrap creates a ProviderError from a raw error, classifying it when the
// cause is a transport error with a known status code. Use this in platform
// implementations to convert transport errors into the unified shape.
func Wrap(platform Platform, op string, err error) error {
	if err == nil {
		return nil
	}
	if pe, ok := err.(*ProviderError); ok && pe.Platform == platform && pe.Op == op {
		return pe
	}
	pe := &ProviderError{Platform: platform, Op: op, Cause: err}
	// Walk the cause chain looking for an HTTP status code. The check order
	// prioritizes cheap interface checks before falling back to reflection.
	for cur := err; cur != nil; {
		// Fast path: statusCoder interface (covers transport.Error and StatusError).
		if sc, ok := cur.(statusCoder); ok {
			pe.StatusCode = sc.StatusCode()
			pe.Cause = classifyStatusCode(pe.StatusCode)
			break
		}
		// Fast path: explicit StatusError type.
		if se, ok := cur.(*StatusError); ok {
			pe.StatusCode = se.Status
			pe.Cause = classifyStatusCode(se.Status)
			break
		}
		// Slow path: reflection-based detection for third-party SDK errors.
		if code, ok := httpStatusFromError(cur); ok {
			pe.StatusCode = code
			pe.Cause = classifyStatusCode(code)
			break
		}
		cur = errors.Unwrap(cur)
	}
	return pe
}

// Wrapf is a convenience for creating a ProviderError with a formatted
// message. The format is intentionally simple ("%s/%s") so platform code can
// embed the resource identifier without re-implementing the prefix logic.
func Wrapf(platform Platform, op, format string, args ...any) error {
	cause := fmt.Errorf(format, args...)
	return &ProviderError{Platform: platform, Op: op, Cause: cause}
}

// New builds a ProviderError directly from the given status code.
func New(platform Platform, op string, status int, body string) error {
	pe := &ProviderError{
		Platform:   platform,
		Op:         op,
		StatusCode: status,
		Cause:      classifyStatusCode(status),
	}
	if body != "" && pe.Cause != nil {
		pe.Cause = fmt.Errorf("%w: %s", pe.Cause, body)
	}
	return pe
}

// StatusError wraps an error with an explicit HTTP status code. Use this in
// platform backends when you need to attach a status code to an error from a
// third-party SDK that doesn't implement the statusCoder interface. This avoids
// the need for the reflection-based fallback in Wrap.
//
// Example:
//
//	err := someSDK.DoSomething()
//	if err != nil {
//	    return provider.WrapStatusError(err, 404)
//	}
type StatusError struct {
	Status int
	Cause  error
}

func (e *StatusError) Error() string   { return e.Cause.Error() }
func (e *StatusError) Unwrap() error   { return e.Cause }
func (e *StatusError) StatusCode() int { return e.Status }

// WrapStatusError wraps an error with an explicit HTTP status code.
// This is the preferred way to attach status codes to third-party SDK errors
// instead of relying on reflection-based detection.
func WrapStatusError(err error, statusCode int) error {
	if err == nil {
		return nil
	}
	return &StatusError{Status: statusCode, Cause: err}
}

// statusCoder is the interface implemented by transport.Error. We avoid
// importing transport here to keep the provider package dependency-free.
type statusCoder interface {
	StatusCode() int
}

// httpStatusFromError inspects err for an HTTP status code. The lookup order
// is designed for performance: cheap interface checks first, reflection only
// as a last resort for third-party SDK errors.
//
// Priority order:
//  1. statusCoder interface (transport.Error, StatusError)
//  2. StatusError type (explicit wrapping by backends)
//  3. Reflection: StatusCode int field (gitlab client-go ErrorResponse)
//  4. Reflection: *http.Response field (go-github ErrorResponse)
//  5. String parsing: "returned NNN", "HTTP NNN", "status NNN"
//
// Backends should prefer WrapStatusError or Wrap/New with explicit status
// codes over relying on the reflection path.
func httpStatusFromError(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	v := reflect.ValueOf(err)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return 0, false
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return 0, false
	}
	// Method first (cheaper than field iteration).
	if c, ok := statusCodeFromMethod(v); ok {
		return c, true
	}
	// Non-struct error values (e.g. url.EscapeError, a string kind) have no
	// fields; NumField panics on non-struct kinds, so fall through to
	// message parsing.
	if v.Kind() != reflect.Struct {
		return parseStatusFromString(err.Error())
	}
	// Fields.
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.CanInterface() {
			continue
		}
		if resp, ok := f.Interface().(*http.Response); ok && resp != nil {
			return resp.StatusCode, true
		}
		// Named "StatusCode" of kind int, treated as a direct status code.
		name := v.Type().Field(i).Name
		if name == "StatusCode" {
			if c, ok := f.Interface().(int); ok && c > 0 {
				return c, true
			}
		}
	}
	// Last resort: scan the error message for "returned NNN" or "HTTP NNN".
	return parseStatusFromString(err.Error())
}

// statusCodeFromMethod reports the result of calling a value method named
// "StatusCode" that returns a single int, when v has one.
func statusCodeFromMethod(v reflect.Value) (int, bool) {
	m := v.MethodByName("StatusCode")
	if !m.IsValid() {
		return 0, false
	}
	out := m.Call(nil)
	if len(out) != 1 {
		return 0, false
	}
	c, ok := out[0].Interface().(int)
	return c, ok
}

// parseStatusFromString extracts a 3-digit HTTP status code from an error
// message. Recognizes the patterns:
//   - "returned 404"
//   - "HTTP 404"
//   - "status 404"
//
// Returns (0, false) when no status code is found.
func parseStatusFromString(msg string) (int, bool) {
	for _, prefix := range []string{"returned ", "HTTP ", "status ", "with "} {
		idx := indexOfCaseInsensitive(msg, prefix)
		if idx < 0 {
			continue
		}
		rest := msg[idx+len(prefix):]
		// Read up to 3 consecutive digits.
		var num int
		var found bool
		for i := 0; i < len(rest) && i < 3; i++ {
			c := rest[i]
			if c < '0' || c > '9' {
				break
			}
			num = num*10 + int(c-'0')
			found = true
		}
		if found && num >= 100 && num <= 599 {
			return num, true
		}
	}
	return 0, false
}

func indexOfCaseInsensitive(s, sub string) int {
	if sub == "" {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a := s[i+j]
			b := sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// ClassifyStatus maps an HTTP status code to a sentinel error. Exposed so
// transports and platform implementations can produce consistent error
// categories without importing each other.
func ClassifyStatus(statusCode int) error {
	return classifyStatusCode(statusCode)
}

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
	case statusCode >= 500 && statusCode < 600:
		return fmt.Errorf("server error (status %d)", statusCode)
	default:
		return fmt.Errorf("HTTP %d", statusCode)
	}
}

// IsNotFound reports whether err wraps ErrNotFound (HTTP 404).
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsAuthentication reports whether err wraps ErrAuthentication (HTTP 401).
func IsAuthentication(err error) bool { return errors.Is(err, ErrAuthentication) }

// IsRateLimited reports whether err wraps ErrRateLimited (HTTP 429).
func IsRateLimited(err error) bool { return errors.Is(err, ErrRateLimited) }

// IsForbidden reports whether err wraps ErrForbidden (HTTP 403).
func IsForbidden(err error) bool { return errors.Is(err, ErrForbidden) }

// IsConflict reports whether err wraps ErrConflict (HTTP 409).
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// IsNotImplemented reports whether err wraps ErrNotImplemented.
func IsNotImplemented(err error) bool { return errors.Is(err, ErrNotImplemented) }

// IsInvalidInput reports whether err wraps ErrInvalidInput.
func IsInvalidInput(err error) bool { return errors.Is(err, ErrInvalidInput) }

// IsWebhookValidation reports whether err wraps ErrWebhookValidation.
func IsWebhookValidation(err error) bool {
	return errors.Is(err, ErrWebhookValidation)
}

// IsPlatformNotSupported reports whether err wraps ErrPlatformNotSupported.
func IsPlatformNotSupported(err error) bool {
	return errors.Is(err, ErrPlatformNotSupported)
}
