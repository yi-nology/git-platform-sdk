package gitbackend

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"regexp"
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

// gitSubcommands is the closed set of git subcommands this backend issues.
// args[0] must be one of them: that whitelists the command verb before any
// caller-controlled value reaches exec.
var gitSubcommands = map[string]bool{
	"add": true, "branch": true, "checkout": true, "cherry-pick": true,
	"clone": true, "commit": true, "config": true, "diff": true,
	"fetch": true, "init": true, "log": true, "ls-remote": true,
	"ls-tree": true, "merge": true, "merge-base": true, "pull": true,
	"push": true, "rebase": true, "remote": true, "rev-list": true,
	"rev-parse": true, "show": true, "stash": true, "status": true,
	"tag": true,
}

// gitExecFlags are git options that hand git an executable or a config
// override. A caller-controlled positional (URL, ref, pathspec) beginning
// with one of these would otherwise be parsed as a flag by git's
// interspersed option parser and turn a routine clone/fetch/push into
// arbitrary command execution or config injection.
var gitExecFlags = []string{
	"-c", "--exec", "--upload-pack", "--receive-pack", "--config-env",
}

// safeConfigOverrides matches the `-c key=value` pairs this backend issues
// itself (CommitWithIdentity identity overrides). git config keys that
// carry executables — core.sshCommand, filter.*.command, protocol.*.command,
// core.pager — are outside this set, so a smuggled `-c` can never turn into
// command execution.
var safeConfigOverrides = regexp.MustCompile(
	`^user\.(name|email)=[^=\x00-\x1f]*$|^commit\.gpgsign=(true|false)$`)

// checkGitArg validates one non-leading argument: no control characters,
// no ext:: transport URLs, none of the exec/config primitives.
func checkGitArg(arg string) error {
	for _, r := range arg {
		if (r < 0x20 && r != '\t') || r == 0x7f {
			return fmt.Errorf("%w: control character in argument", ErrInvalidGitArg)
		}
	}
	if strings.HasPrefix(arg, "ext::") {
		return fmt.Errorf("%w: ext:: transport URLs are not allowed", ErrInvalidGitArg)
	}
	for _, flag := range gitExecFlags {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return fmt.Errorf("%w: %q is not accepted from callers", ErrInvalidGitArg, arg)
		}
	}
	return nil
}

// sanitizeGitArgs enforces the exec boundary of runGit:
//
//  1. any leading `-c` flags are accepted only as pairs whose value
//     matches safeConfigOverrides (the identity overrides this backend
//     issues itself);
//  2. args[i] after those pairs must be a whitelisted git subcommand
//     (gitSubcommands);
//  3. every later argument passes checkGitArg.
//
// Library-issued flags such as --all, --tags or --abort pass through;
// withInsecureArgs prepends the library's own `-c http.sslVerify=false`
// AFTER this check.
func sanitizeGitArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: empty git command", ErrInvalidGitArg)
	}
	i := 0
	for i < len(args) && args[i] == "-c" {
		value := ""
		if i+1 < len(args) {
			value = args[i+1]
		}
		if !safeConfigOverrides.MatchString(value) {
			return fmt.Errorf("%w: -c is only accepted with a known-safe config override", ErrInvalidGitArg)
		}
		i += 2
	}
	if i >= len(args) || !gitSubcommands[args[i]] {
		return fmt.Errorf("%w: %q is not an allowed git subcommand", ErrInvalidGitArg, args[i])
	}
	for ; i < len(args); i++ {
		if err := checkGitArg(args[i]); err != nil {
			return err
		}
	}
	return nil
}

func (b *NativeGitBackend) runGit(ctx context.Context, repoPath string, args []string, auth AuthConfig) (string, string, error) {
	if err := sanitizeGitArgs(args); err != nil {
		return "", "", err
	}
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
			// %q keeps a caller-supplied key path from being interpreted by
			// the shell GIT_SSH_COMMAND runs under (spaces, $, backticks).
			sshCmd := fmt.Sprintf("ssh -i %q -o BatchMode=yes", auth.SSHKey)
			if auth.InsecureSkipTLS {
				// Explicit opt-out: accept any host key, keep no record.
				sshCmd += " -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
			} else {
				// Secure default: verify against the user's known_hosts and
				// auto-accept new hosts on first use (MITM on a known host
				// still fails hard).
				sshCmd += " -o StrictHostKeyChecking=accept-new"
			}
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
