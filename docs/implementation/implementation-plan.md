# 完整开发计划

## 目标

本文件定义系统从空仓库到可用版本的渐进式开发计划。计划需满足三个要求：

- 可渐进：每一阶段结束后，系统都有明确可工作的边界
- 可验收：每一阶段都有可执行的验收项和通过标准
- 可回溯：需求、设计、里程碑、测试和产出之间可建立稳定追踪关系

本计划是 `docs/architecture.md`、`docs/design/*` 和测试策略之间的执行连接层。

---

## 计划原则

### 渐进式原则

- 先稳定配置和数据结构，再扩展行为逻辑
- 先拿到原始节点，再构建分组和路由
- 先完成可渲染的中间表示，再接入 HTTP 服务
- 每个阶段只解决一类主要问题，避免跨阶段耦合

### 可验收原则

- 每个阶段都要有明确产物
- 每个阶段都要有最小可执行验证
- 每个阶段都要说明已覆盖范围与未覆盖风险

### 可回溯原则

- 需求、设计、里程碑、测试统一编号
- 所有实现项都能追溯到需求来源
- 所有验收结果都能追溯到具体阶段和测试项

---

## 编号体系

### 需求编号

| 编号 | 需求 |
|------|------|
| `REQ-01` | 支持单用户、单配置文件运行模式 |
| `REQ-02` | 支持多个 SS 订阅源 |
| `REQ-03` | 支持自定义代理节点 |
| `REQ-04` | 支持链式节点与链式组生成 |
| `REQ-05` | 链式组属于节点组，且策略显式声明 |
| `REQ-06` | 支持地区节点组，且保留书写顺序 |
| `REQ-07` | 支持服务组，且保留书写顺序 |
| `REQ-08` | `@all` 仅展开原始节点，不含链式节点 |
| `REQ-09` | 支持 rulesets、rules、fallback |
| `REQ-10` | 同一中间表示输出 Clash Meta 与 Surge |
| `REQ-11` | 提供 `/generate` 与 `/healthz` |
| `REQ-12` | 配置、引用和循环依赖等错误可校验和报告 |
| `REQ-13` | `@auto` 自动补充 routing 成员（节点组+@all 服务组+DIRECT） |

### 里程碑编号

- `M0`：工程基线
- `M1`：配置与模型
- `M2`：Source 与 Filter
- `M3`：Group 与 Route
- `M4`：校验与渲染
- `M5`：HTTP 服务与端到端验收

### 测试编号建议

- `T-CFG-*`：配置与保序
- `T-SRC-*`：订阅拉取与 SS 解析
- `T-FLT-*`：过滤
- `T-GRP-*`：分组与链式展开
- `T-RTE-*`：服务组与路由
- `T-VAL-*`：图级校验
- `T-RND-*`：渲染
- `T-E2E-*`：端到端测试

---

## 里程碑总览

| 里程碑 | 主题 | 完成状态 |
|------|------|------|
| `M0` | 工程基线 | ✅ 已完成 |
| `M1` | 配置与模型 | ✅ 已完成 |
| `M2` | Source 与 Filter | ✅ 已完成 |
| `M3` | Group 与 Route | ✅ 已完成 |
| `M4` | 校验与渲染 | ✅ 已完成 |
| `M5` | HTTP 与 E2E | ✅ 已完成 |

执行顺序：

```text
M0 -> M1 -> M2 -> M3 -> M4 -> M5
```

关键依赖：

- `M1` 未稳定前，不展开 `M2` 之后的实现
- `M2` 提供稳定原始节点输入，供 `M3` 使用
- `M3` 提供稳定中间表示，供 `M4` 渲染
- `M4` 完成后再接入 `M5`，避免 HTTP 层过早绑定未稳定逻辑

---

## M0: 工程基线 ✅

### 目标

建立后续开发可承载的最小工程骨架，避免后续阶段在目录、依赖和测试结构上反复返工。

### 工作项

- 初始化 `go.mod`（模块路径 `github.com/John-Robertt/subconverter`，Go 1.24）
- 建立推荐目录结构
- 增加 `configs/base_config.yaml`
- 约定 `testdata` 和示例输入目录
- 约定基本命令：格式化、测试、运行（Makefile）
- 明确错误分类：配置错误、拉取错误、构建错误、渲染错误

### 产物

- 最小 Go 工程骨架（`.gitignore`、`go.mod`、`Makefile`）
- 空包结构（`config`、`model`、`fetch`、`pipeline`、`render`、`server`）
- `internal/errtype`：四类错误类型（`ConfigError`、`FetchError`、`BuildError`、`RenderError`）及 9 个单元测试
- 示例配置草稿（`configs/base_config.yaml`）
- 测试数据（`testdata/subscriptions/sample.txt`：base64 编码的 SS URI 样本）
- 入口占位（`cmd/subconverter/main.go`）

