package branchfilter

import (
	"path/filepath"
	"strings"
)

type BranchFilter struct {
	patterns []string
}

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

func (f *BranchFilter) IsEmpty() bool {
	return len(f.patterns) == 0
}

func (f *BranchFilter) Patterns() []string {
	return f.patterns
}
