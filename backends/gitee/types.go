package gitee

import (
	"strconv"
	"time"

	gitee "gitee.com/openeuler/go-gitee/gitee"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// The convertXxx functions below translate go-gitee SDK models into the
// provider-neutral result types. The giteeXxx types further down mirror the
// JSON shapes returned by the Gitee REST API for surfaces still served by the
// raw transport client (the SDK's generated models for those endpoints do not
// match the live wire format — see the per-endpoint registrations in
// commits.go); they are intentionally unexported (lowercase) since they only
// exist to drive the provider-neutral result types.

// convertRepo translates the SDK Project model (gitee's repo object) into a
// PlatformRepo. The SDK Project model has no clone_url field (Gitee's repo
// payload carries ssh_url/html_url only), so CloneURL stays empty.
func convertRepo(r gitee.Project) *provider.PlatformRepo {
	out := &provider.PlatformRepo{
		ID:            int64(r.Id),
		FullName:      r.FullName,
		Name:          r.Name,
		Description:   r.Description,
		SSHURL:        r.SshUrl,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      provider.PlatformGitee,
	}
	if r.Owner != nil {
		out.Owner = r.Owner.Login
	}
	return out
}

// convertBranch translates the SDK Branch model into a PlatformBranch.
func convertBranch(b gitee.Branch) *provider.PlatformBranch {
	return &provider.PlatformBranch{Name: b.Name}
}

// parseGiteeTime parses the RFC3339 timestamp strings the go-gitee SDK uses
// for fields the live API reports as dates (PullRequest.CreatedAt and
// friends). Unparseable/empty values stay zero, matching the previous
// time.Time decoding behaviour for absent fields.
func parseGiteeTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// toInt32 narrows an int for the go-gitee SDK's int32-typed parameters (PR
// numbers, pagination). Inputs are either provider-normalized pagination
// values (page >= 1, perPage <= 100) or Gitee PR/note numbers, which the
// platform numbers well inside the int32 range, so the narrowing cannot
// wrap.
func toInt32(n int) int32 {
	return int32(n) // #nosec:G115 -- inputs bounded by NormalizePageOpts / PR numbering
}

// convertPullRequest translates the SDK PullRequest model into a
// provider.ChangeRequest.
//
// Registration: the live Gitee PR payload carries a top-level boolean "draft"
// field, but the SDK's PullRequest model does not expose it, so
// ChangeRequest.Draft is always false for Gitee until the SDK model catches
// up (upstream swagger omission). Branch names are taken from head.ref /
// base.ref — the REST payload has no top-level source_branch/target_branch
// fields (those exist only in creation/webhook payloads).
func convertPullRequest(pr gitee.PullRequest) *provider.ChangeRequest {
	state := provider.MapBoolStateToCR(pr.State, pr.MergedAt != "")
	var labels []string
	for _, l := range pr.Labels {
		labels = append(labels, l.Name)
	}
	var reviewers []*provider.CRUser
	for i := range pr.Assignees {
		a := pr.Assignees[i]
		reviewers = append(reviewers, &provider.CRUser{ID: int64(a.Id), Username: a.Login})
	}
	mergeStatus := "conflicting"
	if pr.Mergeable {
		mergeStatus = "mergeable"
	}
	out := &provider.ChangeRequest{
		ID:          int64(pr.Id),
		Number:      int(pr.Number),
		Title:       pr.Title,
		Description: pr.Body,
		State:       state,
		Reviewers:   reviewers,
		Labels:      labels,
		MergeStatus: mergeStatus,
		WebURL:      pr.HtmlUrl,
		CreatedAt:   parseGiteeTime(pr.CreatedAt),
		UpdatedAt:   parseGiteeTime(pr.UpdatedAt),
	}
	if pr.Head != nil {
		out.SourceBranch = pr.Head.Ref
		out.HeadSHA = pr.Head.Sha
	}
	if pr.Base != nil {
		out.TargetBranch = pr.Base.Ref
		out.BaseSHA = pr.Base.Sha
	}
	if pr.User != nil {
		out.Author = &provider.CRUser{ID: int64(pr.User.Id), Username: pr.User.Login, Name: pr.User.Name}
	}
	return out
}

// convertPRComment translates the SDK PullRequestComments model into a
// provider.CRComment.
func convertPRComment(c gitee.PullRequestComments) *provider.CRComment {
	out := &provider.CRComment{
		ID:        int64(c.Id),
		Body:      c.Body,
		CreatedAt: parseGiteeTime(c.CreatedAt),
		UpdatedAt: parseGiteeTime(c.UpdatedAt),
	}
	if c.User != nil {
		out.Author = &provider.CRUser{ID: int64(c.User.Id), Username: c.User.Login, Name: c.User.Name}
	}
	return out
}

// convertPRCommit translates the SDK PullRequestCommits model into a
// provider.CRCommit.
func convertPRCommit(c gitee.PullRequestCommits) *provider.CRCommit {
	cc := &provider.CRCommit{SHA: c.Sha}
	if c.Commit != nil {
		cc.Message = c.Commit.Message
		if c.Commit.Author != nil {
			cc.CreatedAt = c.Commit.Author.Date
		}
	}
	if c.Author != nil && c.Author.Id > 0 {
		cc.Author = &provider.CRUser{ID: int64(c.Author.Id), Username: c.Author.Login}
	} else if c.Commit != nil && c.Commit.Author != nil && c.Commit.Author.Name != "" {
		cc.Author = &provider.CRUser{Name: c.Commit.Author.Name}
	}
	return cc
}

type giteeCommitDetail struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"committer"`
	} `json:"commit"`
	Author *struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	} `json:"author"`
	Committer *struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	} `json:"committer"`
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
	} `json:"stats"`
}

func (c *giteeCommitDetail) toCommitInfo() *provider.CommitInfo {
	ci := &provider.CommitInfo{
		SHA:       c.SHA,
		Message:   c.Commit.Message,
		CreatedAt: c.Commit.Author.Date,
		Additions: c.Stats.Additions,
		Deletions: c.Stats.Deletions,
	}
	if c.Author != nil {
		ci.Author = &provider.CRUser{ID: int64(c.Author.ID), Username: c.Author.Login, Name: c.Commit.Author.Name}
	} else if c.Commit.Author.Name != "" {
		ci.Author = &provider.CRUser{Name: c.Commit.Author.Name}
	}
	if c.Committer != nil {
		ci.Committer = &provider.CRUser{ID: int64(c.Committer.ID), Username: c.Committer.Login, Name: c.Commit.Committer.Name}
	}
	return ci
}

// convertPRFile translates the SDK PullRequestFiles model (the live PR-files
// payload) into a provider.ChangedFile. Gitee reports additions/deletions as
// strings on this endpoint and nests the diff details under "patch"; the diff
// line counts act as a fallback when the server omits the numeric stats.
func convertPRFile(f gitee.PullRequestFiles) *provider.ChangedFile {
	out := &provider.ChangedFile{
		OldPath: f.Filename,
		NewPath: f.Filename,
	}
	if f.Patch != nil {
		out.OldPath = orDefault(f.Patch.OldPath, f.Filename)
		out.NewPath = orDefault(f.Patch.NewPath, f.Filename)
		out.Diff = f.Patch.Diff
		out.IsNew = f.Patch.NewFile
		out.IsDeleted = f.Patch.DeletedFile
		out.IsRenamed = f.Patch.RenamedFile
	}
	add, del := provider.CountDiffLines(out.Diff)
	if n, err := strconv.Atoi(f.Additions); err == nil && n > 0 {
		add = n
	}
	if n, err := strconv.Atoi(f.Deletions); err == nil && n > 0 {
		del = n
	}
	out.Additions = add
	out.Deletions = del
	return out
}

func orDefault(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

// giteeCompareFile mirrors the file objects of the compare endpoint's wire
// response (repos/{owner}/{repo}/compare/{base}...{head}), which differs from
// the PR-files payload: numeric additions/deletions, a string "patch" diff,
// and a status vocabulary (added/modified/removed).
type giteeCompareFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
	Truncated bool   `json:"truncated"`
}

func (f *giteeCompareFile) toChangedFile() *provider.ChangedFile {
	add, del := provider.CountDiffLines(f.Patch)
	if f.Additions > 0 {
		add = f.Additions
	}
	if f.Deletions > 0 {
		del = f.Deletions
	}
	return &provider.ChangedFile{
		OldPath:   f.Filename,
		NewPath:   f.Filename,
		Diff:      f.Patch,
		Additions: add,
		Deletions: del,
		IsNew:     f.Status == "added",
		IsDeleted: f.Status == "removed",
		IsRenamed: f.Status == "renamed",
	}
}

type giteeWebhook struct {
	ID     int64    `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

func (h *giteeWebhook) toPlatformWebhook() *provider.PlatformWebhook {
	return &provider.PlatformWebhook{ID: h.ID, URL: h.URL, Events: h.Events}
}

type giteeRelease struct {
	ID          int64      `json:"id"`
	TagName     string     `json:"tag_name"`
	Name        string     `json:"name"`
	Body        string     `json:"body"`
	HTMLURL     string     `json:"html_url"`
	Draft       bool       `json:"draft"`
	Prerelease  bool       `json:"prerelease"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at"`
}

func (r *giteeRelease) toReleaseInfo() *provider.ReleaseInfo {
	ri := &provider.ReleaseInfo{
		ID:         r.ID,
		TagName:    r.TagName,
		Title:      r.Name,
		Body:       r.Body,
		URL:        r.HTMLURL,
		Draft:      r.Draft,
		Prerelease: r.Prerelease,
		CreatedAt:  r.CreatedAt,
	}
	if r.PublishedAt != nil {
		ri.PublishedAt = *r.PublishedAt
	}
	return ri
}