### 验收项

- ✅ `go test ./...` 可执行（errtype 9 个测试通过）
- ✅ 目录结构符合 `project-structure.md`（补充 `internal/errtype`）
- ✅ 示例配置覆盖核心路径：订阅、地区组、链式组、routing、rulesets、fallback

### 实施记录

新增 `internal/errtype` 包作为对 `project-structure.md` 的补充。理由：四类错误横跨所有业务包，放在任何业务包中会造成循环依赖。`errtype` 与 `model` 一样是零依赖叶子包。

### 对应需求

- `REQ-01`

### 回溯点

- 里程碑记录：`M0-baseline`
- 问题边界：若后续出现包循环或目录失衡，回到本阶段修正

### 风险

- 包边界定义不清会导致后续依赖反转

---

## M1: 配置与模型 ✅

### 目标

把用户 YAML 稳定转换为系统内部可操作的配置对象和统一中间表示。

### 工作项

- 实现配置结构定义（`Config`、`Sources`、`CustomProxy`、`RelayThrough`、`Group`、`Filters`）
- 实现 `OrderedMap[V any]` 泛型保序映射
- 实现 YAML 加载器（`Load`）
- 实现静态配置校验（`Validate`，12 项校验规则，收集全部错误后一次返回）
- 实现统一中间表示模型（`Proxy`、`ProxyGroup`、`Ruleset`、`Rule`、`Pipeline`）
- 新增 `base_url` 顶层字段（用于 Surge Managed Profile）
- 让示例配置可成功加载并通过校验

### 产物

- `internal/config`：
  - `orderedmap.go`：自实现的泛型保序映射（~80 行），基于 yaml.v3 `MappingNode.Content` 遍历保序，支持 `Keys()`（防御性拷贝）、`Get()`、`Entries()`（Go 1.23+ `iter.Seq2`）
  - `config.go`：顶层 `Config` 及所有子结构体定义
  - `loader.go`：YAML 文件加载器，错误包装为 `*errtype.ConfigError`
  - `validate.go`：静态校验器，使用 `errors.Join` 收集多个 `*errtype.ConfigError`
- `internal/model`：
  - `model.go`：格式无关的中间表示类型，枚举使用 typed string constants
- 外部依赖：仅新增 `gopkg.in/yaml.v3`
- 测试数据：`testdata/config/minimal_valid.yaml`、`testdata/config/malformed.yaml`

### 验收项

- ✅ `groups`、`routing`、`rulesets` 顺序保持不变（T-CFG-001/002/003）
- ✅ `relay_through.strategy` 必填（T-CFG-004）
- ✅ 所有节点组策略都显式声明（T-CFG-005）
- ✅ 非法正则、缺失字段、非法枚举值、非法 URL 可返回错误
- ✅ 示例配置能加载为内存对象并通过校验
- ✅ `go test ./...` 全部通过

### 对应测试

- `T-CFG-001`：`groups` 保序解析 → `TestIntegration_GroupsOrder`
- `T-CFG-002`：`routing` 保序解析 → `TestIntegration_RoutingOrder`
- `T-CFG-003`：`rulesets` 保序解析 → `TestIntegration_RulesetsOrder`
- `T-CFG-004`：`relay_through.strategy` 缺失时报错 → `TestValidate_RelayThroughMissingStrategy`
- `T-CFG-005`：节点组 `strategy` 非法时报错 → `TestValidate_GroupInvalidStrategy`

### 实施记录

关键设计决策：

| 决策 | 结论 | 原因 |
|------|------|------|
| OrderedMap | 自实现泛型 `OrderedMap[V any]`，不用第三方库 | 需求极简（保序遍历+查找），~80 行代码，遵循依赖克制原则 |
| 保序机制 | 利用 yaml.v3 `MappingNode.Content` 的有序切片 | 标准库能力，无需额外工具 |
| 校验策略 | 收集全部错误后 `errors.Join` 一次返回 | 用户一次看到所有问题，避免逐个修复 |
| Model 枚举 | typed string constants（非 iota） | 可调试、可序列化、日志友好 |
| config 与 model | 两个包完全独立，无 import 关系 | config 是用户配置层，model 是系统语义层，转换在 M2-M3 |
| `base_url` | 顶层可选字段，用于 Surge `#!MANAGED-CONFIG` 头 | Surge 客户端需要自引用 URL 才能自动更新配置 |

### 对应需求

