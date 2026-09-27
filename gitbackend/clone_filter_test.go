package gitbackend

import "testing"

func TestCloneOptions_FilterArgs(t *testing.T) {
	// 直接测 sanitize 允许 --filter
	args := []string{"clone", "--filter", "blob:none", "--recurse-submodules", "https://x/y.git", "/tmp/z"}
	if err := sanitizeGitArgs(args); err != nil {
		t.Fatalf("sanitize rejected filter clone args: %v", err)
	}
	args = []string{"clone", "--filter=blob:none", "https://x/y.git", "/tmp/z"}
	if err := sanitizeGitArgs(args); err != nil {
		t.Fatalf("sanitize rejected --filter= form: %v", err)
	}
}
