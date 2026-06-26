package gitbackend

import (
	"strings"
)

// NewTokenAuth builds an AuthConfig for HTTPS token authentication.
//
// When authenticating against git hosting platforms (GitHub, GitLab, Gitea,
// etc.) over HTTPS the credentials are exchanged via HTTP Basic Auth: the
// username is an arbitrary non-empty placeholder and the real token is sent as
// the password. An empty token collapses to AuthNone so callers can simply pass
// through an optional token without extra branching.
func NewTokenAuth(token string) AuthConfig {
	if token == "" {
		return AuthConfig{Type: AuthNone}
	}
	return AuthConfig{
		Type:     AuthHTTPBasic,
		Username: "token",
		Password: token,
	}
}

// NewHTTPBasicAuth builds an AuthConfig for HTTP Basic authentication.
func NewHTTPBasicAuth(username, password string) AuthConfig {
	if username == "" && password == "" {
		return AuthConfig{Type: AuthNone}
	}
	return AuthConfig{
		Type:     AuthHTTPBasic,
		Username: username,
		Password: password,
	}
}

// NewSSHKeyFileAuth builds an AuthConfig for SSH authentication using a key file on disk.
func NewSSHKeyFileAuth(keyPath, passphrase string) AuthConfig {
	if keyPath == "" {
		return AuthConfig{Type: AuthNone}
	}
	return AuthConfig{
		Type:       AuthSSH,
		SSHKey:     keyPath,
		Passphrase: passphrase,
	}
}

// NewSSHKeyContentAuth builds an AuthConfig for SSH authentication using
// in-memory key content (e.g. from a database). The backend will create a
// temporary file automatically if needed.
func NewSSHKeyContentAuth(keyContent, passphrase string) AuthConfig {
	if keyContent == "" {
		return AuthConfig{Type: AuthNone}
	}
	return AuthConfig{
		Type:          AuthSSH,
		SSHKeyContent: keyContent,
		Passphrase:    passphrase,
	}
}

// AutoDetectAuth attempts to detect the appropriate AuthConfig for a given URL.
// For SSH URLs, it tries common key file locations and the SSH agent.
// For HTTP(S) URLs, it returns AuthNone (caller should provide token/password).
func AutoDetectAuth(urlStr string) AuthConfig {
	if strings.HasPrefix(urlStr, "https://") || strings.HasPrefix(urlStr, "http://") {
		return AuthConfig{Type: AuthNone}
	}

	// For SSH URLs, we can't auto-detect without the credential package.
	// Callers should use pkg/credential.SSHKeyHelper for full auto-detection.
	return AuthConfig{Type: AuthNone}
}