- `REQ-01`
- `REQ-05`
- `REQ-06`
- `REQ-07`
- `REQ-12`

### 回溯点

- 里程碑记录：`M1-config-model`
- 回溯边界：后续若出现顺序错误或字段校验错误，先回查本阶段

### 退出条件

- ✅ 顶层配置结构已冻结：`sources`、`filters`、`groups`、`routing`、`rulesets`、`rules`、`fallback`、`base_url`

### 已知限制

- 静态校验不做跨段引用检查（如 fallback 是否引用 routing 中的 key），留给 M4 图级校验
- 正则只编译不存储，M2/M3 pipeline 阶段按需重新编译
- `base_url` 在 M1 做静态格式校验，M5 server 层只负责将其拼接为外部 managed URL

---

## M2: Source 与 Filter ✅

### 目标

建立稳定的原始节点输入能力，确保系统能正确获取并清洗订阅节点。

### 工作项

- 实现订阅抓取器
- 实现 TTL 缓存
- 实现 SS URI 解析器
- 实现多订阅顺序拉取（顺序保证去重后缀确定性）
- 将自定义代理转换为原始节点
- 实现过滤逻辑

### 产物

- `internal/fetch`：
  - `fetch.go`：`Fetcher` 接口（单方法，便于测试注入）、`HTTPFetcher` 实现、`SanitizeURL`（URL 脱敏）
  - `cache.go`：`CachedFetcher`（TTL 缓存装饰器，实现 `Fetcher` 接口，可注入时钟）
- `internal/pipeline`：
  - `ssuri.go`：SIP002 风格 SS URI 解析器，支持 base64/base64url 与 plain userinfo，并保留 plugin 结构
  - `source.go`：Source 阶段编排——拉取→解码→解析→跨订阅去重→自定义代理转换→名称冲突检查
  - `filter.go`：Filter 阶段——`exclude` 正则仅作用于 `KindSubscription` 节点
- `testdata/subscriptions/sample_sub2.txt`：第二份订阅 fixture（含重名 HK-01，用于去重测试）
- 外部依赖：无新增

### 验收项

注：以下为 M2 当时的阶段验收记录；当前实现已在此基础上继续扩展 SS URI / plugin 支持能力。

- ✅ 支持合法 SS URI 解析（padded/unpadded base64、URL 编码中文 fragment、password 含冒号）
- ✅ 当前实现进一步支持 SIP002 风格 plain userinfo、query 参数与通用 plugin 解析
- ✅ 非法 SS URI 可识别并返回 `*errtype.BuildError{Phase: "source"}`（含端口范围 1-65535 校验）
- ✅ 多订阅结果可合并，跨订阅重名自动追加 ②③ 后缀，且二轮去重解决生成名与原始名碰撞
- ✅ `exclude` 仅影响订阅节点
- ✅ 自定义代理不受过滤影响（即使名称匹配 exclude 正则）
- ✅ 缓存命中与失效行为符合预期（可注入时钟验证）
- ✅ 自定义代理与订阅节点重名时返回错误
- ✅ 混合有效/无效 URI 时跳过无效行，保留有效节点
- ✅ 空订阅（0 个有效节点）返回错误
- ✅ `go test ./...` 全部通过

### 对应测试

- `T-SRC-001`：合法 SS URI 解析 → `TestParseSSURI_Valid`（当前已覆盖 plain userinfo、query、plugin 与转义场景）
- `T-SRC-002`：非法 SS URI 报错 → `TestParseSSURI_Invalid`（当前已覆盖 query 编码与 plugin 转义错误）
- `T-SRC-003`：多订阅合并 → `TestSource_MultiSubscriptionMerge`
- `T-FLT-001`：`exclude` 过滤订阅节点 → `TestFilter_ExcludeSubscriptionNodes`
- `T-FLT-002`：自定义代理不参与过滤 → `TestFilter_CustomProxiesNotFiltered`
- `T-SRC-004`：缓存 TTL 命中与失效 → `TestCachedFetcher_TTLHitAndMiss`
- `T-SRC-005`：去重后缀碰撞解决 → `TestSource_DedupSuffixCollision`

### 实施记录

关键设计决策：

