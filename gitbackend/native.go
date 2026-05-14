package gitbackend

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type NativeGitBackend struct {
	gitPath string
}

func NewNativeGitBackend() (*NativeGitBackend, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git not found in PATH: %w", err)
	}
	return &NativeGitBackend{gitPath: path}, nil
}

func (b *NativeGitBackend) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	args := []string{"fetch", opts.Remote}

	if opts.Prune {
		args = append(args, "--prune")
	}
	if opts.Tags {
		args = append(args, "--tags")
	} else {
		args = append(args, "--no-tags")
	}

	for _, branch := range opts.Branches {
		args = append(args, branch)
	}

	if len(opts.Branches) == 0 {
		args = []string{"fetch", opts.Remote}
		if opts.Prune {
			args = append(args, "--prune")
		}
		if opts.Tags {
			args = append(args, "--tags")
		} else {
			args = append(args, "--no-tags")
		}
	}

	stdout, stderr, err := b.runGit(ctx, opts.RepoPath, args, opts.Auth)
	if err != nil {
		return nil, fmt.Errorf("git fetch: %w, stderr: %s", err, stderr)
	}

	result := &FetchResult{}
	result.FetchedRefs = parseFetchRefs(stdout + stderr)
	return result, nil
}

func (b *NativeGitBackend) Push(ctx context.Context, opts PushOptions) (*PushResult, error) {
	args := []string{"push", opts.Remote}

	if opts.Force {
		args = append(args, "--force")
	}
	if opts.Mirror {
		args = []string{"push", "--mirror", opts.Remote}
	}

	if !opts.Mirror {
		args = append(args, opts.RefSpecs...)
	}

	stdout, stderr, err := b.runGit(ctx, opts.RepoPath, args, opts.Auth)
	if err != nil {
		return nil, fmt.Errorf("git push: %w, stderr: %s", err, stderr)
	}

	result := &PushResult{}
	result.PushedRefs = parsePushRefs(stdout + stderr)
	return result, nil
}

func (b *NativeGitBackend) ListRemoteBranches(ctx context.Context, repoPath, remote string) ([]string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{"branch", "-r", "--list", remote + "/*"}, AuthConfig{})
	if err != nil {
		return nil, fmt.Errorf("git branch -r: %w, stderr: %s", err, stderr)
	}

	branches := []string{}
	prefix := remote + "/"
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, prefix)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func (b *NativeGitBackend) GetCommitsBetween(ctx context.Context, repoPath, from, to string) ([]CommitInfo, error) {
	rangeArg := fmt.Sprintf("%s..%s", from, to)
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{
		"log", rangeArg, "--pretty=format:%H|%s|%an|%ai",
	}, AuthConfig{})
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w, stderr: %s", rangeArg, err, stderr)
	}

	commits := []CommitInfo{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, CommitInfo{
			Hash:    parts[0],
			Message: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return commits, nil
}

func (b *NativeGitBackend) IsAncestor(ctx context.Context, repoPath, ancestor, descendant string) (bool, error) {
	_, _, err := b.runGit(ctx, repoPath, []string{
		"merge-base", "--is-ancestor", ancestor, descendant,
	}, AuthConfig{})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (b *NativeGitBackend) runGit(ctx context.Context, repoPath string, args []string, auth AuthConfig) (string, string, error) {
	cmd := exec.CommandContext(ctx, b.gitPath, args...)
	cmd.Dir = repoPath

	b.configureAuth(cmd, auth)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func (b *NativeGitBackend) configureAuth(cmd *exec.Cmd, auth AuthConfig) {
	switch auth.Type {
	case "http_basic", "http_token":
		token := auth.Token
		if token == "" && auth.Password != "" {
			token = auth.Password
		}
		if token != "" {
			if cmd.Env == nil {
				cmd.Env = make([]string, 0)
			}
		}
	case "ssh":
		if auth.SSHKey != "" {
			if cmd.Env == nil {
				cmd.Env = make([]string, 0)
			}
		}
	}
}

func parseFetchRefs(output string) []string {
	var refs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, " ") && strings.Contains(line, "->") {
			parts := strings.SplitN(line, "->", 2)
			if len(parts) == 2 {
				refs = append(refs, strings.TrimSpace(parts[1]))
			}
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
				parts := strings.Fields(line)
				for _, p := range parts {
					if strings.Contains(p, ":") || strings.Contains(p, "..") {
						refs = append(refs, p)
					}
				}
			}
		}
	}
	return refs
}
