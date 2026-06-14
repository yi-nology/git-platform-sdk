package gitbackend

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/object"
	xhttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	xssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
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

// --- Core operations ---

func (b *GoGitBackend) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	repo, err := git.PlainOpen(opts.RepoPath)
	if err != nil {
		return nil, newGitError("Fetch", opts.RepoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	clientOpts := b.buildClientOptions(opts.Auth)
	if opts.InsecureSkipTLS {
		clientOpts = append(clientOpts, client.WithInsecureSkipTLS())
	}

	fetchOpts := &git.FetchOptions{
		RemoteName:      opts.Remote,
		RefSpecs:        buildFetchRefSpecs(opts),
		ClientOptions:   clientOpts,
		Progress:        opts.Progress,
		Tags:            git.NoTags,
		Prune:           opts.Prune,
		Depth:           opts.Depth,
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

	clientOpts := b.buildClientOptions(opts.Auth)
	if opts.InsecureSkipTLS {
		clientOpts = append(clientOpts, client.WithInsecureSkipTLS())
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
		RemoteName:    opts.Remote,
		RefSpecs:      rs,
		ClientOptions: clientOpts,
		Progress:      opts.Progress,
		Force:         opts.Force,
	}

	err = repo.PushContext(ctx, pushOpts)
	if err != nil {
		return nil, newGitError("Push", opts.RepoPath, "", err)
	}
	return &PushResult{PushedRefs: refSpecs}, nil
}

func (b *GoGitBackend) Clone(ctx context.Context, opts CloneOptions) error {
	clientOpts := b.buildClientOptions(opts.Auth)
	if opts.InsecureSkipTLS {
		clientOpts = append(clientOpts, client.WithInsecureSkipTLS())
	}

	cloneOpts := &git.CloneOptions{
		URL:           opts.URL,
		ReferenceName: plumbing.ReferenceName(opts.Branch),
		Progress:      opts.Progress,
		ClientOptions: clientOpts,
		NoCheckout:    opts.NoCheckout,
		SingleBranch:  opts.SingleBranch,
	}
	if opts.Depth > 0 {
		cloneOpts.Depth = opts.Depth
	}

	_, err := git.PlainCloneContext(ctx, opts.Path, cloneOpts)
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

// --- Branch operations ---

func (b *GoGitBackend) ListRemoteBranches(ctx context.Context, repoPath, remote string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("ListRemoteBranches", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	remoteObj, err := repo.Remote(remote)
	if err != nil {
		return nil, newGitError("ListRemoteBranches", repoPath, "", ErrRemoteNotFound)
	}

	refs, err := remoteObj.List(&git.ListOptions{})
	if err != nil {
		return nil, newGitError("ListRemoteBranches", repoPath, "", err)
	}

	var result []string
	prefix := "refs/heads/"
	for _, ref := range refs {
		name := ref.Name().String()
		if strings.HasPrefix(name, prefix) {
			result = append(result, strings.TrimPrefix(name, prefix))
		}
	}
	return result, nil
}

func (b *GoGitBackend) CreateBranch(ctx context.Context, repoPath, branch, ref string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("CreateBranch", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	hash := plumbing.NewHash(ref)
	if hash.IsZero() {
		head, err := repo.Head()
		if err != nil {
			return newGitError("CreateBranch", repoPath, "", err)
		}
		hash = head.Hash()
	} else if !isCommitSHA(ref) {
		// Try as branch name
		headRef, err := repo.Reference(plumbing.ReferenceName("refs/heads/"+ref), true)
		if err == nil {
			hash = headRef.Hash()
		}
	}

	newRef := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+branch), hash)
	err = repo.Storer.SetReference(newRef)
	if err != nil {
		return newGitError("CreateBranch", repoPath, "", err)
	}
	return nil
}

func (b *GoGitBackend) DeleteBranch(ctx context.Context, repoPath, branch string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("DeleteBranch", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	err = repo.Storer.RemoveReference(plumbing.ReferenceName("refs/heads/" + branch))
	if err != nil {
		return newGitError("DeleteBranch", repoPath, "", ErrBranchNotFound)
	}
	return nil
}

func (b *GoGitBackend) Checkout(ctx context.Context, repoPath, branch string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("Checkout", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("Checkout", repoPath, "", err)
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branch),
	})
	if err != nil {
		return newGitError("Checkout", repoPath, "", err)
	}
	return nil
}

func (b *GoGitBackend) GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", newGitError("GetCurrentBranch", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	head, err := repo.Head()
	if err != nil {
		return "", newGitError("GetCurrentBranch", repoPath, "", err)
	}

	name := head.Name().String()
	if strings.HasPrefix(name, "refs/heads/") {
		return strings.TrimPrefix(name, "refs/heads/"), nil
	}
	return name, nil
}

// --- Status and diff ---

func (b *GoGitBackend) GetStatus(ctx context.Context, repoPath string) (*RepoStatus, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("GetStatus", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	status := &RepoStatus{}

	branch, err := b.GetCurrentBranch(ctx, repoPath)
	if err == nil {
		status.Branch = branch
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, newGitError("GetStatus", repoPath, "", err)
	}

	wtStatus, err := wt.Status()
	if err != nil {
		return nil, newGitError("GetStatus", repoPath, "", err)
	}

	for path, fileStatus := range wtStatus {
		fs := FileStatus{
			Path:     path,
			Staging:  byte(fileStatus.Staging),
			Worktree: byte(fileStatus.Worktree),
		}
		if fileStatus.Staging == git.Untracked {
			status.Untracked = append(status.Untracked, path)
		} else {
			if fileStatus.Staging != git.Unmodified {
				status.Staged = append(status.Staged, fs)
			}
			if fileStatus.Worktree != git.Unmodified {
				status.Unstaged = append(status.Unstaged, fs)
			}
		}
	}

	status.IsClean = len(status.Staged) == 0 && len(status.Unstaged) == 0 && len(status.Untracked) == 0
	return status, nil
}

func (b *GoGitBackend) Diff(ctx context.Context, repoPath string, opts DiffOptions) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", newGitError("Diff", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	// If no From/To specified, diff working tree against HEAD
	if opts.From == "" && opts.To == "" {
		wt, err := repo.Worktree()
		if err != nil {
			return "", newGitError("Diff", repoPath, "", err)
		}
		status, err := wt.Status()
		if err != nil {
			return "", newGitError("Diff", repoPath, "", err)
		}
		return status.String(), nil
	}

	// Diff between two commits
	fromCommit, err := repo.CommitObject(plumbing.NewHash(opts.From))
	if err != nil {
		return "", newGitError("Diff", repoPath, "", err)
	}

	toCommit, err := repo.CommitObject(plumbing.NewHash(opts.To))
	if err != nil {
		return "", newGitError("Diff", repoPath, "", err)
	}

	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", newGitError("Diff", repoPath, "", err)
	}

	toTree, err := toCommit.Tree()
	if err != nil {
		return "", newGitError("Diff", repoPath, "", err)
	}

	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return "", newGitError("Diff", repoPath, "", err)
	}

	var result strings.Builder
	for _, change := range changes {
		patch, err := change.Patch()
		if err != nil {
			continue
		}
		result.WriteString(patch.String())
	}
	return result.String(), nil
}