| 决策 | 结论 | 原因 |
|------|------|------|
| Fetcher 抽象 | 单方法接口 `Fetcher`，pipeline 依赖接口 | 测试可注入 fake，不依赖网络 |
| 缓存模式 | `CachedFetcher` 装饰器，实现同一 `Fetcher` 接口 | 透明包装，可注入时钟测试 TTL |
| 拉取策略 | 顺序拉取，非并发 | 单用户通常 1-3 订阅，顺序保证去重后缀确定性；接口已为并发预留 |
| 去重命名 | 两轮去重：第一轮追加 ②③...⑩/(N) 后缀，第二轮解决生成名与原始名碰撞 | 保证节点名全局唯一，覆盖极端场景 |
| base64 解码 | 四种 base64 编码依次尝试（Std/Raw/URL/RawURL） | 兼容不同订阅商格式 |
| 解析容错 | 逐行解析，跳过无效 URI；整个订阅 0 有效节点报错 | 平衡容错与错误发现 |
| URL 脱敏 | `SanitizeURL` 剥离 query 和 fragment | 防止泄露用户 token |
| 缓存隔离 | 存储和返回都做防御性拷贝 | 防止调用方修改污染缓存数据 |

### 对应需求

- `REQ-02`
- `REQ-03`
- `REQ-12`

### 回溯点

- 里程碑记录：`M2-source-filter`
- 固定测试订阅响应作为回归输入

### 退出条件

- ✅ `Source` 输出稳定的 `[]model.Proxy`，可供 M3 Group 阶段消费
- ✅ `Filter` 输出过滤后的节点集合，自定义代理不受影响

### 已知限制

- 不支持并发拉取订阅（接口设计已预留，可后续添加 `errgroup` 并发而无破坏性变更）
- 不处理 `RelayThrough`（留给 M3 Group 阶段）
- 订阅体中非 SS URI 格式的行直接跳过，不记录警告日志（可在 M5 接入日志后补充）

### 风险

- 订阅返回格式存在兼容性差异（已通过 SIP002 解析、userinfo 多形态兼容与 plugin 解析缓解）
- 订阅可能包含空行、无效行或异常编码内容（已通过逐行解析 + 跳过无效行处理）

---

## M3: Group 与 Route ✅

### 目标

把节点集合稳定转换成节点组、服务组和路由绑定，形成系统业务语义层。

### 工作项

- 根据 `groups` 生成地区节点组
- 根据 `relay_through` 生成链式节点
- 自动生成链式组
- 计算 `@all`
- 构建服务组（含 `@auto` 展开）
- 装配 `rulesets`、`rules` 和 `fallback`

### 产物

- `internal/pipeline/`:
  - `group.go`：Group 阶段编排——地区组正则匹配→链式节点/组生成→@all 计算
  - `route.go`：Route 阶段编排——@auto 展开→服务组构建→@all 展开→规则集映射→内联规则解析→fallback 记录
- 外部依赖：无新增

### 验收项

- ✅ 地区组能按正则匹配节点（仅订阅节点参与匹配）
- ✅ 链式展开支持 `group`、`select`、`all` 三种模式
- ✅ 链式组策略来自 `relay_through.strategy`
- ✅ 链式组出现在节点组层（`Scope: ScopeNode`）
- ✅ 链式组命名为 `🔗 <custom_proxy.name>`（config-schema.md 约定）
- ✅ 链式节点属性（Type/Server/Port/Params/Dialer）正确传递
- ✅ `@all` 不包含链式节点
- ✅ 服务组能引用节点组、服务组、`DIRECT`、`REJECT`
- ✅ `@all` 在服务组 Members 中正确展开
- ✅ ruleset 与 fallback 能绑定到目标服务组
- ✅ 内联规则 Policy 从最后逗号后提取
- ✅ `@auto` 展开为节点组+@all 服务组+DIRECT（声明序）
- ✅ `@auto` 自动去重，组不包含自身
- ✅ `REJECT` 不在 `@auto` 中，需显式声明且位置保持不变
- ✅ 同一 entry 中重复 `@auto` 会被静态校验拒绝
- ✅ `@auto` 与 `@all` 在同一 entry 中互斥（静态校验拦截）
- ✅ 不含 `@auto` 的 entry 行为不变（向后兼容）
- ✅ `Route(cfg, nil)` 按空 `GroupResult` 处理，不发生 panic
- ✅ `go test ./...` 全部通过

### 对应测试

