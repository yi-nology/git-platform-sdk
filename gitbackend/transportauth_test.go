package gitbackend

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"testing"

	xhttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	xssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

// TestTransportAuth covers the exported entry point used by callers that drive
// go-git directly (mirror pipelines, tools) instead of going through a backend.
func TestTransportAuth(t *testing.T) {
	t.Run("anonymous", func(t *testing.T) {
		got, err := TransportAuth(AuthConfig{})
		if err != nil || got != nil {
			t.Fatalf("empty AuthConfig: got (%T, %v), want (nil, nil)", got, err)
		}
		got, err = TransportAuth(AuthConfig{Type: AuthNone})
		if err != nil || got != nil {
			t.Fatalf("AuthNone: got (%T, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("http basic", func(t *testing.T) {
		got, err := TransportAuth(AuthConfig{Type: AuthHTTPBasic, Username: "u", Password: "p"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ba, ok := got.(*xhttp.BasicAuth)
		if !ok || ba.Username != "u" || ba.Password != "p" {
			t.Fatalf("got %T %+v, want *xhttp.BasicAuth{u p}", got, got)
		}
	})

	t.Run("http token", func(t *testing.T) {
		got, err := TransportAuth(AuthConfig{Type: AuthHTTPToken, Token: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ta, ok := got.(*xhttp.TokenAuth)
		if !ok || ta.Token != "tok" {
			t.Fatalf("got %T %+v, want *xhttp.TokenAuth{tok}", got, got)
		}
	})

	t.Run("ssh key content with pinned fingerprint", func(t *testing.T) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		block, err := cryptossh.MarshalPrivateKey(key, "")
		if err != nil {
			t.Fatalf("marshal key: %v", err)
		}
		pemBytes := string(pem.EncodeToMemory(block))

		signer, err := cryptossh.NewSignerFromKey(key)
		if err != nil {
			t.Fatalf("signer: %v", err)
		}
		fingerprint := cryptossh.FingerprintSHA256(signer.PublicKey())

		got, err := TransportAuth(AuthConfig{
			Type:               AuthSSH,
			SSHKeyContent:      pemBytes,
			HostKeyFingerprint: fingerprint,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pk, ok := got.(*xssh.PublicKeys)
		if !ok {
			t.Fatalf("got %T, want *xssh.PublicKeys", got)
		}
		if pk.HostKeyCallback == nil {
			t.Fatal("expected a host-key callback to be installed")
		}
		if err := pk.HostKeyCallback("example.com:22", dummyAddr{}, signer.PublicKey()); err != nil {
			t.Fatalf("pinned key rejected: %v", err)
		}

		// 指纹不匹配必须拒绝(fail-closed)。
		if err := pk.HostKeyCallback("example.com:22", dummyAddr{}, otherPublicKey(t)); err == nil {
			t.Fatal("expected fingerprint mismatch to be rejected")
		}
	})

	t.Run("ssh invalid key wraps ErrAuthFailed", func(t *testing.T) {
		_, err := TransportAuth(AuthConfig{Type: AuthSSH, SSHKeyContent: "not a key"})
		if err == nil {
			t.Fatal("expected error for invalid SSH key content")
		}
		if !errors.Is(err, ErrAuthFailed) {
			t.Fatalf("err = %v, want to wrap %v", err, ErrAuthFailed)
		}
	})
}

func otherPublicKey(t *testing.T) cryptossh.PublicKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := cryptossh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return signer.PublicKey()
}

type dummyAddr struct{}

func (dummyAddr) Network() string { return "tcp" }
func (dummyAddr) String() string  { return "127.0.0.1:22" }
