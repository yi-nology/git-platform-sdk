package credential

import (
	"fmt"
	"strings"
)

// Manager provides helpers for constructing authenticated git remote URLs
// and SSH commands. It is stateless and safe for concurrent use.
type Manager struct{}

// NewManager creates a new credential Manager.
func NewManager() *Manager {
	return &Manager{}
}

// BuildAuthURL embeds credentials into a git remote URL. Supported authType
// values: "http_basic" (user:pass), "http_token" (token only), "ssh" (passthrough).
func (m *Manager) BuildAuthURL(remoteURL, authType, username, secret string) string {
	switch authType {
	case "http_basic", "http_token":
		token := secret
		if authType == "http_basic" && username != "" {
			token = fmt.Sprintf("%s:%s", username, secret)
		}
		if strings.HasPrefix(remoteURL, "https://") {
			return fmt.Sprintf("https://%s@%s", token, remoteURL[8:])
		}
		if strings.HasPrefix(remoteURL, "http://") {
			return fmt.Sprintf("http://%s@%s", token, remoteURL[7:])
		}
	case "ssh":
		return remoteURL
	}
	return remoteURL
}

// BuildSSHCommand returns a GIT_SSH_COMMAND value that uses the given key file
// with strict host key checking enabled (the secure default).
// Use BuildSSHCommandInsecure for CI environments where known_hosts is unavailable.
func (m *Manager) BuildSSHCommand(sshKeyPath string) string {
	if sshKeyPath == "" {
		return ""
	}
	return fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=yes -o UserKnownHostsFile=~/.ssh/known_hosts", sshKeyPath)
}

// BuildSSHCommandInsecure returns a GIT_SSH_COMMAND value with host key
// checking disabled. Only suitable for CI/server environments where
// known_hosts cannot be populated. Do NOT use in production.
func (m *Manager) BuildSSHCommandInsecure(sshKeyPath string) string {
	if sshKeyPath == "" {
		return ""
	}
	return fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", sshKeyPath)
}
