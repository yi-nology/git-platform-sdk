# Git Platform SDK

Go 语言的多平台 Git 统一 SDK，支持 GitHub、GitLab、Gitea、Forgejo、Gitee、GitCode 等平台。

[![Go Reference](https://pkg.go.dev/badge/github.com/yi-nology/git-platform-sdk.svg)](https://pkg.go.dev/github.com/yi-nology/git-platform-sdk)

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

// GitCode URL
result, _ = provider.DetectPlatform("https://gitcode.com/owner/repo.git")
// result.Platform == provider.PlatformGitCode

// 自托管实例
result, _ = provider.DetectPlatform("https://my-gitea.example.com/owner/repo.git")
// result.Platform == provider.PlatformGitea (默认)
```

## API 使用

### Provider 接口

所有平台实现统一的 `Provider` 接口：

```go
type Provider interface {
    Platform() Platform
    ListRepos(ctx context.Context, opts ListRepoOptions) ([]*PlatformRepo, error)
    GetRepo(ctx context.Context, owner, repo string) (*PlatformRepo, error)
    CreateCR(ctx context.Context, opts CreateCROptions) (*ChangeRequest, error)
    GetCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error)
    ListCRs(ctx context.Context, opts ListCROptions) ([]*ChangeRequest, int, error)
    MergeCR(ctx context.Context, owner, repo string, number int, opts MergeCROptions) (*ChangeRequest, error)
    CloseCR(ctx context.Context, owner, repo string, number int) (*ChangeRequest, error)
    CreateWebhook(ctx context.Context, opts CreateWebhookOptions) (*PlatformWebhook, error)
    DeleteWebhook(ctx context.Context, owner, repo string, webhookID int64) error
    ListWebhooks(ctx context.Context, owner, repo string) ([]*PlatformWebhook, error)
    // ... 更多方法
}
```

### GitCode 平台示例

```go
// 创建 GitCode Provider
p, err := provider.NewProvider(provider.Config{
    Platform: provider.PlatformGitCode,
    Token:    "your-gitcode-token",
})

// 列出仓库
repos, _ := p.ListRepos(ctx, provider.ListRepoOptions{})

// 创建 PR
pr, _ := p.CreateCR(ctx, provider.CreateCROptions{
    Owner:        "owner",
    Repo:         "repo",
    Title:        "新功能",
    SourceBranch: "feature",
    TargetBranch: "main",
})

// 创建 Webhook
hook, _ := p.CreateWebhook(ctx, provider.CreateWebhookOptions{
    Owner:  "owner",
    Repo:   "repo",
    URL:    "https://your-webhook.com",
    Events: []string{"push", "pull_request"},
})
```

### 多平台统一调用

```go
// 根据不同平台创建 Provider
configs := map[provider.Platform]provider.Config{
    provider.PlatformGitHub:  {Platform: provider.PlatformGitHub, Token: githubToken},
    provider.PlatformGitLab:  {Platform: provider.PlatformGitLab, Token: gitlabToken},
    provider.PlatformGitCode: {Platform: provider.PlatformGitCode, Token: gitcodeToken},
}

// 统一调用
for platform, cfg := range configs {
    p, _ := provider.NewProvider(cfg)
    repos, _ := p.ListRepos(ctx, provider.ListRepoOptions{Page: 1, PerPage: 5})
    fmt.Printf("%s: %d 个仓库\n", platform, len(repos))
}
```

## 项目结构

```
git-platform-sdk/
├── provider/
│   ├── provider.go      # Provider 接口定义
│   ├── factory.go       # Provider 工厂
│   ├── detect.go        # 平台自动检测
│   ├── base.go          # 基础 HTTP 客户端
│   ├── github.go        # GitHub 实现
│   ├── gitlab.go        # GitLab 实现
│   ├── gitea.go         # Gitea 实现
│   ├── forgejo.go       # Forgejo 实现
│   ├── gitee.go         # Gitee 实现
│   ├── gitcode.go       # GitCode 实现
│   └── tencent_code.go  # Tencent Code 实现
├── gitbackend/          # Git 操作
├── credential/          # 凭证管理
└── branchfilter/        # 分支过滤
```

## 相关项目

- [gitcode_api](https://github.com/yi-nology/gitcode_api) - GitCode 专用 API 客户端

## 许可证

MIT

## 贡献

欢迎提交 Issue 和 Pull Request！

// test code review at Sun Jun 14 07:09:10 CST 2026
