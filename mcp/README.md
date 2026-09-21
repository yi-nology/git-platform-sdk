# git-platform-mcp

一个 MCP(Model Context Protocol)server,把 [git-platform-sdk](../) 的七平台统一
API(GitHub / GitLab / Gitea / Forgejo / Gitee / GitCode / 腾讯工蜂)暴露成
AI agent 可直接调用的工具面。

四个平台的官方 MCP server 都只覆盖自家;这个底座一次接入七家,国内三家
(Gitee / GitCode / 工蜂)此前没有任何 MCP 实现。

## 工具面

按 toolset 分组,构造时按 provider 的 `Capabilities()` 声明做能力门控——
平台不支持的工具根本不会注册,模型永远看不到注定失败的调用:

| Toolset | 工具 |
|---|---|
| `core`(必有) | `get_repo` `list_repos` `get_file` `list_branches` `list_crs` `get_cr` `get_cr_files` `get_commit` `list_commits` |
| `crs`(必有) | `create_cr` `merge_cr` `add_cr_comment`(写) |
| `issues`(需 Capability) | `list_issues` `get_issue` `create_issue`(写) `close_issue`(写) `add_issue_comment`(写) |
| `status`(需 CommitStatuses) | `get_commit_statuses` `set_commit_status`(写) `wait_for_status` |
| `search`(需 Search) | `search_repositories` `search_issues` `search_users` |

上下文经济设计(承接 [github/github-mcp-server] 的 toolset 实践):

- `--read-only` 在**注册期**直接丢弃全部写工具,模型看不见也就调不了;
- `list_crs` / `list_issues` 支持 `fields` 投影参数(复用
  [pkg/projection](../pkg/projection)),大列表只取
  `["number","title","head.ref"]` 这类字段,不再为 50 个 PR 烧掉整段上下文;
- `wait_for_status` 内置轮询原语(provider.WaitForCommitStatus),
  agent 提交状态后一条工具调用即可等待 CI 终态。

## 使用

```bash
go build ./cmd/git-platform-mcp

# Gitea 自托管实例,token 从环境变量读
GIT_PLATFORM_TOKEN=xxx ./git-platform-mcp \
  --platform gitea --base-url https://gitea.example.com --read-only
```

客户端配置(stdio):

```json
{
  "mcpServers": {
    "git-platform": {
      "command": "git-platform-mcp",
      "args": ["--platform", "gitlab", "--token-env", "GITLAB_TOKEN", "--read-only"]
    }
  }
}
```

Flags:`--platform`(必填)`--base-url` `--token` / `--token-env`
`--skip-tls` `--read-only` `--toolsets core,status`(逗号分隔子集)。

## 测试

```bash
go test ./
```

测试通过官方 SDK 的 InMemoryTransport 做端到端验证:toolset 挂载、
能力门控、只读模式丢写工具、真实工具调用往返。
