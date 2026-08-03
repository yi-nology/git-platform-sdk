package gitbackend

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

// stashRefName returns the plumbing.ReferenceName for stash@{n}.
func stashRefName(n int) plumbing.ReferenceName {
	return plumbing.ReferenceName(fmt.Sprintf("refs/stash@{%d}", n))
}

// stashCount returns the number of existing stash entries by probing refs/stash@{n}.
func stashCount(storer storage.Storer) int {
	for i := 0; ; i++ {
		_, err := storer.Reference(stashRefName(i))
		if err != nil {
			return i
		}
	}
}

// --- Stash operations ---

func (b *GoGitBackend) StashList(ctx context.Context, repoPath string) ([]StashEntry, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("StashList", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	count := stashCount(repo.Storer)
	if count == 0 {
		return nil, nil
	}

	entries := make([]StashEntry, 0, count)
	for i := 0; i < count; i++ {
		ref, err := repo.Storer.Reference(stashRefName(i))
		if err != nil {
			break
		}
		commit, err := repo.CommitObject(ref.Hash())
		if err != nil {
			entries = append(entries, StashEntry{Index: i, Message: ref.Hash().String()})
			continue
		}
		msg := strings.TrimSpace(commit.Message)
		entries = append(entries, StashEntry{Index: i, Message: msg})
	}
	return entries, nil
}

func (b *GoGitBackend) StashSave(ctx context.Context, repoPath, message string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("StashSave", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	head, err := repo.Head()
	if err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}

	// Check if worktree is clean.
	status, err := wt.Status()
	if err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}
	if isWorktreeClean(status) {
		return nil // nothing to stash
	}

	// Stage all working tree changes into the index so we can capture them.
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}

	// Build a tree from the current index.
	idx, err := repo.Storer.Index()
	if err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}

	treeHash, err := buildStashTree(repo.Storer, wt, idx)
	if err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}

	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}

	// Build the commit message.
	branchName := head.Name().Short()
	firstLine := strings.SplitN(headCommit.Message, "\n", 2)[0]
	firstLine = strings.TrimSpace(firstLine)
	msg := message
	if msg == "" {
		msg = fmt.Sprintf("WIP on %s: %s %s", branchName, headCommit.Hash.String()[:7], firstLine)
	}

	// Create the stash commit.
	committer := object.Signature{
		Name:  "stash",
		Email: "",
		When:  time.Now(),
	}
	author := headCommit.Author
	if message != "" {
		author = committer
	}

	stashCommit := &object.Commit{
		Author:       author,
		Committer:    committer,
		Message:      msg + "\n",
		TreeHash:     treeHash,
		ParentHashes: []plumbing.Hash{head.Hash()},
	}

	obj := repo.Storer.NewEncodedObject()
	if err := stashCommit.Encode(obj); err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}
	stashHash, err := repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}

	// Shift existing stash refs: stash@{N} -> stash@{N+1}.
	count := stashCount(repo.Storer)
	for i := count - 1; i >= 0; i-- {
		ref, err := repo.Storer.Reference(stashRefName(i))
		if err != nil {
			continue
		}
		newRef := plumbing.NewHashReference(stashRefName(i+1), ref.Hash())
		if err := repo.Storer.SetReference(newRef); err != nil {
			return newGitError("StashSave", repoPath, "", err)
		}
	}

	// Set stash@{0} to the new stash commit.
	stashRef := plumbing.NewHashReference(stashRefName(0), stashHash)
	if err := repo.Storer.SetReference(stashRef); err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}

	// Reset the working tree to HEAD (hard reset).
	if err := wt.Reset(&git.ResetOptions{
		Commit: head.Hash(),
		Mode:   git.HardReset,
	}); err != nil {
		return newGitError("StashSave", repoPath, "", err)
	}

	return nil
}

func (b *GoGitBackend) StashApply(ctx context.Context, repoPath string, index int) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("StashApply", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	head, err := repo.Head()
	if err != nil {
		return newGitError("StashApply", repoPath, "", err)
	}

	stashRef, err := repo.Storer.Reference(stashRefName(index))
	if err != nil {
		return newGitError("StashApply", repoPath, "", fmt.Errorf("stash@{%d} not found", index))
	}

	stashCommit, err := repo.CommitObject(stashRef.Hash())
	if err != nil {
		return newGitError("StashApply", repoPath, "", err)
	}

	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return newGitError("StashApply", repoPath, "", err)
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return newGitError("StashApply", repoPath, "", err)
	}

	stashTree, err := stashCommit.Tree()
	if err != nil {
		return newGitError("StashApply", repoPath, "", err)
	}

	changes, err := object.DiffTree(headTree, stashTree)
	if err != nil {
		return newGitError("StashApply", repoPath, "", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return newGitError("StashApply", repoPath, "", err)
	}

	if err := applyChanges(wt, stashTree, changes); err != nil {
		return newGitError("StashApply", repoPath, "", err)
	}

	return nil
}

