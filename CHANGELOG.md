# Changelog

All notable changes to this project are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.71.0] - 2026-10-01

### Fixed

- **native 输出解析器加固**（深度审查发现的正确性缺陷，均实测复现）：
  - `GetCommitsBetween`/`GetCommit`/`GetFileHistory`：提交标题含 `|`
  时（如 "fix: handle a | b"），旧的管道分隔把 **Author/Date 解析
  错位**；改用 NUL（`%x00`）分隔 + 共享解析函数
  - `GetTagList`/`ListBranches`：TAB 分隔 + subject 置尾整体吸收
  （for-each-ref 类 format 不支持 `%x00`，TAB 是能做到的最强分隔）
  - **annotated tag 的 Author 恒空**（顺带修复）：tag 对象身份在
  tagger 而非 author，条件格式 `%(if)%(taggername)…%(else)
  %(authorname)%(end)` 兼容 annotated/lightweight 两种
  - `isConflictOutput` 收紧为 `CONFLICT (` 大写标记：合并不存在的
  "conflict-*" 分支等无关失败曾被误判为 `ErrMergeConflict`
- 回归测试：含 `|` 的提交标题、标签消息、分支 subject，及冲突判定
  分类用例表

### Changed

- 依赖复核：v0.69.0 后无新版直接依赖发布，全部仍为最新
  （go-git v6 仍为 alpha，不上）

## [0.70.0] - 2026-10-01

### Added

- **native 后端 SSH 指纹钉扎真正生效**（`HostKeyFingerprint`）：
  - 此前 native 路径的钉扎只退化为 `StrictHostKeyChecking=yes` + 已有
    known_hosts，指纹从未被比对；gogit 路径却严格匹配——同一配置两个
    后端语义不一致，native 用户"钉了个寂寞"
  - 现在跑 git 前解析本次命令要连的主机（argv 的 ssh:// URL，或经
    `git remote get-url` 解析 remote，支持 `fetch --all` 多 remote），
    `ssh-keyscan` 抓公钥、在 Go 里比对 SHA256 指纹，匹配的行写入
    `0600` 临时 known_hosts（用后即删），git 强制校验
  - **fail-closed**：联网命令解析不出主机、keyscan 不可用、指纹零匹配
    （可能 MITM/pin 过期）均直接报错；非联网子命令（status 等）不受影响
  - TOFU-then-pin：密钥经网络获得后立即与钉扎比对，MITM 假密钥不会命中
  - 端到端测试内嵌 x/crypto/ssh 服务端 + 真实 ssh-keyscan（正向/反向/
    remote 解析/clone argv 四路）

## [0.69.0] - 2026-10-01

### Changed

- **依赖全量保鲜**（`go get -u ./...`）：
  - 直接：`gitlab client-go/v3` v3.12.0 → **v3.15.0**；`gitea.dev/sdk`
    v1.2.0 → **v1.3.0**
  - 间接（crypto/传输类）：`ProtonMail/go-crypto` v1.5.2、
    `cloudflare/circl` v1.6.5、`pjbgf/sha1cd` v0.7.0、
    `skeema/knownhosts` v1.3.3、`x/net` v0.59.0、`go-openapi` 全家 0.29.x
  - 验证：全量 `go test -race`、hermetic 模拟环境、lint、govulncheck
    （0 可达漏洞；`x/crypto/openpgp` GO-2026-5932 为不可达且无修复版本
    的"无人维护"公告，升级前即存在）
  - 其余直接依赖已最新：go-github v92（无 v93）、go-git v5.19.2
    （v6 仅 alpha）、forgejo-sdk v3

## [0.68.2] - 2026-10-01

### Fixed

- **Lint 修复（CI Lint 腿红）**：`gitlab.Ptr` 已被 go-gitlab 废弃（SA1019），
  10 处全部改用 go 1.26 内置 `new(value)`；`tailOfReader` 参数 `max`
  遮蔽内建标识符改名 `budget`；注释 `//` 后补空格；`types.go` gofmt。

## [0.68.1] - 2026-10-01

### Fixed

- **测试夹具 hermetic 化补全（CI/Release 连续全红的根因）**：夹具的
  `git commit/merge/rebase/cherry-pick` 依赖本机全局 git 身份；CI runner
  无全局身份 → `git` exit 128，自 09-27 起（含 v0.65.0~v0.68.0）所有
  CI 与 Release 工作流失败。
  - `gitOutput` 统一注入 `GIT_AUTHOR_*`/`GIT_COMMITTER_*`，失败时带出 stderr
  - `createTestRepo` 设置仓库本地身份——被测 backend 的 Merge/Rebase
    也不依赖机器环境
  - 裸 `exec.Command(...).Run()`（吞错）的夹具调用全部改走 `gitOutput`
  - 本地以 `GIT_CONFIG_GLOBAL=/dev/null` 模拟 runner 复现并验证归零