- `T-GRP-001`：地区组正则匹配 → `TestGroup_RegionGroupMatching`
- `T-GRP-002`：`relay_through=group` 生成链式组 → `TestGroup_ChainedTypeGroup`
- `T-GRP-003`：`relay_through=select` 生成链式组 → `TestGroup_ChainedTypeSelect`
- `T-GRP-004`：`relay_through=all` 生成链式组 → `TestGroup_ChainedTypeAll`
- `T-GRP-005`：`@all` 排除链式节点 → `TestGroup_AllProxiesExcludesChained`
- `T-GRP-006`：`type=group` 引用不存在 → `TestGroup_ChainedGroupRefNotFound`
- `T-GRP-007`：无 relay_through → `TestGroup_NoChaining`
- `T-GRP-008`：多个链式组 → `TestGroup_MultipleChainedGroups`
- `T-GRP-009`：链式节点属性 → `TestGroup_ChainedNodeProperties`
- `T-GRP-010`：合并顺序 → `TestGroup_ProxiesMergeOrder`
- `T-RTE-001`：服务组声明序 → `TestRoute_ServiceGroups`
- `T-RTE-002`：`@all` 展开 → `TestRoute_AllExpansion`
- `T-RTE-003`：ruleset 映射 → `TestRoute_Rulesets`
- `T-RTE-004`：Rules 解析 → `TestRoute_RulesParsing`
- `T-RTE-005`：Rule 无逗号 → `TestRoute_RuleNoComma`
- `T-RTE-006`：Fallback 传递 → `TestRoute_Fallback`
- `T-RTE-007`：空 routing → `TestRoute_EmptyRouting`
- `T-RTE-008`：@all 空列表 → `TestRoute_AllExpansionEmpty`
- `T-RTE-009`：@auto 基本展开 → `TestRoute_AutoFillBasic`
- `T-RTE-010`：@auto 首选+补充 → `TestRoute_AutoFillWithPreferred`
- `T-RTE-011`：@auto 排除自身 → `TestRoute_AutoFillExcludesSelf`
- `T-RTE-012`：@auto 含链式组 → `TestRoute_AutoFillIncludesChainedGroups`
- `T-RTE-013`：@auto 含@all 服务组 → `TestRoute_AutoFillIncludesAllRouteGroups`
- `T-RTE-014`：@auto 展开顺序 → `TestRoute_AutoFillOrder`
- `T-RTE-015`：无@auto 向后兼容 → `TestRoute_NoAutoFill`
- `T-CFG-006`：同一 entry 中重复 @auto 报错 → `TestValidate_RoutingAutoRepeatedRejected`
- `T-RTE-016`：`Route(cfg, nil)` 不 panic → `TestRoute_NilGroupResult`
- `T-RTE-017`：手动 REJECT 位置保持不变 → `TestRoute_AutoFillPreservesManualRejectPlacement`

### 实施记录

关键设计决策：

| 决策 | 结论 | 原因 |
|------|------|------|
| 链式组命名 | `🔗 ` + custom proxy name（系统前缀） | config-schema.md 明确约定 |
| 链式节点命名 | `{upstream}→{custom}`（含 `→` 字符） | 天然与普通节点名不冲突 |
| 地区组匹配范围 | 仅 KindSubscription 节点 | 自定义代理由用户显式管理 |
| Params 隔离 | 每个链式节点独立 `make(map[string]string)` | 防止共享篡改 |
| 空组处理 | Group/Route 阶段不报错 | 留给 M4 ValidateGraph |
| @all 计算时机 | 在链式节点生成前，从原始 proxies 收集 | 天然排除链式节点 |
| 服务组策略 | 固定 `"select"` | 设计约定 |
| Rule Policy 提取 | `strings.LastIndex(raw, ",")` | 透传方案，只提取 Policy 用于引用校验 |
| 结果类型 | GroupResult / RouteResult 独立结构体 | 便于测试和后续 Pipeline 组装 |
| @auto 展开位置 | Route 阶段（`expandAutoFill`），在 `@all` 展开之前 | Route 阶段有 GroupResult（节点组列表），是唯一正确的展开点 |
| @auto 补充池顺序 | 节点组（声明序）→ @all 服务组（声明序）→ DIRECT | `REJECT` 需由用户显式决定是否加入 |
| REJECT 处理 | 不参与 @auto 自动补充，位置由用户显式控制 | 避免把拒绝策略隐式塞进所有服务组 |
| @auto 次数限制 | 同一 entry 最多一次，由 config.Validate 拦截 | 多次出现没有额外语义，只会增加歧义 |
| @auto 与 @all 互斥 | 同一 entry 静态校验拦截 | 两者语义不同（组级 vs 节点级），混用无合理场景 |
| Route 签名 | `Route(cfg, gr *GroupResult)` | @auto 展开需要 NodeGroups，直接传入 GroupResult |
| Route nil 保护 | `gr == nil` 时按空 `GroupResult` 处理 | 兼容旧调用方式，避免迁移遗漏导致 panic |

### 对应需求

- `REQ-04`
- `REQ-05`
- `REQ-06`
- `REQ-07`
- `REQ-08`
- `REQ-09`
- `REQ-13`

### 回溯点

- 里程碑记录：`M3-group-route`
- 固定一组中间表示样本，作为回归基线

### 退出条件

