package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yi-nology/git-platform-sdk/pkg/projection"
	"github.com/yi-nology/git-platform-sdk/provider"
)

// toolsetNames are the selectable toolset identifiers (Options.Toolsets).
const (
	toolsetCore   = "core"   // repos, branches, files, CR reads
	toolsetCRs    = "crs"    // CR lifecycle writes
	toolsetIssues = "issues" // issue reads/writes
	toolsetStatus = "status" // commit status read/write/wait
	toolsetSearch = "search" // repo/issue/user search
)

var toolsets = []toolset{
	{name: toolsetCore, enabled: func(provider.CapabilitySet) bool { return true }, registered: registerCore},
	{name: toolsetCRs, enabled: func(provider.CapabilitySet) bool { return true }, registered: registerCRs},
	{name: toolsetIssues, enabled: func(c provider.CapabilitySet) bool { return c.Issues }, registered: registerIssues},
	{name: toolsetStatus, enabled: func(c provider.CapabilitySet) bool { return c.CommitStatuses }, registered: registerStatus},
	{name: toolsetSearch, enabled: func(c provider.CapabilitySet) bool { return c.Search }, registered: registerSearch},
}

var knownToolsets = map[string]bool{
	toolsetCore: true, toolsetCRs: true, toolsetIssues: true,
	toolsetStatus: true, toolsetSearch: true,
}

// validCommitStatusStates is the write-side vocabulary for
// set_commit_status. Read-side states like "running"/"canceled" describe
// pipeline states on some platforms and have no portable write mapping,
// so they are rejected at the tool boundary instead of failing deep
// inside a platform SDK with an obscure 4xx.
var validCommitStatusStates = map[string]bool{
	"pending": true, "success": true, "failure": true, "error": true,
}

// validCRStates / validIssueStates gate the list filters at the tool
// boundary so a typo surfaces as a readable tool error instead of a
// platform-side 4xx.
var validCRStates = map[string]bool{
	"open": true, "opened": true, "closed": true, "merged": true, "all": true,
}

var validIssueStates = map[string]bool{
	"open": true, "closed": true, "all": true,
}

// --- registration helpers ---

// add mounts one tool. write tools (write=true) are dropped entirely
// under ReadOnly: the model should not see (and cannot call) mutations
// it is not allowed to make — the strongest form of read-only mode.
func add[In, Out any](s *mcp.Server, st *state, name, title, desc string, write bool, h func(context.Context, In) (Out, error)) {
	if write && st.readonly {
		return
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        name,
		Description: desc,
		Annotations: &mcp.ToolAnnotations{Title: title},
	}, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		out, err := h(ctx, in)
		if err != nil {
			// Provider failures are tool outcomes, not protocol errors:
			// surface them as IsError text the model can read and react
			// to (retry, adjust, report). The structured payload is
			// zeroed so clients never see a success-shaped body paired
			// with an error.
			var zero Out
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, zero, nil
		}
		return nil, out, nil
	})
}

// --- core: repos, branches, files, CR reads ---

type ownerRepo struct {
	Owner string `json:"owner" jsonschema:"repository owner (organization or user)"`
	Repo  string `json:"repo" jsonschema:"repository name"`
}

type getRepoOut struct {
	Repo *provider.PlatformRepo `json:"repo"`
}

type listReposIn struct {
	Owner string `json:"owner" jsonschema:"list repositories owned by this account"`
	Page  int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

type getFileIn struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Path  string `json:"path" jsonschema:"file path inside the repository"`
	Ref   string `json:"ref,omitempty" jsonschema:"branch, tag or sha; empty = default branch"`
}

type getFileOut struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type listBranchesIn ownerRepo

type listBranchesOut struct {
	Branches []*provider.PlatformBranch `json:"branches"`
}

type listCRsIn struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	State string `json:"state,omitempty" jsonschema:"open|closed|merged|all; empty = platform default"`
	Page  int    `json:"page,omitempty"`
	// Fields trims every CR to the selected projection paths (e.g.
	// ["number","title","head.ref"]); empty returns full CRs. Use this
	// to keep large listings inside your context budget.
	Fields []string `json:"fields,omitempty"`
}

type listCRsOut struct {
	Total int                       `json:"total"`
	CRs   []*provider.ChangeRequest `json:"crs,omitempty"`
	// Projected carries the trimmed documents when fields was given.
	Projected []map[string]any `json:"projected,omitempty"`
}

type getCRIn struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number string `json:"number" jsonschema:"change request number"`
}

type getCommitIn struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	SHA   string `json:"sha"`
}

