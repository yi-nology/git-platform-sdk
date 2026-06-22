package provider

import "testing"

func TestMapMRStateToCR(t *testing.T) {
	tests := []struct {
		input    string
		expected CRState
	}{
		{"opened", CRStateOpened},
		{"open", CRStateOpened},
		{"merged", CRStateMerged},
		{"closed", CRStateClosed},
		{"unknown", CRStateOpened},
	}
	for _, tt := range tests {
		result := MapMRStateToCR(tt.input)
		if result != tt.expected {
			t.Errorf("MapMRStateToCR(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMapBoolStateToCR(t *testing.T) {
	tests := []struct {
		state    string
		merged   bool
		expected CRState
	}{
		{"closed", true, CRStateMerged},
		{"closed", false, CRStateClosed},
		{"opened", false, CRStateOpened},
	}
	for _, tt := range tests {
		result := MapBoolStateToCR(tt.state, tt.merged)
		if result != tt.expected {
			t.Errorf("MapBoolStateToCR(%q, %v) = %q, want %q", tt.state, tt.merged, result, tt.expected)
		}
	}
}

func TestMapStateToCR_NilFn(t *testing.T) {
	result := MapStateToCR("merged", nil)
	if result != CRStateMerged {
		t.Fatalf("expected CRStateMerged, got %q", result)
	}
}
