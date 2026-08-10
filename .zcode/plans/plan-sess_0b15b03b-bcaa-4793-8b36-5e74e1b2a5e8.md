## Plan: Extend Provider Interface with IssueManager and SearchManager

### Overview
Add two new sub-interfaces to the Provider: `IssueManager` (Issues CRUD, comments, labels) and `SearchManager` (search repos/issues/users). The gitcode backend implements them; other backends return `ErrNotImplemented`.

### New Types (in provider/provider.go and provider/options.go)

**Domain types:**
- `Issue` — ID, Number, Title, Body, State, Author(*CRUser), Labels([]string), Assignees([]string), Milestone, WebURL, CreatedAt, UpdatedAt, ClosedAt
- `IssueComment` — ID, Body, Author(*CRUser), CreatedAt, UpdatedAt
- `IssueLabel` — ID, Name, Color
- `SearchRepoResult` — FullName, Description, WebURL, Stars, Forks, DefaultBranch, Private
- `SearchIssueResult` — Number, Title, Body, State, WebURL, Labels, Comments, CreatedAt
- `SearchUserResult` — Login, Name, AvatarURL, WebURL

**Options types:**
- `ListIssuesOptions` — Owner, Repo, State, Assignee, Labels, Page, PerPage
- `CreateIssueOptions` — Owner, Repo, Title, Body, Assignees, Labels
- `UpdateIssueOptions` — Title, Body, State, Assignees, Labels
- `ListIssueCommentsOptions` — (just pagination via Page, PerPage)
- `SearchReposOptions` — Query, Sort, Order, Page, PerPage
- `SearchIssuesOptions` — Query, Repo, State, Sort, Order, Page, PerPage
- `SearchUsersOptions` — Query, Sort, Order, Page, PerPage

### New Interfaces (new files)

**provider/iface_issues.go:**
```go
type IssueManager interface {
    ListIssues(ctx, opts ListIssuesOptions) ([]*Issue, int, error)
    GetIssue(ctx, owner, repo string, number int) (*Issue, error)
    CreateIssue(ctx, opts CreateIssueOptions) (*Issue, error)
    UpdateIssue(ctx, owner, repo string, number int, opts UpdateIssueOptions) (*Issue, error)
    CloseIssue(ctx, owner, repo string, number int) (*Issue, error)
    ReopenIssue(ctx, owner, repo string, number int) (*Issue, error)
    ListIssueComments(ctx, owner, repo string, number int) ([]*IssueComment, error)
    CreateIssueComment(ctx, owner, repo string, number int, body string) (*IssueComment, error)
    ListIssueLabels(ctx, owner, repo string) ([]*IssueLabel, error)
    AddIssueLabels(ctx, owner, repo string, number int, labels []string) error
    RemoveIssueLabel(ctx, owner, repo string, number int, name string) error
}
```

**provider/iface_search.go:**
```go
type SearchManager interface {
    SearchRepos(ctx, opts SearchReposOptions) ([]*SearchRepoResult, int, error)
    SearchIssues(ctx, opts SearchIssuesOptions) ([]*SearchIssueResult, int, error)
    SearchUsers(ctx, opts SearchUsersOptions) ([]*SearchUserResult, int, error)
}
```

### Provider Interface Update (provider/provider.go)
Add `IssueManager` and `SearchManager` to the Provider interface composition.

### Backend Implementations

**backends/gitcode/issues.go** — Full implementation using gitcode_api Client methods
**backends/gitcode/search.go** — Full implementation using gitcode_api Client methods

**All other backends** (github, gitlab, gitea, forgejo, gitee, tencentcode):
- Add stub methods returning `provider.ErrNotImplemented` for both interfaces
- Each file: `issues.go` and `search.go`

### Files to Create/Modify

| File | Action |
|------|--------|
| `provider/iface_issues.go` | Create — IssueManager interface |
| `provider/iface_search.go` | Create — SearchManager interface |
| `provider/provider.go` | Modify — Add IssueManager, SearchManager to Provider |
| `provider/options.go` | Modify — Add Issue/Search options and result types |
| `backends/gitcode/issues.go` | Create — Full implementation |
| `backends/gitcode/search.go` | Create — Full implementation |
| `backends/github/issues.go` | Create — ErrNotImplemented stubs |
| `backends/github/search.go` | Create — ErrNotImplemented stubs |
| `backends/gitlab/issues.go` | Create — ErrNotImplemented stubs |
| `backends/gitlab/search.go` | Create — ErrNotImplemented stubs |
| `backends/gitea/issues.go` | Create — ErrNotImplemented stubs |
| `backends/gitea/search.go` | Create — ErrNotImplemented stubs |
| `backends/forgejo/issues.go` | Create — ErrNotImplemented stubs |
| `backends/forgejo/search.go` | Create — ErrNotImplemented stubs |
| `backends/gitee/issues.go` | Create — ErrNotImplemented stubs |
| `backends/gitee/search.go` | Create — ErrNotImplemented stubs |
| `backends/tencentcode/issues.go` | Create — ErrNotImplemented stubs |
| `backends/tencentcode/search.go` | Create — ErrNotImplemented stubs |
| `backends/contracttest/contracttest.go` | Modify — Add IssueManager/SearchManager to contract tests |

### Build Order
1. Define types in provider/options.go
2. Define interfaces in provider/iface_issues.go and provider/iface_search.go
3. Update Provider interface in provider/provider.go
4. Implement gitcode backend
5. Add stubs for all other backends
6. Update contract tests
7. Run lint + build + test