// --- Commit operations ---

func (b *GoGitBackend) GetCommitsBetween(ctx context.Context, repoPath, from, to string) ([]CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("GetCommitsBetween", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	fromHash := plumbing.NewHash(from)
	toHash := plumbing.NewHash(to)

	commitIter, err := repo.Log(&git.LogOptions{From: toHash})
	if err != nil {
		return nil, newGitError("GetCommitsBetween", repoPath, "", err)
	}
	defer commitIter.Close()

	var commits []CommitInfo
	err = commitIter.ForEach(func(c *object.Commit) error {
		if c.Hash == fromHash {
			return io.EOF
		}
		commits = append(commits, CommitInfo{
			Hash:    c.Hash.String(),
			Message: strings.TrimRight(c.Message, "\n"),
			Author:  c.Author.Name,
			Date:    c.Author.When.Format(time.RFC3339),
		})
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, newGitError("GetCommitsBetween", repoPath, "", err)
	}
	return commits, nil
}

func (b *GoGitBackend) IsAncestor(ctx context.Context, repoPath, ancestor, descendant string) (bool, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return false, newGitError("IsAncestor", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	ancestorCommit, err := repo.CommitObject(plumbing.NewHash(ancestor))
	if err != nil {
		return false, newGitError("IsAncestor", repoPath, "", err)
	}
	descendantCommit, err := repo.CommitObject(plumbing.NewHash(descendant))
	if err != nil {
		return false, newGitError("IsAncestor", repoPath, "", err)
	}
	return ancestorCommit.IsAncestor(descendantCommit)
}

func (b *GoGitBackend) Merge(ctx context.Context, repoPath, branch string, opts MergeOptions) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("Merge", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	// Get current HEAD
	head, err := repo.Head()
	if err != nil {
		return newGitError("Merge", repoPath, "", err)
	}

	// Get target branch commit
	ref, err := repo.Reference(plumbing.ReferenceName("refs/heads/"+branch), true)
	if err != nil {
		return newGitError("Merge", repoPath, "", ErrBranchNotFound)
	}

	// Already up to date
	if head.Hash() == ref.Hash() {
		return nil
	}

	// Check if ancestor (fast-forward possible)
	headCommit, _ := repo.CommitObject(head.Hash())
	branchCommit, _ := repo.CommitObject(ref.Hash())

	if headCommit != nil && branchCommit != nil {
		isAncestor, _ := headCommit.IsAncestor(branchCommit)
		if isAncestor {
			// Fast-forward: just move HEAD
			newRef := plumbing.NewHashReference(head.Name(), ref.Hash())
			return repo.Storer.SetReference(newRef)
		}
	}

	// Full merge using CherryPick
	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("Merge", repoPath, "", err)
	}

	commitOpts := &git.CommitOptions{
		AllowEmptyCommits: true,
	}

	ortStrategy := git.TheirsMergeStrategy
	if opts.Squash {
		ortStrategy = git.OursMergeStrategy
	}

	err = wt.CherryPick(commitOpts, ortStrategy, branchCommit)
	if err != nil {
		if strings.Contains(err.Error(), "conflict") || strings.Contains(err.Error(), "CONFLICT") {
			return newGitError("Merge", repoPath, "", ErrMergeConflict)
		}
		return newGitError("Merge", repoPath, "", err)
	}
	return nil
}

// --- Remote operations ---

func (b *GoGitBackend) AddRemote(ctx context.Context, repoPath, name, url string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("AddRemote", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{url},
	})
	if err != nil {
		return newGitError("AddRemote", repoPath, "", err)
	}
	return nil
}

func (b *GoGitBackend) RemoveRemote(ctx context.Context, repoPath, name string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("RemoveRemote", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	err = repo.DeleteRemote(name)
	if err != nil {
		return newGitError("RemoveRemote", repoPath, "", err)
	}
	return nil
}

// --- Tag operations ---

func (b *GoGitBackend) CreateTag(ctx context.Context, repoPath, name, ref string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("CreateTag", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	hash := plumbing.NewHash(ref)
	if ref == "" || hash.IsZero() {
		head, err := repo.Head()
		if err != nil {
			return newGitError("CreateTag", repoPath, "", err)
		}
		hash = head.Hash()
	}

	_, err = repo.CreateTag(name, hash, nil)
	if err != nil {
		return newGitError("CreateTag", repoPath, "", err)
	}
	return nil
}

// --- File operations ---

func (b *GoGitBackend) GetFileAtRevision(ctx context.Context, repoPath, path, ref string) ([]byte, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("GetFileAtRevision", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	hash := plumbing.NewHash(ref)
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, newGitError("GetFileAtRevision", repoPath, "", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, newGitError("GetFileAtRevision", repoPath, "", err)
	}

	file, err := tree.File(path)
	if err != nil {
		return nil, newGitError("GetFileAtRevision", repoPath, "", ErrFileNotFound)
	}

	content, err := file.Contents()
	if err != nil {
		return nil, newGitError("GetFileAtRevision", repoPath, "", err)
	}
	return []byte(content), nil
}

// --- Auth helpers ---

// buildClientOptions creates client.Option slice from AuthConfig.
func (b *GoGitBackend) buildClientOptions(auth AuthConfig) []client.Option {
	switch auth.Type {
	case AuthHTTPBasic:
		return []client.Option{
			client.WithHTTPAuth(&xhttp.BasicAuth{
				Username: auth.Username,
				Password: auth.Password,
			}),
		}
	case AuthHTTPToken:
		return []client.Option{
			client.WithHTTPAuth(&xhttp.TokenAuth{
				Token: auth.Token,
			}),
		}
	case AuthSSH:
		if auth.SSHKey != "" {
			if _, err := os.Stat(auth.SSHKey); err == nil {
				signer, err := xssh.NewPublicKeysFromFile("git", auth.SSHKey, "")
				if err == nil {
					return []client.Option{client.WithSSHAuth(signer)}
				}
			}
		}
	}
	return nil
}

// --- Utilities ---

func buildFetchRefSpecs(opts FetchOptions) []config.RefSpec {
	if len(opts.Branches) == 0 {
		return []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/*:refs/remotes/%s/*", opts.Remote)),
		}
	}
	specs := make([]config.RefSpec, 0, len(opts.Branches))
	for _, branch := range opts.Branches {
		// Skip SHA hashes - refspecs require actual ref paths
		if isCommitSHA(branch) {
			continue
		}
		// Handle full ref prefix
		if strings.HasPrefix(branch, "refs/") {
			// Extract branch name from ref path for destination
			branchName := strings.TrimPrefix(branch, "refs/heads/")
			specs = append(specs, config.RefSpec(fmt.Sprintf("+%s:refs/remotes/%s/%s", branch, opts.Remote, branchName)))
		} else {
			specs = append(specs, config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", branch, opts.Remote, branch)))
		}
	}
	// If all were SHAs, fall back to fetching all branches
	if len(specs) == 0 {
		return []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/*:refs/remotes/%s/*", opts.Remote)),
		}
	}
	return specs
}

func isCommitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