- ✅ `Group` 输出稳定的 `GroupResult`（全部节点 + 节点组 + @all），可供 M4 校验
- ✅ `Route` 输出稳定的 `RouteResult`（服务组 + 规则集 + 规则 + fallback），可供 M4 校验

### 已知限制

- 不做大部分图级校验（空组、循环引用、routing/ruleset/fallback 引用不存在等留给 M4）；仅 `relay_through.type=group` 的局部引用在 M3 fail-fast
- 不组装最终 `model.Pipeline`（留给 M4/M5 orchestrator）
- 正则编译为防御性检查（静态校验已拦截）

### 风险

- 链式展开边界和组引用边界最容易出错（已通过 10 个 Group 测试覆盖）
- 节点名与组名冲突时需要明确错误策略（`🔗 ` 前缀提供命名空间隔离）

---

## M4: 校验与渲染 ✅

### 目标

把中间表示转换为目标客户端配置文本，并在渲染前完成图级语义校验。

### 工作项

- 实现图级校验
- 实现 Clash Meta 渲染器
- 实现 Surge 渲染器
- 为两种输出建立 golden tests
- 新增模板注入机制（底版配置 + 生成段合并）
- 新增统一资源加载（本地路径 / HTTP URL）
- Config 层新增 `templates` 字段，Loader 支持远程加载

### 产物

- `internal/pipeline/validate.go`：ValidateGraph 实现（8 项校验规则，DFS 循环检测，collector 模式收集全部错误）
- `internal/render/clash.go`：Clash Meta 渲染器（yaml.Node API，模板合并，provider 名称提取/去重）
- `internal/render/surge.go`：Surge 渲染器（bytes.Buffer，INI section 切分/替换合并）
- `internal/fetch/resource.go`：`LoadResource` 统一加载函数（按 URL 前缀分发 local/remote）
- `internal/config/config.go`：新增 `Templates` 结构体
- `internal/config/loader.go`：`Load` 签名扩展为 `(ctx, location, fetcher)`
- 测试数据：`testdata/render/clash_golden.yaml`、`testdata/render/surge_golden.conf`
- 外部依赖：无新增

### 验收项

- ✅ 引用不存在时报错（T-VAL-001）
- ✅ 服务组循环引用时报错（T-VAL-002）
- ✅ 空链式组时报错（T-VAL-003）
- ✅ `routing` 显式引用原始代理名时报错（T-VAL-004）
- ✅ 代理名/组名共享命名空间冲突时报错（T-VAL-005）
- ✅ Clash Meta 输出包含正确节点、组、规则和 rule-providers（T-RND-001）
- ✅ Surge 输出包含正确节点、组、规则和 FINAL（T-RND-002）
- ✅ 链式节点映射到正确字段：`dialer-proxy` / `underlying-proxy`（T-RND-003）
- ✅ 规则顺序正确：rulesets → inline rules → fallback（T-RND-004）
- ✅ Clash / Surge 的 `url-test` 默认参数一致（含 `tolerance=100`）
- ✅ 模板合并保留底版通用设置
- ✅ `go test ./...` 全部通过

### 对应测试

- `T-VAL-001`：服务组引用不存在时报错 → `TestValidateGraph_RouteGroupMemberNotFound`
- `T-VAL-002`：服务组循环引用时报错 → `TestValidateGraph_CircularReference`
- `T-VAL-002b`：自引用时报错 → `TestValidateGraph_SelfReference`
- `T-VAL-003`：链式组展开为空时报错 → `TestValidateGraph_EmptyChainedGroup`
- `T-VAL-004`：`routing` 显式引用原始代理名时报错 → `TestValidateGraph_RouteGroupExplicitProxyMemberRejected`
- `T-VAL-005`：代理名/组名共享命名空间冲突时报错 → `TestValidateGraph_ProxyAndNodeGroupNameCollision`
- `T-RND-001`：Clash Meta 输出快照 → `TestClash_GoldenNoTemplate`
- `T-RND-002`：Surge 输出快照 → `TestSurge_GoldenNoTemplate`
- `T-RND-003`：链式节点渲染字段正确 → `TestClash_ChainedProxyHasDialerProxy` / `TestSurge_ChainedProxyHasUnderlyingProxy`
- `T-RND-004`：ruleset 顺序与 fallback 位置正确 → `TestClash_RuleOrder` / `TestSurge_RuleOrder`
- `T-RND-005`：Clash `url-test` 补齐 `tolerance` → `TestClash_URLTestHasTolerance`

### 实施记录

关键设计决策：

