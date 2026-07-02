package gitea

import (
	"time"

	gitea "code.gitea.io/sdk/gitea"

	"github.com/yi-nology/git-platform-sdk/provider"
)

func convertRepo(r *gitea.Repository) *provider.PlatformRepo {
	if r == nil {
		return nil
	}
	owner := ""
	if r.Owner != nil {
		owner = r.Owner.UserName
	}
	return &provider.PlatformRepo{
		ID:            r.ID,
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         owner,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      provider.PlatformGitea,
	}
}

func convertPR(pr *gitea.PullRequest) *provider.ChangeRequest {
	if pr == nil {
		return nil
	}
	var author *provider.CRUser
	if pr.Poster != nil {
		author = &provider.CRUser{ID: pr.Poster.ID, Username: pr.Poster.UserName, AvatarURL: pr.Poster.AvatarURL}
	}
	var labels []string
	for _, l := range pr.Labels {
		if l != nil {
			labels = append(labels, l.Name)
		}
	}
	var reviewers []*provider.CRUser
	for _, r := range pr.RequestedReviewers {
		if r != nil {
			reviewers = append(reviewers, &provider.CRUser{ID: r.ID, Username: r.UserName, AvatarURL: r.AvatarURL})
		}
	}
	return &provider.ChangeRequest{
		ID:           pr.ID,
		Number:       int(pr.Index),
		Title:        pr.Title,
		Description:  pr.Body,
		State:        mapState(string(pr.State), pr.HasMerged),
		SourceBranch: pr.Head.Ref,
		TargetBranch: pr.Base.Ref,
		HeadSHA:      pr.Head.Sha,
		BaseSHA:      pr.Base.Sha,
		Author:       author,
		Reviewers:    reviewers,
		Labels:       labels,
		WebURL:       pr.HTMLURL,
		CreatedAt:    timeOrZero(pr.Created),
		UpdatedAt:    timeOrZero(pr.Updated),
	}
}

func convertBranch(b *gitea.Branch) *provider.PlatformBranch {
	if b == nil {
		return nil
	}
	return &provider.PlatformBranch{Name: b.Name}
}

func convertHook(h *gitea.Hook) *provider.PlatformWebhook {
	if h == nil {
		return nil
	}
	return &provider.PlatformWebhook{
		ID:     h.ID,
		URL:    h.Config["url"],
		Events: h.Events,
	}
}

func convertCommit(c *gitea.Commit) *provider.CommitInfo {
	if c == nil {
		return nil
	}
	sha := ""
	if c.CommitMeta != nil {
		sha = c.CommitMeta.SHA
	}
	ci := &provider.CommitInfo{SHA: sha}
	if c.RepoCommit != nil {
		ci.Message = c.RepoCommit.Message
		if c.RepoCommit.Author != nil {
			ci.Author = &provider.CRUser{Name: c.RepoCommit.Author.Name}
		}
	}
	if c.CommitMeta != nil {
		ci.CreatedAt = c.CommitMeta.Created
	}
	return ci
}

func convertRelease(r *gitea.Release) *provider.ReleaseInfo {
	if r == nil {
		return nil
	}
	return &provider.ReleaseInfo{
		ID:          r.ID,
		TagName:     r.TagName,
		Title:       r.Title,
		Body:        r.Note,
		URL:         r.URL,
		Draft:       r.IsDraft,
		Prerelease:  r.IsPrerelease,
		CreatedAt:   r.CreatedAt,
		PublishedAt: r.PublishedAt,
	}
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func mapState(state string, merged bool) provider.CRState {
	return provider.MapBoolStateToCR(state, merged)
}

// parseTotalCount extracts the X-Total-Count header from a gitea response.
func parseTotalCount(resp *gitea.Response) int {
	if resp == nil {
		return 0
	}
	return provider.ParseTotalCountHeader(resp.Header, 0)
}