func registerCore(s *mcp.Server, st *state) {
	add(s, st, "get_repo", "Get repository", "Fetch one repository's metadata.", false, func(ctx context.Context, in ownerRepo) (getRepoOut, error) {
		repo, err := st.p.GetRepo(ctx, in.Owner, in.Repo)
		return getRepoOut{Repo: repo}, err
	})

	add(s, st, "list_repos", "List repositories", "List the repositories owned by an account.", false, func(ctx context.Context, in listReposIn) ([]*provider.PlatformRepo, error) {
		return st.p.ListRepos(ctx, provider.ListRepoOptions{Owner: in.Owner, Page: in.Page, PerPage: 30})
	})

	add(s, st, "get_file", "Get file content", "Read one file from the repository at a ref (empty = default branch).", false, func(ctx context.Context, in getFileIn) (getFileOut, error) {
		content, err := st.p.GetFileContent(ctx, in.Owner, in.Repo, in.Path, in.Ref)
		return getFileOut{Path: in.Path, Content: content}, err
	})

	add(s, st, "list_branches", "List branches", "List the branches of a repository.", false, func(ctx context.Context, in listBranchesIn) (listBranchesOut, error) {
		branches, err := st.p.ListBranches(ctx, in.Owner, in.Repo)
		return listBranchesOut{Branches: branches}, err
	})

	add(s, st, "list_crs", "List change requests", "List pull/merge requests of a repository, optionally trimmed to the given fields to save context.", false, func(ctx context.Context, in listCRsIn) (listCRsOut, error) {
		if in.State != "" && !validCRStates[in.State] {
			return listCRsOut{}, fmt.Errorf("invalid state %q (open|opened|closed|merged|all)", in.State)
		}
		opts := provider.ListCROptions{Owner: in.Owner, Repo: in.Repo, Page: in.Page, PerPage: 30}
		if in.State != "" {
			opts.State = provider.CRState(in.State)
		}
		crs, total, err := st.p.ListCRs(ctx, opts)
		if err != nil {
			return listCRsOut{}, err
		}
		if len(in.Fields) == 0 {
			return listCRsOut{Total: total, CRs: crs}, nil
		}
		trimmed, err := projection.ProjectList(crs, in.Fields...)
		if err != nil {
			return listCRsOut{}, err
		}
		return listCRsOut{Total: total, Projected: trimmed}, nil
	})

	add(s, st, "get_cr", "Get change request", "Fetch one pull/merge request with its full metadata.", false, func(ctx context.Context, in getCRIn) (*provider.ChangeRequest, error) {
		return st.p.GetCR(ctx, in.Owner, in.Repo, in.Number)
	})

	add(s, st, "get_cr_files", "Get change request files", "List the files changed by a pull/merge request with patch text.", false, func(ctx context.Context, in getCRIn) ([]*provider.ChangedFile, error) {
		return st.p.GetCRFiles(ctx, in.Owner, in.Repo, in.Number)
	})

	add(s, st, "get_commit", "Get commit", "Fetch one commit's metadata, message, and files.", false, func(ctx context.Context, in getCommitIn) (*provider.CommitInfo, error) {
		return st.p.GetCommit(ctx, in.Owner, in.Repo, in.SHA)
	})

	add(s, st, "list_commits", "List commits", "List a repository's commits (newest first), optionally on one branch.", false, func(ctx context.Context, in listCommitsIn) ([]*provider.CommitInfo, error) {
		return st.p.ListCommits(ctx, in.Owner, in.Repo, provider.ListCommitsOptions{Branch: in.Branch, Page: in.Page, PerPage: 30})
	})
}

// --- crs: CR lifecycle writes ---

type createCRIn struct {
	Owner        string   `json:"owner"`
	Repo         string   `json:"repo"`
	Title        string   `json:"title"`
	SourceBranch string   `json:"source_branch"`
	TargetBranch string   `json:"target_branch"`
	Description  string   `json:"description,omitempty"`
	Labels       []string `json:"labels,omitempty"`
}

type mergeCRIn struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number string `json:"number"`
	Squash bool   `json:"squash,omitempty" jsonschema:"squash commits into one on merge"`
}

type addCRCommentIn struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number string `json:"number"`
	Body   string `json:"body"`
}

