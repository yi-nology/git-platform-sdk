package credential

import "testing"

func TestBuildAuthURL_HTTPSToken(t *testing.T) {
	m := NewManager()
	result := m.BuildAuthURL("https://github.com/owner/repo.git", "http_token", "", "mytoken")
	expected := "https://mytoken@github.com/owner/repo.git"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestBuildAuthURL_HTTPSBasic(t *testing.T) {
	m := NewManager()
	result := m.BuildAuthURL("https://github.com/owner/repo.git", "http_basic", "user", "pass")
	expected := "https://user:pass@github.com/owner/repo.git"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestBuildAuthURL_HTTPBasicNoUser(t *testing.T) {
	m := NewManager()
	result := m.BuildAuthURL("https://github.com/owner/repo.git", "http_basic", "", "pass")
	expected := "https://pass@github.com/owner/repo.git"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestBuildAuthURL_HTTP(t *testing.T) {
	m := NewManager()
	result := m.BuildAuthURL("http://gitea.local/owner/repo.git", "http_token", "", "mytoken")
	expected := "http://mytoken@gitea.local/owner/repo.git"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestBuildAuthURL_SSH(t *testing.T) {
	m := NewManager()
	result := m.BuildAuthURL("git@github.com:owner/repo.git", "ssh", "", "")
	expected := "git@github.com:owner/repo.git"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestBuildAuthURL_UnknownType(t *testing.T) {
	m := NewManager()
	result := m.BuildAuthURL("https://github.com/owner/repo.git", "unknown", "", "")
	expected := "https://github.com/owner/repo.git"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}
