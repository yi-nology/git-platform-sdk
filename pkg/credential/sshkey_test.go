package credential

import (
	"crypto/ed25519"
	crand "crypto/rand"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

// TestSSHKeyHelper_HostKeysConcurrent guards M5: the host-key store must be
// safe for concurrent access. Under -race this fails instantly if the
// hostKeys map is mutated without synchronization.
func TestSSHKeyHelper_HostKeysConcurrent(t *testing.T) {
	h := NewSSHKeyHelper()
	cb := h.GetHostKeyCallback()

	// A few distinct test public keys to insert/verify concurrently.
	keys := make([]ssh.PublicKey, 4)
	for i := range keys {
		pub, _, err := ed25519.GenerateKey(crand.Reader)
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		pk, err := ssh.NewPublicKey(pub)
		if err != nil {
			t.Fatalf("new public key: %v", err)
		}
		keys[i] = pk
	}

	const goroutines = 60
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			known := "known.example.com"
			tofu := "tofu.example.com"
			k := keys[i%len(keys)]
			h.AddHostKey(known, k)
			_ = cb(known, nil, k) // verify against stored key
			_ = cb(tofu, nil, k)  // trust-on-first-use insert
		}(i)
	}
	wg.Wait()
}

// TestSSHKeyHelper_HostKeyMismatch ensures a returning host with a different
// key is rejected (TOFU verification actually compares keys).
func TestSSHKeyHelper_HostKeyMismatch(t *testing.T) {
	h := NewSSHKeyHelper()

	pub1, _, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		t.Fatalf("generate key1: %v", err)
	}
	pub2, _, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		t.Fatalf("generate key2: %v", err)
	}
	pk1, _ := ssh.NewPublicKey(pub1)
	pk2, _ := ssh.NewPublicKey(pub2)

	cb := h.GetHostKeyCallback()
	if err := cb("host.example.com", nil, pk1); err != nil {
		t.Fatalf("first TOFU accept failed: %v", err)
	}
	if err := cb("host.example.com", nil, pk2); err == nil {
		t.Fatal("expected mismatch error when a known host presents a different key")
	}
	// Same key must still verify cleanly.
	if err := cb("host.example.com", nil, pk1); err != nil {
		t.Fatalf("expected clean verify for matching key, got %v", err)
	}
}
