package tencentcode

import (
	"strings"
	"time"

	"github.com/yi-nology/git-platform-sdk/provider"
)

// tcTime is a flexible timestamp parser that accepts the multiple date
// formats Tencent 工蜂 emits (some lack the colon in the timezone offset).
type tcTime struct {
	time.Time
}

// UnmarshalJSON parses the timestamp, trying several common layouts.
func (t *tcTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), "\"")
	if s == "null" || s == "" {
		return nil
	}
	formats := []string{
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, f := range formats {
		if parsed, err := time.Parse(f, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return nil // tolerant: leave zero time on parse failure
}

// tcMR mirrors the Tencent 工蜂 merge request JSON shape.
type tcMR struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	Author       struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"author"`
	Labels      []string `json:"labels"`
	MergeStatus string   `json:"merge_status"`
	WebURL      string   `json:"web_url"`
	CreatedAt   tcTime   `json:"created_at"`
	UpdatedAt   tcTime   `json:"updated_at"`
	DiffRefs    struct {
		BaseSHA  string `json:"base_sha"`
		StartSHA string `json:"start_sha"`
		HeadSHA  string `json:"head_sha"`
	} `json:"diff_refs"`
}

func (mr *tcMR) toCR() *provider.ChangeRequest {
	return &provider.ChangeRequest{
		ID:           int64(mr.IID),
		Number:       mr.IID,
		Title:        mr.Title,
		Description:  mr.Description,
		State:        mapState(mr.State),
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		Author: &provider.CRUser{
			ID: int64(mr.Author.ID), Username: mr.Author.Username, Name: mr.Author.Name,
		},
		Labels:      mr.Labels,
		MergeStatus: mr.MergeStatus,
		WebURL:      mr.WebURL,
		CreatedAt:   mr.CreatedAt.Time,
		UpdatedAt:   mr.UpdatedAt.Time,
		BaseSHA:     mr.DiffRefs.BaseSHA,
		StartSHA:    mr.DiffRefs.StartSHA,
		HeadSHA:     mr.DiffRefs.HeadSHA,
	}
}

func mapState(state string) provider.CRState {
	return provider.MapMRStateToCR(state)
}
