package gitcode_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// TestGitCode_GetArchive_WireFormat verifies the wire path and format
// mapping of archive downloads: GitCode's archive endpoint takes the ref and
// extension as a single `ref.ext` path segment
// (GET /repos/{owner}/{repo}/archive/{archive}). The backend maps format
// "tar.gz" to `ref.tar.gz` and everything else (including empty/"zip") to
// `ref.zip`, so a v1.0.0 tarball request must hit
// /repos/owner/repo/archive/v1.0.0.tar.gz and return the raw bytes.
func TestGitCode_GetArchive_WireFormat(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	const payload = "fake-tarball-bytes"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.EscapedPath())
		mu.Unlock()
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	p, err := provider.NewProvider(provider.Config{Platform: provider.PlatformGitCode, BaseURL: srv.URL, Token: "t"})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	rm := p.(provider.ReleaseManager)

	got, err := rm.GetArchive(context.Background(), "owner", "repo", "v1.0.0", "tar.gz")
	if err != nil {
		t.Fatalf("GetArchive: %v", err)
	}
	if errors.Is(err, provider.ErrNotImplemented) {
		t.Fatal("GetArchive still returns ErrNotImplemented; archive is a real endpoint since go-gitcode v0.7.0")
	}
	if string(got) != payload {
		t.Errorf("archive bytes = %q, want %q", got, payload)
	}

	// Default format falls back to zip: empty and "zip" both request
	// ref.zip, mirroring the gitee default-zip semantics.
	for _, format := range []string{"", "zip"} {
		if _, err := rm.GetArchive(context.Background(), "owner", "repo", "master", format); err != nil {
			t.Fatalf("GetArchive(format=%q): %v", format, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"GET /repos/owner/repo/archive/v1.0.0.tar.gz",
		"GET /repos/owner/repo/archive/master.zip",
		"GET /repos/owner/repo/archive/master.zip",
	}
	if len(paths) != len(want) {
		t.Fatalf("recorded paths %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("request %d = %q, want %q", i, paths[i], want[i])
		}
	}
}

// TestGitCode_CreateCommitStatus_WireFormat verifies the wire form of commit
// status creation: POST /repos/{owner}/{repo}/statuses/{sha} with the
// state/context/description/target_url keys carried 1:1 from
// provider.CommitStatusOptions.
func TestGitCode_CreateCommitStatus_WireFormat(t *testing.T) {
	var mu sync.Mutex
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			mu.Lock()
			gotPath = r.Method + " " + r.URL.EscapedPath()
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"sha":"abc123","state":"success","context":"ci/build"}`))
	}))
	defer srv.Close()

	p, err := provider.NewProvider(provider.Config{Platform: provider.PlatformGitCode, BaseURL: srv.URL, Token: "t"})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	cm := p.(provider.CommitStatusManager)

	err = cm.CreateCommitStatus(context.Background(), "o", "r", "abc123", provider.CommitStatusOptions{
		State:       "success",
		Context:     "ci/build",
		Description: "build passed",
		TargetURL:   "https://ci.example.com/1",
	})
	if err != nil {
		t.Fatalf("CreateCommitStatus: %v", err)
	}
	if errors.Is(err, provider.ErrNotImplemented) {
		t.Fatal("CreateCommitStatus still returns ErrNotImplemented; statuses are a real endpoint since go-gitcode v0.7.0")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotPath != "POST /repos/o/r/statuses/abc123" {
		t.Errorf("wire path = %q, want %q", gotPath, "POST /repos/o/r/statuses/abc123")
	}
	if state, _ := gotBody["state"].(string); state != "success" {
		t.Errorf("body state = %q, want %q", state, "success")
	}
	if ctx, _ := gotBody["context"].(string); ctx != "ci/build" {
		t.Errorf("body context = %q, want %q", ctx, "ci/build")
	}
	if d, _ := gotBody["description"].(string); d != "build passed" {
		t.Errorf("body description = %q, want %q", d, "build passed")
	}
	if u, _ := gotBody["target_url"].(string); u != "https://ci.example.com/1" {
		t.Errorf("body target_url = %q, want %q", u, "https://ci.example.com/1")
	}
}
