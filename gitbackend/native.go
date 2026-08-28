package gitbackend

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"
)

type NativeGitBackend struct {
	gitPath string
	logger  Logger
}

func NewNativeGitBackend(opts Options) (*NativeGitBackend, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitNotFound, err)
	}
	logger := opts.Logger
	if logger == nil {
		logger = NewNoopLogger()
	}
	return &NativeGitBackend{gitPath: path, logger: logger}, nil
}

// --- Internal helpers ---

func (b *NativeGitBackend) runGit(ctx context.Context, repoPath string, args []string, auth AuthConfig) (string, string, error) {
	args = withInsecureArgs(auth, args)

	//nolint:gosec // G204: args are intentionally dynamic — this wraps arbitrary git commands.
	cmd := exec.CommandContext(ctx, b.gitPath, args...)
	if repoPath != "" {
		cmd.Dir = repoPath
	}

	// resolveAuth may create a temp key file for SSHKeyContent; cleanup after run.
	resolvedAuth, cleanup := b.resolveAuth(auth)
	defer cleanup()

	b.configureAuth(cmd, resolvedAuth)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// withInsecureArgs prepends `git -c http.sslVerify=false` to args when the
// auth requests skipping TLS verification. This keeps skip-SSL handling in one
// place so every command (fetch/push/clone/pull/ls-remote/...) honors it.
func withInsecureArgs(auth AuthConfig, args []string) []string {
	if auth.InsecureSkipTLS {
		return append([]string{"-c", "http.sslVerify=false"}, args...)
	}
	return args
}

// resolveAuth handles SSHKeyContent by writing it to a temp file so that
// the native git command can use it via GIT_SSH_COMMAND. Returns the resolved
// AuthConfig (with SSHKey pointing to the temp file) and a cleanup function.
func (b *NativeGitBackend) resolveAuth(auth AuthConfig) (AuthConfig, func()) {
	cleanup := func() {}

	if auth.Type == AuthSSH && auth.SSHKeyContent != "" && auth.SSHKey == "" {
		tmpFile, err := os.CreateTemp("", "git_ssh_key_*")
		if err != nil {
			return auth, cleanup
		}
		keyContent := auth.SSHKeyContent
		if !strings.HasSuffix(keyContent, "\n") {
			keyContent += "\n"
		}
		if _, err := tmpFile.WriteString(keyContent); err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			return auth, cleanup
		}
		_ = tmpFile.Close()
		_ = os.Chmod(tmpFile.Name(), 0o600)

		auth.SSHKey = tmpFile.Name()
		cleanup = func() { _ = os.Remove(tmpFile.Name()) }
	}

	return auth, cleanup
}

func (b *NativeGitBackend) configureAuth(cmd *exec.Cmd, auth AuthConfig) {
	if cmd.Env == nil {
		cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	}

	switch auth.Type {
	case AuthHTTPBasic, AuthHTTPToken:
		token := auth.Token
		if token == "" {
			token = auth.Password
		}
		username := auth.Username
		if username == "" {
			username = "token"
		}
		if token != "" {
			// Use http.extraheader with Basic auth — works for all git
			// operations (clone, fetch, push) without modifying URLs.
			cred := base64.StdEncoding.EncodeToString([]byte(username + ":" + token))
			cmd.Env = append(cmd.Env,
				"GIT_CONFIG_COUNT=1",
				"GIT_CONFIG_KEY_0=http.extraheader",
				fmt.Sprintf("GIT_CONFIG_VALUE_0=Authorization: Basic %s", cred),
			)
		}
	case AuthSSH:
		if auth.SSHKey != "" {
			sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", auth.SSHKey)
			cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))
		}
	}
}

// --- Output parsers ---

func parseFetchRefs(output string) []string {
	var refs []string
	for _, raw := range strings.Split(output, "\n") {
		// git's fetch ref lines are indented (" * [new branch] main ->
		// origin/main"); the leading-space check must run on the raw line,
		// before trimming — a trimmed line can never match it.
		if !strings.HasPrefix(raw, " ") {
			continue
		}
		line := strings.TrimSpace(raw)
		if !strings.Contains(line, "->") {
			continue
		}
		parts := strings.SplitN(line, "->", 2)
		if len(parts) == 2 {
			refs = append(refs, strings.TrimSpace(parts[1]))
		}
	}
	return refs
}

func parsePushRefs(output string) []string {
	var refs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "To ") || strings.Contains(line, "..") {
			if strings.Contains(line, " ") {
				for _, p := range strings.Fields(line) {
					if strings.Contains(p, ":") || strings.Contains(p, "..") {
						refs = append(refs, p)
					}
				}
			}
		}
	}
	return refs
}

func isText(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	// Check for null bytes
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return utf8.Valid(data)
}