func (b *GoGitBackend) StashPop(ctx context.Context, repoPath string, index int) error {
	if err := b.StashApply(ctx, repoPath, index); err != nil {
		return err
	}
	return b.StashDrop(ctx, repoPath, index)
}

func (b *GoGitBackend) StashDrop(ctx context.Context, repoPath string, index int) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("StashDrop", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	_, err = repo.Storer.Reference(stashRefName(index))
	if err != nil {
		return newGitError("StashDrop", repoPath, "", fmt.Errorf("stash@{%d} not found", index))
	}

	count := stashCount(repo.Storer)
	if index >= count {
		return newGitError("StashDrop", repoPath, "", fmt.Errorf("stash@{%d} not found", index))
	}

	// Shift stashes above the dropped index down by one.
	for i := index; i < count-1; i++ {
		ref, err := repo.Storer.Reference(stashRefName(i + 1))
		if err != nil {
			break
		}
		newRef := plumbing.NewHashReference(stashRefName(i), ref.Hash())
		if err := repo.Storer.SetReference(newRef); err != nil {
			return newGitError("StashDrop", repoPath, "", err)
		}
	}

	// Remove the highest stash ref (now a duplicate).
	if err := repo.Storer.RemoveReference(stashRefName(count - 1)); err != nil {
		return newGitError("StashDrop", repoPath, "", err)
	}

	return nil
}

func (b *GoGitBackend) StashClear(ctx context.Context, repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("StashClear", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	count := stashCount(repo.Storer)
	for i := 0; i < count; i++ {
		_ = repo.Storer.RemoveReference(stashRefName(i))
	}
	return nil
}

// --- Internal stash helpers ---

// isWorktreeClean reports whether a go-git worktree status has no changes.
func isWorktreeClean(status git.Status) bool {
	for _, fileStatus := range status {
		if fileStatus.Staging != git.Unmodified || fileStatus.Worktree != git.Unmodified {
			return false
		}
	}
	return true
}

// applyChanges applies a set of tree-diff changes to the working tree and stages them.
func applyChanges(wt *git.Worktree, stashTree *object.Tree, changes object.Changes) error {
	root := wt.Filesystem.Root()
	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			continue
		}

		switch action {
		case merkletrie.Insert, merkletrie.Modify:
			file, err := stashTree.File(change.To.Name)
			if err != nil {
				continue
			}
			reader, err := file.Blob.Reader()
			if err != nil {
				continue
			}
			fullPath := filepath.Join(root, change.To.Name)
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
			fullPath := filepath.Join(root, change.From.Name)
			_ = os.Remove(fullPath)
			_, _ = wt.Remove(change.From.Name)
		}
	}
	return nil
}

// stashTreeNode represents a node in the tree being built for a stash commit.
type stashTreeNode struct {
	name     string
	mode     filemode.FileMode
	hash     plumbing.Hash
	children map[string]*stashTreeNode
}

// buildStashTree builds a tree object from the go-git index and stores it.
// This mirrors go-git's internal buildTreeHelper logic.
func buildStashTree(s storage.Storer, wt *git.Worktree, idx *index.Index) (plumbing.Hash, error) {
	root := &stashTreeNode{
		name:     "",
		children: make(map[string]*stashTreeNode),
	}

	for _, e := range idx.Entries {
		parts := strings.Split(e.Name, "/")
		current := root
		for i, part := range parts {
			if i == len(parts)-1 {
				// Leaf: file entry.
				current.children[part] = &stashTreeNode{
					name: part,
					mode: e.Mode,
					hash: e.Hash,
				}
			} else {
				// Directory: traverse or create.
				if child, ok := current.children[part]; ok {
					current = child
				} else {
					child := &stashTreeNode{
						name:     part,
						mode:     filemode.Dir,
						children: make(map[string]*stashTreeNode),
					}
					current.children[part] = child
					current = child
				}
			}
		}
	}

	return storeTreeNode(s, root)
}

