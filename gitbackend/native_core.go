package gitbackend

import (
	"context"
	"fmt"
)

// --- Core operations ---

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
	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}
	if opts.InsecureSkipTLS {
		args = append([]string{"-c", "http.sslVerify=false"}, args...)
	}
	if len(opts.Branches) > 0 {
		// Filter out SHA hashes - they can't be used directly as refspecs
		branchArgs := make([]string, 0, len(opts.Branches))
		for _, branch := range opts.Branches {
			if !isCommitSHA(branch) {
				branchArgs = append(branchArgs, branch)
			}
		}
		if len(branchArgs) > 0 {
			args = append(args, branchArgs...)
		}
	}

	stdout, stderr, err := b.runGit(ctx, opts.RepoPath, args, opts.Auth)
	if err != nil {
		return nil, newGitError("Fetch", opts.RepoPath, stderr, err)
	}
	return &FetchResult{FetchedRefs: parseFetchRefs(stdout + stderr)}, nil
}

func (b *NativeGitBackend) Push(ctx context.Context, opts PushOptions) (*PushResult, error) {
	args := []string{"push", opts.Remote}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.Mirror {
		args = []string{"push", "--mirror", opts.Remote}
	} else {
		args = append(args, opts.RefSpecs...)
	}
	if opts.InsecureSkipTLS {
		args = append([]string{"-c", "http.sslVerify=false"}, args...)
	}

	stdout, stderr, err := b.runGit(ctx, opts.RepoPath, args, opts.Auth)
	if err != nil {
		return nil, newGitError("Push", opts.RepoPath, stderr, err)
	}
	return &PushResult{PushedRefs: parsePushRefs(stdout + stderr)}, nil
}

func (b *NativeGitBackend) Clone(ctx context.Context, opts CloneOptions) error {
	args := []string{"clone"}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}
	if opts.NoCheckout {
		args = append(args, "--no-checkout")
	}
	if opts.InsecureSkipTLS {
		args = append([]string{"-c", "http.sslVerify=false"}, args...)
	}
	args = append(args, opts.URL, opts.Path)

	_, stderr, err := b.runGit(ctx, "", args, opts.Auth)
	if err != nil {
		return newGitError("Clone", opts.Path, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) Init(ctx context.Context, repoPath string) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"init"}, AuthConfig{})
	if err != nil {
		return newGitError("Init", repoPath, stderr, err)
	}
	return nil
}

// --- Extended core operations ---

func (b *NativeGitBackend) FetchAll(ctx context.Context, repoPath string, auth AuthConfig) error {
	_, stderr, err := b.runGit(ctx, repoPath, []string{"fetch", "--all", "--tags"}, auth)
	if err != nil {
		return newGitError("FetchAll", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) Pull(ctx context.Context, repoPath, remote, branch string, auth AuthConfig) error {
	args := []string{"pull", remote}
	if branch != "" {
		args = append(args, branch)
	}
	_, stderr, err := b.runGit(ctx, repoPath, args, auth)
	if err != nil {
		return newGitError("Pull", repoPath, stderr, err)
	}
	return nil
}

func (b *NativeGitBackend) RunRaw(ctx context.Context, repoPath string, args []string) (string, string, error) {
	return b.runGit(ctx, repoPath, args, AuthConfig{})
}

func (b *NativeGitBackend) TestConnection(ctx context.Context, url string, auth AuthConfig) error {
	_, stderr, err := b.runGit(ctx, "", []string{"ls-remote", "--heads", url}, auth)
	if err != nil {
		return newGitError("TestConnection", "", stderr, err)
	}
	return nil
}
