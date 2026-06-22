package provider

import "testing"

func TestSplitFullName(t *testing.T) {
	tests := []struct {
		input       string
		wantOwner   string
		wantName    string
	}{
		{"owner/repo", "owner", "repo"},
		{"org/sub/repo", "org", "sub/repo"},
		{"repo-only", "", "repo-only"},
		{"", "", ""},
	}
	for _, tt := range tests {
		owner, name := SplitFullName(tt.input)
		if owner != tt.wantOwner {
			t.Errorf("SplitFullName(%q) owner = %q, want %q", tt.input, owner, tt.wantOwner)
		}
		if name != tt.wantName {
			t.Errorf("SplitFullName(%q) name = %q, want %q", tt.input, name, tt.wantName)
		}
	}
}

func TestExtractOwnerFromFullName(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"owner/repo", "owner"},
		{"repo-only", ""},
		{"", ""},
	}
	for _, tt := range tests {
		result := ExtractOwnerFromFullName(tt.input)
		if result != tt.expect {
			t.Errorf("ExtractOwnerFromFullName(%q) = %q, want %q", tt.input, result, tt.expect)
		}
	}
}

func TestBuildEventRepo(t *testing.T) {
	er := BuildEventRepo("owner/repo")
	if er.FullName != "owner/repo" {
		t.Fatalf("expected owner/repo, got %q", er.FullName)
	}
	if er.Owner != "owner" {
		t.Fatalf("expected owner, got %q", er.Owner)
	}
	if er.Name != "repo" {
		t.Fatalf("expected repo, got %q", er.Name)
	}
}

func TestBuildEventRepo_NoSlash(t *testing.T) {
	er := BuildEventRepo("repo-only")
	if er.FullName != "repo-only" {
		t.Fatalf("expected repo-only, got %q", er.FullName)
	}
	if er.Owner != "" {
		t.Fatalf("expected empty owner, got %q", er.Owner)
	}
	if er.Name != "repo-only" {
		t.Fatalf("expected repo-only, got %q", er.Name)
	}
}
