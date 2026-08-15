package transport

import (
	"errors"
	"net/http"
	"testing"
)

func TestError_Is(t *testing.T) {
	a := &Error{Method: "GET", Path: "/x", StatusCode: 404}
	b := &Error{Method: "GET", Path: "/x", StatusCode: 404}
	if !errors.Is(a, b) {
		t.Error("expected identical errors to be Is-equal")
	}
	// Different status codes should NOT be Is-equal even with same method/path
	c := &Error{Method: "GET", Path: "/x", StatusCode: 500}
	if errors.Is(a, c) {
		t.Error("expected different status codes to not match")
	}
	// Same status, different path should still match (status-based identity)
	d := &Error{Method: "POST", Path: "/y", StatusCode: 404}
	if !errors.Is(a, d) {
		t.Error("expected same status code to match regardless of method/path")
	}
}

func TestError_IsStatusCode(t *testing.T) {
	e := &Error{StatusCode: 404}
	if !errors.Is(e, &Error{StatusCode: 404, Method: "GET", Path: "/x"}) {
		t.Error("expected same status code to match via Is")
	}
}

func TestError_IsStatus(t *testing.T) {
	e := &Error{StatusCode: 502}
	if !e.IsStatus(502) {
		t.Error("expected IsStatus(502) true")
	}
	if e.IsStatus(500) {
		t.Error("expected IsStatus(500) false")
	}
}

func TestError_StatusClasses(t *testing.T) {
	client := &Error{StatusCode: 404}
	server := &Error{StatusCode: 503}
	if !client.IsClientError() {
		t.Error("expected 404 to be client error")
	}
	if !server.IsServerError() {
		t.Error("expected 503 to be server error")
	}
	if client.IsServerError() || server.IsClientError() {
		t.Error("cross-class detection should be false")
	}
}

func TestError_IsStatusClass(t *testing.T) {
	e := &Error{StatusCode: 599}
	if !e.IsStatusClass(http.StatusInternalServerError) {
		t.Error("599 should be 5xx")
	}
	e2 := &Error{StatusCode: 200}
	if e2.IsStatusClass(http.StatusInternalServerError) {
		t.Error("200 should not be 5xx")
	}
}

func TestNewStatusError(t *testing.T) {
	e := NewStatusError("POST", "/x", 500, []byte("oops")).(*Error)
	if e.StatusCode != 500 {
		t.Errorf("expected 500, got %d", e.StatusCode)
	}
	if string(e.Body) != "oops" {
		t.Errorf("expected body 'oops', got %q", e.Body)
	}
	if e.Method != "POST" || e.Path != "/x" {
		t.Errorf("expected method/path preserved, got %s/%s", e.Method, e.Path)
	}
}

func TestError_TruncatesLargeBodies(t *testing.T) {
	big := make([]byte, 1024)
	for i := range big {
		big[i] = 'x'
	}
	e := NewStatusError("GET", "/x", 500, big).(*Error)
	if len(e.Body) != 1024 {
		// Body is the full bytes; truncation only happens in Error()
		t.Errorf("expected body to be stored fully, got %d", len(e.Body))
	}
	if !containsString(e.Error(), "truncated") {
		t.Errorf("expected Error() to mention truncation, got %q", e.Error())
	}
}

func containsString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
