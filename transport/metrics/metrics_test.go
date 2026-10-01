package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClassifyPath(t *testing.T) {
	cases := map[string]string{
		"/api/v4/projects/1234/repository/commits": "/api/v4/projects/{id}/repository/commits",
		"/repos/octocat/hello/pulls/42/files":      "/repos/octocat/hello/pulls/{id}/files",
		"/repos/o/r/commits/abc123def7890":         "/repos/o/r/commits/{id}",
		"/user/repos":                              "/user/repos",
		"":                                         "/",
	}
	for in, want := range cases {
		if got := ClassifyPath(in); got != want {
			t.Errorf("ClassifyPath(%q) = %q, want %q", in, got, want)
		}
	}
	// Repo names must survive (bounded cardinality, useful slice).
	if got := ClassifyPath("/repos/my-org/my-repo"); got != "/repos/my-org/my-repo" {
		t.Errorf("repo-shaped path collapsed: %q", got)
	}
}

func TestResponseHookObserves(t *testing.T) {
	type obs struct {
		method, path string
		status       int
		dur          time.Duration
		err          error
	}
	got := make(chan obs, 1)
	hook := ResponseHook(RecorderFunc(func(m, p string, s int, d time.Duration, err error) {
		got <- obs{m, p, s, d, err}
	}), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v4/projects/99/refs", nil)
	resp := &http.Response{StatusCode: 200}
	hook(nil, req, resp, 250*time.Millisecond, nil)

	o := <-got
	if o.method != http.MethodGet || o.status != 200 || o.dur != 250*time.Millisecond {
		t.Fatalf("observation = %+v", o)
	}
	if o.path != "/api/v4/projects/{id}/refs" {
		t.Fatalf("path label = %q, want classified", o.path)
	}

	// Failed exchange: no response, error passes through.
	hook(nil, req, nil, time.Second, http.ErrHandlerTimeout)
	o = <-got
	if o.status != 0 || o.err != http.ErrHandlerTimeout {
		t.Fatalf("failed observation = %+v", o)
	}
}
