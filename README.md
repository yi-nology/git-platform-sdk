# Git Platform SDK

A unified Go SDK for interacting with 7 Git hosting platforms through a single interface.

[English](#english) | [中文](#简介)

[![Go Reference](https://pkg.go.dev/badge/github.com/yi-nology/git-platform-sdk.svg)](https://pkg.go.dev/github.com/yi-nology/git-platform-sdk)
[![CI](https://github.com/yi-nology/git-platform-sdk/actions/workflows/ci.yml/badge.svg)](https://github.com/yi-nology/git-platform-sdk/actions/workflows/ci.yml)
[![Release](https://github.com/yi-nology/git-platform-sdk/actions/workflows/release.yml/badge.svg)](https://github.com/yi-nology/git-platform-sdk/actions/workflows/release.yml)
[![Latest Release](https://img.shields.io/github/v/release/yi-nology/git-platform-sdk?include_prereleases)](https://github.com/yi-nology/git-platform-sdk/releases)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)

## English

### Overview

`git-platform-sdk` provides a unified Go interface for 7 Git hosting platforms: **GitHub**, **GitLab**, **Gitea**, **Forgejo**, **Gitee**, **GitCode**, and **Tencent Code**. Write once, run against any platform.

**Key features:**

- **Unified transport layer** — shared auth/retry/hooks/logging pipeline across all platforms; third-party SDKs (go-github, gitlab client-go, etc.) plug in via `http.RoundTripper`
- **Per-platform packages** — each backend is isolated in `backends/<platform>/`, split by responsibility (repos, CRs, webhooks, branches, commits, files, diffs, releases)
- **Contract tests** — cross-platform test suite ensures consistent behavior
- **Structured errors** — `ProviderError` auto-extracts HTTP status codes from 4 sources (StatusCode method/field, `*http.Response` field, error message string)
- **Proactive rate limiting** — `RateLimiter` tracks `X-RateLimit-*` headers and throttles before hitting limits
- **Secure by default** — SSH host key verification enabled, SHA-256 token hashing in cache keys, constant-time webhook signature comparison

### Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/yi-nology/git-platform-sdk/backends/all"
    "github.com/yi-nology/git-platform-sdk/provider"
)

func main() {
    p, err := provider.NewProvider(provider.Config{
        Platform: provider.PlatformGitHub,
        Token:    "ghp_...",
    })
    if err != nil {
        log.Fatal(err)
    }

    repos, err := p.ListRepos(context.Background(), provider.ListRepoOptions{
        Owner: "my-org",
    })
    if err != nil {
        log.Fatal(err)
    }
    for _, r := range repos {
        fmt.Println(r.FullName)
    }
}
```

### Supported Platforms

| Platform | Status | Coverage |
|----------|--------|----------|
| GitHub | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses |
| GitLab | ✅ Stable | Repos/MR/Webhook/Branches/Commits/Files/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses |
| Gitea | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses |
| Forgejo | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses |
| Gitee | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels/Issues/Milestones/Search |
| GitCode | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses |
| Tencent Code | ✅ Stable | Repos/MR/Webhook/Branches/Commits/Files/Release/Labels/Issues/Milestones/Reviews/CommitStatuses + exclusive features |

### Installation

```bash
go get github.com/yi-nology/git-platform-sdk
```

### Rate Limiting

```go
import "github.com/yi-nology/git-platform-sdk/transport"

client := transport.NewClient("https://api.github.com", auth)
client.Limiter = transport.NewRateLimiter(
    transport.WithRPS(30),              // max 30 requests/sec
    transport.WithThrottleThreshold(5), // slow down when < 5 remaining
)
```

### SSH Security

SSH commands default to **strict host key checking**. For CI environments:

```go
mgr := credential.NewManager()
// Secure (default) — requires known_hosts
cmd := mgr.BuildSSHCommand("/path/to/key")
// Insecure (CI only) — disables host key checking
cmd := mgr.BuildSSHCommandInsecure("/path/to/key")
```

## Divergence ledger

Every backend registers the places where its behavior departs from the
unified semantics (stub / ignore / mapping / detour) in a machine-readable
ledger, surfaced by `Provider.Divergences()` and helper predicates such as
`provider.Ignores`. The rendered document lives at
[docs/divergence-ledger.md](docs/divergence-ledger.md); regenerate it with
`go generate ./...` after editing any backend's `divergence.go`. The
contract suite fails when a ledger entry drifts from actual behavior.

---

## 简介

`git-platform-sdk` 提供统一的接口来操作不同的 Git 平台，无需关心底层 API 差异。支持自动平台检测、统一的仓库/Issue/PR/Webhook 管理。

### 架构亮点

- **统一传输层** (`transport/`): 所有平台共享 auth/retry/hooks/logger 管道, 第三方 SDK (go-github, gitlab client-go 等) 通过 `http.RoundTripper` 包装接入
- **按平台拆包** (`backends/<platform>/`): 每个平台独立包, 按职责拆文件 (repos/crs/webhooks/branches/commits/files/diffs/releases)
- **契约测试** (`backends/contracttest/`): 跨平台统一测试套件, 确保接口行为一致
- **错误归一** (`provider.ProviderError`): 自动从 4 种来源 (StatusCode 方法/字段, *http.Response 字段, 错误字符串) 提取 HTTP 状态码

## 支持的平台

| 平台 | 状态 | API 覆盖 | 默认 API |
|------|------|----------|----------|
| GitHub | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses | `https://api.github.com` |
| GitLab | ✅ 稳定 | 仓库/MR/Webhook/分支/提交/文件/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses | `https://gitlab.com/api/v4` |
| Gitea | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses | `https://gitea.com/api/v1` |
| Forgejo | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses | `https://codeberg.org` |
| Gitee | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels/Issues/Milestones/Search | `https://gitee.com/api/v5` |
| GitCode | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels/Issues/Milestones/Reviews/Search/CommitStatuses | `https://api.gitcode.com/api/v5` |
| Tencent Code | ✅ 稳定 | 仓库/MR/Webhook/分支/提交/文件/Release/Labels/Issues/Milestones/Reviews/CommitStatuses + 工蜂专属能力 | `https://git.code.tencent.com/api/v3` |

## 安装

```bash
go get github.com/yi-nology/git-platform-sdk
```

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/yi-nology/git-platform-sdk/backends/all" // 注册所有平台
    _ "github.com/yi-nology/git-platform-sdk/backends/all"
    "github.com/yi-nology/git-platform-sdk/provider"
)

func main() {
    ctx := context.Background()

    // 方式 1: 自动检测平台
    result, err := provider.DetectPlatform("https://github.com/owner/repo.git")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("检测到平台: %s\n", result.Platform)

    // 方式 2: 手动指定平台
    p, err := provider.NewProvider(provider.Config{
        Platform: provider.PlatformGitHub,
        Token:    "your-token",
    })
    if err != nil {
        log.Fatal(err)
    }

    // 获取仓库信息
    repo, err := p.GetRepo(ctx, "owner", "repo")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("仓库: %s\n", repo.FullName)
}
```

> **重要**: 必须导入 `backends/all` (blank import) 才能注册所有平台后端。
> 如果只需要特定平台, 可以单独导入, 例如 `_ "github.com/yi-nology/git-platform-sdk/backends/github"`。

## 平台检测

SDK 支持自动检测远程 URL 对应的平台:

```go
// HTTPS URL
result, _ := provider.DetectPlatform("https://github.com/owner/repo.git")
// result.Platform == provider.PlatformGitHub

// SSH URL
result, _ = provider.DetectPlatform("git@gitlab.com:owner/repo.git")
// result.Platform == provider.PlatformGitLab

// 自托管实例
result, _ = provider.DetectPlatform("https://my-gitea.example.com/owner/repo.git")
// result.Platform == provider.PlatformGitea (默认)
```

## API 使用

### Provider Manager（带缓存 + 统计）

Provider Manager 提供带 TTL 缓存的 Provider 管理, 并支持命中率统计和后台自动清理:

```go
import "github.com/yi-nology/git-platform-sdk/provider"

// 创建管理器, 缓存 30 分钟过期, 最多缓存 100 个 provider
mgr := provider.NewManager(30*time.Minute, provider.WithMaxSize(100))

// 启动后台 janitor, 每 5 分钟清理过期条目
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
mgr.StartJanitor(ctx, 5*time.Minute)
defer mgr.Stop()

// 通过 URL 自动检测平台并获取 Provider
p, err := mgr.GetByURL("https://github.com/owner/repo.git", "your-token")

// 查看缓存统计
stats := mgr.Stats()
fmt.Printf("Hits: %d, Misses: %d, Size: %d\n", stats.Hits, stats.Misses, stats.Size)

// 缓存管理
mgr.Remove(cfg) // 移除指定缓存
mgr.Purge()     // 清空所有缓存
mgr.Cleanup()   // 手动清除过期条目
```

**缓存键安全性**: Manager 使用 `SHA256(token)[:16]` 作为缓存键的一部分, 不会在内存或日志中泄露原始 token。

### Provider 接口

Provider 接口由 8 个子接口组合而成, 消费者可以只依赖需要的子接口:

```go
type Provider interface {
    Platform() Platform
    TestConnection(ctx context.Context) (*TestConnectionResult, error)

    RepoManager          // ListRepos, GetRepo, DeleteRepo, UpdateRepo, ForkRepo
    ChangeRequestManager // CreateCR, GetCR, ListCRs, MergeCR, CloseCR, ReopenCR, UpdateCR, ...
    WebhookManager       // CreateWebhook, DeleteWebhook, ListWebhooks, ParseWebhookEvent, ...
    BranchManager        // ListBranches, CreateBranch, DeleteBranch
    DiffManager          // GetCRDiff, GetCRFiles, CreateNote, DeleteNote, CreateDiscussion
    CommitManager        // GetCommit, ListCommits, CompareCommits
    FileManager          // GetFileContent, CreateFile, UpdateFile, DeleteFile
    ReleaseManager       // ListTags, ListReleases, CreateRelease, GetReleaseByTag, UpdateRelease, DeleteRelease, GetArchive
}
```

### 使用子接口

```go
// 只需要 Webhook 功能
type WebhookHandler struct {
    wh provider.WebhookManager
}

func (h *WebhookHandler) HandleEvent(r *http.Request) error {
    event, err := h.wh.ParseWebhookEvent(r, secret)
    // ...
}
```

### 可选能力（Issues / Search / Labels / Milestones / Reviews / CommitStatuses）

并非所有平台都支持全部可选能力。这些接口**不在** `Provider` 组合中，调用方通过
`Capabilities()` 声明式判断（或直接类型断言）：

```go
p, _ := provider.NewProvider(cfg)
caps := p.Capabilities()

if caps.Issues {
    ism := p.(provider.IssueManager)
    issues, _, _ := ism.ListIssues(ctx, provider.ListIssuesOptions{Owner: "o", Repo: "r"})
    // ...
}
if caps.Labels {
    lm := p.(provider.LabelManager)
    labels, _ := lm.ListLabels(ctx, "o", "r", provider.ListLabelsOptions{})
    // ...
}
```

| 能力 | 接口 | 支持平台 |
|------|------|----------|
| Issues | `IssueManager` | GitHub / GitLab / Gitea / Forgejo / Gitee / GitCode / Tencent Code |
| Search | `SearchManager` | GitHub / GitLab / Gitea / Forgejo / Gitee / GitCode |
| Labels | `LabelManager` | GitHub / GitLab / Gitea / Forgejo / Gitee / GitCode / Tencent Code |
| Milestones | `MilestoneManager` | GitHub / GitLab / Gitea / Forgejo / Gitee / GitCode / Tencent Code |
| Reviews | `ReviewManager` | GitHub / GitLab / Gitea / Forgejo / GitCode / Tencent Code |
| CommitStatuses | `CommitStatusManager` | GitHub / GitLab / Gitea / Forgejo / GitCode / Tencent Code |

说明：

- **Gitee 不声明 Reviews**: Gitee API 只有 PR 审查人员（Testers）指派，没有
  review 列表/创建/驳回端点，不满足能力门槛（spec §4.6），整接口不做。
- **Gitee 不声明 CommitStatuses**: `CreateCommitStatus` 已从核心 `CommitManager`
  拆入可选 `CommitStatusManager`（`Capabilities().CommitStatuses` 门控）；Gitee 公开
  REST API 没有 commit-status 端点，原来的 `ErrNotImplemented` 桩已随拆分移除——
  能力缺失以不声明表达，而非桩方法。
- **Release 加厚属于核心接口而非可选能力**: `ReleaseManager` 新增
  `GetReleaseByTag` / `UpdateRelease` / `DeleteRelease`（一律按 tag 寻址），
  7 个平台全部实现。

### 已知限制

- **Gitee `ChangeRequest.Draft` 恒为 `false`**: 线上 PR 载荷有原生 `draft`
  布尔字段，但 go-gitee SDK 的 `PullRequest` 模型缺该字段（上游 swagger 遗漏），
  SDK 补齐前无法如实返回。
- **GitLab Reviews 是 approvals 汇总映射**（已登记，spec §4.6）: GitLab 没有
  逐条 review 对象，`ListReviews`/`GetReview` 走 MR 审批状态并按审批人合成
  `approved` 汇总条目（ID 均为 MR IID）；`RequestReviewers` 为登记忽略
  （`reviewer_ids` 需 username→ID 解析，SDK 无此面）。
- **Gitea / Forgejo 的 `REQUEST_CHANGES`/`COMMENT` 评审需要 body 或行内评论**:
  两平台 SDK 的客户端校验拒绝空 body 且无评论的非 APPROVE 评审（APPROVE 豁免）。
- **Milestone 寻址语义随平台不同**: `MilestoneRef.Number` / `Milestone.Number`
  在 GitHub 上是 milestone number，在 GitLab/Gitea/Forgejo/GitCode/Tencent Code 上是
  milestone ID，在 Gitee 上是里程碑序号（载荷 `number` 字段）；跨平台传递 ID 不可移植。
- **CR/评审寻址已全面 string 化（迁移提示）**: `ChangeRequestManager` 与
  `DiffManager` 的 number 参数、`ChangeRequest.Number` 及 `ReviewManager` 各方法
  均以 string 寻址，与 Issues/Milestones/Search 同一规则；数字平台内部解析，
  非法输入返回包裹的 `invalid pull request number` 错误。旧代码中传 `1` 的
  调用点改为传 `"1"`。
- **Gitee 企业版 issue 状态原样透传**: `Issue.State` 是开放字符串词表而非封闭
  枚举；Gitee 企业空间在 open/closed 之外还有 progressing/rejected 等工作流状态，
  按平台返回值原样出现在 `Issue.State` 中（已登记）。
- **Search 的 `Sort`/`Order` 走各平台自身词表**: 取值随平台不同（如 GitHub 的
  stars/forks/updated + asc/desc）；Gitea/Forgejo 对未知值返回 HTTP 422，
  GitLab 搜索 API 无 sort/order 参数（登记忽略）。请按目标平台文档取值。
- **Tencent Code 工蜂 `Label.ID` 恒为 0**: 工蜂标签按名寻址（更新/删除经
  options 携带当前名，无需 name→ID 解析扫描），gongfeng Label 模型无 id
  字段，`Label.ID` 在该平台恒为 0（标签端到端按名操作）。
- **Tencent Code 工蜂 issue 四处登记忽略/缺省**: `Assignees` 字段被忽略且
  `ListIssuesOptions.Assignee` 过滤不携带——工蜂 issue 写面收 `assignee_ids`
  （数值用户 ID csv），username→ID 需 Users API 而 SDK 无按名查询面；移除
  issue 的最后一个标签是 no-op——空 label csv 因 `omitempty` 上不了 PUT 体，
  标签保留；`Issue.WebURL` 恒为空、`Issue.ClosedAt` 恒为 nil——gongfeng
  issue 模型无这两个字段。
- **Tencent Code 工蜂 Reviews 是 MR notes 映射（已登记）**: 工蜂原生评审以
  携带 `reviewer_state` verdict 的 MR note 表达，与普通 MR 评论共用同一集合
  ——普通评论会以 `commented` 评审混入 `ListReviews`（工蜂 system 记账 note
  已过滤）；读侧无 verdict 字段，`Review.State` 恒为 `commented`；
  `CreateReview` 的 `Comments`/`CommitID` 不映射（一条 note 至多携带一个行内
  位置，且无 commit 概念）；`RequestReviewers` 为登记忽略（工蜂 MR 更新面
  不收评审人，原生邀请端点按数值 user ID 寻址，username→ID 不可达）；
  `DismissReview` 为登记桩返回 `provider.ErrNotImplemented`（工蜂无评审
  撤销面）。

### Tencent 工蜂专属能力

Tencent 工蜂 backend 额外实现了 `TencentCodeExtras` 接口, 暴露工蜂独有的功能:

```go
import "github.com/yi-nology/git-platform-sdk/backends/tencentcode"

p, _ := provider.NewProvider(provider.Config{
    Platform: provider.PlatformTencentCode,
    Token:    "your-token",
})

// 通过类型断言获取专属能力
if tc, ok := p.(*tencentcode.Provider); ok {
    // 原生代码评审
    review, _ := tc.CreateCodeReview(ctx, owner, repo, tencentcode.CreateCodeReviewOptions{
        Title: "code review", SourceBranch: "feature", TargetBranch: "main",
    })
    // MR 评审流程
    _ = tc.SubmitMRReview(ctx, owner, repo, 42, tencentcode.SubmitReviewOptions{
        Event: tencentcode.ReviewEventApprove, Summary: "LGTM",
    })
    // 分支保护
    _ = tc.ProtectBranch(ctx, owner, repo, "main", tencentcode.ProtectBranchOptions{})
}
```

### 统一 Webhook 验证

SDK 内置 6 种 Webhook 签名验证策略, 通过注册表统一管理:

```go
// 使用默认注册表 (init 时自动注册所有平台)
err := provider.DefaultWebhookRegistry().Validate(
    provider.PlatformGitHub, r, body, secret,
)

// 自定义验证器
registry := provider.NewWebhookValidatorRegistry()
registry.Register(provider.Platform("custom"), provider.HMACSHA256Validator{Header: "X-Custom-Sig"})
```

### 配置

```go
p, err := provider.NewProvider(provider.Config{
    Platform: provider.PlatformGitHub,
    BaseURL:  "https://github.example.com/api/v3", // 可选, 用于自托管
    Token:    "your-token",
    SkipTLS:  true,                                 // 可选, 跳过 TLS 验证
    Logger:   myLogger,                             // 可选, 注入日志
    RetryConfig: &provider.RetryConfig{             // 可选, 自动重试
        MaxRetries: 3,
        BaseDelay:  500 * time.Millisecond,
    },
    Hooks: &provider.Hooks{                         // 可选, 请求/响应 Hook
        Response: []provider.ResponseHook{
            func(ctx context.Context, req *http.Request, resp *http.Response, d time.Duration, err error) {
                log.Printf("%s %s %d %v", req.Method, req.URL.Path, resp.StatusCode, d)
            },
        },
    },
})
```

**Retry/Hooks/Logger 对所有平台生效** (包括使用第三方 SDK 的 GitHub/GitLab/Gitea/Forgejo), 因为它们都通过 `transport.RoundTripper` 包装。

### Git 后端操作

`gitbackend` 提供本地 Git 仓库的底层操作 (Fetch/Push/Clone/状态/Diff/分支/标签/文件…), 有两个后端实现, 通过工厂自动选择:

| 后端 | Type | 说明 |
|------|------|------|
| 原生 git | `"native"` | 调用本地 `git` 命令, 功能最全 (支持 Rebase/Stash/RunRaw) |
| go-git | `"gogit"` | 纯 Go 实现 (基于 go-git/v5), 无需 git 二进制, 部分高级操作返回 `ErrNotSupported` |

```go
import "github.com/yi-nology/git-platform-sdk/gitbackend"

// 显式指定后端
backend, _ := gitbackend.NewGitBackend(gitbackend.Options{Type: "native"})
// 留空则自动选择 (优先 native, 回退 gogit)
backend, _ = gitbackend.NewGitBackend(gitbackend.Options{})
```

#### 认证方式 (SSH / HTTPS / 跳过 SSL)

```go
// 1) HTTPS Token
auth := gitbackend.NewTokenAuth("your-access-token")

// 2) HTTP Basic
auth := gitbackend.NewHTTPBasicAuth("user", "pass")

// 3) SSH 私钥文件
auth := gitbackend.NewSSHKeyFileAuth("/home/user/.ssh/id_ed25519", "passphrase")

// 4) SSH 私钥内容
auth := gitbackend.NewSSHKeyContentAuth(pemContent, "passphrase")

// 跳过 TLS
auth.InsecureSkipTLS = true
```

#### Repository 封装

```go
repo, err := gitbackend.CloneRepository(ctx, backend,
    "https://git.example.com/owner/repo.git", "/path/to/repo", auth, true)
defer repo.Close()

repo.Fetch(ctx, "main")
repo.RevParse(ctx, "HEAD")
repo.Diff(ctx, baseSHA, headSHA)
```

## 项目结构

```
git-platform-sdk/
├── provider/                    # 公共 API (类型 + 接口 + 工厂 + Manager)
│   ├── provider.go              # Provider interface, Platform, 核心类型
│   ├── options.go               # 所有 Options/Result 类型 (集中定义)
│   ├── errors.go                # ProviderError + Wrap/New 助手 + 状态码反射
│   ├── webhook.go               # WebhookValidator 接口 + 注册表 + 6 种策略
│   ├── manager.go               # TTL 缓存 Manager (SHA256 键 + Stats + Janitor)
│   ├── detect.go                # 平台自动检测
│   ├── factory.go               # 平台注册 + NewProvider
│   ├── pagination.go            # NormalizePageOpts + X-Total-Count 解析
│   ├── diffutil.go              # BuildRawDiff / CountDiffLines / SumDiffStats
│   ├── stateutil.go             # MapStateToCR 状态映射
│   ├── middleware.go            # Hooks (RequestHook / ResponseHook)
│   ├── retry.go                 # RetryConfig
│   └── logger.go                # Logger 接口
│
├── transport/                   # 统一 HTTP 传输层
│   ├── client.go                # Client + Do/DoJSON/DoRaw + RoundTripper
│   ├── auth.go                  # AuthStrategy (Bearer/Token/PrivateToken/None)
│   ├── retry.go                 # RetryConfig + 指数退避 + jitter
│   ├── hooks.go                 # transport.Hooks
│   ├── errors.go                # transport.Error + IsStatus
│   └── logger.go                # transport.Logger + slog 适配
│
├── backends/                    # 平台实现 (每个独立包)
│   ├── github/                  # GitHub (go-github SDK + transport 包装)
│   ├── gitlab/                  # GitLab (client-go SDK + transport 包装)
│   ├── gitea/                   # Gitea (gitea SDK + transport 包装)
│   ├── forgejo/                 # Forgejo (forgejo SDK + transport 包装)
│   ├── gitcode/                 # GitCode (go-gitcode SDK)
│   ├── gitee/                   # Gitee (go-gitee SDK + transport 包装, 个别写端点登记 raw 绕行)
│   ├── tencentcode/             # Tencent 工蜂 (transport.Client + Extras)
│   ├── all/                     # 一行 blank import 注册所有平台
│   └── contracttest/            # 跨平台契约测试套件
│
├── gitbackend/                  # 本地 Git 操作 (native + gogit 双后端)
├── pkg/
│   ├── branchfilter/            # 分支过滤
│   ├── credential/              # 凭证管理 + AES-GCM 加密
│   └── encoding/                # Base64 工具
├── Makefile                     # test/lint/fmt/cover 等命令
├── .golangci.yml                # lint 配置
└── go.mod
```

## 开发

### 常用命令

```bash
make test       # 运行所有测试 (race + coverage)
make lint       # golangci-lint
make fmt        # gofmt + goimports
make vet        # go vet
make check      # CI 门禁 (vet + lint + test)
make cover      # 打印覆盖率摘要
```

### 添加新平台

1. 创建 `backends/<platform>/` 目录
2. 实现 `provider.Provider` 接口 (参考 `backends/gitee/` 作为模板)
3. 添加 `init.go` 注册到 `provider.Register`
4. 在 `backends/all/all.go` 添加 blank import
5. 创建 `contract_test.go` 调用 `contracttest.Run` 验证契约

### CI/CD

本项目通过 GitHub Actions 实现自动化测试与发布, 配置位于 `.github/workflows/`。

#### 单元测试 (CI)

- **触发条件**: 推送到 `main` 分支、针对 `main` 的 Pull Request
- **运行矩阵**: `ubuntu-latest` + `macos-latest`
- **执行步骤**: `go vet` + `go build` + `go test -race -coverprofile`

#### 发布 Release

- **触发条件**: 推送 `v*` 格式的 tag
- **发布流程**: 测试门禁 → 编译 → 自动识别预发布 → 创建 GitHub Release

```bash
git tag v0.28.0
git push origin v0.28.0
```

## 相关项目

- [go-gitcode](https://github.com/yi-nology/go-gitcode) - GitCode 专用 API 客户端

## 许可证

MIT

## 贡献

欢迎提交 Issue 和 Pull Request！
