package gitlab

import (
	"strconv"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// convertProject maps a gitlab.Project to the provider-neutral type.
func convertProject(p *gitlab.Project) *provider.PlatformRepo {
	if p == nil {
		return nil
	}
	owner, _ := provider.SplitFullName(p.PathWithNamespace)
	return &provider.PlatformRepo{
		ID:            p.ID,
		FullName:      p.PathWithNamespace,
		Name:          p.Name,
		Owner:         owner,
		Description:   p.Description,
		CloneURL:      p.HTTPURLToRepo,
		SSHURL:        p.SSHURLToRepo,
		DefaultBranch: p.DefaultBranch,
		Private:       p.Visibility != "public",
		Platform:      provider.PlatformGitLab,
	}
}

// convertMR maps a gitlab.MergeRequest to the provider-neutral type.
func convertMR(mr *gitlab.MergeRequest) *provider.ChangeRequest {
	if mr == nil {
		return nil
	}
	var author *provider.CRUser
	if mr.Author != nil {
		author = &provider.CRUser{ID: mr.Author.ID, Username: mr.Author.Username, Name: mr.Author.Name, AvatarURL: mr.Author.AvatarURL}
	}
	var reviewers []*provider.CRUser
	for _, r := range mr.Reviewers {
		reviewers = append(reviewers, &provider.CRUser{ID: r.ID, Username: r.Username, Name: r.Name, AvatarURL: r.AvatarURL})
	}
	var assignees []*provider.CRUser
	for _, a := range mr.Assignees {
		assignees = append(assignees, &provider.CRUser{ID: a.ID, Username: a.Username, Name: a.Name, AvatarURL: a.AvatarURL})
	}
	return &provider.ChangeRequest{
		ID:           mr.IID,
		Number:       strconv.FormatInt(mr.IID, 10),
		Title:        mr.Title,
		Description:  mr.Description,
		State:        mapGLState(mr.State),
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		Author:       author,
		Assignees:    assignees,
		Reviewers:    reviewers,
		Labels:       mr.Labels,
		MergeStatus:  mr.DetailedMergeStatus,
		WebURL:       mr.WebURL,
		HeadSHA:      mrHeadSHA(mr),
		BaseSHA:      mr.DiffRefs.BaseSha,
		StartSHA:     mr.DiffRefs.StartSha,
		CreatedAt:    timeOrZero(mr.CreatedAt),
		UpdatedAt:    timeOrZero(mr.UpdatedAt),
	}
}

func mrHeadSHA(mr *gitlab.MergeRequest) string {
	if mr.DiffRefs.HeadSha != "" {
		return mr.DiffRefs.HeadSha
	}
	return mr.SHA
}

// convertBasicMR maps the lighter-weight gitlab.BasicMergeRequest (returned
// by list endpoints) to the provider-neutral type.
func convertBasicMR(mr *gitlab.BasicMergeRequest) *provider.ChangeRequest {
	if mr == nil {
		return nil
	}
	var author *provider.CRUser
	if mr.Author != nil {
		author = &provider.CRUser{ID: mr.Author.ID, Username: mr.Author.Username, Name: mr.Author.Name, AvatarURL: mr.Author.AvatarURL}
	}
	var reviewers []*provider.CRUser
	for _, r := range mr.Reviewers {
		reviewers = append(reviewers, &provider.CRUser{ID: r.ID, Username: r.Username, Name: r.Name, AvatarURL: r.AvatarURL})
	}
	var assignees []*provider.CRUser
	for _, a := range mr.Assignees {
		assignees = append(assignees, &provider.CRUser{ID: a.ID, Username: a.Username, Name: a.Name, AvatarURL: a.AvatarURL})
	}
	return &provider.ChangeRequest{
		ID:           mr.IID,
		Number:       strconv.FormatInt(mr.IID, 10),
		Title:        mr.Title,
		Description:  mr.Description,
		State:        mapGLState(mr.State),
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		Author:       author,
		Assignees:    assignees,
		Reviewers:    reviewers,
		Labels:       mr.Labels,
		MergeStatus:  mr.DetailedMergeStatus,
		WebURL:       mr.WebURL,
		CreatedAt:    timeOrZero(mr.CreatedAt),
		UpdatedAt:    timeOrZero(mr.UpdatedAt),
	}
}

func convertBranch(b *gitlab.Branch) *provider.PlatformBranch {
	if b == nil {
		return nil
	}
	return &provider.PlatformBranch{Name: b.Name}
}

func convertHook(h *gitlab.ProjectHook) *provider.PlatformWebhook {
	if h == nil {
		return nil
	}
	var events []string
	if h.PushEvents {
		events = append(events, "push")
	}
	if h.MergeRequestsEvents {
		events = append(events, "merge_request")
	}
	if h.TagPushEvents {
		events = append(events, "tag_push")
	}
	if h.NoteEvents {
		events = append(events, "note")
	}
	return &provider.PlatformWebhook{ID: h.ID, URL: h.URL, Events: events}
}

func convertCommit(c *gitlab.Commit) *provider.CommitInfo {
	if c == nil {
		return nil
	}
	ci := &provider.CommitInfo{SHA: c.ID, Message: c.Message}
	if c.Stats != nil {
		ci.Additions = int(c.Stats.Additions)
		ci.Deletions = int(c.Stats.Deletions)
	}
	ci.CreatedAt = timeOrZero(c.CreatedAt)
	if c.AuthorName != "" || c.AuthorEmail != "" {
		ci.Author = &provider.CRUser{Name: c.AuthorName}
	}
	return ci
}

// timeOrZero dereferences a *time.Time, returning the zero value when nil.
func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func mapGLState(state string) provider.CRState {
	return provider.MapMRStateToCR(state)
}

func mapCommitState(state string) gitlab.BuildStateValue {
	switch state {
	case "success":
		return gitlab.Success
	case "failed":
		return gitlab.Failed
	case "pending":
		return gitlab.Pending
	case "running":
		return gitlab.Running
	default:
		return gitlab.Pending
	}
}
