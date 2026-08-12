package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// roundTripFunc adapts a function into an http.RoundTripper for tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClient_Do_GET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/user" {
			t.Errorf("expected /user, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"login":"octocat"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, None{})
	var out struct {
		Login string `json:"login"`
	}
	resp, err := c.DoJSON(context.Background(), &Request{
		Method: http.MethodGet,
		Path:   "/user",
		Result: &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if out.Login != "octocat" {
		t.Errorf("expected octocat, got %q", out.Login)
	}
}

func TestClient_DoJSON_PostsBody(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, None{})
	type body struct {
		Name string `json:"name"`
	}
	var out struct {
		ID int `json:"id"`
	}
	_, err := c.DoJSON(context.Background(), &Request{
		Method: http.MethodPost,
		Path:   "/items",
		Body:   body{Name: "thing"},
		Result: &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capturedBody, `"name":"thing"`) {
		t.Errorf("expected name=thing in body, got %q", capturedBody)
	}
	if out.ID != 42 {
		t.Errorf("expected id 42, got %d", out.ID)
	}
}

func TestClient_AuthStrategies(t *testing.T) {
	tests := []struct {
		name   string
		auth   AuthStrategy
		header string
		value  string
	}{
		{"bearer", BearerToken{Token: "abc"}, "Authorization", "Bearer abc"},
		{"private", PrivateToken{Token: "xyz"}, "PRIVATE-TOKEN", "xyz"},
		{"token", TokenHeader{Token: "tk"}, "Authorization", "token tk"},
		{"static", StaticAuth{Header: "X-Api-Key", Value: "k"}, "X-Api-Key", "k"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get(tc.header); got != tc.value {
					t.Errorf("header %q = %q, want %q", tc.header, got, tc.value)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()
			c := NewClient(srv.URL, tc.auth)
			_, err := c.Do(context.Background(), &Request{Method: http.MethodGet, Path: "/x"})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClient_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`not found`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	_, err := c.Do(context.Background(), &Request{Method: http.MethodGet, Path: "/x"})
	if err == nil {
		t.Fatal("expected error")
	}
	var se *Error
	if !errors.As(err, &se) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if se.StatusCode != 404 {
		t.Errorf("expected 404, got %d", se.StatusCode)
	}
	if string(se.Body) != "not found" {
		t.Errorf("expected body 'not found', got %q", se.Body)
	}
	if !se.IsClientError() {
		t.Error("expected IsClientError true")
	}
}

func TestClient_QueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("expected page=2, got %q", r.URL.Query().Get("page"))
		}
		if r.URL.Query().Get("per_page") != "10" {
			t.Errorf("expected per_page=10, got %q", r.URL.Query().Get("per_page"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	_, err := c.Do(context.Background(), &Request{
		Method: http.MethodGet,
		Path:   "/items",
		Query:  map[string][]string{"page": {"2"}, "per_page": {"10"}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_RoundTripper_AuthInjectedForThirdParty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, BearerToken{Token: "my-token"})
	rt := c.RoundTripper()
	httpClient := &http.Client{Transport: rt}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/user", nil)
	_, err := httpClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer my-token" {
		t.Errorf("expected Bearer my-token, got %q", gotAuth)
	}
}

func TestHooks_RequestErrorShortCircuits(t *testing.T) {
	wantErr := errors.New("hook rejected")
	hooks := &Hooks{}
	hooks.AddRequest(func(ctx context.Context, req *http.Request) error { return wantErr })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be hit")
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Hooks = hooks
	_, err := c.Do(context.Background(), &Request{Method: http.MethodGet, Path: "/x"})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected hook error, got %v", err)
	}
}

func TestHooks_ResponseObserved(t *testing.T) {
	var observedStatus int
	hooks := &Hooks{}
	hooks.AddResponse(func(ctx context.Context, req *http.Request, resp *http.Response, d time.Duration, err error) {
		if resp != nil {
			observedStatus = resp.StatusCode
		}
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, None{})
	c.Hooks = hooks
	_, _ = c.Do(context.Background(), &Request{Method: http.MethodGet, Path: "/x"})
	if observedStatus != http.StatusTeapot {
		t.Errorf("expected 418 observed, got %d", observedStatus)
	}
}

// TestRetryingRoundTripper_PreservesBodyOnExhaustion guards the H1 fix: when
// every attempt lands on a retryable status, the final response must still
// carry a readable body (not a closed one) and, per the http.RoundTripper
// contract, a nil error. Previously the body was closed without buffering and
// the real HTTP status was masked by an EOF/"read on closed body" error from
// the SDK decoder sitting on top.
func TestRetryingRoundTripper_PreservesBodyOnExhaustion(t *testing.T) {
	const wantBody = `{"message":"bad gateway"}`
	var calls int32
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader(wantBody)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})
	rt := &retryingRoundTripper{
		inner:  inner,
		cfg:    &RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond},
		logger: NoopLogger(),
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/x", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("expected nil error for a received 5xx response, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected 502 response, got %+v", resp)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
	gotBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("reading restored body: %v", readErr)
	}
	if string(gotBody) != wantBody {
		t.Fatalf("body not preserved across retries: want %q, got %q", wantBody, string(gotBody))
	}
}

// TestRetryingRoundTripper_NetworkErrorReturnsError confirms that a transport
// error (no response received at all) still surfaces as a non-nil error.
func TestRetryingRoundTripper_NetworkErrorReturnsError(t *testing.T) {
	boom := errors.New("dial tcp: connection refused")
	inner := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, boom
	})
	rt := &retryingRoundTripper{
		inner:  inner,
		cfg:    &RetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond},
		logger: NoopLogger(),
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/x", nil)
	resp, err := rt.RoundTrip(req)
	if !errors.Is(err, boom) {
		t.Fatalf("expected network error to surface, got resp=%v err=%v", resp, err)
	}
	if resp != nil {
		t.Errorf("expected nil response on transport error, got %+v", resp)
	}
}
