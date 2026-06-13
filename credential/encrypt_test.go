package credential

import (
	"os"
	"sync"
	"testing"
)

func TestEncryptDecryptGCM(t *testing.T) {
	// Set a valid 32-byte key for testing
	os.Setenv("ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes!!")
	defer os.Unsetenv("ENCRYPTION_KEY")

	// Reset keyOnce for testing
	keyOnce = sync.Once{}
	key = nil

	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty string", ""},
		{"simple text", "hello world"},
		{"special chars", "!@#$%^&*()_+{}|:<>?"},
		{"unicode", "你好世界"},
		{"long text", "this is a longer plaintext message that should still encrypt and decrypt correctly"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := EncryptGCM(tt.plaintext)
			if err != nil {
				t.Fatalf("EncryptGCM failed: %v", err)
			}

			if tt.plaintext == "" {
				if encrypted != "" {
					t.Fatalf("expected empty string for empty input, got %q", encrypted)
				}
				return
			}

			if encrypted == "" {
				t.Fatal("expected non-empty encrypted output")
			}

			if encrypted == tt.plaintext {
				t.Fatal("encrypted output should differ from plaintext")
			}

			decrypted, err := DecryptGCM(encrypted)
			if err != nil {
				t.Fatalf("DecryptGCM failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Fatalf("expected %q, got %q", tt.plaintext, decrypted)
			}
		})
	}
}

func TestEncryptGCMNoKey(t *testing.T) {
	// Reset to force re-evaluation
	keyOnce = sync.Once{}
	key = nil
	os.Unsetenv("ENCRYPTION_KEY")

	_, err := EncryptGCM("test")
	if err == nil {
		t.Fatal("expected error when ENCRYPTION_KEY is not set")
	}
}

func TestDecryptGCMInvalidData(t *testing.T) {
	os.Setenv("ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes!!")
	defer os.Unsetenv("ENCRYPTION_KEY")
	keyOnce = sync.Once{}
	key = nil

	_, err := DecryptGCM("invalid-base64-data!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64 data")
	}
}

func TestDecryptGCMTooShort(t *testing.T) {
	os.Setenv("ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes!!")
	defer os.Unsetenv("ENCRYPTION_KEY")
	keyOnce = sync.Once{}
	key = nil

	// Encrypt something, then truncate the result
	encrypted, err := EncryptGCM("test message")
	if err != nil {
		t.Fatalf("EncryptGCM failed: %v", err)
	}

	// Truncate to make it too short
	if len(encrypted) > 10 {
		_, err = DecryptGCM(encrypted[:5])
		if err == nil {
			t.Fatal("expected error for truncated ciphertext")
		}
	}
}

func TestBuildAuthURL(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name      string
		remoteURL string
		authType  string
		username  string
		secret    string
		expected  string
	}{
		{
			name:      "https with token",
			remoteURL: "https://github.com/owner/repo.git",
			authType:  "http_token",
			username:  "",
			secret:    "mytoken",
			expected:  "https://mytoken@github.com/owner/repo.git",
		},
		{
			name:      "https with basic auth",
			remoteURL: "https://github.com/owner/repo.git",
			authType:  "http_basic",
			username:  "user",
			secret:    "pass",
			expected:  "https://user:pass@github.com/owner/repo.git",
		},
		{
			name:      "ssh passthrough",
			remoteURL: "git@github.com:owner/repo.git",
			authType:  "ssh",
			username:  "",
			secret:    "",
			expected:  "git@github.com:owner/repo.git",
		},
		{
			name:      "http with token",
			remoteURL: "http://gitea.local/owner/repo.git",
			authType:  "http_token",
			username:  "",
			secret:    "mytoken",
			expected:  "http://mytoken@gitea.local/owner/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.BuildAuthURL(tt.remoteURL, tt.authType, tt.username, tt.secret)
			if result != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestBuildSSHCommand(t *testing.T) {
	m := NewManager()

	result := m.BuildSSHCommand("/path/to/key")
	expected := "ssh -i /path/to/key -o StrictHostKeyChecking=no"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}

	result = m.BuildSSHCommand("")
	if result != "" {
		t.Fatalf("expected empty string for empty key path, got %q", result)
	}
}
