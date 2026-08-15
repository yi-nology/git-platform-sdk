package gitbackend

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

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

func (b *GoGitBackend) GetRemoteURL(ctx context.Context, repoPath, name string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", newGitError("GetRemoteURL", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	remote, err := repo.Remote(name)
	if err != nil {
		return "", newGitError("GetRemoteURL", repoPath, "", ErrRemoteNotFound)
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", newGitError("GetRemoteURL", repoPath, "", fmt.Errorf("no URLs for remote %s", name))
	}
	return urls[0], nil
}

func (b *GoGitBackend) GetRemotes(ctx context.Context, repoPath string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("GetRemotes", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}
	remotes, err := repo.Remotes()
	if err != nil {
		return nil, newGitError("GetRemotes", repoPath, "", err)
	}
	var names []string
	for _, r := range remotes {
		names = append(names, r.Config().Name)
	}
	return names, nil
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

func (b *GoGitBackend) DeleteTag(ctx context.Context, repoPath, name string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("DeleteTag", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	err = repo.DeleteTag(name)
	if err != nil {
		return newGitError("DeleteTag", repoPath, "", err)
	}
	return nil
}

func (b *GoGitBackend) PushTag(ctx context.Context, repoPath, remote, name string, auth AuthConfig) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return newGitError("PushTag", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	refSpec := config.RefSpec(fmt.Sprintf("refs/tags/%s:refs/tags/%s", name, name))
	pushOpts := &git.PushOptions{
		RemoteName:      remote,
		RefSpecs:        []config.RefSpec{refSpec},
		Auth:            b.buildTransportAuth(auth),
		InsecureSkipTLS: auth.InsecureSkipTLS,
	}

	err = repo.PushContext(ctx, pushOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return newGitError("PushTag", repoPath, "", err)
	}
	return nil
}

func (b *GoGitBackend) GetTagList(ctx context.Context, repoPath string) ([]TagInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, newGitError("GetTagList", repoPath, "", fmt.Errorf("%w: %v", ErrRepoNotFound, err))
	}

	iter, err := repo.Tags()
	if err != nil {
		return nil, newGitError("GetTagList", repoPath, "", err)
	}

	var tags []TagInfo
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		tagObj, tagErr := repo.TagObject(ref.Hash())
		if tagErr == nil {
			// Annotated Tag
			tags = append(tags, TagInfo{
				Name:    ref.Name().Short(),
				Hash:    ref.Hash().String(),
				Message: tagObj.Message,
				Author:  tagObj.Tagger.Name,
			})
		} else {
			// Lightweight Tag (commit)
			commit, commitErr := repo.CommitObject(ref.Hash())
			if commitErr == nil {
				tags = append(tags, TagInfo{
					Name:    ref.Name().Short(),
					Hash:    ref.Hash().String(),
					Message: commit.Message,
					Author:  commit.Author.Name,
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, newGitError("GetTagList", repoPath, "", err)
	}
	return tags, nil
}
