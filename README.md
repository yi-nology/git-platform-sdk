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
| GitHub | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels/Issues |
| GitLab | ✅ Stable | Repos/MR/Webhook/Branches/Commits/Files/Release/Labels/Issues |
| Gitea | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels/Issues |
| Forgejo | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels/Issues |
| Gitee | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels |
| GitCode | ✅ Stable | Repos/PR/Webhook/Branches/Commits/Files/Release/Labels/Issues |
| Tencent Code | ✅ Stable | Repos/MR/Webhook/Branches/Commits/Files/Release + exclusive features |

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
| GitHub | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels/Issues | `https://api.github.com` |
| GitLab | ✅ 稳定 | 仓库/MR/Webhook/分支/提交/文件/Release/Labels/Issues | `https://gitlab.com/api/v4` |
| Gitea | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels/Issues | `https://gitea.com/api/v1` |
| Forgejo | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels/Issues | `https://codeberg.org` |
| Gitee | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels | `https://gitee.com/api/v5` |
| GitCode | ✅ 稳定 | 仓库/PR/Webhook/分支/提交/文件/Release/Labels/Issues | `https://api.gitcode.com/api/v5` |
| Tencent Code | ✅ 稳定 | 仓库/MR/Webhook/分支/提交/文件/Release + 工蜂专属能力 | `https://git.code.tencent.com/api/v3` |

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
    CommitManager        // GetCommit, ListCommits, CompareCommits, CreateCommitStatus
    FileManager          // GetFileContent, CreateFile, UpdateFile, DeleteFile
    ReleaseManager       // ListTags, ListReleases, CreateRelease, GetArchive
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

### 可选能力（Issues / Search / Labels）

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
| Issues | `IssueManager` | GitCode / GitHub / GitLab / Gitea / Forgejo / Gitee |
| Search | `SearchManager` | GitCode |
| Labels | `LabelManager` | GitHub / GitLab / Gitea / Forgejo / Gitee / GitCode |

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
│   ├── gitcode/                 # GitCode (gitcode_api SDK)
│   ├── gitee/                   # Gitee (直接使用 transport.Client)
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

- [gitcode_api](https://github.com/yi-nology/gitcode_api) - GitCode 专用 API 客户端

## 许可证

MIT

## 贡献

欢迎提交 Issue 和 Pull Request！
