package provider

import (
	"context"
	"fmt"
	"net/http"
)

type giteeProvider struct {
	baseURL string
	token   string
}

func NewGiteeProvider(baseURL, token string) Provider {
	return &giteeProvider{baseURL: baseURL, token: token}
}

func (g *giteeProvider) Platform() Platform { return PlatformGitee }

func (g *giteeProvider) TestConnection(ctx context.Context) (*TestConnectionResult, error) {
	return &TestConnectionResult{Connected: false, Message: "Gitee SDK integration in progress"}, nil
}

func (g *giteeProvider) ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) DeleteRepo(ctx context.Context, owner, repo string) error {
	return fmt.Errorf("not implemented")
}

func (g *giteeProvider) CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) UpdateCR(ctx context.Context, owner, repo string, number int, opts UpdateCROptions) (*ChangeRequest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error) {
	return nil, 0, fmt.Errorf("not implemented")
}

func (g *giteeProvider) CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) ReopenCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (g *giteeProvider) CreateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) UpdateFile(ctx context.Context, owner, repo string, opts FileOptions) (*FileResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) DeleteFile(ctx context.Context, owner, repo string, opts FileDeleteOptions) (*FileResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error {
	return fmt.Errorf("not implemented")
}

func (g *giteeProvider) ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) ParseWebhookEvent(r *http.Request, secret string) (*NormalizedEvent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) ValidateWebhookSignature(r *http.Request, secret string) error {
	return fmt.Errorf("not implemented")
}

func (g *giteeProvider) ListBranches(ctx context.Context, owner, repo string) ([]*PlatformBranch, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) CreateBranch(ctx context.Context, owner, repo, branch, ref string) (*PlatformBranch, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) DeleteBranch(ctx context.Context, owner, repo, branch string) error {
	return fmt.Errorf("not implemented")
}

func (g *giteeProvider) GetCRDiff(ctx context.Context, owner, repo string, number int) (*MergeDiff, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) GetCRFiles(ctx context.Context, owner, repo string, number int) ([]*ChangedFile, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) CreateNote(ctx context.Context, owner, repo string, number int, body string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (g *giteeProvider) DeleteNote(ctx context.Context, owner, repo string, number int, noteID string) error {
	return fmt.Errorf("not implemented")
}

func (g *giteeProvider) CreateDiscussion(ctx context.Context, owner, repo string, number int, opts DiscussionOptions) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (g *giteeProvider) CreateCommitStatus(ctx context.Context, owner, repo, sha string, opts CommitStatusOptions) error {
	return fmt.Errorf("not implemented")
}

func (g *giteeProvider) UpdateCRLabels(ctx context.Context, owner, repo string, number int, labels []string) error {
	return fmt.Errorf("not implemented")
}

func (g *giteeProvider) ListCRComments(ctx context.Context, owner, repo string, number int) ([]*CRComment, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) ListCRCommits(ctx context.Context, owner, repo string, number int) ([]*CRCommit, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) ForkRepo(ctx context.Context, owner, repo string, opts ForkRepoOptions) (*PlatformRepo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) UpdateRepo(ctx context.Context, owner, repo string, opts UpdateRepoOptions) (*PlatformRepo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) GetCommit(ctx context.Context, owner, repo, sha string) (*CommitInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) ListCommits(ctx context.Context, owner, repo string, opts ListCommitsOptions) ([]*CommitInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) ListTags(ctx context.Context, owner, repo string) ([]*TagInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) ListReleases(ctx context.Context, owner, repo string) ([]*ReleaseInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) CreateRelease(ctx context.Context, owner, repo string, opts CreateReleaseOptions) (*ReleaseInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *giteeProvider) GetArchive(ctx context.Context, owner, repo, ref, format string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
