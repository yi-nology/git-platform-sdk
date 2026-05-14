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

type GoGitBackend struct{}

func NewGoGitBackend() *GoGitBackend {
	return &GoGitBackend{}
}

func (b *GoGitBackend) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	repo, err := git.PlainOpen(opts.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	clientOpts := b.buildClientOptions(opts.Auth)
	refSpecs := buildFetchRefSpecs(opts)

	fetchOpts := &git.FetchOptions{
		RemoteName:    opts.Remote,
		RefSpecs:      refSpecs,
		ClientOptions: clientOpts,
		Progress:      opts.Progress,
		Tags:          git.NoTags,
	}

	if opts.Tags {
		fetchOpts.Tags = git.AllTags
	}

	err = repo.FetchContext(ctx, fetchOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	result := &FetchResult{}
	return result, nil
}

func (b *GoGitBackend) Push(ctx context.Context, opts PushOptions) (*PushResult, error) {
	repo, err := git.PlainOpen(opts.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	clientOpts := b.buildClientOptions(opts.Auth)

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
		return nil, fmt.Errorf("push: %w", err)
	}

	result := &PushResult{
		PushedRefs: refSpecs,
	}
	return result, nil
}

func (b *GoGitBackend) ListRemoteBranches(ctx context.Context, repoPath, remote string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	remoteObj, err := repo.Remote(remote)
	if err != nil {
		return nil, fmt.Errorf("get remote %s: %w", remote, err)
	}

	refs, err := remoteObj.List(&git.ListOptions{})
	if err != nil {
		return nil, err
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

func (b *GoGitBackend) GetCommitsBetween(ctx context.Context, repoPath, from, to string) ([]CommitInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	fromHash := plumbing.NewHash(from)
	toHash := plumbing.NewHash(to)

	commitIter, err := repo.Log(&git.LogOptions{
		From: toHash,
	})
	if err != nil {
		return nil, err
	}
	defer commitIter.Close()

	var commits []CommitInfo
	err = commitIter.ForEach(func(c *object.Commit) error {
		if c.Hash == fromHash {
			return io.EOF
		}
		commits = append(commits, CommitInfo{
			Hash:    c.Hash.String(),
			Message: c.Message,
			Author:  c.Author.Name,
			Date:    c.Author.When.Format(time.RFC3339),
		})
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, err
	}
	return commits, nil
}

func (b *GoGitBackend) IsAncestor(ctx context.Context, repoPath, ancestor, descendant string) (bool, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return false, fmt.Errorf("open repo: %w", err)
	}

	ancestorHash := plumbing.NewHash(ancestor)
	descendantHash := plumbing.NewHash(descendant)

	ancestorCommit, err := repo.CommitObject(ancestorHash)
	if err != nil {
		return false, err
	}

	descendantCommit, err := repo.CommitObject(descendantHash)
	if err != nil {
		return false, err
	}

	return ancestorCommit.IsAncestor(descendantCommit)
}

func (b *GoGitBackend) buildClientOptions(auth AuthConfig) []client.Option {
	switch auth.Type {
	case "http_basic":
		return []client.Option{
			client.WithHTTPAuth(&xhttp.BasicAuth{
				Username: auth.Username,
				Password: auth.Password,
			}),
		}
	case "http_token":
		return []client.Option{
			client.WithHTTPAuth(&xhttp.TokenAuth{
				Token: auth.Token,
			}),
		}
	case "ssh":
		if auth.SSHKey != "" {
			if _, err := os.Stat(auth.SSHKey); err == nil {
				signer, err := xssh.NewPublicKeysFromFile("git", auth.SSHKey, "")
				if err == nil {
					return []client.Option{
						client.WithSSHAuth(signer),
					}
				}
			}
		}
		return nil
	default:
		return nil
	}
}

func buildFetchRefSpecs(opts FetchOptions) []config.RefSpec {
	if len(opts.Branches) == 0 {
		return []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/*:refs/remotes/%s/*", opts.Remote)),
		}
	}
	specs := make([]config.RefSpec, 0, len(opts.Branches))
	for _, branch := range opts.Branches {
		specs = append(specs, config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", branch, opts.Remote, branch)))
	}
	return specs
}
