package provider

import "context"

// JobLog is one failed CI job's identifying metadata plus the tail of its
// execution log, sized for LLM consumption rather than archival.
type JobLog struct {
	Name    string `json:"name"`
	Stage   string `json:"stage"`
	WebURL  string `json:"web_url,omitempty"`
	Trace   string `json:"trace"` // 失败日志尾部（后端截断，量级 ~20KB）
	Truncat bool   `json:"truncated"`
}

// CILogManager fetches failure diagnostics from a platform's CI system.
// It is an optional capability interface: consumers should type-assert
// before use. Only GitLab exposes per-job logs through its public REST API
// today; GitHub check-runs carry only summaries/annotations, so absence is
// expressed by not implementing the interface instead of stubbing.
type CILogManager interface {
	// ListFailedJobLogs returns the failed jobs of the most recent pipeline
	// for a commit, each with the tail of its execution log. A commit with
	// no pipeline (or a green one) yields an empty slice, not an error.
	ListFailedJobLogs(ctx context.Context, owner, repo, sha string) ([]JobLog, error)
}