func registerCRs(s *mcp.Server, st *state) {
	add(s, st, "create_cr", "Create change request", "Open a pull/merge request from source_branch into target_branch.", true, func(ctx context.Context, in createCRIn) (*provider.ChangeRequest, error) {
		return st.p.CreateCR(ctx, provider.CreateCROptions{
			Owner: in.Owner, Repo: in.Repo, Title: in.Title,
			Description: in.Description, SourceBranch: in.SourceBranch,
			TargetBranch: in.TargetBranch, Labels: in.Labels,
		})
	})

	add(s, st, "merge_cr", "Merge change request", "Merge a pull/merge request.", true, func(ctx context.Context, in mergeCRIn) (*provider.ChangeRequest, error) {
		return st.p.MergeCR(ctx, in.Owner, in.Repo, in.Number, provider.MergeCROptions{Squash: in.Squash})
	})

	add(s, st, "add_cr_comment", "Comment on change request", "Add a note/comment to a pull/merge request.", true, func(ctx context.Context, in addCRCommentIn) (map[string]any, error) {
		id, err := st.p.CreateNote(ctx, in.Owner, in.Repo, in.Number, in.Body)
		return map[string]any{"note_id": id}, err
	})
}

// --- issues ---

type listIssuesIn struct {
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	State  string   `json:"state,omitempty" jsonschema:"open|closed|all; empty = platform default"`
	Page   int      `json:"page,omitempty"`
	Fields []string `json:"fields,omitempty" jsonschema:"projection paths (e.g. [\"number\",\"title\"]) to trim each issue"`
}

type listIssuesOut struct {
	Total     int               `json:"total"`
	Issues    []*provider.Issue `json:"issues,omitempty"`
	Projected []map[string]any  `json:"projected,omitempty"`
}

type createIssueIn struct {
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

type addIssueCommentIn struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number string `json:"number"`
	Body   string `json:"body"`
}

func registerIssues(s *mcp.Server, st *state) {
	im, ok := st.p.(provider.IssueManager)
	if !ok {
		return
	}

	add(s, st, "list_issues", "List issues", "List issues of a repository, optionally trimmed to the given fields.", false, func(ctx context.Context, in listIssuesIn) (listIssuesOut, error) {
		if in.State != "" && !validIssueStates[in.State] {
			return listIssuesOut{}, fmt.Errorf("invalid state %q (open|closed|all)", in.State)
		}
		opts := provider.ListIssuesOptions{Owner: in.Owner, Repo: in.Repo, Page: in.Page, PerPage: 30}
		if in.State != "" {
			opts.State = provider.IssueState(in.State)
		}
		issues, total, err := im.ListIssues(ctx, opts)
		if err != nil {
			return listIssuesOut{}, err
		}
		if len(in.Fields) == 0 {
			return listIssuesOut{Total: total, Issues: issues}, nil
		}
		trimmed, err := projection.ProjectList(issues, in.Fields...)
		if err != nil {
			return listIssuesOut{}, err
		}
		return listIssuesOut{Total: total, Projected: trimmed}, nil
	})

	add(s, st, "get_issue", "Get issue", "Fetch one issue with its full metadata.", false, func(ctx context.Context, in getCRIn) (*provider.Issue, error) {
		return im.GetIssue(ctx, in.Owner, in.Repo, in.Number)
	})

	add(s, st, "create_issue", "Create issue", "Open a new issue.", true, func(ctx context.Context, in createIssueIn) (*provider.Issue, error) {
		return im.CreateIssue(ctx, provider.CreateIssueOptions{
			Owner: in.Owner, Repo: in.Repo, Title: in.Title, Body: in.Body, Labels: in.Labels,
		})
	})

	add(s, st, "add_issue_comment", "Comment on issue", "Add a comment to an issue.", true, func(ctx context.Context, in addIssueCommentIn) (*provider.IssueComment, error) {
		return im.CreateIssueComment(ctx, in.Owner, in.Repo, in.Number, in.Body)
	})

	add(s, st, "close_issue", "Close issue", "Close an issue.", true, func(ctx context.Context, in closeIssueIn) (*provider.Issue, error) {
		return im.CloseIssue(ctx, in.Owner, in.Repo, in.Number)
	})
}

// --- status ---

type setStatusIn struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	SHA         string `json:"sha"`
	State       string `json:"state" jsonschema:"pending|success|failure|error"`
	Context     string `json:"context" jsonschema:"status context, e.g. ci/lint"`
	Description string `json:"description,omitempty"`
	TargetURL   string `json:"target_url,omitempty"`
}

type waitStatusIn struct {
	Owner          string   `json:"owner"`
	Repo           string   `json:"repo"`
	SHA            string   `json:"sha"`
	Contexts       []string `json:"contexts,omitempty" jsonschema:"wait only for these status contexts; empty = all"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"overall wait bound in seconds; default 600, negative = wait unboundedly"`
}

type waitStatusOut struct {
	State provider.CommitStatusState `json:"state"`
}

