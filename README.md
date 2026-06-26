# Git Platform SDK

Go 语言的多平台 Git 统一 SDK，支持 GitHub、GitLab、Gitea、Forgejo、Gitee、GitCode 等平台。

[![Go Reference](https://pkg.go.dev/badge/github.com/yi-nology/git-platform-sdk.svg)](https://pkg.go.dev/github.com/yi-nology/git-platform-sdk)
[![CI](https://github.com/yi-nology/git-platform-sdk/actions/workflows/ci.yml/badge.svg)](https://github.com/yi-nology/git-platform-sdk/actions/workflows/ci.yml)
[![Release](https://github.com/yi-nology/git-platform-sdk/actions/workflows/release.yml/badge.svg)](https://github.com/yi-nology/git-platform-sdk/actions/workflows/release.yml)
[![Latest Release](https://img.shields.io/github/v/release/yi-nology/git-platform-sdk?include_prereleases)](https://github.com/yi-nology/git-platform-sdk/releases)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)

## 简介

`git-platform-sdk` 提供统一的接口来操作不同的 Git 平台，无需关心底层 API 差异。支持自动平台检测、统一的仓库/Issue/PR/Webhook 管理。

## 支持的平台

| 平台 | 状态 | 默认 API |
|------|------|----------|
| GitHub | ✅ 完整支持 | `https://api.github.com` |
| GitLab | ✅ 完整支持 | `https://gitlab.com/api/v4` |
| Gitea | ✅ 完整支持 | `https://gitea.com/api/v1` |
| Forgejo | ✅ 完整支持 | `https://codeberg.org` |
| Gitee | ✅ 完整支持 | `https://gitee.com/api/v5` |
| GitCode / AtomGit | ✅ 完整支持 | `https://api.gitcode.com/api/v5` |
| Tencent Code | ✅ 完整支持 | `https://git.code.tencent.com/api/v3` |

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

## 平台检测

SDK 支持自动检测远程 URL 对应的平台：

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

### Provider Manager（带缓存）

Provider Manager 提供带 TTL 缓存的 Provider 管理，避免重复创建实例：

```go
import "github.com/yi-nology/git-platform-sdk/provider"

// 创建管理器，缓存 30 分钟过期
mgr := provider.NewManager(30 * time.Minute)

// 通过 URL 自动检测平台并获取 Provider
p, err := mgr.GetByURL("https://github.com/owner/repo.git", "your-token")

// 通过 Config 获取
p, err = mgr.Get(provider.Config{
    Platform: provider.PlatformGitHub,
    Token:    "your-token",
})

// 缓存管理
mgr.Len()       // 当前缓存数量
mgr.Remove(cfg) // 移除指定缓存
mgr.Purge()     // 清空所有缓存
mgr.Cleanup()   // 清除过期条目
```

### Provider 接口

Provider 接口由 8 个子接口组合而成，消费者可以只依赖需要的子接口：

```go
type Provider interface {
    Platform() Platform
    TestConnection(ctx context.Context) (*TestConnectionResult, error)

    RepoManager          // ListRepos, GetRepo, DeleteRepo, UpdateRepo, ForkRepo
    ChangeRequestManager // CreateCR, GetCR, ListCRs, MergeCR, CloseCR, ReopenCR, UpdateCR, ...
    WebhookManager       // CreateWebhook, DeleteWebhook, ListWebhooks, ParseWebhookEvent, ...
    BranchManager        // ListBranches, CreateBranch, DeleteBranch
    DiffManager          // GetCRDiff, GetCRFiles, CreateNote, DeleteNote, CreateDiscussion, CreateReview
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

### GitCode 平台示例

```go
p, err := provider.NewProvider(provider.Config{
    Platform: provider.PlatformGitCode,
    Token:    "your-gitcode-token",
})

repos, _ := p.ListRepos(ctx, provider.ListRepoOptions{})

pr, _ := p.CreateCR(ctx, provider.CreateCROptions{
    Owner:        "owner",
    Repo:         "repo",
    Title:        "新功能",
    SourceBranch: "feature",
    TargetBranch: "main",
})
```

### 多平台统一调用

```go
configs := map[provider.Platform]provider.Config{
    provider.PlatformGitHub:  {Platform: provider.PlatformGitHub, Token: githubToken},
    provider.PlatformGitLab:  {Platform: provider.PlatformGitLab, Token: gitlabToken},
    provider.PlatformGitCode: {Platform: provider.PlatformGitCode, Token: gitcodeToken},
}

for platform, cfg := range configs {
    p, _ := provider.NewProvider(cfg)
    repos, _ := p.ListRepos(ctx, provider.ListRepoOptions{Page: 1, PerPage: 5})
    fmt.Printf("%s: %d 个仓库\n", platform, len(repos))
}
```

### Git 后端操作

```go
backend, _ := gitbackend.NewGitBackend(gitbackend.Options{Type: "native"})

// Fetch
backend.Fetch(ctx, gitbackend.FetchOptions{
    RepoPath: "/path/to/repo",
    Remote:   "origin",
    Branches: []string{"main"},
})

// GetStatus
status, _ := backend.GetStatus(ctx, "/path/to/repo")
fmt.Printf("Branch: %s, Clean: %v\n", status.Branch, status.IsClean)

// Diff between commits
diff, _ := backend.Diff(ctx, "/path/to/repo", gitbackend.DiffOptions{
    From: "abc123",
    To:   "def456",
})
```

## 项目结构

```
git-platform-sdk/
├── provider/
│   ├── provider.go          # Provider 接口定义 + 子接口组合
│   ├── registry.go          # Provider 注册表
│   ├── factory.go           # Provider 工厂
│   ├── manager.go           # Provider Manager（带缓存）
│   ├── detect.go            # 平台自动检测
│   ├── errors.go            # 结构化错误类型
│   ├── logger.go            # Logger 接口
│   ├── retry.go             # HTTP 重试/限流
│   ├── middleware.go         # 请求/响应 Hook
│   ├── base.go              # 基础 HTTP 客户端
│   ├── pagination.go        # 分页工具
│   ├── diffutil.go          # Diff 工具
│   ├── stateutil.go         # 状态映射工具
│   ├── convertutil.go       # 转换工具
│   ├── util.go              # 通用工具
│   ├── iface_*.go           # 8 个子接口定义
│   ├── github.go            # GitHub 实现
│   ├── gitlab.go            # GitLab 实现
│   ├── gitea.go             # Gitea 实现
│   ├── forgejo.go           # Forgejo 实现
│   ├── gitee.go             # Gitee 实现
│   ├── gitcode.go           # GitCode 实现
│   └── tencent_code.go      # Tencent Code 实现
├── gitbackend/
│   ├── backend.go            # GitBackend 接口 (55 方法) + 类型定义
│   ├── auth.go               # AuthConfig 构造工具
│   ├── errors.go             # 结构化错误类型
│   ├── factory.go            # 后端工厂 + 注册表 (native/gogit 自动选择)
│   ├── logger.go             # Logger 复用 provider
│   ├── repository.go         # Repository 状态化封装 (绑定路径+认证)
│   ├── util.go               # 共享工具 (isCommitSHA)
│   ├── gogit.go              # go-git v5 后端: 结构体 + 认证 + 工具
│   ├── gogit_core.go         #   核心: Fetch/Push/Clone/Init/Pull/FetchAll/RunRaw/TestConnection
│   ├── gogit_branch.go       #   分支: List*/Create/Delete/Rename/Checkout/GetBranchSyncInfo
│   ├── gogit_status.go       #   状态/Diff/DiffNames/DeletedFiles/RevParse/MergeBase
│   ├── gogit_commit.go       #   提交/合并/CherryPick/Rebase/Add/CommitWithIdentity
│   ├── gogit_remote_tag.go   #   远程管理 + 标签
│   ├── gogit_file.go         #   文件/Tree/Blob/Checkout + 内部 diff 工具
│   ├── gogit_stash_config.go #   Stash (桩) + Config
│   ├── native.go             # 原生 git 后端: 结构体 + 命令执行/认证/解析
│   ├── native_core.go        #   核心操作
│   ├── native_branch.go      #   分支操作
│   ├── native_status.go      #   状态/Diff/RevParse/MergeBase
│   ├── native_commit.go      #   提交/合并/Rebase
│   ├── native_remote_tag.go  #   远程 + 标签
│   ├── native_file.go        #   文件/Tree/Blob/Checkout
│   └── native_stash_config.go#   Stash + Config
├── pkg/
│   ├── branchfilter/        # 分支过滤
│   ├── credential/          # 凭证管理 + AES-GCM 加密
│   └── encoding/            # Base64 工具
└── go.mod
```

## 配置

```go
p, err := provider.NewProvider(provider.Config{
    Platform: provider.PlatformGitHub,
    BaseURL:  "https://github.example.com/api/v3", // 可选，用于自托管
    Token:    "your-token",
    SkipTLS:  true,                                 // 可选，跳过 TLS 验证
    Logger:   myLogger,                             // 可选，注入日志
    RetryConfig: &provider.RetryConfig{             // 可选，自动重试
        MaxRetries: 3,
        BaseDelay:  500 * time.Millisecond,
    },
    Hooks: &provider.Hooks{                         // 可选，请求/响应 Hook
        Response: []provider.ResponseHook{
            func(ctx context.Context, req *http.Request, resp *http.Response, d time.Duration, err error) {
                log.Printf("%s %s %d %v", req.Method, req.URL.Path, resp.StatusCode, d)
            },
        },
    },
})
```

## CI/CD

本项目通过 GitHub Actions 实现自动化测试与发布，配置位于 `.github/workflows/`。

### 单元测试 (CI)

工作流文件：`.github/workflows/ci.yml`

- **触发条件**：推送到 `main` 分支、针对 `main` 的 Pull Request
- **运行矩阵**：`ubuntu-latest` + `macos-latest`（覆盖 Linux/macOS 两种环境，验证原生 git 后端）
- **执行步骤**：
  - `go mod download` 下载依赖
  - `go vet ./...` 静态检查
  - `go build ./...` 编译全部包
  - `go test -race -coverprofile=coverage.out ./...` 运行竞态检测 + 覆盖率统计
  - 打印覆盖率摘要，并将 `coverage.out` 作为 Artifact 上传（仅 ubuntu，保留 14 天）

### 发布 Release

工作流文件：`.github/workflows/release.yml`

- **触发条件**：推送 `v*` 格式的 tag（如 `v0.28.0`）
- **发布流程**：
  1. 拉取完整历史（`fetch-depth: 0`，用于生成变更日志）
  2. 运行 `go test ./...` 作为发布门禁 —— 测试失败则中止发布
  3. 编译全部包
  4. 自动识别预发布版本（tag 含 `rc`/`beta`/`alpha`/`pre` 时标记为 prerelease）
  5. 通过 `gh release create --generate-notes` 创建 GitHub Release，自动从提交/PR 生成发布说明

> 推送 tag 即可触发发布：
> ```bash
> git tag v0.28.0
> git push origin v0.28.0
> ```
> Go module proxy 会在 tag 推送后自动索引该版本，无需额外操作。

## 相关项目

- [gitcode_api](https://github.com/yi-nology/gitcode_api) - GitCode 专用 API 客户端

## 许可证

MIT

## 贡献

欢迎提交 Issue 和 Pull Request！
