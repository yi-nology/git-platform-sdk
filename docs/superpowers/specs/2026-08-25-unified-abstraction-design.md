# git-platform-sdk 统一抽象全量架构评审与演进路线图

日期:2026-08-25
状态:已批准(2026-08-25,见 §6 决策记录)
范围:SDK 级统一抽象(provider / backends / transport / contracttest),不含 gitbackend 内部演进(仅涉及其与 provider 的边界)

---

## 0. 结论摘要

**总体判断:架构分层正确、不需要重写。** 统一模型单一归属、声明式能力发现、契约测试双向锁定、transport RoundTripper 注入,这四个根基决策都经受住了 7 个平台的检验,应保持不动。

真正悬而未决的是 4 个架构级决策点:

| # | 决策点 | 严重度 | 推荐方案 |
|---|--------|--------|----------|
| A | 平台差异语义显式化(ignore/mapping 无可编程信号) | P0 | 差异台账代码化(`Divergence` 注册表 + 文档生成) |
| B | 能力降级模型(核心 8 能力无降级,弱平台被迫造 stub) | P1 | 保持两层结构,用台账表达方法级缺席,不拆接口 |
| C | 扩展成本收敛(7 份样板、3 份重复 label 扫描) | P1 | 扩充 `backends/internal` 适配器工具箱 |
| D | mapping ledger 设计文档(spec §4.6)不在仓库 | P1 | 与 A 合并:台账即代码,文档由 go:generate 产出 |

核心设计思路一句话:**把今天散落在代码注释里的 "Registered mapping / stub / ignore / detour" 约定,升级为 provider 包内机器可读的差异注册表;它同时是丢失的 §4.6 文档、运行时可探测的降级信号、以及契约测试的锁定对象。**

---

## 1. 现状盘点

### 1.1 分层架构

```
应用层(git-sync-service、examples)
│
├── provider/        统一抽象层
│   ├── 13 个能力接口(8 核心 + 5 可选)
│   ├── 统一模型(PlatformRepo、ChangeRequest…全部归属此包)
│   ├── CapabilitySet 声明式能力发现 + 注册表/工厂/Manager(TTL+LRU 缓存)
│   └── webhook 归一化 + 签名验证注册表
│
├── backends/<7 平台>  适配器层(gitcode/gitlab/github/gitee/gitea/forgejo/tencentcode)
│   └── 全部包装第三方 SDK,types.go 只做 SDK 模型 → provider 模型转换
│
├── transport/       唯一 HTTP 出口(auth/retry/hooks/ratelimit)
│   └── 以 RoundTripper 注入第三方 SDK 的 http.Client;raw client 留作 SDK 缺口旁路
│
├── backends/contracttest/  跨后端行为契约测试(7 平台全覆盖)
├── backends/internal/backendutil  后端间共享装配代码(仅 HTTP 装配)
│
├── gitbackend/      本地 git 操作第二抽象轴(native + gogit),与 provider 正交
└── pkg/             credential / encoding / branchfilter
```

### 1.2 做对了的事(评审结论:保持,不要动)

1. **统一模型单一归属**:所有模型类型在 `provider` 包定义,后端只写 `convertXxx`,消费方零平台类型泄漏。
2. **声明式能力发现**:`Capabilities() CapabilitySet`(provider/provider.go:35)静态声明 + 契约测试 `Capabilities_Consistency` 双向锁定"声明=实现",拒绝运行时探测。
3. **transport 注入模式**:7 个后端构造函数同构,第三方 SDK 流量全部经过 auth/retry/hooks 管线;raw detour 旁路也走同一 transport(gitee 的多处 detour 才能被日志/重试覆盖)。
4. **契约测试框架**:连 `Retry_On5xx` 都断言"服务端确实收到 ≥2 次请求"这种接线证据;可选能力套件双向防漂移(声明能力必须提供 config,未声明不得提供)。
5. **"登记"文档约定**:registered mapping / stub / ignore / detour 四类变形行为在注释中有台账,平台差异透明可追溯——问题是它只在注释里(见 §2 P0)。
6. **规整工具下沉**:`NormalizePageOpts`、标签颜色规整、`MapStateToCR`、`SumDiffStats` 等横切规整已进 provider 包。

### 1.3 在途工作确认

当前工作区未提交改动为 `gitcode_api → go-gitcode` 依赖更名(go.mod v0.7.0 → v0.7.2 及引用替换),与本路线图无冲突。

---

## 2. 架构问题分级清单