| 决策 | 结论 | 原因 |
|------|------|------|
| 通用设置来源 | 用户提供底版模板文件（`templates.clash` / `templates.surge`） | 通用设置因用户环境而异，不可硬编码 |
| 模板路径 | 支持本地文件路径或 HTTP(S) URL | 统一 `LoadResource` 按前缀分发 |
| 模板缓存 | 走 CachedFetcher，与订阅共享 TTL | 避免每次请求重复拉取 |
| Clash 输出方式 | yaml.v3 Node API | 精确控制字段顺序和转义；支持底版注释保留 |
| Surge 输出方式 | bytes.Buffer + section 切分替换 | INI 风格纯文本 |
| 无底版时行为 | 仅输出生成段 | 降低使用门槛 |
| rule-provider behavior | `classical` | ACL4SSR 规则集格式 |
| url-test 默认参数 | url=gstatic, interval=300, tolerance=100 | 业界标准 |
| routing 校验粒度 | 校验原始 `routing` 声明，不接受显式代理名 | 保持“服务组选出口、节点组选节点”的分层，避免 `@all` 展开结果掩盖非法配置 |
| 图校验命名空间 | 代理名、节点组名、服务组名统一登记校验 | 避免重名导致引用歧义或重复渲染 |
| Config.Load 签名 | `(ctx, location, fetcher)` | 支持远程加载，nil fetcher 限定仅本地 |
| 错误收集 | graphCollector + errors.Join | 与 M1 config.Validate 一致 |

### 对应需求

- `REQ-09`
- `REQ-10`
- `REQ-12`

### 回溯点

- 里程碑记录：`M4-validate-render`
- 固定 golden files：Clash 和 Surge 各一套

### 退出条件

- ✅ ValidateGraph 输出稳定的 `*model.Pipeline`，可供渲染器消费
- ✅ Clash / Surge 渲染器输出通过 golden 比对
- ✅ 模板合并逻辑正确保留底版设置

### 已知限制

- 不校验远程规则集 URL 的可达性或内容格式
- Clash rule-provider behavior 固定为 `classical`，不支持 `domain` 或 `ipcidr`
- Surge 模板合并基于 `[Section]` header 文本匹配，不支持嵌套 section

### 风险

- 同一语义在两个客户端格式中的映射不完全对称（已通过 golden tests 固化预期）

---

## M5: HTTP 服务与端到端验收 ✅

### 目标

接入 HTTP 层，形成用户可直接调用的服务版本。

### 工作项

- 实现 `GET /generate?format=clash|surge`
- 实现 `GET /healthz`
- 接入启动参数
- 串联配置加载、管道和渲染器
- 实现端到端测试
- 完成错误码映射

### 产物

- `internal/pipeline/`:
  - `execute.go`：`Execute(ctx, cfg, fetcher)` 管道顶层编排，串联 Source→Filter→Group→Route→ValidateGraph
- `internal/server/`:
  - `server.go`：`Server` 结构体、`New` 构造函数、`Handler()` 路由注册
  - `handler.go`：`handleGenerate`（管道→模板→渲染→响应）、`handleHealthz`
  - `errors.go`：`mapError` 错误类型→HTTP 状态码映射
- `cmd/subconverter/main.go`：flag 解析、依赖创建、服务启动、优雅关闭
- 外部依赖：无新增

### 验收项

- ✅ `format=clash` 返回 YAML（T-E2E-001）
- ✅ `format=surge` 返回 conf（T-E2E-002）
- ✅ 非法 `format` 返回 `400`（T-E2E-003）
- ✅ 订阅拉取失败返回 `502`（T-E2E-004）
- ✅ 内部错误返回 `500`（T-E2E-005）
- ✅ `healthz` 返回 `200`（T-E2E-006）
- ✅ 示例配置可完成端到端生成
- ✅ `go test ./...` 全部通过

### 对应测试

- `T-E2E-001`：HTTP 生成 Clash 成功 → `TestE2E_GenerateClash`
- `T-E2E-002`：HTTP 生成 Surge 成功 → `TestE2E_GenerateSurge`
- `T-E2E-003`：非法 `format` 返回 `400` → `TestE2E_InvalidFormat`
- `T-E2E-004`：订阅拉取失败返回 `502` → `TestE2E_FetchFailure`
- `T-E2E-005`：图校验失败返回 `500` → `TestE2E_BuildError`
- `T-E2E-006`：`/healthz` 返回 `200` → `TestE2E_Healthz`
- `T-EXE-001`：Execute happy path → `TestExecute_HappyPath`
- `T-EXE-002`：Execute fetch error → `TestExecute_FetchError`
- `T-EXE-003`：Execute filter excludes → `TestExecute_FilterExcludes`

### 实施记录

关键设计决策：

