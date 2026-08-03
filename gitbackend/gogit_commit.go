package gitbackend

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

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

// resolveRef resolves a branch name, full ref, or commit hash to a plumbing.Hash.
func resolveRef(repo *git.Repository, ref string) (plumbing.Hash, error) {
	// Try as a local branch
	branchRef, err := repo.Reference(plumbing.ReferenceName("refs/heads/"+ref), true)
	if err == nil {
		return branchRef.Hash(), nil
	}
	// Try as a full reference
	fullRef, err := repo.Reference(plumbing.ReferenceName(ref), true)
	if err == nil {
		return fullRef.Hash(), nil
	}
	// Try as a commit hash
	hash := plumbing.NewHash(ref)
	_, err = repo.CommitObject(hash)
	if err == nil {
		return hash, nil
	}
	return plumbing.ZeroHash, fmt.Errorf("reference not found: %s", ref)
}

// applyChangesToWorktree computes the diff between baseTree and sourceTree and
// applies those changes to the working tree, staging each file.
func (b *GoGitBackend) applyChangesToWorktree(repoPath string, baseTree, sourceTree *object.Tree, wt *git.Worktree) error {
	changes, err := object.DiffTree(baseTree, sourceTree)
	if err != nil {
		return err
	}

	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			continue
		}

		switch action {
		case merkletrie.Insert, merkletrie.Modify:
			file, err := sourceTree.File(change.To.Name)
			if err != nil {
				continue
			}
			reader, err := file.Blob.Reader()
			if err != nil {
				continue
			}
			fullPath := filepath.Join(repoPath, change.To.Name)
			_ = os.MkdirAll(filepath.Dir(fullPath), 0o750)
			f, err := os.Create(fullPath)
			if err != nil {
				_ = reader.Close()
				continue
			}
			_, _ = io.Copy(f, reader)
			_ = f.Close()
			_ = reader.Close()
			_, _ = wt.Add(change.To.Name)

		case merkletrie.Delete:
			fullPath := filepath.Join(repoPath, change.From.Name)
			_ = os.Remove(fullPath)
			_, _ = wt.Remove(change.From.Name)
		}
	}
	return nil
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

	// Non-fast-forward merge
	if opts.FFOnly {
		return newGitError("Merge", repoPath, "", fmt.Errorf("not a fast-forward merge"))
	}

	// Find merge-base
	baseHash, err := b.MergeBase(ctx, repoPath, head.Hash().String(), ref.Hash().String())
	if err != nil || baseHash == "" {
		if !opts.AllowUnrelated {
			return newGitError("Merge", repoPath, "", fmt.Errorf("no common ancestor"))
		}
	}

	// Get trees
	headTree, err := headCommit.Tree()
	if err != nil {
		return newGitError("Merge", repoPath, "", err)
	}
	branchTree, err := branchCommit.Tree()
	if err != nil {
		return newGitError("Merge", repoPath, "", err)
	}

	var baseTree *object.Tree
	if baseHash != "" {
		baseCommit, err := repo.CommitObject(plumbing.NewHash(baseHash))
		if err != nil {
			return newGitError("Merge", repoPath, "", err)
		}
		baseTree, err = baseCommit.Tree()
		if err != nil {
			return newGitError("Merge", repoPath, "", err)
		}
	} else {
		// No common ancestor — use empty tree
		baseTree = &object.Tree{}
	}

	// Compute changes on both sides from the base
	baseToHead, err := object.DiffTree(baseTree, headTree)
	if err != nil {
		return newGitError("Merge", repoPath, "", err)
	}
	baseToBranch, err := object.DiffTree(baseTree, branchTree)
	if err != nil {
		return newGitError("Merge", repoPath, "", err)
	}

	// Check for conflicts: if both sides modify the same file it is a conflict.
	headChanges := make(map[string]bool)
	for _, change := range baseToHead {
		headChanges[changePath(change)] = true
	}
	for _, change := range baseToBranch {
		if headChanges[changePath(change)] {
			return newGitError("Merge", repoPath, "", ErrMergeConflict)
		}
	}

	// Apply changes from the branch side
	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("Merge", repoPath, "", err)
	}
	if err := b.applyChangesToWorktree(repoPath, headTree, branchTree, wt); err != nil {
		return newGitError("Merge", repoPath, "", err)
	}

	if opts.NoCommit {
		return nil
	}

	// Create merge commit
	message := opts.Message
	if message == "" {
		message = fmt.Sprintf("Merge branch '%s'", branch)
	}

	if opts.Squash {
		_, err = wt.Commit(message, &git.CommitOptions{
			Parents: []plumbing.Hash{head.Hash()},
		})
	} else {
		_, err = wt.Commit(message, &git.CommitOptions{
			Parents: []plumbing.Hash{head.Hash(), ref.Hash()},
		})
	}
	if err != nil {
		return newGitError("Merge", repoPath, "", err)
	}

	return nil
}

