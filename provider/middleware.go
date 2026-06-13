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

// Hooks holds request and response lifecycle hooks.
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

// executeRequestHooks runs all request hooks and returns the (possibly modified) context.
func (h *Hooks) executeRequestHooks(ctx context.Context, req *http.Request) context.Context {
	if h == nil {
		return ctx
	}
	for _, hook := range h.Request {
		ctx = hook(ctx, req)
	}
	return ctx
}

// executeResponseHooks runs all response hooks.
func (h *Hooks) executeResponseHooks(ctx context.Context, req *http.Request, resp *http.Response, duration time.Duration, err error) {
	if h == nil {
		return
	}
	for _, hook := range h.Response {
		hook(ctx, req, resp, duration, err)
	}
}
