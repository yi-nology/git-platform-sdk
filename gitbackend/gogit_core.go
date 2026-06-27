package gitbackend

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// --- Core operations ---

func (b *GoGitBackend) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	repo, err := git.PlainOpen(opts.RepoPath)
	if err != nil {
		return nil, newGitError("Fetch", opts.RepoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	fetchOpts := &git.FetchOptions{
		RemoteName:      opts.Remote,
		RefSpecs:        buildFetchRefSpecs(opts),
		Auth:            b.buildTransportAuth(opts.Auth),
		Progress:        opts.Progress,
		Tags:            git.NoTags,
		Prune:           opts.Prune,
		Depth:           opts.Depth,
		InsecureSkipTLS: opts.InsecureSkipTLS || opts.Auth.InsecureSkipTLS,
	}
	if opts.Tags {
		fetchOpts.Tags = git.AllTags
	}

	err = repo.FetchContext(ctx, fetchOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, newGitError("Fetch", opts.RepoPath, "", err)
	}
	return &FetchResult{}, nil
}

func (b *GoGitBackend) Push(ctx context.Context, opts PushOptions) (*PushResult, error) {
	repo, err := git.PlainOpen(opts.RepoPath)
	if err != nil {
		return nil, newGitError("Push", opts.RepoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	refSpecs := opts.RefSpecs
	if opts.Mirror {
		refSpecs = []string{"+refs/*:refs/*"}
	}
	if len(refSpecs) == 0 {
		refSpecs = []string{"refs/heads/*:refs/heads/*"}
	}

	rs := make([]config.RefSpec, 0, len(refSpecs))
	for _, s := range refSpecs {
		rs = append(rs, config.RefSpec(s))
	}

	pushOpts := &git.PushOptions{
		RemoteName:      opts.Remote,
		RefSpecs:        rs,
		Auth:            b.buildTransportAuth(opts.Auth),
		Progress:        opts.Progress,
		Force:           opts.Force,
		InsecureSkipTLS: opts.InsecureSkipTLS || opts.Auth.InsecureSkipTLS,
	}

	err = repo.PushContext(ctx, pushOpts)
	if err != nil {
		return nil, newGitError("Push", opts.RepoPath, "", err)
	}
	return &PushResult{PushedRefs: refSpecs}, nil
}

func (b *GoGitBackend) Clone(ctx context.Context, opts CloneOptions) error {
	cloneOpts := &git.CloneOptions{
		URL:             opts.URL,
		ReferenceName:   plumbing.ReferenceName(opts.Branch),
		Progress:        opts.Progress,
		Auth:            b.buildTransportAuth(opts.Auth),
		NoCheckout:      opts.NoCheckout,
		SingleBranch:    opts.SingleBranch,
		InsecureSkipTLS: opts.InsecureSkipTLS || opts.Auth.InsecureSkipTLS,
	}
	if opts.Depth > 0 {
		cloneOpts.Depth = opts.Depth
	}

	_, err := git.PlainCloneContext(ctx, opts.Path, false, cloneOpts)
	if err != nil {
		return newGitError("Clone", opts.Path, "", err)
	}
	return nil
}

func (b *GoGitBackend) Init(ctx context.Context, repoPath string) error {
	_, err := git.PlainInit(repoPath, false)
	if err != nil {
		return newGitError("Init", repoPath, "", err)
	}
	return nil
}

// --- Extended core operations ---

func (b *GoGitBackend) FetchAll(ctx context.Context, repoPath string, auth AuthConfig) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("FetchAll", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	remotes, err := repo.Remotes()
	if err != nil {
		return newGitError("FetchAll", repoPath, "", err)
	}
	for _, r := range remotes {
		err = repo.FetchContext(ctx, &git.FetchOptions{
			RemoteName:      r.Config().Name,
			Auth:            b.buildTransportAuth(auth),
			Tags:            git.AllTags,
			InsecureSkipTLS: auth.InsecureSkipTLS,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return newGitError("FetchAll", repoPath, "", err)
		}
	}
	return nil
}

func (b *GoGitBackend) Pull(ctx context.Context, repoPath, remote, branch string, auth AuthConfig) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("Pull", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("Pull", repoPath, "", err)
	}
	err = wt.PullContext(ctx, &git.PullOptions{
		RemoteName:      remote,
		ReferenceName:   plumbing.ReferenceName("refs/heads/" + branch),
		Auth:            b.buildTransportAuth(auth),
		InsecureSkipTLS: auth.InsecureSkipTLS,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return newGitError("Pull", repoPath, "", err)
	}
	return nil
}

func (b *GoGitBackend) RunRaw(ctx context.Context, repoPath string, args []string) (string, string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", "", newGitError("RunRaw", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	_ = repo // verify it's a git repo
	// go-git doesn't support arbitrary commands; this is a no-op stub.
	// Use the native backend for RunRaw.
	return "", "", newGitError("RunRaw", repoPath, "", fmt.Errorf("RunRaw not supported in gogit backend, use native backend"))
}

func (b *GoGitBackend) TestConnection(ctx context.Context, url string, auth AuthConfig) error {
	remote := git.NewRemote(nil, &config.RemoteConfig{
		Name: "test",
		URLs: []string{url},
	})
	_, err := remote.List(&git.ListOptions{
		Auth:            b.buildTransportAuth(auth),
		InsecureSkipTLS: auth.InsecureSkipTLS,
	})
	if err != nil {
		return newGitError("TestConnection", "", "", err)
	}
	return nil
}