func registerStatus(s *mcp.Server, st *state) {
	csm, ok := st.p.(provider.CommitStatusManager)
	if !ok {
		return
	}

	add(s, st, "get_commit_statuses", "Get commit statuses", "List the CI statuses reported on a commit (full history, newest first; use the first entry per context).", false, func(ctx context.Context, in getCommitIn) ([]*provider.CommitStatus, error) {
		list, err := csm.ListCommitStatuses(ctx, in.Owner, in.Repo, in.SHA)
		if err != nil {
			return nil, err
		}
		out := make([]*provider.CommitStatus, len(list))
		for i := range list {
			out[i] = &list[i]
		}
		return out, nil
	})

	add(s, st, "set_commit_status", "Set commit status", "Report a CI status on a commit (e.g. after running checks).", true, func(ctx context.Context, in setStatusIn) (map[string]any, error) {
		if !validCommitStatusStates[in.State] {
			return nil, fmt.Errorf("invalid state %q (pending|success|failure|error)", in.State)
		}
		err := csm.CreateCommitStatus(ctx, in.Owner, in.Repo, in.SHA, provider.CommitStatusOptions{
			State: in.State, Context: in.Context,
			Description: in.Description, TargetURL: in.TargetURL,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})

	add(s, st, "wait_for_status", "Wait for commit status", "Poll until a commit's combined CI state is terminal and return it. States: pending/running until done, then success/failure/error/canceled. CI re-runs are folded: only the newest report per context counts.", false, func(ctx context.Context, in waitStatusIn) (waitStatusOut, error) {
		opts := provider.WaitOptions{Interval: 5 * time.Second}
		if in.TimeoutSeconds != 0 {
			opts.Timeout = time.Duration(in.TimeoutSeconds) * time.Second
		}
		opts.Contexts = in.Contexts
		state, err := provider.WaitForCommitStatus(ctx, st.p, in.Owner, in.Repo, in.SHA, opts)
		if err != nil {
			return waitStatusOut{}, err
		}
		return waitStatusOut{State: state}, nil
	})
}

// --- search ---

type searchReposIn struct {
	Query string `json:"query" jsonschema:"platform search query"`
	Page  int    `json:"page,omitempty"`
}

type searchReposOut struct {
	Total int                          `json:"total"`
	Repos []*provider.SearchRepoResult `json:"repos"`
}

type searchIssuesIn struct {
	Query string `json:"query" jsonschema:"platform search query"`
	Page  int    `json:"page,omitempty"`
}

type searchIssuesOut struct {
	Total  int                           `json:"total"`
	Issues []*provider.SearchIssueResult `json:"issues"`
}

type searchUsersIn struct {
	Query string `json:"query" jsonschema:"platform search query"`
	Page  int    `json:"page,omitempty"`
}

type searchUsersOut struct {
	Total int                          `json:"total"`
	Users []*provider.SearchUserResult `json:"users"`
}

type listCommitsIn struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// Branch restricts the listing to one branch; empty = default branch.
	Branch string `json:"branch,omitempty"`
	Page   int    `json:"page,omitempty"`
}

type closeIssueIn struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number string `json:"number"`
}

func registerSearch(s *mcp.Server, st *state) {
	sm, ok := st.p.(provider.SearchManager)
	if !ok {
		return
	}

	add(s, st, "search_repositories", "Search repositories", "Search repositories by query.", false, func(ctx context.Context, in searchReposIn) (searchReposOut, error) {
		repos, total, err := sm.SearchRepos(ctx, provider.SearchReposOptions{Query: in.Query, Page: in.Page})
		if err != nil {
			return searchReposOut{}, err
		}
		t := 0
		if total != nil {
			t = *total
		}
		return searchReposOut{Total: t, Repos: repos}, nil
	})

	add(s, st, "search_issues", "Search issues", "Search issues and pull/merge requests across repositories by query.", false, func(ctx context.Context, in searchIssuesIn) (searchIssuesOut, error) {
		issues, total, err := sm.SearchIssues(ctx, provider.SearchIssuesOptions{Query: in.Query, Page: in.Page})
		if err != nil {
			return searchIssuesOut{}, err
		}
		t := 0
		if total != nil {
			t = *total
		}
		return searchIssuesOut{Total: t, Issues: issues}, nil
	})

	add(s, st, "search_users", "Search users", "Search users and organizations by query.", false, func(ctx context.Context, in searchUsersIn) (searchUsersOut, error) {
		users, total, err := sm.SearchUsers(ctx, provider.SearchUsersOptions{Query: in.Query, Page: in.Page})
		if err != nil {
			return searchUsersOut{}, err
		}
		t := 0
		if total != nil {
			t = *total
		}
		return searchUsersOut{Total: t, Users: users}, nil
	})
}
