package provider

import "testing"

func TestIsCommitSHA(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc123def456789012345678901234567890abcd", true},
		{"ABC123DEF456789012345678901234567890ABCD", true},
		{"0000000000000000000000000000000000000000", true},
		{"main", false},
		{"abc123", false},
		{"", false},
		{"abc123def456789012345678901234567890abcg", false}, // 'g' is not hex
		{"abc123def456789012345678901234567890abc", false},  // 39 chars
		{"abc123def456789012345678901234567890abcde", false}, // 41 chars
	}
	for _, tt := range tests {
		result := isCommitSHA(tt.input)
		if result != tt.expected {
			t.Errorf("isCommitSHA(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