### P0:语义静默降级(调用方不可探测)

**问题**:四类变形行为中,只有 stub 有可编程信号;ignore 完全无信号,mapping 只有文档:

| 行为 | 返回值 | 调用方可探测? | 例证 |
|------|--------|--------------|------|
| registered stub | `Wrap(..., ErrNotImplemented)` | ✅ `errors.Is` | gitee `CreateCommitStatus`(backends/gitee/commits.go:86) |
| registered ignore | `nil` | ❌ 假成功 | gitlab/tencentcode `RequestReviewers`(backends/gitlab/reviews.go:78)、gitlab/工蜂 issue `Assignees` 被丢弃(backends/gitlab/issues.go:76) |
| registered mapping | 合成/近似数据 | ❌ 语义变形 | gitlab Reviews = approvals 汇总,合成 review 的 ID 全是 MR IID(backends/gitlab/reviews.go:13-105) |
| registered detour | 正常 | ❌(可接受) | gitee 多处 raw 绕行(backends/gitee/gitee.go:5) |

契约测试甚至把"ignore 返回 nil + 零网络"锁定为正确行为(contracttest/reviews.go 对 `RequestReviewers` 的断言)——在缺上下文时这是对的选择,但调用方(git-sync-service)无法区分"生效了"和"什么都没发生"。

### P1-a:核心 8 能力无降级路径

`Provider` 强制组合 8 个 Manager(provider/provider.go:58),弱平台被迫造 stub 凑接口(gitee `CreateCommitStatus`)。若未来接入连分支写都不支持的平台,接口模型无法表达。

### P1-b:mapping ledger 文档缺失

全仓库多处注释引用 "Registered …(spec §4.6)",但该 spec 不在仓库内(仅 README/CHANGELOG/CONTRIBUTING)。新人无法追溯这些登记的完整规则,登记约定靠口口相传。

### P1-c:Capabilities 是手工同步点

新增可选能力需同时改:CapabilitySet 字段 → 7 个后端 `Capabilities()` → 契约测试(provider.go:32 注释自认)。

### P2-a:重复样板(扩展成本)

- `resolveLabelID` 名→ID 翻页扫描在 gitlab/gitea/forgejo 三份近似复制(各 :83/:81/:81),且 50 页上限内找不到会误报 not found;
- username→ID 解析缺失直接造成 2 个 registered ignore(gitlab `Assignees`、`ReviewerIDs`);
- 7 份同构构造函数已被 backendutil 部分吸收,但仅限 HTTP 装配。

### P2-b:标识符语义漂移

`Milestone.Number` 在 7 平台承载三种含义(GitHub=序号 / GitLab 系=ID / Gitee=另一套序号),已在接口文档坦白登记(provider/iface_milestones.go:5);`ChangeRequest.BaseSHA` 在 GitHub/Gitea 退化为 target tip(provider/provider.go:116)。跨平台移植标识符不可行,但"仅本平台往返"的约定只有文档。

### P2-c:其余已知限制(登记,不单独立项)

gitea/forgejo SDK 方法不收 ctx;搜索无统一 total(GitLab 系用页大小冒充);gitlab `UpdateRelease` 忽略 Draft/Prerelease;gitee `ChangeRequest.Draft` 恒 false;label 扫描无缓存。

---

## 3. 决策点与方案

### 决策 A:平台差异语义显式化(P0)

**目标**:让消费方在编译期或启动期知道"这个平台在这个方法上会变形",而不是在生产里踩坑。

**方案 A1:方法签名携带警告**(如返回 `*Result{Data, Warnings}`)
- ❌ 全部 13 个接口签名重写,7 个后端全动,消费方全动;Go 惯例上双返回值 `(T, error)` 之外加警告通道很笨重。否决。

**方案 A2:ignore 也返回 `ErrNotImplemented` 类错误**
- ❌ 混淆"方法整体缺席"和"个别字段被忽略"两种粒度;把"部分成功"报告为失败,破坏现有契约(git-sync-service 会把能用的调用也判死)。否决。

**方案 A3(推荐):差异台账代码化——`Divergence` 注册表**

