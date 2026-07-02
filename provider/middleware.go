package provider

import (
	"context"
	"net/http"
	"time"
)

// RequestHook is called before an HTTP request is sent.
// It can modify the context (e.g., add tracing headers) or inspect the request.
type RequestHook func(ctx context.Context, req *http.Request) context.Context

// ResponseHook is called after an HTTP response is received.
type ResponseHook func(ctx context.Context, req *http.Request, resp *http.Response, duration time.Duration, err error)

// Hooks holds request and response lifecycle hooks. These are mapped into
// transport.Hooks by each backend's constructor; direct callers should use
// the Hooks struct to register hooks via provider.Config.
type Hooks struct {
	Request  []RequestHook
	Response []ResponseHook
}

// AddRequestHook appends a request hook.
func (h *Hooks) AddRequestHook(hook RequestHook) {
	h.Request = append(h.Request, hook)
}

// AddResponseHook appends a response hook.
func (h *Hooks) AddResponseHook(hook ResponseHook) {
	h.Response = append(h.Response, hook)
}
