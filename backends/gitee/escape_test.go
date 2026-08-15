package gitee_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yi-nology/git-platform-sdk/backends/gitee"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// TestGitee_PathEscaping verifies that owner/repo/label-name/file-path
// segments are percent-encoded on the wire and decode back to the original
// values server-side. Characters like '#', '?', '%', spaces, and non-ASCII
// would otherwise corrupt or truncate the URL.
func TestGitee_PathEscaping(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	var refs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		refs = append(refs, r.URL.Query().Get("ref"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`))
		} else {
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	p, err := gitee.New(provider.Config{BaseURL: srv.URL, Token: "t"})
	if err != nil {
		t.Fatalf("gitee.New: %v", err)
	}
	ctx := context.Background()
	const (
		owner = "ow#ner"
		repo  = "re?po"
	)
	_, _ = p.ListBranches(ctx, owner, repo)
	_ = p.(provider.LabelManager).DeleteLabel(ctx, owner, repo, "la bel%1")
	_, _ = p.GetFileContent(ctx, owner, repo, "dir one/file#2.txt", "bra nch")

	if len(paths) != 3 {
		t.Fatalf("expected 3 recorded requests, got %d: %v", len(paths), paths)
	}
	for _, path := range paths {
		if !strings.Contains(path, owner) {
			t.Errorf("path %q lost owner %q (segment not escaped)", path, owner)
		}
		if !strings.Contains(path, repo) {
			t.Errorf("path %q lost repo %q (segment not escaped)", path, repo)
		}
	}
	if !strings.Contains(paths[1], "la bel%1") {
		t.Errorf("path %q lost label name (segment not escaped)", paths[1])
	}
	if !strings.Contains(paths[2], "dir one/file#2.txt") {
		t.Errorf("path %q lost file path (segments not escaped; '/' must survive)", paths[2])
	}
	if refs[2] != "bra nch" {
		t.Errorf("ref query decoded to %q, want %q (ref not query-escaped)", refs[2], "bra nch")
	}
}