```go
// provider/divergence.go(新增)
type DivergenceKind string

const (
    DivergenceStub    DivergenceKind = "stub"    // 方法整体缺席,返回 ErrNotImplemented
    DivergenceIgnore  DivergenceKind = "ignore"  // 字段/参数被静默忽略,方法仍返回成功
    DivergenceMapping DivergenceKind = "mapping" // 语义映射/合成,返回近似数据
    DivergenceDetour  DivergenceKind = "detour"  // 绕过第三方 SDK 走 raw wire
)

type Divergence struct {
    Capability string        // "ReviewManager"
    Method     string        // "RequestReviewers"
    Field      string        // "reviewers"(ignore/mapping 粒度时)
    Kind       DivergenceKind
    Reason     string        // 一句话原因,供生成文档与 UI 提示
}

// Provider 接口新增必须方法(与 CapabilitySet 同构的静态声明):
Divergences() []Divergence
```

配套:
- **消费方助手**:`provider.IgnoresField(p, "UpdateIssue", "assignees") bool`,git-sync-service 可在 UI 上展示"该平台不支持指派人";
- **文档生成**:`go generate` 遍历注册表产出 `docs/divergence-ledger.md`,重建丢失的 §4.6;
- **契约测试新增 `Divergence_Suite`**:声明 ignore 的方法 → mock 零请求 + 成功返回;声明 stub 的方法 → `errors.Is(err, ErrNotImplemented)` + 零请求(把 reviews 套件里已有的这两个断言模式推广成通用套件);
- **注释规范反转**:后端代码注释从"Registered ignore (spec §4.6)"改为引用注册表条目,单一事实来源。

- ✅ 不改任何方法签名;零消费方 breaking(新增接口方法只冲击仓库内 7 个后端,它们本来就要登记);运行时行为完全不变;与 CapabilitySet 模式同构。
- **决策(2026-08-25 拍板):采用 Provider 必须方法形态,不做可选接口备案。**

### 决策 B:能力降级模型(P1-a)

**方案 B1:保持现状**(核心 8 强制 + 5 可选)
- 弱平台继续造 stub。stub 本身有 `ErrNotImplemented` 信号,机制是对的。

**方案 B2:13 个能力全部可选**
- ❌ 现有 7 平台都实现了核心 8,可选化是为假想的未来平台向所有真实消费方收税(每次调用都要断言)。否决(YAGNI)。

**方案 B3(推荐,已采纳):保持两层结构 + 一处外科手术式拆分**
- "某平台缺某方法"由 `Divergence{Kind: stub}` 声明,消费方启动期即可拼出每平台真实方法矩阵;只有当真实出现"连核心读能力都缺"的平台时,再把该能力降级为可选——到那时再付这个成本。
- 唯一拆分:`CreateCommitStatus` 从 `CommitManager` 移入新的可选 `CommitStatusManager`(`CapabilitySet` 增加 `CommitStatuses` 字段)。它是纯 CI 语义的方法,与 commit 读操作无关;拆出后 gitee 的 stub 直接消失(能力建模优于 stub),消费方按 `Capabilities().CommitStatuses` 门控,与 Issues/Reviews 的既有模式一致。
- 配套:`Capabilities_Consistency` 套件扩展为同时校验"台账中 stub 的方法所属能力,若为可选能力则不得同时声明该能力"(防止 stub 与 Capabilities 打架)。

### 决策 C:扩展成本收敛(P2-a)

**方案 C1(推荐):扩充 `backends/internal` 适配器工具箱**

1. `internal/backendutil` 新增 `LabelResolver`:名→ID 解析 + 分页扫描,内置 per-provider 缓存(TTL 与 Manager 对齐),三份重复实现收敛为一份,顺带修掉 50 页上限误报问题(改为显式返回"标签过多"错误而非 not found);
2. 新增 `UserResolver`:username→ID(gitlab Users API / 工蜂用户查询),**直接消灭 2 个 registered ignore**(gitlab issue Assignees、ReviewerIDs),把 P0 问题从"文档化"升级为"修复";
3. `convertXxx` 公共片段(时间解析、指针解引用、颜色规整)按需下沉,不追求一次性抽象。

**方案 C2:从 OpenAPI 生成适配器骨架** —— ❌ 过度工程,7 平台中 4 个的第三方 SDK 本身就是生成器产物,再包一层生成只会放大 go-gitee 式的质量问题。否决(YAGNI)。

**方案 C3:不动** —— 每新增一个能力的边际成本继续是 7×(接口+登记+样板),与 contracttest 的双向锁定一起构成主要维护负担。仅当 C1 与决策 A 冲突时退回。

### 决策 D:mapping ledger 回归(P1-b)

- **D1(推荐)**:并入决策 A——台账即代码,`docs/divergence-ledger.md` 由 go:generate 从注册表产出,永不过期。
- D2:手写 markdown 文档 —— 立刻过时,重蹈 §4.6 覆辙。否决。