func (b *GoGitBackend) CherryPick(ctx context.Context, repoPath, commitHash string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("CherryPick", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	commit, err := repo.CommitObject(plumbing.NewHash(commitHash))
	if err != nil {
		return newGitError("CherryPick", repoPath, "", err)
	}

	commitTree, err := commit.Tree()
	if err != nil {
		return newGitError("CherryPick", repoPath, "", err)
	}

	parentCommit, err := commit.Parent(0)
	if err != nil {
		return newGitError("CherryPick", repoPath, "", err)
	}

	parentTree, err := parentCommit.Tree()
	if err != nil {
		return newGitError("CherryPick", repoPath, "", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("CherryPick", repoPath, "", err)
	}

	if err := b.applyChangesToWorktree(repoPath, parentTree, commitTree, wt); err != nil {
		return newGitError("CherryPick", repoPath, "", err)
	}

	_, err = wt.Commit(commit.Message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  commit.Author.Name,
			Email: commit.Author.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return newGitError("CherryPick", repoPath, "", err)
	}
	return nil
}

// rebaseStateDir returns the path to the rebase-merge state directory.
func rebaseStateDir(repoPath string) string {
	return filepath.Join(repoPath, ".git", "rebase-merge")
}

// rebaseStateFile reads a trimmed string from a rebase state file.
func rebaseStateFile(repoPath, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(rebaseStateDir(repoPath), name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// writeRebaseStateFile writes a string to a rebase state file.
func writeRebaseStateFile(repoPath, name, value string) error {
	return os.WriteFile(filepath.Join(rebaseStateDir(repoPath), name), []byte(value), 0o644)
}

// applyRebaseCommit applies a single commit during a rebase (or rebase --continue).
// It returns ErrMergeConflict when the cherry-pick cannot be applied cleanly.
func (b *GoGitBackend) applyRebaseCommit(repoPath string, repo *git.Repository, commitHash string) error {
	commit, err := repo.CommitObject(plumbing.NewHash(commitHash))
	if err != nil {
		return err
	}

	commitTree, err := commit.Tree()
	if err != nil {
		return err
	}

	head, err := repo.Head()
	if err != nil {
		return err
	}
	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return err
	}
	headTree, err := headCommit.Tree()
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	if err := b.applyChangesToWorktree(repoPath, headTree, commitTree, wt); err != nil {
		return ErrMergeConflict
	}

	_, err = wt.Commit(commit.Message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  commit.Author.Name,
			Email: commit.Author.Email,
			When:  time.Now(),
		},
	})
	return err
}

func (b *GoGitBackend) Rebase(ctx context.Context, repoPath, onto string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("Rebase", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	head, err := repo.Head()
	if err != nil {
		return newGitError("Rebase", repoPath, "", err)
	}

	ontoHash, err := resolveRef(repo, onto)
	if err != nil {
		return newGitError("Rebase", repoPath, "", ErrBranchNotFound)
	}

	if head.Hash() == ontoHash {
		return nil // already up to date
	}

	// Find merge-base
	baseHash, err := b.MergeBase(ctx, repoPath, head.Hash().String(), ontoHash.String())
	if err != nil || baseHash == "" {
		return newGitError("Rebase", repoPath, "", fmt.Errorf("no common ancestor"))
	}

	// Commits to replay: (base, HEAD]
	commits, err := b.GetCommitsBetween(ctx, repoPath, baseHash, head.Hash().String())
	if err != nil {
		return newGitError("Rebase", repoPath, "", err)
	}
	if len(commits) == 0 {
		return nil // nothing to rebase
	}

	// Save rebase state
	dir := rebaseStateDir(repoPath)
	_ = os.MkdirAll(dir, 0o750)
	_ = writeRebaseStateFile(repoPath, "orig-head", head.Hash().String())
	_ = writeRebaseStateFile(repoPath, "onto", ontoHash.String())
	_ = writeRebaseStateFile(repoPath, "head-name", head.Name().String())
	_ = writeRebaseStateFile(repoPath, "end", strconv.Itoa(len(commits)))

	// Move HEAD to onto
	if err := repo.Storer.SetReference(plumbing.NewHashReference(head.Name(), ontoHash)); err != nil {
		return newGitError("Rebase", repoPath, "", err)
	}

	// Re-play each commit
	for i, ci := range commits {
		_ = writeRebaseStateFile(repoPath, "msgnum", strconv.Itoa(i))

		if err := b.applyRebaseCommit(repoPath, repo, ci.Hash); err != nil {
			if err == ErrMergeConflict {
				return newGitError("Rebase", repoPath, "", ErrMergeConflict)
			}
			return newGitError("Rebase", repoPath, "", err)
		}
	}

	// Success — clean up state
	_ = os.RemoveAll(dir)
	return nil
}

func (b *GoGitBackend) RebaseAbort(ctx context.Context, repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("RebaseAbort", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	dir := rebaseStateDir(repoPath)

	origHead, err := rebaseStateFile(repoPath, "orig-head")
	if err != nil {
		return newGitError("RebaseAbort", repoPath, "", fmt.Errorf("no rebase in progress"))
	}

	headName, err := rebaseStateFile(repoPath, "head-name")
	if err != nil {
		return newGitError("RebaseAbort", repoPath, "", err)
	}

	// Restore original HEAD
	ref := plumbing.NewHashReference(plumbing.ReferenceName(headName), plumbing.NewHash(origHead))
	if err := repo.Storer.SetReference(ref); err != nil {
		return newGitError("RebaseAbort", repoPath, "", err)
	}

	_ = os.RemoveAll(dir)
	return nil
}

func (b *GoGitBackend) RebaseContinue(ctx context.Context, repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("RebaseContinue", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	dir := rebaseStateDir(repoPath)

	origHead, err := rebaseStateFile(repoPath, "orig-head")
	if err != nil {
		return newGitError("RebaseContinue", repoPath, "", fmt.Errorf("no rebase in progress"))
	}
	onto, err := rebaseStateFile(repoPath, "onto")
	if err != nil {
		return newGitError("RebaseContinue", repoPath, "", err)
	}
	msgnumStr, err := rebaseStateFile(repoPath, "msgnum")
	if err != nil {
		return newGitError("RebaseContinue", repoPath, "", err)
	}
	endStr, err := rebaseStateFile(repoPath, "end")
	if err != nil {
		return newGitError("RebaseContinue", repoPath, "", err)
	}

	msgnum, _ := strconv.Atoi(msgnumStr)
	end, _ := strconv.Atoi(endStr)

	// Get the full list of commits to replay
	commits, err := b.GetCommitsBetween(ctx, repoPath, origHead, onto)
	if err != nil {
		return newGitError("RebaseContinue", repoPath, "", err)
	}

	// If the working tree is dirty the user has resolved conflicts — commit them.
	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("RebaseContinue", repoPath, "", err)
	}
	status, err := wt.Status()
	if err != nil {
		return newGitError("RebaseContinue", repoPath, "", err)
	}
	if !status.IsClean() && msgnum < len(commits) {
		ci := commits[msgnum]
		commit, err := repo.CommitObject(plumbing.NewHash(ci.Hash))
		if err != nil {
			return newGitError("RebaseContinue", repoPath, "", err)
		}
		_, err = wt.Commit(commit.Message, &git.CommitOptions{
			Author: &object.Signature{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
				When:  time.Now(),
			},
		})
		if err != nil {
			return newGitError("RebaseContinue", repoPath, "", err)
		}
	}

	// Continue with remaining commits
	for i := msgnum + 1; i < end; i++ {
		_ = writeRebaseStateFile(repoPath, "msgnum", strconv.Itoa(i))

		if err := b.applyRebaseCommit(repoPath, repo, commits[i].Hash); err != nil {
			if err == ErrMergeConflict {
				return newGitError("RebaseContinue", repoPath, "", ErrMergeConflict)
			}
			return newGitError("RebaseContinue", repoPath, "", err)
		}
	}

	// Success — clean up state
	_ = os.RemoveAll(dir)
	return nil
}

// --- Commit query and index operations ---

func (b *GoGitBackend) GetCommit(ctx context.Context, repoPath, hashStr string) (*CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("GetCommit", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	commit, err := repo.CommitObject(plumbing.NewHash(hashStr))
	if err != nil {
		return nil, newGitError("GetCommit", repoPath, "", err)
	}
	return &CommitInfo{
		Hash:    commit.Hash.String(),
		Message: strings.TrimRight(commit.Message, "\n"),
		Author:  commit.Author.Name,
		Date:    commit.Author.When.Format(time.RFC3339),
	}, nil
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
