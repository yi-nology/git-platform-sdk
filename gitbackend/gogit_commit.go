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

	changes, err := object.DiffTree(parentTree, commitTree)
	if err != nil {
		return newGitError("CherryPick", repoPath, "", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("CherryPick", repoPath, "", err)
	}

	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			continue
		}

		switch action {
		case merkletrie.Insert, merkletrie.Modify:
			file, err := commitTree.File(change.To.Name)
			if err != nil {
				continue
			}
			reader, err := file.Blob.Reader()
			if err != nil {
				continue
			}
			fullPath := filepath.Join(repoPath, change.To.Name)
			_ = os.MkdirAll(filepath.Dir(fullPath), 0o755)
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

func (b *GoGitBackend) Rebase(ctx context.Context, repoPath, onto string) error {
	return newGitError("Rebase", repoPath, "", fmt.Errorf("rebase not supported in go-git backend, use native backend"))
}

func (b *GoGitBackend) RebaseAbort(ctx context.Context, repoPath string) error {
	return newGitError("RebaseAbort", repoPath, "", fmt.Errorf("rebase not supported in go-git backend, use native backend"))
}

func (b *GoGitBackend) RebaseContinue(ctx context.Context, repoPath string) error {
	return newGitError("RebaseContinue", repoPath, "", fmt.Errorf("rebase not supported in go-git backend, use native backend"))
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