| 决策 | 结论 | 原因 |
|------|------|------|
| 管道编排位置 | `pipeline.Execute` | project-structure.md 将"管道编排"归于 pipeline 包 |
| Server 依赖注入 | main.go 创建 Config + CachedFetcher，注入 Server | 保持 server 可测试，不依赖 flag 解析 |
| 模板加载位置 | server handler 中（pipeline.Execute 之后、render 之前） | 模板是格式特定的，pipeline 应保持格式无关 |
| 错误映射 | `errors.As` 按类型分发（ConfigError→400, FetchError→502, BuildError/RenderError→500） | Go 1.24 `errors.As` 可穿透 `errors.Join`，映射简洁 |
| E2E 测试方式 | httptest.Server + fake fetcher，black-box 包 | 只测公共 API，与内部实现解耦 |
| 优雅关闭 | `signal.NotifyContext` + `httpServer.Shutdown` + 10s 超时 | 标准模式，防止慢请求阻塞关闭 |
| 路由注册 | Go 1.22+ method pattern `"GET /generate"` | 避免 handler 内手动检查 HTTP 方法 |
| 服务端日志 | 错误路径 `log.Printf` | 单用户服务，错误可能未被客户端消费 |

### 对应需求

- `REQ-01`
- `REQ-10`
- `REQ-11`
- `REQ-12`

### 回溯点

- 里程碑记录：`M5-http-e2e`
- 发布候选版本：`rc1`

### 退出条件

- ✅ `Execute` 输出稳定的 `*model.Pipeline`，可供渲染器消费
- ✅ HTTP 层仅做请求校验 + 编排 + 错误映射，不承担业务转换逻辑
- ✅ 6 个 E2E 测试 + 3 个 Execute 单测全部通过
- ✅ `make build` 和 `make run` 可用

### 已知限制

- HTTP 服务未设 `ReadTimeout` / `WriteTimeout`（单用户场景无 slowloris 风险）
- 不支持运行时配置热重载（修改配置后需重启服务）
- 错误消息直接返回给客户端（单用户场景，便于调试；多用户需脱敏）
- `handleGenerate` 对 format 做 3 次 switch（当前仅 2 格式，不做提前抽象）

### 风险

- 错误映射与用户预期不一致（已通过 E2E 测试覆盖主要路径）
- HTTP 层吸收过多业务逻辑，破坏模块边界（已通过依赖注入和 pipeline.Execute 隔离）

---

## 每阶段通用完成标准

每个里程碑完成前都必须满足：

- `go test ./...` 通过
- 本阶段新增能力有对应测试
- 示例配置覆盖新增路径
- 文档同步更新到对应设计或实施文档
- 至少覆盖一个错误路径
- 不引入与当前里程碑无关的功能扩展

---

## 验收矩阵

| 里程碑 | 验收方式 | 核心证据 |
|------|------|------|
| `M0` | 结构验收 | 目录、模块、基础命令 |
| `M1` | 配置验收 | 配置加载测试、保序测试、校验测试 |
| `M2` | 输入验收 | 订阅 fixture、SS 解析测试、过滤测试 |
| `M3` | 组装验收 | 节点组/服务组样本、`@all` 测试、链式展开测试 |
| `M4` | 输出验收 | Clash/Surge golden tests、图校验测试 |
| `M5` | 端到端验收 | HTTP 集成测试、示例配置生成结果 |

---

## 回溯机制

### 设计回溯

每个里程碑提交时应记录：

- 实现了哪些 `REQ-*`
- 依赖了哪些设计文档
- 新增了哪些测试项
- 当前已知限制是什么

### 结果回溯

每个里程碑都应固化以下证据：

- 示例输入
- 中间表示样本或 golden 输出
- 测试结果
- 已知错误案例

### 问题定位顺序

出现问题时，按以下层级回溯：

1. 配置解析层
2. 原始节点获取层
3. 分组与路由层
4. 图级校验层
5. 渲染层
6. HTTP 包装层

---

## 风险前置清单

正式编码前应优先验证这些高风险问题：

- SS 链接样本是否存在不兼容格式
- `relay_through.select` 是否只作用于订阅节点
- 节点名和组名冲突时的错误策略
- Clash Meta 与 Surge 的链式字段映射是否稳定
- ruleset 在两种客户端中的引用是否需要额外命名规范

---

## 最终交付定义

项目达到可交付状态至少满足：

- 能读取单一 YAML 配置文件
- 能拉取订阅并生成原始节点
- 能生成地区组、链式组和服务组
- 能输出 Clash Meta 与 Surge 配置
- 能通过 `/generate` 提供结果
- 能通过测试证明顺序、链式组、`@all`、fallback 和双格式输出语义一致
