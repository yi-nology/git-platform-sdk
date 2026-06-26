package gitbackend

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	xhttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	xssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"
)

type GoGitBackend struct {
	logger Logger
}

func NewGoGitBackend(opts Options) *GoGitBackend {
	logger := opts.Logger
	if logger == nil {
		logger = NewNoopLogger()
	}
	return &GoGitBackend{logger: logger}
}

func (b *GoGitBackend) buildTransportAuth(auth AuthConfig) transport.AuthMethod {
	switch auth.Type {
	case AuthHTTPBasic:
		return &xhttp.BasicAuth{
			Username: auth.Username,
			Password: auth.Password,
		}
	case AuthHTTPToken:
		return &xhttp.TokenAuth{
			Token: auth.Token,
		}
	case AuthSSH:
		// Try SSHKeyContent first (for DB-stored keys)
		if auth.SSHKeyContent != "" {
			signer, err := xssh.NewPublicKeys("git", []byte(auth.SSHKeyContent), auth.Passphrase)
			if err == nil {
				signer.HostKeyCallback = insecureHostKeyCallback()
				return signer
			}
		}
		// Fall back to SSHKey file path
		if auth.SSHKey != "" {
			if _, err := os.Stat(auth.SSHKey); err == nil {
				signer, err := xssh.NewPublicKeysFromFile("git", auth.SSHKey, auth.Passphrase)
				if err == nil {
					signer.HostKeyCallback = insecureHostKeyCallback()
					return signer
				}
			}
		}
	}
	return nil
}

func insecureHostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return nil
	}
}

// buildFetchRefSpecs builds the refspecs for a fetch operation.
func buildFetchRefSpecs(opts FetchOptions) []config.RefSpec {
	if len(opts.Branches) == 0 {
		return []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/*:refs/remotes/%s/*", opts.Remote)),
		}
	}
	specs := make([]config.RefSpec, 0, len(opts.Branches))
	for _, branch := range opts.Branches {
		if isCommitSHA(branch) {
			continue
		}
		if strings.HasPrefix(branch, "refs/") {
			branchName := strings.TrimPrefix(branch, "refs/heads/")
			specs = append(specs, config.RefSpec(fmt.Sprintf("+%s:refs/remotes/%s/%s", branch, opts.Remote, branchName)))
		} else {
			specs = append(specs, config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", branch, opts.Remote, branch)))
		}
	}
	if len(specs) == 0 {
		return []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/*:refs/remotes/%s/*", opts.Remote)),
		}
	}
	return specs
}
