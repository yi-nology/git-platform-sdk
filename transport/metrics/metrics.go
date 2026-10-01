// Package metrics defines a small, dependency-free observation surface for
// the transport pipeline. Embedders that already run Prometheus or
// OpenTelemetry adapt this interface to their counters/histograms in a few
// lines; the SDK itself stays free of metrics dependencies.
//
// Wiring: mount the returned hook via provider.Config.Hooks (or
// transport.Hooks) and every platform request — including those made by
// wrapped third-party SDKs — is observed exactly once per exchange:
//
//	rec := myPromAdapter{} // implements metrics.Recorder
//	cfg := provider.Config{
//	    // ...
//	    Hooks: &provider.Hooks{Response: []provider.ResponseHook{metrics.ResponseHook(rec, metrics.ClassifyPath)}},
//	}
//
// The classify function reduces a request path to a low-cardinality label
// (see ClassifyPath); pass nil to use ClassifyPath, or identity for
// single-tenant deployments that want raw paths.
package metrics

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yi-nology/go-git-platform/transport"
)

// Recorder receives one observation per completed HTTP exchange: the
// request method, a (preferably low-cardinality) path label, the response
// status (0 when the exchange failed before a response), the total
// pipeline duration, and the error (nil on success). Implementations must
// be safe for concurrent use.
type Recorder interface {
	ObserveRequest(method, path string, status int, duration time.Duration, err error)
}

// RecorderFunc adapts a function into a Recorder.
type RecorderFunc func(method, path string, status int, duration time.Duration, err error)

// ObserveRequest implements Recorder.
func (f RecorderFunc) ObserveRequest(method, path string, status int, duration time.Duration, err error) {
	f(method, path, status, duration, err)
}

// ResponseHook returns a transport.ResponseHook observing every completed
// exchange into r. classify maps the request path to the label handed to
// the recorder; nil selects ClassifyPath.
func ResponseHook(r Recorder, classify func(*http.Request) string) transport.ResponseHook {
	if classify == nil {
		classify = func(req *http.Request) string { return ClassifyPath(req.URL.Path) }
	}
	return func(_ context.Context, req *http.Request, resp *http.Response, d time.Duration, err error) {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		r.ObserveRequest(req.Method, classify(req), status, d, err)
	}
}

// ClassifyPath reduces an API path to a low-cardinality template by
// replacing path segments that look like identifiers with "{id}":
// pure numbers (issue/PR/project IDs) and hex-ish strings of 7+ characters
// (commit SHAs, GitHub node IDs). Repo names are deliberately kept — they
// are bounded per deployment and usually exactly what operators want to
// slice by. Multi-tenant hosts can wrap this with their own collapsing.
//
//	/api/v4/projects/1234/repository/commits → /api/v4/projects/{id}/repository/commits
//	/repos/octocat/hello/pulls/42/files      → /repos/octocat/hello/pulls/{id}/files
func ClassifyPath(path string) string {
	if path == "" {
		return "/"
	}
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i, s := range segs {
		if looksLikeID(s) {
			segs[i] = "{id}"
		}
	}
	return "/" + strings.Join(segs, "/")
}

func looksLikeID(s string) bool {
	if s == "" {
		return false
	}
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}
	if len(s) < 7 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
