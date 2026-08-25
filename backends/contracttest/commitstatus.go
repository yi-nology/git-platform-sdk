package contracttest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CommitStatusHarnessConfig mounts the commit-status suite. The suite is
// self-driving: it records requests, invokes CreateCommitStatus, and asserts
// exactly one status-reporting request reached the wire.
type CommitStatusHarnessConfig struct{}

func testCommitStatusSuite(t *testing.T, h Harness) {
	capsDeclared := false
	{
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(h.EmptyListResponse))
		}))
		defer srv.Close()
		p := h.NewProvider(t, baseCfg(h, srv.URL))
		capsDeclared = p.Capabilities().CommitStatuses
	}
	switch {
	case h.CommitStatus == nil && !capsDeclared:
		t.Skipf("%s declares no CommitStatuses capability", h.Name)
	case h.CommitStatus == nil:
		t.Errorf("%s declares Capabilities().CommitStatuses but its Harness provides no CommitStatus config", h.Name)
	case !capsDeclared:
		t.Errorf("%s Harness provides a CommitStatus config but the platform does not declare Capabilities().CommitStatuses", h.Name)
	}

	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"state":"pending"}`))
	}))
	defer srv.Close()
	p := h.NewProvider(t, baseCfg(h, srv.URL))
	csm, ok := p.(provider.CommitStatusManager)
	if !ok {
		t.Fatalf("%s: provider does not implement CommitStatusManager", h.Name)
	}
	if err := csm.CreateCommitStatus(context.Background(), "owner", "repo", "deadbeef",
		provider.CommitStatusOptions{State: "pending", Context: "ci"}); err != nil {
		t.Fatalf("CreateCommitStatus: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 1 {
		t.Fatalf("%s: CreateCommitStatus made %d requests (%v), want exactly 1", h.Name, len(paths), paths)
	}
	if !strings.Contains(paths[0], "status") && !strings.Contains(paths[0], "statuses") {
		t.Errorf("%s: status request path %q does not look like a commit-status endpoint", h.Name, paths[0])
	}
}
