package encoding

import "testing"

func TestBase64EncodeDecode(t *testing.T) {
	tests := []string{"", "hello", "你好世界", "!@#$%^&*()"}
	for _, s := range tests {
		encoded := Base64Encode(s)
		if s == "" {
			if encoded != "" {
				t.Fatalf("expected empty, got %q", encoded)
			}
			continue
		}
		decoded, err := Base64Decode(encoded)
		if err != nil {
			t.Fatalf("Base64Decode(%q) error: %v", encoded, err)
		}
		if decoded != s {
			t.Fatalf("roundtrip failed: %q -> %q -> %q", s, encoded, decoded)
		}
	}
}

func TestBase64URLEncodeDecode(t *testing.T) {
	tests := []string{"", "hello", "你好世界", "!@#$%^&*()", "abc+def/ghi="}
	for _, s := range tests {
		encoded := Base64URLEncode(s)
		if s == "" {
			if encoded != "" {
				t.Fatalf("expected empty, got %q", encoded)
			}
			continue
		}
		decoded, err := Base64URLDecode(encoded)
		if err != nil {
			t.Fatalf("Base64URLDecode(%q) error: %v", encoded, err)
		}
		if decoded != s {
			t.Fatalf("roundtrip failed: %q -> %q -> %q", s, encoded, decoded)
		}
	}
}

func TestBase64URLNoSlash(t *testing.T) {
	// URL-safe encoding should not contain '+' or '/'
	encoded := Base64URLEncode("hello world test string!!")
	for _, c := range encoded {
		if c == '+' || c == '/' {
			t.Fatalf("URL encoding should not contain %c, got %q", c, encoded)
		}
	}
}
