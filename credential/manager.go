package credential

import (
	"fmt"
	"strings"
)

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

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

func (m *Manager) BuildSSHCommand(sshKeyPath string) string {
	if sshKeyPath == "" {
		return ""
	}
	return fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", sshKeyPath)
}