## [0.68.0] - 2026-10-01

### Changed

- **认证路径重构（credential helper）**：HTTPS 令牌改经**临时 credential helper +
  GIT_ASKPASS** 注入 git（对标 gickup）：
  - 令牌写入 `0600` 临时文件；helper 脚本读文件回 `password=`
  - **不再**把 `Authorization: Basic …` 放进 `http.extraheader` / `GIT_CONFIG_VALUE_*`
  - **令牌不进 argv、不进 git 进程 environ 明文**；会话目录 RAII 清理
  - `configureAuth` 返回 cleanup；`runGitEnv` 修复 extraEnv 覆盖认证 env 的问题
  - 建会话失败时回退旧 extraheader（保持可用性，并 `logger.Warn` 留痕）
  - env 注入**空串 `credential.helper=` 先于本次 helper**：清空主机已配置的
    helper 列表（osxkeychain / store / cache）。已实测两处回归风险：
    (a) 主机 helper 先被查询，个人凭证会**遮蔽**本次令牌（GitHub/GitLab
    推送成错误身份）；(b) 认证成功后 git 对全部 helper 执行 `store`，
    一次性令牌会被**持久化进用户钥匙串 / 明文凭证文件**。
  - 移除无效的 `credential.useHttpPath`；helper/askpass 路径 `ToSlash`
    以兼容 Windows 上的 msys sh
  - 集成测试用真实 git 钉死上述两条（主机 helper 不被 get/store 触达）

## [0.67.1] - 2026-10-01

### Fixed

- **GitLab `convertBasicMR` 补 `Draft`/`HeadSHA` 映射**：poller 列表路径
  恒零值导致上层兜底逻辑全静默失效。

## [0.67.0] - 2026-10-01

### Added

- **GitLab CI 失败日志能力 + pipeline 失败事件**：`CILogManager` 可选接口，
  pipeline 失败终态进入事件流，供上层拉取失败作业日志。

## [0.66.0] - 2026-09-28

### Added

- **GitLab diff 感知行内定位**：评论 position 经 hunk 映射到新旧行号，
  定位失败降级为 file 级 position。

## [0.65.0] - 2026-09-27

### Added

- **GitLab `CreateReview` 行内评论（discussions）**：支持 position 级
  行内 discussion。

### Fixed

- 行内 discussion 失败留痕（`logger.Warn`），不再静默全丢。
- `diff_refs` 降级与行内路径日志；清理编辑残渣。

## [0.64.0] - 2026-09-27

### Added

- **PlatformRepo 仓库元数据**: `Archived`/`Fork`/`Stars`/`Language`(供导入过滤);
  `CloneOptions` 部分克隆(`Filter: blob:none` 等)与 `Submodules`。

### Fixed

- **webhook 验证对齐真实平台协议**: GitCode 改用文档规定的 `X-GitCode-Token`
  头; Gitee 实现 sign(Base64 HMAC-SHA256 + 时间戳新鲜度)与 password 双模式。
- **gitbackend**: Fetch 结果两后端对称(新增/更新/删除分支分类); `FetchAll`
  改单次全量(原逐分支 N+1); worktree 写回保留可执行位/符号链接、不再吞错;
  工厂回退 gogit 时留痕; `branchfilter` 拒绝坏模式; 两后端默认启用 SSH
  主机密钥校验; tag/rebase/checkout(`CheckoutDetached`)一批语义修复。
- **transport**: RoundTrip 并发竞态(共享 Client 不再被改写); 大响应体不再
  截断; 限流器锁内不再休眠(剩余 0 也节流、预约定防惊群); `Retry-After`
  封顶; 重试幂等感知(POST 仅 DNS/dial 失败可重试); 拒绝型请求 hook 在
  SDK 路径同样生效。
- **provider**: `DetectPlatform` 精确主机匹配(自托管域名不再误路由公共云);
  Manager janitor 可重启; `Wrap` 保留原始错误链; 状态码提取白名单化;
  batch 取消快速失败。
- `CommitWithIdentity` 显式设 `GIT_AUTHOR_*/GIT_COMMITTER_*` 压过进程环境;
  测试 hermetic 化第一轮(修复 4 个受本机身份污染的用例)。

### Changed

- `transport.Error.StatusCode` 改为方法(启用 statusCoder 快路径)。

## [v0.62.0] - 2026-09-20

### Added

- **无分页参数的列表方法全量翻页**: 14 个 List 方法此前只回服务器默认首页
  (10-30 条), 统一走 `backendutil.AllPages`; 带 `Page` 字段的方法获得双语义
  (`Page==0` 全量、`>0` 调用方驱动单页), 契约套件钉死翻页走查。