---

## 4. 演进路线图

> 工作量为粗估(单人,含测试与文档);每项可独立提交、独立回滚。Phase 间无硬依赖,但推荐按序。

### Phase 0:台账落地(决策 A+D)——估 2~3 天

1. `provider/divergence.go`:类型 + `Provider.Divergences()` 方法 + `IgnoresField` 等助手;
2. 7 个后端登记存量差异(把现有注释台账翻译成注册表条目,预计 25~35 条);
3. `contracttest` 新增 `Divergence_Suite`(推广 reviews 套件的零请求断言);
4. `go generate` 产出 `docs/divergence-ledger.md`;
5. README 增补"差异台账"一节。

**验收**:7 后端契约测试全绿;ledger 文档与注册表条目一一对应(生成器测试锁定);`IgnoresField` 有单测。

### Phase 1:工具收敛(决策 C)——估 2~3 天

1. `LabelResolver` 收敛三份扫描 + 缓存 + 上限语义修复;
2. `UserResolver` 落地 gitlab(client-go 暴露 Users 查询),修复 gitlab 的两个 registered ignore(issue `Assignees`、`RequestReviewers` 的 ReviewerIDs;**从台账中删除这两条,消费方行为变化需在 CHANGELOG 显著标注**)。工蜂是否纳入视其 SDK 是否暴露用户查询接口再定;
3. 相应契约测试更新。

**验收**:gitlab `UpdateIssue` 带 Assignees 时 mock 收到 assignee_ids;三平台 label 更新路径共享同一 resolver。

### Phase 2:能力矩阵硬化(决策 B)——估 1 天

1. `Capabilities_Consistency` 扩展覆盖新增能力字段(`CommitStatuses`)。注意:方法级 stub 与已声明的可选能力并存是合法形态(部分缺席,如 tencentcode 的 DismissReview);全能力缺席已由既有双向一致性测试强制走 Capabilities 声明,无需额外互斥校验(2026-08-25 计划阶段精化);
2. CONTRIBUTING 固化"新增可选能力 checklist"(CapabilitySet 字段 → 各后端 → 契约套件 → 台账登记,四处联动清单)。

### Phase 3:清洁 API 收口——估 2~3 天(2026-08-25 拍板:由触发式改为立即实施)

1. **`CommitStatusManager` 拆分**(决策 B):`CreateCommitStatus` 移出 `CommitManager`,gitee 删除 stub,支持的平台声明 `Capabilities().CommitStatuses`;契约测试新增对应套件,基础套件同步调整。
2. **搜索 total 诚实化**:各 `Search*Result.Total` 改为 `*int`(nil = 平台不返回),移除"用页大小冒充 total"的行为;CHANGELOG 标注 breaking。

以下两项**以台账条目显式登记为永久限制**——可见的限制不是历史债,伪造的取消语义和投机的 API 才是:

- gitea/forgejo ctx 传播:其 SDK 方法不接受 ctx,goroutine+select 只能假装取消(底层请求继续跑),属于制造新债,不做;
- Milestone 跨平台可移植标识:无真实消费需求前不造投机 API。

### 风险与回滚

- 最大的行为变更是 Phase 1 的两个 ignore 修复(原来"成功无效果"变为"真实生效"),对 git-sync-service 是功能增强但需回归测试。**已拍板:直接生效,不加过渡开关**——过渡开关本身就是历史债。

---

## 5. 明确不做(YAGNI)

1. **不重写任何后端**,不替换 go-gitee(其缺陷由 detour + transport 兜底,成本已沉没);
2. **不引入运行时能力探测**(静态声明 + 契约测试锁定的现有路线是对的);
3. **不做适配器代码生成**;
4. **不追求跨平台标识符可移植**(Milestone.Number/BaseSHA 维持"仅本平台往返"文档约定);
5. **不合并 gitbackend 进 provider**(REST 平台轴与本地 git 轴正交,现边界正确);
6. **不做方法签名层面的警告通道**(A1 已否决)。

---

## 6. 决策记录(2026-08-25,用户拍板:"全部实施,不留历史债")

1. `Divergences()` 为 `Provider` 必须方法,不做可选接口备案。
2. Phase 1 的 ignore 修复直接生效,不加过渡开关。
3. Phase 3 前两项(CommitStatusManager 拆分、搜索 total 诚实化)提升为立即实施;后两项(ctx 传播、Milestone 可移植标识)登记为台账中的永久限制。
