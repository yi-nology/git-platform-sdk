package gitbackend

import (
	"context"
	"fmt"
	"strings"
)

// --- Core operations ---

func (b *NativeGitBackend) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}

	// Snapshot the refs a fetch can move before running it, so the result can
	// be derived from a before/after diff exactly like the gogit backend.
	before, err := b.snapshotFetchRefs(ctx, opts.RepoPath, remote)
	if err != nil {
		return nil, newGitError("Fetch", opts.RepoPath, "", err)
	}

	// --prune is always passed so DeletedBranch is actually reachable: without
	// pruning, a branch deleted on the remote leaves its remote-tracking ref
	// in place and the before/after diff shows no deletion.
	args := []string{"fetch", "--prune", remote}
	if opts.Tags {
		args = append(args, "--tags")
	} else {
		args = append(args, "--no-tags")
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
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

	auth := mergeInsecure(opts.Auth, opts.InsecureSkipTLS)
	_, stderr, err := b.runGit(ctx, opts.RepoPath, args, auth)
	if err != nil {
		return nil, newGitError("Fetch", opts.RepoPath, stderr, err)
	}

	after, err := b.snapshotFetchRefs(ctx, opts.RepoPath, remote)
	if err != nil {
		return nil, newGitError("Fetch", opts.RepoPath, "", err)
	}
	return diffFetchRefs(remote, before, after), nil
}

// snapshotFetchRefs snapshots the refs a fetch can move — the remote's
// remote-tracking refs and all tags — as refname→hash. It mirrors the gogit
// backend's collectFetchRefs.
func (b *NativeGitBackend) snapshotFetchRefs(ctx context.Context, repoPath, remote string) (map[string]string, error) {
	stdout, stderr, err := b.runGit(ctx, repoPath, []string{
		"for-each-ref", "--format=%(refname) %(objectname)",
		"refs/remotes/" + remote + "/", "refs/tags/",
	}, AuthConfig{})
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref: %w: %s", err, strings.TrimSpace(stderr))
	}
	refs := make(map[string]string)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format is "%(refname) %(objectname)": refnames cannot contain
		// spaces, so everything before the last space is the refname.
		idx := strings.LastIndexByte(line, ' ')
		if idx <= 0 {
			continue
		}
		refs[line[:idx]] = line[idx+1:]
	}
	return refs, nil
}

// diffFetchRefs classifies a before/after pair of ref snapshots into a
// FetchResult, applying the same rules as the gogit backend: refs created by
// the fetch go to FetchedRefs + NewBranches/NewTags, refs that moved go to
// FetchedRefs + UpdatedBranch, and pruned remote-tracking refs go to
// DeletedBranch. NewBranches/UpdatedBranch/DeletedBranch carry short names
// (after the refs/remotes/<remote>/ or refs/tags/ prefix); FetchedRefs keeps
// the full refname of every ref the fetch created or moved.
func diffFetchRefs(remote string, before, after map[string]string) *FetchResult {
	remotePrefix := "refs/remotes/" + remote + "/"
	tagsPrefix := "refs/tags/"

	result := &FetchResult{}

	for ref, hash := range after {
		oldHash, existed := before[ref]
		switch {
		case !existed:
			// New ref after fetch.
			result.FetchedRefs = append(result.FetchedRefs, ref)
			switch {
			case strings.HasPrefix(ref, remotePrefix):
				result.NewBranches = append(result.NewBranches, strings.TrimPrefix(ref, remotePrefix))
			case strings.HasPrefix(ref, tagsPrefix):
				result.NewTags = append(result.NewTags, strings.TrimPrefix(ref, tagsPrefix))
			}
		case oldHash != hash:
			// Existing ref moved to a different commit.
			result.FetchedRefs = append(result.FetchedRefs, ref)
			if strings.HasPrefix(ref, remotePrefix) {
				result.UpdatedBranch = append(result.UpdatedBranch, strings.TrimPrefix(ref, remotePrefix))
			}
		}
	}

	for ref := range before {
		// Only remote-tracking refs can be pruned away by a fetch; tag refs
		// are never pruned.
		if _, exists := after[ref]; !exists && strings.HasPrefix(ref, remotePrefix) {
			result.DeletedBranch = append(result.DeletedBranch, strings.TrimPrefix(ref, remotePrefix))
		}
	}

	return result
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

	auth := mergeInsecure(opts.Auth, opts.InsecureSkipTLS)
	stdout, stderr, err := b.runGit(ctx, opts.RepoPath, args, auth)
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
	args = append(args, opts.URL, opts.Path)

	auth := mergeInsecure(opts.Auth, opts.InsecureSkipTLS)
	_, stderr, err := b.runGit(ctx, "", args, auth)
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
	// One full fetch across all configured remotes: git itself walks the
	// remotes in a single subprocess (no per-branch round trips) and --prune
	// keeps remote-tracking refs aligned so deletions surface.
	_, stderr, err := b.runGit(ctx, repoPath, []string{"fetch", "--all", "--tags", "--prune"}, auth)
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