// storeTreeNode recursively stores a stashTreeNode as git tree objects.
func storeTreeNode(s storage.Storer, node *stashTreeNode) (plumbing.Hash, error) {
	if len(node.children) == 0 && node.hash != plumbing.ZeroHash {
		return node.hash, nil
	}

	entries := make([]object.TreeEntry, 0, len(node.children))
	for _, child := range node.children {
		hash := child.hash
		if len(child.children) > 0 {
			var err error
			hash, err = storeTreeNode(s, child)
			if err != nil {
				return plumbing.ZeroHash, err
			}
		}
		entries = append(entries, object.TreeEntry{
			Name: child.name,
			Mode: child.mode,
			Hash: hash,
		})
	}

	sort.Sort(stashSortableEntries(entries))

	tree := &object.Tree{Entries: entries}
	o := s.NewEncodedObject()
	if err := tree.Encode(o); err != nil {
		return plumbing.ZeroHash, err
	}
	return s.SetEncodedObject(o)
}

// stashSortableEntries sorts tree entries using git's directory-first convention.
type stashSortableEntries []object.TreeEntry

func (s stashSortableEntries) sortName(te object.TreeEntry) string {
	if te.Mode == filemode.Dir {
		return te.Name + "/"
	}
	return te.Name
}
func (s stashSortableEntries) Len() int           { return len(s) }
func (s stashSortableEntries) Less(i, j int) bool { return s.sortName(s[i]) < s.sortName(s[j]) }
func (s stashSortableEntries) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// --- Config operations ---

// parseConfigKey splits a git config key like "remote.origin.url" or "core.bare"
// into (section, subsection, option). For "section.option" keys the subsection is empty.
func parseConfigKey(key string) (section, subsection, option string, err error) {
	parts := strings.Split(key, ".")
	switch len(parts) {
	case 2:
		// section.option (e.g. "core.bare")
		return parts[0], "", parts[1], nil
	case 3:
		// section.subsection.option (e.g. "remote.origin.url")
		return parts[0], parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("invalid config key: %s (expected section.option or section.subsection.option)", key)
	}
}

// GetConfig reads any git config value. Supports both simple keys (e.g.
// "core.bare") and subsection keys (e.g. "remote.origin.url"). The special
// keys "user.name" and "user.email" are handled via the high-level Author
// struct for backward compatibility.
func (b *GoGitBackend) GetConfig(ctx context.Context, repoPath, key string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", newGitError("GetConfig", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	cfg, err := repo.Config()
	if err != nil {
		return "", newGitError("GetConfig", repoPath, "", err)
	}

	// Fast path for the two most common keys.
	if key == "user.name" {
		return cfg.Author.Name, nil
	}
	if key == "user.email" {
		return cfg.Author.Email, nil
	}

	section, subsection, option, err := parseConfigKey(key)
	if err != nil {
		return "", newGitError("GetConfig", repoPath, "", err)
	}

	if cfg.Raw == nil {
		return "", newGitError("GetConfig", repoPath, "", fmt.Errorf("config key not found: %s", key))
	}

	sec := cfg.Raw.Section(section)
	var val string
	if subsection != "" {
		val = sec.Subsection(subsection).Option(option)
	} else {
		val = sec.Option(option)
	}
	if val == "" {
		return "", newGitError("GetConfig", repoPath, "", fmt.Errorf("config key not found: %s", key))
	}
	return val, nil
}

// SetConfig writes any git config value. Supports both simple keys and
// subsection keys, matching the same format as GetConfig. The special keys
// "user.name" and "user.email" are written via the high-level Author struct.
func (b *GoGitBackend) SetConfig(ctx context.Context, repoPath, key, value string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("SetConfig", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	cfg, err := repo.Config()
	if err != nil {
		return newGitError("SetConfig", repoPath, "", err)
	}

	// Fast path for the two most common keys.
	if key == "user.name" {
		cfg.Author.Name = value
		return repo.Storer.SetConfig(cfg)
	}
	if key == "user.email" {
		cfg.Author.Email = value
		return repo.Storer.SetConfig(cfg)
	}

	section, subsection, option, err := parseConfigKey(key)
	if err != nil {
		return newGitError("SetConfig", repoPath, "", err)
	}

	// Use the Raw config helpers which accept section/subsection strings
	// directly, avoiding the Section/Subsection type mismatch.
	cfg.Raw.SetOption(section, subsection, option, value)

	return repo.Storer.SetConfig(cfg)
}
