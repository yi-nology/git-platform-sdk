package gitbackend

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func fakeHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return sshPub
}

func TestHostKeyCallback_FingerprintPin(t *testing.T) {
	key := fakeHostKey(t)
	want := ssh.FingerprintSHA256(key)

	cb, err := hostKeyCallbackWithConfig(AuthConfig{HostKeyFingerprint: want})
	require.NoError(t, err)

	addr := &net.TCPAddr{}
	require.NoError(t, cb("git.example.com", addr, key))

	// 换一把 key 应拒绝
	other := fakeHostKey(t)
	require.Error(t, cb("git.example.com", addr, other))
}

func TestHostKeyCallback_Insecure(t *testing.T) {
	cb, err := hostKeyCallbackWithConfig(AuthConfig{InsecureSkipTLS: true})
	require.NoError(t, err)
	require.NoError(t, cb("any.host", &net.TCPAddr{}, fakeHostKey(t)))
}

func TestHostKeyCallback_MissingKnownHosts(t *testing.T) {
	_, err := hostKeyCallbackWithConfig(AuthConfig{KnownHostsPath: "/nonexistent/known_hosts"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
