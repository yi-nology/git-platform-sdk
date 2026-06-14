package gitbackend

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	xhttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	xssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/utils/merkletrie"
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

	fetchOpts := &git.FetchOptions{
		RemoteName: opts.Remote,
		RefSpecs:   buildFetchRefSpecs(opts),
		Auth:       b.buildTransportAuth(opts.Auth),
		Progress:   opts.Progress,
		Tags:       git.NoTags,
		Prune:      opts.Prune,
		Depth:      opts.Depth,
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
		RemoteName: opts.Remote,
		RefSpecs:   rs,
		Auth:       b.buildTransportAuth(opts.Auth),
		Progress:   opts.Progress,
		Force:      opts.Force,
	}

	err = repo.PushContext(ctx, pushOpts)
	if err != nil {
		return nil, newGitError("Push", opts.RepoPath, "", err)
	}
	return &PushResult{PushedRefs: refSpecs}, nil
}

func (b *GoGitBackend) Clone(ctx context.Context, opts CloneOptions) error {
	cloneOpts := &git.CloneOptions{
		URL:           opts.URL,
		ReferenceName: plumbing.ReferenceName(opts.Branch),
		Progress:      opts.Progress,
		Auth:          b.buildTransportAuth(opts.Auth),
		NoCheckout:    opts.NoCheckout,
		SingleBranch:  opts.SingleBranch,
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

	head, err := repo.Head()
	if err != nil {
		return newGitError("Merge", repoPath, "", err)
	}

	ref, err := repo.Reference(plumbing.ReferenceName("refs/heads/"+branch), true)
	if err != nil {
		return newGitError("Merge", repoPath, "", ErrBranchNotFound)
	}

	if head.Hash() == ref.Hash() {
		return nil
	}

	headCommit, _ := repo.CommitObject(head.Hash())
	branchCommit, _ := repo.CommitObject(ref.Hash())

	if headCommit != nil && branchCommit != nil {
		isAncestor, _ := headCommit.IsAncestor(branchCommit)
		if isAncestor {
			newRef := plumbing.NewHashReference(head.Name(), ref.Hash())
			return repo.Storer.SetReference(newRef)
		}
	}

	return newGitError("Merge", repoPath, "", fmt.Errorf("non-fast-forward merge not supported in go-git, use native backend"))
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

	var hash plumbing.Hash
	if ref == "" || ref == "HEAD" {
		head, err := repo.Head()
		if err != nil {
			return nil, newGitError("GetFileAtRevision", repoPath, "", err)
		}
		hash = head.Hash()
	} else {
		hash = plumbing.NewHash(ref)
	}

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
		if auth.SSHKey != "" {
			if _, err := os.Stat(auth.SSHKey); err == nil {
				signer, err := xssh.NewPublicKeysFromFile("git", auth.SSHKey, "")
				if err == nil {
					return signer
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

// --- Advanced operations ---

func (b *GoGitBackend) RevParse(ctx context.Context, repoPath, ref string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", newGitError("RevParse", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return "", newGitError("RevParse", repoPath, "", err)
	}
	return hash.String(), nil
}

func (b *GoGitBackend) MergeBase(ctx context.Context, repoPath, a, other string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", newGitError("MergeBase", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	commitA, err := repo.CommitObject(plumbing.NewHash(a))
	if err != nil {
		return "", newGitError("MergeBase", repoPath, "", err)
	}
	commitB, err := repo.CommitObject(plumbing.NewHash(other))
	if err != nil {
		return "", newGitError("MergeBase", repoPath, "", err)
	}
	bases, err := commitA.MergeBase(commitB)
	if err != nil {
		return "", newGitError("MergeBase", repoPath, "", err)
	}
	if len(bases) == 0 {
		return "", nil
	}
	return bases[0].Hash.String(), nil
}

func (b *GoGitBackend) DiffNames(ctx context.Context, repoPath, from, to string) ([]string, error) {
	changes, err := b.treeChanges(repoPath, from, to)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(changes))
	for _, c := range changes {
		result = append(result, changePath(c))
	}
	return result, nil
}

func (b *GoGitBackend) DeletedFiles(ctx context.Context, repoPath, from, to string) ([]string, error) {
	changes, err := b.treeChanges(repoPath, from, to)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, c := range changes {
		action, err := c.Action()
		if err != nil {
			continue
		}
		if action == merkletrie.Delete {
			result = append(result, c.From.Name)
		}
	}
	return result, nil
}

func (b *GoGitBackend) CheckoutRef(ctx context.Context, repoPath, ref string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("CheckoutRef", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return newGitError("CheckoutRef", repoPath, "", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("CheckoutRef", repoPath, "", err)
	}
	return wt.Checkout(&git.CheckoutOptions{Hash: *hash, Force: true})
}

func (b *GoGitBackend) CheckoutFiles(ctx context.Context, repoPath, ref string, files []string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("CheckoutFiles", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return newGitError("CheckoutFiles", repoPath, "", err)
	}
	commitObj, err := repo.CommitObject(*hash)
	if err != nil {
		return newGitError("CheckoutFiles", repoPath, "", err)
	}
	tree, err := commitObj.Tree()
	if err != nil {
		return newGitError("CheckoutFiles", repoPath, "", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("CheckoutFiles", repoPath, "", err)
	}
	var lastErr error
	for _, file := range files {
		treeFile, err := tree.File(file)
		if err != nil {
			lastErr = fmt.Errorf("file %s not found in tree: %w", file, err)
			continue
		}
		reader, err := treeFile.Blob.Reader()
		if err != nil {
			lastErr = fmt.Errorf("read blob for %s: %w", file, err)
			continue
		}
		fullPath := filepath.Join(repoPath, file)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0o755)
		f, err := os.Create(fullPath)
		if err != nil {
			_ = reader.Close()
			lastErr = fmt.Errorf("create file %s: %w", file, err)
			continue
		}
		_, copyErr := io.Copy(f, reader)
		_ = f.Close()
		_ = reader.Close()
		if copyErr != nil {
			lastErr = fmt.Errorf("write file %s: %w", file, copyErr)
			continue
		}
		if _, err := wt.Add(file); err != nil {
			lastErr = fmt.Errorf("git add %s: %w", file, err)
		}
	}
	return lastErr
}

func (b *GoGitBackend) Add(ctx context.Context, repoPath string, files []string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("Add", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("Add", repoPath, "", err)
	}
	for _, file := range files {
		if _, err := wt.Add(file); err != nil {
			return newGitError("Add", repoPath, "", fmt.Errorf("git add %s: %w", file, err))
		}
	}
	return nil
}

func (b *GoGitBackend) CommitWithIdentity(ctx context.Context, repoPath, name, email, message string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("CommitWithIdentity", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("CommitWithIdentity", repoPath, "", err)
	}
	_, err = wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  name,
			Email: email,
			When:  time.Now(),
		},
		AllowEmptyCommits: true,
	})
	if err != nil {
		return newGitError("CommitWithIdentity", repoPath, "", err)
	}
	return nil
}

// --- Internal helpers for advanced operations ---

func (b *GoGitBackend) treeChanges(repoPath, from, to string) (object.Changes, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("treeChanges", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	commitFrom, err := repo.CommitObject(plumbing.NewHash(from))
	if err != nil {
		return nil, newGitError("treeChanges", repoPath, "", err)
	}
	commitTo, err := repo.CommitObject(plumbing.NewHash(to))
	if err != nil {
		return nil, newGitError("treeChanges", repoPath, "", err)
	}
	treeFrom, err := commitFrom.Tree()
	if err != nil {
		return nil, newGitError("treeChanges", repoPath, "", err)
	}
	treeTo, err := commitTo.Tree()
	if err != nil {
		return nil, newGitError("treeChanges", repoPath, "", err)
	}
	return object.DiffTree(treeFrom, treeTo)
}

func changePath(c *object.Change) string {
	if c.To.Name != "" {
		return c.To.Name
	}
	return c.From.Name
}
