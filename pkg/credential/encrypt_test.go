package credential

import (
	"os"
	"testing"
)

func TestCryptoManager_EncryptDecrypt(t *testing.T) {
	mgr := NewCryptoManagerFromKey("test-key-32-bytes-long-for-aes!!")

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
			encrypted, err := mgr.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
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

			decrypted, err := mgr.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Fatalf("expected %q, got %q", tt.plaintext, decrypted)
			}
		})
	}
}

func TestCryptoManager_ShortKey_Derived(t *testing.T) {
	// A short passphrase should be expanded via SHA-256 to 32 bytes.
	mgr := NewCryptoManagerFromKey("short")

	encrypted, err := mgr.Encrypt("hello")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := mgr.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != "hello" {
		t.Fatalf("expected %q, got %q", "hello", decrypted)
	}
}

func TestCryptoManager_LongKey_Derived(t *testing.T) {
	// A long key should also be hashed to 32 bytes.
	longKey := "this-key-is-much-longer-than-32-bytes-and-should-be-hashed"
	mgr := NewCryptoManagerFromKey(longKey)

	encrypted, err := mgr.Encrypt("world")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := mgr.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != "world" {
		t.Fatalf("expected %q, got %q", "world", decrypted)
	}
}

func TestCryptoManager_RotateKey(t *testing.T) {
	mgr := NewCryptoManagerFromKey("old-key-exactly-thirty-two-byte!")

	encrypted, err := mgr.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Rotate to new key
	mgr.RotateKey("new-key-exactly-thirty-two-byte!")

	// Decrypting old ciphertext with new key should fail
	_, err = mgr.Decrypt(encrypted)
	if err == nil {
		t.Fatal("expected error when decrypting with rotated key")
	}

	// Encrypting with new key should work
	encrypted2, err := mgr.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt after rotation failed: %v", err)
	}

	decrypted, err := mgr.Decrypt(encrypted2)
	if err != nil {
		t.Fatalf("Decrypt after rotation failed: %v", err)
	}

	if decrypted != "secret" {
		t.Fatalf("expected %q, got %q", "secret", decrypted)
	}
}

func TestCryptoManager_DifferentKeys_CantDecrypt(t *testing.T) {
	mgr1 := NewCryptoManagerFromKey("key-one-exactly-thirty-two-byte!")
	mgr2 := NewCryptoManagerFromKey("key-two-exactly-thirty-two-byte!")

	encrypted, err := mgr1.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = mgr2.Decrypt(encrypted)
	if err == nil {
		t.Fatal("expected error when decrypting with different key")
	}
}

func TestCryptoManager_InvalidCiphertext(t *testing.T) {
	mgr := NewCryptoManagerFromKey("test-key-32-bytes-long-for-aes!!")

	_, err := mgr.Decrypt("invalid-base64-data!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64 data")
	}
}

func TestCryptoManager_TruncatedCiphertext(t *testing.T) {
	mgr := NewCryptoManagerFromKey("test-key-32-bytes-long-for-aes!!")

	encrypted, err := mgr.Encrypt("test message")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if len(encrypted) > 10 {
		_, err = mgr.Decrypt(encrypted[:5])
		if err == nil {
			t.Fatal("expected error for truncated ciphertext")
		}
	}
}

func TestNewCryptoManager_NoEnvVar(t *testing.T) {
	// Save and restore
	oldVal, hadVal := os.LookupEnv("ENCRYPTION_KEY")
	os.Unsetenv("ENCRYPTION_KEY")
	defer func() {
		if hadVal {
			os.Setenv("ENCRYPTION_KEY", oldVal)
		}
	}()

	_, err := NewCryptoManager()
	if err == nil {
		t.Fatal("expected error when ENCRYPTION_KEY is not set")
	}
}

func TestNewCryptoManager_FromEnv(t *testing.T) {
	os.Setenv("ENCRYPTION_KEY", "test-key-32-bytes-long-for-aes!!")
	defer os.Unsetenv("ENCRYPTION_KEY")

	mgr, err := NewCryptoManager()
	if err != nil {
		t.Fatalf("NewCryptoManager failed: %v", err)
	}

	encrypted, err := mgr.Encrypt("hello")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := mgr.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != "hello" {
		t.Fatalf("expected %q, got %q", "hello", decrypted)
	}
}

func TestDeriveKey_Exact32(t *testing.T) {
	input := make([]byte, 32)
	for i := range input {
		input[i] = byte(i)
	}
	result := deriveKey(input)
	if len(result) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(result))
	}
	// Should be a copy, not alias
	input[0] = 0xFF
	if result[0] == 0xFF {
		t.Fatal("deriveKey should return a copy, not alias the input")
	}
}

func TestDeriveKey_Short(t *testing.T) {
	result := deriveKey([]byte("short"))
	if len(result) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(result))
	}
	// Same input should produce same output
	result2 := deriveKey([]byte("short"))
	for i := range result {
		if result[i] != result2[i] {
			t.Fatal("deriveKey should be deterministic")
		}
	}
}

func TestDeriveKey_DifferentInputs(t *testing.T) {
	r1 := deriveKey([]byte("aaa"))
	r2 := deriveKey([]byte("bbb"))
	same := true
	for i := range r1 {
		if r1[i] != r2[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different inputs should produce different keys")
	}
}