- **`CommitStatusManager.ListCommitStatuses`**(七平台)+ 统一
  `CommitStatus`/`CommitStatusState` 词表。
- **Agent 原语落地**: `provider.WaitForCommitStatus`(CI 门禁轮询)、
  batch 有界并发读(`provider/batch.go`)、字段投影(`pkg/projection`)。
- **`mcp/` 模块**: MCP server(独立 go.mod, toolset 能力门控 + 读写分离)。
- `examples/capabilities` 能力巡检; CONTRIBUTING 落地 SemVer 契约。

### Changed

- **三后端 SDK 大迁移**(统一面不变): GitHub go-github v72→v92、
  GitLab client-go v2→v3.12、Gitea SDK v0.25→新版; 其余依赖追平。

### Fixed

- **gitbackend 注入加固**: `runGit` 子命令白名单 + `-c`/`--exec` 等
  exec 类 flag 拒收; `GetConfig`/`SetConfig` 配置键注入防护。

## 历史版本摘要(v0.38.0 – v0.61.0)

> 更早版本的完整条目见各版 GitHub Release 与 git 历史;此处保留一行摘要
> 供检索迁移线索。⚠️ = 含破坏性变更。

- **v0.61.0**(2026-09-03): Webhook 改进、`ListCRReactions`(CR 表情读面)、CI 加固。
- **v0.60.0**(2026-09-03): Gitea/Forgejo `convertUser` 补 `Name` 字段。
- **v0.59.0**(2026-09-03): 全后端空值防护/错误包装/分页修复/去重。
- **v0.58.0**(2026-09-02): `UserManager` 可选能力(username→平台 ID 解析)。
- **v0.57.0**(2026-09-01): 标签名→ID 批量解析 + per-provider TTL 缓存(原逐标签一次调用)。
- **v0.56.0**(2026-09-01): 去重/错误处理标准化/webhook 事件归一(`NormalizedEvent`)。
- **v0.55.0**(2026-09-01): Gitee `DeploymentKeyManager` + `CommitStatusManager`(Checks API 映射)。
- **v0.54.0**(2026-09-01): Gitee 后端弃用 swagger 生成 SDK, 换 `next-bin/go-gitee`(22 处 raw 绕行重落新 SDK)。
- **v0.53.0**(2026-09-01): `BranchProtection/Collaborators/DeployKeys/RepoStats` 四项可选能力 + `ChangeRequest.Assignees`。
- **v0.52.0**(2026-08-31): `NotificationManager` + `ReactionManager` 可选能力。
- **v0.51.0**(2026-08-31): GitLab `TokenStyle`(bearer, 如 `CI_JOB_TOKEN`)。
- **v0.50.0**(2026-08-30): `ListIssueLabels`/`ListIssueComments` 七平台翻页补全(原只回首页)。
- **v0.49.0**(2026-08-30): `IssueManager.UpdateIssueComment` 七平台。
- **v0.48.0**(2026-08-29): 依赖刷新(go-github v72 等)+ toolchain 钉 go1.26.6(修 7 个 stdlib CVE)。
- **v0.47.0**(2026-08-28): 工蜂 issue assignees 生效; gitbackend 9 个 bug 修复(gogit merge/fetch/rebase、native 冲突判定等), 覆盖率 15.8%→76.9%; Dependabot + 覆盖率地板 45%。
- **v0.46.0**(2026-08-27): Manager 真 LRU 驱逐; RateLimiter 改 `x/time/rate`; 移除 `pkg/encoding`。
- **v0.45.0**(2026-08-26): GitLab `RequestReviewers`/issue assignees 真正生效(username→ID 解析)。
- **v0.44.0**(2026-08-24): GitLab `ListRepos` 限定 token 用户范围。
- **v0.43.0**(2026-08-16): GitCode SDK v0.7.0, 解锁 `GetArchive`/`CreateCommitStatus` 最后两个桩。
- **v0.42.0**(2026-08-16): 工蜂声明 `Reviews` 能力(MR notes 映射, 四项登记限制)。
- **v0.41.0**(2026-08-16) ⚠️: CR/DiffManager 13 个方法 `number` int→string。
- **v0.40.0**(2026-08-15) ⚠️: IssueManager 8 个方法与 `Issue.Number` int→string; Milestone 选项/`MilestoneRef.Number` string 化; `CreateReview` 移入新的 `ReviewManager`。
- **v0.39.0**(2026-08-15) ⚠️: `Issue.Milestone` string→`*MilestoneRef{Number,Title}`。
- **v0.38.1**(2026-08-15): `provider.Wrap` 对非 struct 错误不再 panic。
- **v0.38.0**(2026-08-15) ⚠️: `IssueManager`/`SearchManager` 移出 `Provider` 接口(可选能力化)。
