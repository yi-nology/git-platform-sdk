package credential

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/crypto/ssh"
)

// SSHKeyHelper provides SSH key processing, temp file management,
// and host key verification for git operations.
type SSHKeyHelper struct {
	hostKeys map[string]ssh.PublicKey
}

// NewSSHKeyHelper creates a new SSHKeyHelper instance.
func NewSSHKeyHelper() *SSHKeyHelper {
	return &SSHKeyHelper{
		hostKeys: make(map[string]ssh.PublicKey),
	}
}

// ProcessPrivateKey normalizes a private key string and, if a passphrase is
// provided, decrypts it and re-encodes as unencrypted PEM. The returned
// content is ready to be written to a temp file or passed to ssh.ParsePrivateKey.
func (h *SSHKeyHelper) ProcessPrivateKey(privateKey, passphrase string) (string, error) {
	keyContent := strings.ReplaceAll(privateKey, "\r\n", "\n")
	keyContent = strings.ReplaceAll(keyContent, "\r", "")
	keyContent = strings.TrimSpace(keyContent)
	if !strings.HasSuffix(keyContent, "\n") {
		keyContent += "\n"
	}

	if passphrase != "" {
		rawKey, err := ssh.ParseRawPrivateKeyWithPassphrase([]byte(keyContent), []byte(passphrase))
		if err != nil {
			return "", fmt.Errorf("failed to parse encrypted private key: %v", err)
		}

		pemBytes, err := x509.MarshalPKCS8PrivateKey(rawKey)
		if err != nil {
			return "", fmt.Errorf("failed to marshal private key: %v", err)
		}

		pemBlock := &pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: pemBytes,
		}
		keyContent = string(pem.EncodeToMemory(pemBlock))
	}

	return keyContent, nil
}

// CreateTempKeyFile writes keyContent to a temporary file with 0600 permissions.
// Returns the file path. Caller must defer CleanupTempFile(path).
func (h *SSHKeyHelper) CreateTempKeyFile(keyContent string) (string, error) {
	tmpFile, err := os.CreateTemp("", "git_ssh_key_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp key file: %v", err)
	}

	if _, err := tmpFile.WriteString(keyContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write key file: %v", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to set key file permissions: %v", err)
	}

	return tmpFile.Name(), nil
}

// BuildSSHCommand returns a GIT_SSH_COMMAND value that uses the given key file
// with host key checking disabled (suitable for CI/server environments).
func (h *SSHKeyHelper) BuildSSHCommand(keyPath string) string {
	return fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", keyPath)
}

// BuildSecureSSHCommand returns a GIT_SSH_COMMAND value that uses the given key
// file with strict host key checking enabled (requires known_hosts to be populated).
func (h *SSHKeyHelper) BuildSecureSSHCommand(keyPath string) string {
	return fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=yes -o UserKnownHostsFile=~/.ssh/known_hosts", keyPath)
}

// CleanupTempFile removes a temporary key file. Safe to call with empty string.
func (h *SSHKeyHelper) CleanupTempFile(filePath string) {
	if filePath != "" {
		os.Remove(filePath)
	}
}

// AddHostKey registers a known public key for a host.
func (h *SSHKeyHelper) AddHostKey(host string, key ssh.PublicKey) {
	h.hostKeys[host] = key
}

// GetHostKeyCallback returns an ssh.HostKeyCallback that accepts new hosts
// and verifies returning hosts against stored keys.
func (h *SSHKeyHelper) GetHostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if knownKey, ok := h.hostKeys[hostname]; ok {
			if bytes.Equal(ssh.Marshal(key), ssh.Marshal(knownKey)) {
				return nil
			}
			return fmt.Errorf("host key mismatch for %s", hostname)
		}
		h.hostKeys[hostname] = key
		return nil
	}
}

// DetectKeyType returns the algorithm of the private key: rsa, ecdsa, ed25519, dsa, or unknown.
func (h *SSHKeyHelper) DetectKeyType(privateKey, passphrase string) string {
	keyContent := strings.ReplaceAll(privateKey, "\r\n", "\n")
	keyContent = strings.ReplaceAll(keyContent, "\r", "\n")
	keyContent = strings.TrimSpace(keyContent)
	if !strings.HasSuffix(keyContent, "\n") {
		keyContent += "\n"
	}

	var rawKey any
	var err error
	if passphrase != "" {
		rawKey, err = ssh.ParseRawPrivateKeyWithPassphrase([]byte(keyContent), []byte(passphrase))
	} else {
		rawKey, err = ssh.ParseRawPrivateKey([]byte(keyContent))
	}
	if err == nil {
		switch rawKey.(type) {
		case *rsa.PrivateKey:
			return "rsa"
		case *ecdsa.PrivateKey:
			return "ecdsa"
		case ed25519.PrivateKey, *ed25519.PrivateKey:
			return "ed25519"
		default:
			typeName := fmt.Sprintf("%T", rawKey)
			if strings.Contains(strings.ToLower(typeName), "dsa") {
				return "dsa"
			}
			return "unknown"
		}
	}

	block, _ := pem.Decode([]byte(keyContent))
	if block != nil {
		t := strings.ToLower(block.Type)
		switch {
		case strings.Contains(t, "rsa"):
			return "rsa"
		case strings.Contains(t, "ec"):
			return "ecdsa"
		case strings.Contains(t, "dsa"):
			return "dsa"
		case strings.Contains(t, "openssh"):
			if pub, extractErr := h.ExtractPublicKeyFromPrivateKey(privateKey, passphrase); extractErr == nil {
				parts := strings.Fields(pub)
				if len(parts) > 0 {
					switch parts[0] {
					case "ssh-rsa":
						return "rsa"
					case "ssh-ed25519":
						return "ed25519"
					case "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521":
						return "ecdsa"
					case "ssh-dss":
						return "dsa"
					}
				}
			}
		}
	}

	return "unknown"
}

// ExtractPublicKeyFromPrivateKey extracts the public key in authorized_keys format
// from a private key string.
func (h *SSHKeyHelper) ExtractPublicKeyFromPrivateKey(privateKey, passphrase string) (string, error) {
	keyContent := strings.ReplaceAll(privateKey, "\r\n", "\n")
	keyContent = strings.ReplaceAll(keyContent, "\r", "\n")
	keyContent = strings.TrimSpace(keyContent)
	if !strings.HasSuffix(keyContent, "\n") {
		keyContent += "\n"
	}

	var signer ssh.Signer
	var err error

	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(keyContent), []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(keyContent))
	}

	if err == nil {
		return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))), nil
	}

	tmpFile, tmpErr := h.CreateTempKeyFile(keyContent)
	if tmpErr != nil {
		return "", fmt.Errorf("failed to parse private key: %v", err)
	}
	defer h.CleanupTempFile(tmpFile)

	args := []string{"-y", "-f", tmpFile}
	if passphrase != "" {
		args = []string{"-y", "-P", passphrase, "-f", tmpFile}
	}
	output, execErr := exec.Command("ssh-keygen", args...).Output()
	if execErr != nil {
		return "", fmt.Errorf("failed to parse private key: %v", err)
	}

	return strings.TrimSpace(string(output)), nil
}
