package gitbackend

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

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

func (b *GoGitBackend) GetFileHistory(ctx context.Context, repoPath, path string, limit int) ([]CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("GetFileHistory", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	head, err := repo.Head()
	if err != nil {
		return nil, newGitError("GetFileHistory", repoPath, "", err)
	}

	commitIter, err := repo.Log(&git.LogOptions{
		From:     head.Hash(),
		FileName: &path,
	})
	if err != nil {
		return nil, newGitError("GetFileHistory", repoPath, "", err)
	}
	defer commitIter.Close()

	var commits []CommitInfo
	count := 0
	err = commitIter.ForEach(func(c *object.Commit) error {
		if limit > 0 && count >= limit {
			return io.EOF
		}
		commits = append(commits, CommitInfo{
			Hash:    c.Hash.String(),
			Message: strings.TrimRight(c.Message, "\n"),
			Author:  c.Author.Name,
			Date:    c.Author.When.Format(time.RFC3339),
		})
		count++
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, newGitError("GetFileHistory", repoPath, "", err)
	}
	return commits, nil
}

// --- Tree and blob queries ---

func (b *GoGitBackend) GetTree(ctx context.Context, repoPath, ref, dirPath string, recursive bool) ([]TreeEntry, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("GetTree", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	var hash plumbing.Hash
	if ref == "" || ref == "HEAD" {
		head, err := repo.Head()
		if err != nil {
			return nil, newGitError("GetTree", repoPath, "", err)
		}
		hash = head.Hash()
	} else {
		resolved, err := repo.ResolveRevision(plumbing.Revision(ref))
		if err != nil {
			return nil, newGitError("GetTree", repoPath, "", err)
		}
		hash = *resolved
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, newGitError("GetTree", repoPath, "", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, newGitError("GetTree", repoPath, "", err)
	}

	dirPath = strings.TrimSpace(dirPath)
	if dirPath == "." || dirPath == "" {
		dirPath = ""
	}
	if dirPath != "" && dirPath != "/" {
		dirPath = strings.TrimPrefix(dirPath, "/")
		tree, err = tree.Tree(dirPath)
		if err != nil {
			return nil, newGitError("GetTree", repoPath, "", err)
		}
	}

	var entries []TreeEntry
	if recursive {
		_ = tree.Files().ForEach(func(f *object.File) error {
			entries = append(entries, TreeEntry{
				Name: filepath.Base(f.Name),
				Path: f.Name,
				Type: TreeEntryFile,
				Size: f.Size,
				Mode: f.Mode.String(),
				Hash: f.Hash.String(),
			})
			return nil
		})
	} else {
		for _, entry := range tree.Entries {
			entryType := TreeEntryFile
			if entry.Mode == filemode.Dir {
				entryType = TreeEntryDir
			}
			path := entry.Name
			if dirPath != "" {
				path = filepath.Join(dirPath, entry.Name)
			}
			entries = append(entries, TreeEntry{
				Name: entry.Name,
				Path: path,
				Type: entryType,
				Mode: entry.Mode.String(),
				Hash: entry.Hash.String(),
			})
		}
	}
	return entries, nil
}

func (b *GoGitBackend) GetBlob(ctx context.Context, repoPath, ref, filePath string) (*BlobContent, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("GetBlob", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	var hash plumbing.Hash
	if ref == "" || ref == "HEAD" {
		head, err := repo.Head()
		if err != nil {
			return nil, newGitError("GetBlob", repoPath, "", err)
		}
		hash = head.Hash()
	} else {
		resolved, err := repo.ResolveRevision(plumbing.Revision(ref))
		if err != nil {
			return nil, newGitError("GetBlob", repoPath, "", err)
		}
		hash = *resolved
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, newGitError("GetBlob", repoPath, "", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, newGitError("GetBlob", repoPath, "", err)
	}
	file, err := tree.File(filePath)
	if err != nil {
		return nil, newGitError("GetBlob", repoPath, "", ErrFileNotFound)
	}
	content, err := file.Contents()
	if err != nil {
		return nil, newGitError("GetBlob", repoPath, "", err)
	}

	data := []byte(content)
	isBinary := !utf8.Valid(data) || containsNullByte(data)
	result := &BlobContent{
		Size:     int64(len(data)),
		IsBinary: isBinary,
	}
	if isBinary {
		result.Content = base64.StdEncoding.EncodeToString(data)
		result.Encoding = EncodingBase64
	} else {
		result.Content = content
		result.Encoding = EncodingUTF8
	}
	return result, nil
}

// --- Checkout helpers ---

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
		_ = os.MkdirAll(filepath.Dir(fullPath), 0o750)
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

// --- Internal helpers for advanced operations ---

func containsNullByte(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

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
