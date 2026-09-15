package gitbackend

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeGitArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		// accepted: every subcommand the backend issues, with typical flags
		{"clone", []string{"clone", "https://example.com/r.git", "dst"}, false},
		{"fetch all tags", []string{"fetch", "--all", "--tags"}, false},
		{"ls-remote heads", []string{"ls-remote", "--heads", "https://example.com/r.git"}, false},
		{"rebase abort", []string{"rebase", "--abort"}, false},
		{"rebase continue", []string{"rebase", "--continue"}, false},
		{"config write", []string{"config", "user.name", "bot"}, false},
		{"ref with tilde", []string{"rev-parse", "HEAD~1"}, false},
		{"tab inside path", []string{"add", "a\tb.txt"}, false},

		// -c: accepted only when paired with a known-safe override
		{"identity -c pair", []string{"-c", "user.name=bot", "-c", "user.email=bot@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "msg"}, false},
		{"unsafe -c pair", []string{"fetch", "-c", "core.sshCommand=evil"}, true},
		{"-c without value", []string{"fetch", "-c"}, true},

		// rejected: exec/config primitives, bare or =-joined, any position
		{"upload-pack bare", []string{"clone", "--upload-pack", "evil"}, true},
		{"upload-pack joined", []string{"clone", "--upload-pack=evil", "url"}, true},
		{"receive-pack joined", []string{"push", "origin", "--receive-pack=evil"}, true},
		{"exec joined", []string{"fetch", "--exec=evil"}, true},
		{"config-env joined", []string{"clone", "--config-env=foo=bar"}, true},
		{"first arg not a subcommand", []string{"--version"}, true},
		{"no subcommand at all", []string{}, true},

		// rejected: ext:: transport runs an arbitrary helper command
		{"ext url", []string{"clone", "ext::sh -c evil"}, true},

		// rejected: control characters
		{"newline in ref", []string{"checkout", "-b", "main\nextra"}, true},
		{"escape char in path", []string{"show", "\x1b[31m"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizeGitArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("sanitizeGitArgs(%q) = nil, want error", tt.args)
				}
				if !errors.Is(err, ErrInvalidGitArg) {
					t.Fatalf("sanitizeGitArgs(%q) error = %v, want ErrInvalidGitArg", tt.args, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("sanitizeGitArgs(%q) = %v, want nil", tt.args, err)
			}
		})
	}
}

func TestSanitizeGitArgsAllowsLibraryInsecureArgs(t *testing.T) {
	// withInsecureArgs runs after the guard, so the library's own `-c`
	// prepend must never be an input to sanitizeGitArgs.
	args, err := func() ([]string, error) {
		base := []string{"fetch", "origin"}
		if err := sanitizeGitArgs(base); err != nil {
			return nil, err
		}
		return withInsecureArgs(AuthConfig{InsecureSkipTLS: true}, base), nil
	}()
	if err != nil {
		t.Fatalf("guard rejected library pipeline: %v", err)
	}
	if args[0] != "-c" || !strings.Contains(args[1], "sslVerify=false") {
		t.Fatalf("unexpected insecure args: %v", args)
	}
}
