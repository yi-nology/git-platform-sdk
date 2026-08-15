package gitlab_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// TestGitLab_ResolveLabelID_Paginates verifies that UpdateLabel resolves a
// label that sits on page 2, past the 100-label first page the resolver
// used to stop at.
func TestGitLab_ResolveLabelID_Paginates(t *testing.T) {
	page1 := buildLabelsPage(100, "l") // 100 labels named l0..l99, none matching "target"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("page") == "2":
			_, _ = w.Write([]byte(`[{"id":42,"name":"target","color":"#4cc917"}]`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(page1))
		default: // PUT update after resolution
			_, _ = w.Write([]byte(`{"id":42,"name":"renamed","color":"#4cc917"}`))
		}
	}))
	defer srv.Close()

	p, err := provider.NewProvider(provider.Config{Platform: provider.PlatformGitLab, BaseURL: srv.URL, Token: "t"})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	newName := "renamed"
	lm := p.(provider.LabelManager)
	if _, err := lm.UpdateLabel(context.Background(), "owner", "repo", "target", provider.UpdateLabelOptions{NewName: &newName}); err != nil {
		t.Fatalf("UpdateLabel should resolve %q on page 2, got: %v", "target", err)
	}
}

// buildLabelsPage renders n labels named prefix0..prefix(n-1).
func buildLabelsPage(n int, prefix string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"id":` + strconv.Itoa(i+1) + `,"name":"` + prefix + strconv.Itoa(i) + `","color":"#4cc917"}`)
	}
	b.WriteByte(']')
	return b.String()
}
