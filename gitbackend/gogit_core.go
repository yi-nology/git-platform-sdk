package gitbackend

import (
	"context"
	"fmt"
	"strings"

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

	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}

	// Snapshot remote refs before the fetch.
	before := make(map[string]plumbing.Hash)
	if err := collectRemoteRefs(repo, remote, before); err != nil {
		return nil, newGitError("Fetch", opts.RepoPath, "", err)
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

	// Snapshot remote refs after the fetch.
	after := make(map[string]plumbing.Hash)
	if err := collectRemoteRefs(repo, remote, after); err != nil {
		return nil, newGitError("Fetch", opts.RepoPath, "", err)
	}

	// Diff before/after to populate the result.
	remotePrefix := "refs/remotes/" + remote + "/"
	headsPrefix := remotePrefix + "heads/"
	tagsPrefix := remotePrefix + "tags/"

	result := &FetchResult{}

	for ref, hash := range after {
		if oldHash, existed := before[ref]; !existed {
			// New ref after fetch.
			result.FetchedRefs = append(result.FetchedRefs, ref)
			if strings.HasPrefix(ref, headsPrefix) {
				result.NewBranches = append(result.NewBranches, strings.TrimPrefix(ref, headsPrefix))
			} else if strings.HasPrefix(ref, tagsPrefix) {
				result.NewTags = append(result.NewTags, strings.TrimPrefix(ref, tagsPrefix))
			}
		} else if oldHash != hash {
			// Existing ref moved to a different commit.
			result.FetchedRefs = append(result.FetchedRefs, ref)
			if strings.HasPrefix(ref, headsPrefix) {
				result.UpdatedBranch = append(result.UpdatedBranch, strings.TrimPrefix(ref, headsPrefix))
			}
		}
	}

	for ref := range before {
		if _, exists := after[ref]; !exists {
			// Ref was pruned or deleted.
			result.DeletedBranch = append(result.DeletedBranch, ref)
		}
	}

	return result, nil
}

// collectRemoteRefs populates the target map with all remote-tracking refs
// (refs/remotes/<remote>/*) and their current hashes.
func collectRemoteRefs(repo *git.Repository, remote string, target map[string]plumbing.Hash) error {
	prefix := "refs/remotes/" + remote + "/"
	iter, err := repo.References()
	if err != nil {
		return err
	}
	return iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		if strings.HasPrefix(name, prefix) {
			target[name] = ref.Hash()
		}
		return nil
	})
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
	// go-git's PushContext does not expose per-ref success/failure details,
	// so we return the requested refspecs as PushedRefs. Callers that need
	// granular per-ref error reporting should use the native backend.
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

// RunRaw is not supported by the pure-Go (gogit) backend because go-git does
// not shell out to the git binary. This method exists to satisfy the
// GitBackend interface; callers that need arbitrary git commands should use
// the native backend instead.
func (b *GoGitBackend) RunRaw(ctx context.Context, repoPath string, args []string) (string, string, error) {
	_, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", "", newGitError("RunRaw", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	return "", "", newGitError("RunRaw", repoPath, "", fmt.Errorf("RunRaw not supported in gogit backend (pure-Go); use native backend for arbitrary git commands"))
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
