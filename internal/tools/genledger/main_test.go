package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderMatchesCommittedLedger(t *testing.T) {
	committed, err := os.ReadFile(filepath.Join(repoRoot(), "docs", "divergence-ledger.md"))
	if err != nil {
		t.Fatalf("read committed ledger: %v (run go generate ./... first)", err)
	}
	if got := render(); got != string(committed) {
		t.Fatal("docs/divergence-ledger.md is stale: regenerate with `go generate ./...`")
	}
}
