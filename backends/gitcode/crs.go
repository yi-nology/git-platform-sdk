package gitcode

import (
	"context"
	"strconv"

	gitcode "github.com/yi-nology/gitcode_api"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// CreateCR implements provider.ChangeRequestManager.
func (p *Provider) CreateCR(ctx context.Context, opts provider.CreateCROptions) (*provider.ChangeRequest, error) {
	pr, err := p.client.CreatePullRequest(ctx, opts.Owner, opts.Repo, gitcode.CreatePullRequestOptions{
		Title: opts.Title, Body: opts.Description,
		Head: opts.SourceBranch, Base: opts.TargetBranch,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CreateCR", err)
	}
	return convertPullRequest(pr), nil
}

// GetCR implements provider.ChangeRequestManager.
func (p *Provider) GetCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	pr, err := p.client.GetPullRequest(ctx, owner, repo, number)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "GetCR", err)
	}
	return convertPullRequest(pr), nil
}

// ListCRs implements provider.ChangeRequestManager.
func (p *Provider) ListCRs(ctx context.Context, opts provider.ListCROptions) ([]*provider.ChangeRequest, int, error) {
	state := gitcode.PullRequestStateOpen
	switch opts.State {
	case provider.CRStateClosed:
		state = gitcode.PullRequestStateClosed
	case provider.CRStateMerged:
		state = gitcode.PullRequestStateClosed
	}
	prs, err := p.client.ListPullRequests(ctx, opts.Owner, opts.Repo, gitcode.ListPullRequestsOptions{
		ListOptions: gitcode.ListOptions{Page: opts.Page, PerPage: opts.PerPage},
		State:       state,
	})
	if err != nil {
		return nil, 0, provider.Wrap(provider.PlatformGitCode, "ListCRs", err)
	}
	result := make([]*provider.ChangeRequest, 0, len(prs))
	for _, pr := range prs {
		result = append(result, convertPullRequest(pr))
	}
	return result, len(result), nil
}

// MergeCR implements provider.ChangeRequestManager.
func (p *Provider) MergeCR(ctx context.Context, owner, repo string, number int, opts provider.MergeCROptions) (*provider.ChangeRequest, error) {
	err := p.client.MergePullRequest(ctx, owner, repo, number, &gitcode.MergePullRequestOptions{
		CommitMessage: opts.MergeCommitMessage,
		Squash:        opts.Squash,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "MergeCR", err)
	}
	return p.GetCR(ctx, owner, repo, number)
}

// CloseCR implements provider.ChangeRequestManager.
func (p *Provider) CloseCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	pr, err := p.client.ClosePullRequest(ctx, owner, repo, number)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "CloseCR", err)
	}
	return convertPullRequest(pr), nil
}

// ReopenCR implements provider.ChangeRequestManager.
func (p *Provider) ReopenCR(ctx context.Context, owner, repo string, number int) (*provider.ChangeRequest, error) {
	pr, err := p.client.ReopenPullRequest(ctx, owner, repo, number)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ReopenCR", err)
	}
	return convertPullRequest(pr), nil
}

// UpdateCR implements provider.ChangeRequestManager.
func (p *Provider) UpdateCR(ctx context.Context, owner, repo string, number int, opts provider.UpdateCROptions) (*provider.ChangeRequest, error) {
	pr, err := p.client.UpdatePullRequest(ctx, owner, repo, number, gitcode.UpdatePullRequestOptions{
		Title: opts.Title, Body: opts.Description, Base: opts.TargetBranch,
	})
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "UpdateCR", err)
	}
	return convertPullRequest(pr), nil
}

// UpdateCRLabels implements provider.ChangeRequestManager.
func (p *Provider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	err := p.client.AddIssueLabels(ctx, owner, repo, number, labels)
	if err != nil {
		return provider.Wrap(provider.PlatformGitCode, "UpdateCRLabels", err)
	}
	return nil
}

// ListCRComments implements provider.ChangeRequestManager.
func (p *Provider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*provider.CRComment, error) {
	comments, err := p.client.ListIssueComments(ctx, owner, repo, number)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListCRComments", err)
	}
	result := make([]*provider.CRComment, 0, len(comments))
	for _, c := range comments {
		cc := &provider.CRComment{ID: c.ID, Body: c.Body, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
		if c.Author != nil {
			authorID, _ := strconv.ParseInt(string(c.Author.ID), 10, 64)
			cc.Author = &provider.CRUser{
				ID: authorID, Username: c.Author.Login, AvatarURL: c.Author.AvatarURL,
			}
		}
		result = append(result, cc)
	}
	return result, nil
}

// ListCRCommits implements provider.ChangeRequestManager.
func (p *Provider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*provider.CRCommit, error) {
	commits, err := p.client.ListPullRequestCommits(ctx, owner, repo, number)
	if err != nil {
		return nil, provider.Wrap(provider.PlatformGitCode, "ListCRCommits", err)
	}
	result := make([]*provider.CRCommit, 0, len(commits))
	for _, c := range commits {
		cc := &provider.CRCommit{SHA: c.SHA, Message: c.Message, CreatedAt: c.CreatedAt}
		if c.Author != nil {
			authorID, _ := strconv.ParseInt(string(c.Author.ID), 10, 64)
			cc.Author = &provider.CRUser{
				ID: authorID, Username: c.Author.Login, AvatarURL: c.Author.AvatarURL,
			}
		}
		result = append(result, cc)
	}
	return result, nil
}

func convertPullRequest(pr *gitcode.PullRequest) *provider.ChangeRequest {
	if pr == nil {
		return nil
	}
	state := provider.CRStateOpened
	if pr.State == gitcode.PullRequestStateClosed {
		if pr.Merged {
			state = provider.CRStateMerged
		} else {
			state = provider.CRStateClosed
		}
	}
	cr := &provider.ChangeRequest{
		ID:          pr.ID,
		Number:      pr.Number,
		Title:       pr.Title,
		Description: pr.Body,
		State:       state,
		WebURL:      pr.HTMLURL,
		CreatedAt:   pr.CreatedAt,
		UpdatedAt:   pr.UpdatedAt,
	}
	if pr.Head != nil {
		cr.SourceBranch = pr.Head.Ref
	}
	if pr.Base != nil {
		cr.TargetBranch = pr.Base.Ref
	}
	if pr.Author != nil {
		authorID, _ := strconv.ParseInt(string(pr.Author.ID), 10, 64)
		cr.Author = &provider.CRUser{
			ID: authorID, Username: pr.Author.Login, AvatarURL: pr.Author.AvatarURL,
		}
	}
	return cr
}

var _ provider.ChangeRequestManager = (*Provider)(nil)
