package transport

import (
	"context"
	"net/http"
	"time"
)

// RequestHook is invoked just before a request is sent. It can mutate the
// context (e.g. add tracing metadata) or the request (e.g. add headers).
// Hooks are run in registration order. The first non-nil error returned by a
// hook aborts the request.
//
// Hooks observe the in-flight request, but they do NOT have visibility into
// retries. Each retry uses a fresh request (and therefore re-invokes the
// hooks) only when the round tripper is wrapped with retry behavior. The
// default Client.Do uses retry internally; if you need end-to-end visibility
// including retries, use Client.RoundTripper() and configure retries via
// Client.NewRetryingRoundTripper().
type RequestHook func(ctx context.Context, req *http.Request) error

// ResponseHook is invoked after a response is received or an error occurred.
// It cannot change the outcome. Use it for logging, metrics, or tracing.
type ResponseHook func(ctx context.Context, req *http.Request, resp *http.Response, duration time.Duration, err error)

// Hooks is an ordered collection of request and response hooks. The zero
// value is usable; nil receivers are no-ops.
type Hooks struct {
	Request  []RequestHook
	Response []ResponseHook
}

// AddRequest appends a request hook.
func (h *Hooks) AddRequest(hook RequestHook) {
	if hook == nil {
		return
	}
	h.Request = append(h.Request, hook)
}

// AddResponse appends a response hook.
func (h *Hooks) AddResponse(hook ResponseHook) {
	if hook == nil {
		return
	}
	h.Response = append(h.Response, hook)
}

// ExecuteRequest runs every registered request hook in order. The first
// non-nil error short-circuits. A nil *Hooks is a no-op.
func (h *Hooks) ExecuteRequest(ctx context.Context, req *http.Request) error {
	if h == nil {
		return nil
	}
	for _, hook := range h.Request {
		if err := hook(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteResponse runs every registered response hook in order. Hooks
// themselves do not affect the outcome. A nil *Hooks is a no-op.
func (h *Hooks) ExecuteResponse(ctx context.Context, req *http.Request, resp *http.Response, duration time.Duration, err error) {
	if h == nil {
		return
	}
	for _, hook := range h.Response {
		hook(ctx, req, resp, duration, err)
	}
}
