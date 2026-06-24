package gitbackend

import (
	"context"
	"fmt"
)

// Repository is a stateful convenience wrapper around GitBackend that binds a
// repository path together with its authentication and TLS settings. It removes
// the need for callers to repeat repoPath/auth/insecure on every operation.
//
// Create instances with CloneRepository (clones a remote first) or
// OpenRepository (operates on an already-present working tree).
type Repository struct {
	backend  GitBackend
	dir      string
	auth     AuthConfig
	insecure bool
}

// OpenRepository wraps an existing working tree located at dir with the given
// auth and TLS settings. The backend must already be created by the caller.
func OpenRepository(b GitBackend, dir string, auth AuthConfig, insecure bool) *Repository {
	return &Repository{backend: b, dir: dir, auth: auth, insecure: insecure}
}

// CloneRepository clones url into path and returns a Repository bound to it.
func CloneRepository(ctx context.Context, b GitBackend, url, path string, auth AuthConfig, insecure bool) (*Repository, error) {
	err := b.Clone(ctx, CloneOptions{
		URL:             url,
		Path:            path,
		Auth:            auth,
		InsecureSkipTLS: insecure,
	})
	if err != nil {
		return nil, err
	}
	return OpenRepository(b, path, auth, insecure), nil
}

// Dir returns the on-disk path of the repository working tree.
func (r *Repository) Dir() string { return r.dir }

// Auth returns the bound authentication configuration.
func (r *Repository) Auth() AuthConfig { return r.auth }

// Fetch fetches a single refspec from the "origin" remote.
func (r *Repository) Fetch(ctx context.Context, refspec string) error {
	_, err := r.backend.Fetch(ctx, FetchOptions{
		RepoPath:        r.dir,
		Remote:          "origin",
		Branches:        []string{refspec},
		Auth:            r.auth,
		InsecureSkipTLS: r.insecure,
	})
	return err
}

// FetchAll fetches tags and every remote branch from the "origin" remote.
func (r *Repository) FetchAll(ctx context.Context) error {
	_, err := r.backend.Fetch(ctx, FetchOptions{
		RepoPath:        r.dir,
		Remote:          "origin",
		Tags:            true,
		Auth:            r.auth,
		InsecureSkipTLS: r.insecure,
	})
	if err != nil {
		return err
	}
	branches, branchErr := r.backend.ListRemoteBranches(ctx, r.dir, "origin")
	if branchErr != nil {
		return fmt.Errorf("list remote branches: %w", branchErr)
	}
	for _, branch := range branches {
		_, _ = r.backend.Fetch(ctx, FetchOptions{
			RepoPath:        r.dir,
			Remote:          "origin",
			Branches:        []string{branch},
			Tags:            true,
			Auth:            r.auth,
			InsecureSkipTLS: r.insecure,
		})
	}
	return nil
}

// Checkout checks out the given ref (branch, tag or commit).
func (r *Repository) Checkout(ctx context.Context, ref string) error {
	return r.backend.CheckoutRef(ctx, r.dir, ref)
}

// CheckoutDetached force-checks out the given ref in detached HEAD state.
func (r *Repository) CheckoutDetached(ctx context.Context, ref string) error {
	return r.backend.CheckoutRef(ctx, r.dir, ref)
}

// RevParse resolves a ref to its full object SHA.
func (r *Repository) RevParse(ctx context.Context, ref string) (string, error) {
	return r.backend.RevParse(ctx, r.dir, ref)
}

// MergeBase returns the best common ancestor of two commits.
func (r *Repository) MergeBase(ctx context.Context, a, b string) (string, error) {
	return r.backend.MergeBase(ctx, r.dir, a, b)
}

// Diff returns the textual diff between two commits.
func (r *Repository) Diff(ctx context.Context, baseSHA, headSHA string) (string, error) {
	return r.backend.Diff(ctx, r.dir, DiffOptions{From: baseSHA, To: headSHA})
}

// DiffNameOnly returns the list of files changed between two commits.
func (r *Repository) DiffNameOnly(ctx context.Context, baseSHA, headSHA string) ([]string, error) {
	return r.backend.DiffNames(ctx, r.dir, baseSHA, headSHA)
}

// DeletedFiles returns the list of files deleted between two commits.
func (r *Repository) DeletedFiles(ctx context.Context, baseSHA, headSHA string) ([]string, error) {
	return r.backend.DeletedFiles(ctx, r.dir, baseSHA, headSHA)
}

// CheckoutFiles restores the given files from ref into the working tree.
func (r *Repository) CheckoutFiles(ctx context.Context, ref string, files []string) error {
	return r.backend.CheckoutFiles(ctx, r.dir, ref, files)
}

// CommitWithIdentity creates a commit using an explicit author identity.
func (r *Repository) CommitWithIdentity(ctx context.Context, name, email, msg string) error {
	return r.backend.CommitWithIdentity(ctx, r.dir, name, email, msg)
}

// Close releases any resources associated with the repository.
//
// The default implementation is a no-op; backends may override it in the
// future (e.g. to remove credential helpers). Callers should always invoke it
// (typically via defer) so cleanup keeps working when the implementation
// changes.
func (r *Repository) Close() error { return nil }
