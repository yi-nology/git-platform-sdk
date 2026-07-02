package gitee

import (
	"time"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// The giteeXxx types below mirror the JSON shapes returned by the Gitee REST
// API. They are intentionally unexported (lowercase) since they only exist to
// drive the provider-neutral result types.

type giteeRepo struct {
	ID            int    `json:"id"`
	FullName      string `json:"full_name"`
	Name          string `json:"name"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
	Description   string `json:"description"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
}

func (r *giteeRepo) toPlatformRepo() *provider.PlatformRepo {
	return &provider.PlatformRepo{
		ID:            int64(r.ID),
		FullName:      r.FullName,
		Name:          r.Name,
		Owner:         r.Owner.Login,
		Description:   r.Description,
		CloneURL:      r.CloneURL,
		SSHURL:        r.SSHURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Platform:      provider.PlatformGitee,
	}
}

type giteePR struct {
	ID           int    `json:"id"`
	Number       int    `json:"number"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	HTMLURL      string `json:"html_url"`
	Head         struct {
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		SHA string `json:"sha"`
	} `json:"base"`
	User struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	} `json:"user"`
	Labels     []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignees []struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	} `json:"assignees"`
	Mergeable bool       `json:"mergeable"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	MergedAt  *time.Time `json:"merged_at"`
}

func (pr *giteePR) toChangeRequest() *provider.ChangeRequest {
	state := provider.MapBoolStateToCR(pr.State, pr.MergedAt != nil)
	var labels []string
	for _, l := range pr.Labels {
		labels = append(labels, l.Name)
	}
	var reviewers []*provider.CRUser
	for _, a := range pr.Assignees {
		reviewers = append(reviewers, &provider.CRUser{ID: int64(a.ID), Username: a.Login})
	}
	mergeStatus := "unknown"
	if pr.Mergeable {
		mergeStatus = "mergeable"
	} else {
		mergeStatus = "conflicting"
	}
	return &provider.ChangeRequest{
		ID:           int64(pr.ID),
		Number:       pr.Number,
		Title:        pr.Title,
		Description:  pr.Body,
		State:        state,
		SourceBranch: pr.SourceBranch,
		TargetBranch: pr.TargetBranch,
		HeadSHA:      pr.Head.SHA,
		BaseSHA:      pr.Base.SHA,
		Author:       &provider.CRUser{ID: int64(pr.User.ID), Username: pr.User.Login, Name: pr.User.Name},
		Reviewers:    reviewers,
		Labels:       labels,
		MergeStatus:  mergeStatus,
		WebURL:       pr.HTMLURL,
		CreatedAt:    pr.CreatedAt,
		UpdatedAt:    pr.UpdatedAt,
	}
}

type giteeComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	User      struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	} `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *giteeComment) toCRComment() *provider.CRComment {
	return &provider.CRComment{
		ID:        c.ID,
		Body:      c.Body,
		Author:    &provider.CRUser{ID: int64(c.User.ID), Username: c.User.Login, Name: c.User.Name},
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

type giteeCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Author struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	} `json:"author"`
}

func (c *giteeCommit) toCRCommit() *provider.CRCommit {
	cc := &provider.CRCommit{
		SHA:       c.SHA,
		Message:   c.Commit.Message,
		CreatedAt: c.Commit.Author.Date,
	}
	if c.Author.ID > 0 {
		cc.Author = &provider.CRUser{ID: int64(c.Author.ID), Username: c.Author.Login}
	} else if c.Commit.Author.Name != "" {
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
	Author    *struct {
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

type giteePRFile struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	Diff        string `json:"diff"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

func (f *giteePRFile) toChangedFile() *provider.ChangedFile {
	add, del := provider.CountDiffLines(f.Diff)
	if f.Additions > 0 {
		add = f.Additions
	}
	if f.Deletions > 0 {
		del = f.Deletions
	}
	return &provider.ChangedFile{
		OldPath:   f.OldPath,
		NewPath:   f.NewPath,
		Diff:      f.Diff,
		Additions: add,
		Deletions: del,
		IsNew:     f.NewFile,
		IsDeleted: f.DeletedFile,
		IsRenamed: f.RenamedFile,
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
