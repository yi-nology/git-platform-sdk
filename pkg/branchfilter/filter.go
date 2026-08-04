package branchfilter

import (
	"path/filepath"
	"strings"
)

// BranchFilter matches branch names against a set of comma-separated glob
// patterns. An empty filter matches all branches.
type BranchFilter struct {
	patterns []string
}

// New creates a BranchFilter from a comma-separated list of glob patterns
// (e.g. "main,release-*,feature/*"). An empty string matches everything.
func New(filterStr string) *BranchFilter {
	if filterStr == "" {
		return &BranchFilter{patterns: nil}
	}

	raw := strings.Split(filterStr, ",")
	patterns := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	return &BranchFilter{patterns: patterns}
}

// Match reports whether branchName matches any of the filter's patterns.
// Returns true (matches everything) when the filter has no patterns.
func (f *BranchFilter) Match(branchName string) bool {
	if len(f.patterns) == 0 {
		return true
	}

	for _, pattern := range f.patterns {
		matched, err := filepath.Match(pattern, branchName)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// FilterBranches returns only the branches that match the filter.
func (f *BranchFilter) FilterBranches(branches []string) []string {
	if len(f.patterns) == 0 {
		return branches
	}

	result := make([]string, 0, len(branches))
	for _, b := range branches {
		if f.Match(b) {
			result = append(result, b)
		}
	}
	return result
}

// IsEmpty reports whether the filter has no patterns (matches everything).
func (f *BranchFilter) IsEmpty() bool {
	return len(f.patterns) == 0
}

// Patterns returns the raw glob patterns that were used to create this filter.
func (f *BranchFilter) Patterns() []string {
	return f.patterns
}
