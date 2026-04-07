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
| `M2` | Source 与 Filter | 能拉取订阅、解析 SS、过滤节点 |
| `M3` | Group 与 Route | 能生成地区组、链式组、服务组和路由绑定 |
| `M4` | 校验与渲染 | 能校验引用关系并输出 Clash Meta / Surge |
| `M5` | HTTP 与 E2E | 能通过 HTTP 生成配置并完成端到端验收 |

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
- 增加 `configs/example.yaml`
- 约定 `testdata` 和示例输入目录
- 约定基本命令：格式化、测试、运行（Makefile）
- 明确错误分类：配置错误、拉取错误、构建错误、渲染错误

### 产物

- 最小 Go 工程骨架（`.gitignore`、`go.mod`、`Makefile`）
- 空包结构（`config`、`model`、`fetch`、`pipeline`、`render`、`server`）
- `internal/errtype`：四类错误类型（`ConfigError`、`FetchError`、`BuildError`、`RenderError`）及 9 个单元测试
- 示例配置草稿（`configs/example.yaml`）
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
- ✅ 非法正则、缺失字段、非法枚举值可返回错误（22 个校验测试）
- ✅ 示例配置能加载为内存对象并通过校验
- ✅ `go test ./...` 全部通过（config 35 + errtype 9 + model 3 = 47 个测试）

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
- `base_url` 的 URL 格式校验留给 M5 server 层

---

## M2: Source 与 Filter

### 目标

建立稳定的原始节点输入能力，确保系统能正确获取并清洗订阅节点。

### 工作项

- 实现订阅抓取器
- 实现 TTL 缓存
- 实现 SS URI 解析器
- 实现多订阅并发拉取
- 将自定义代理转换为原始节点
- 实现过滤逻辑

### 产物

- `internal/fetch`
- `pipeline/source`
- `pipeline/ssuri`
- `pipeline/filter`

### 验收项

- 支持合法 SS URI 解析
- 非法 SS URI 可识别并返回错误
- 多订阅结果可合并
- `exclude` 仅影响订阅节点
- 自定义代理不受过滤影响
- 缓存命中与失效行为符合预期

### 对应测试

- `T-SRC-001`：合法 SS URI 解析
- `T-SRC-002`：非法 SS URI 报错
- `T-SRC-003`：多订阅合并
- `T-FLT-001`：`exclude` 过滤订阅节点
- `T-FLT-002`：自定义代理不参与过滤
- `T-SRC-004`：缓存 TTL 命中与失效

### 对应需求

- `REQ-02`
- `REQ-03`
- `REQ-12`

### 回溯点

- 里程碑记录：`M2-source-filter`
- 固定测试订阅响应作为回归输入

### 风险

- 订阅返回格式存在兼容性差异
- 订阅可能包含空行、无效行或异常编码内容

---

## M3: Group 与 Route

### 目标

把节点集合稳定转换成节点组、服务组和路由绑定，形成系统业务语义层。

### 工作项

- 根据 `groups` 生成地区节点组
- 根据 `relay_through` 生成链式节点
- 自动生成链式组
- 计算 `@all`
- 构建服务组
- 装配 `rulesets`、`rules` 和 `fallback`

### 产物

- `pipeline/group`
- `pipeline/route`

### 验收项

- 地区组能按正则匹配节点
- 链式展开支持 `group`、`select`、`all`
- 链式组策略来自 `relay_through.strategy`
- 链式组出现在节点组层
- `@all` 不包含链式节点
- 服务组能引用节点组、服务组、`DIRECT`、`REJECT`
- ruleset 与 fallback 能绑定到目标服务组

### 对应测试

- `T-GRP-001`：地区组正则匹配
- `T-GRP-002`：`relay_through=group` 生成链式组
- `T-GRP-003`：`relay_through=select` 生成链式组
- `T-GRP-004`：`relay_through=all` 生成链式组
- `T-RTE-001`：服务组引用节点组
- `T-RTE-002`：`@all` 不包含链式节点
- `T-RTE-003`：ruleset 与 fallback 绑定正确

### 对应需求

- `REQ-04`
- `REQ-05`
- `REQ-06`
- `REQ-07`
- `REQ-08`
- `REQ-09`

### 回溯点

- 里程碑记录：`M3-group-route`
- 固定一组中间表示样本，作为回归基线

### 风险

- 链式展开边界和组引用边界最容易出错
- 节点名与组名冲突时需要明确错误策略

---

## M4: 校验与渲染

### 目标

把中间表示转换为目标客户端配置文本，并在渲染前完成图级语义校验。

### 工作项

- 实现图级校验
- 实现 Clash Meta 渲染器
- 实现 Surge 渲染器
- 为两种输出建立 golden tests

### 产物

- `pipeline/validate`
- `render/clash`
- `render/surge`

### 验收项

- 引用不存在时报错
- 服务组循环引用时报错
- 空链式组时报错
- Clash Meta 输出包含正确节点、组、规则和 rule-providers
- Surge 输出包含正确节点、组、规则和 FINAL
- 链式节点映射到正确字段：`dialer-proxy` / `underlying-proxy`

### 对应测试

- `T-VAL-001`：服务组引用不存在时报错
- `T-VAL-002`：服务组循环引用时报错
- `T-VAL-003`：链式组展开为空时报错
- `T-RND-001`：Clash Meta 输出快照
- `T-RND-002`：Surge 输出快照
- `T-RND-003`：链式节点渲染字段正确
- `T-RND-004`：ruleset 顺序与 fallback 位置正确

### 对应需求

- `REQ-09`
- `REQ-10`
- `REQ-12`

### 回溯点

- 里程碑记录：`M4-validate-render`
- 固定 golden files：Clash 和 Surge 各一套

### 风险

- 同一语义在两个客户端格式中的映射不完全对称

---

## M5: HTTP 服务与端到端验收

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

- `internal/server`
- `cmd/subconverter/main.go`
- 最小可运行服务

### 验收项

- `format=clash` 返回 YAML
- `format=surge` 返回 conf
- 非法 `format` 返回 `400`
- 订阅拉取失败返回 `502`
- 内部错误返回 `500`
- `healthz` 返回 `200`
- 示例配置可完成端到端生成

### 对应测试

- `T-E2E-001`：HTTP 生成 Clash 成功
- `T-E2E-002`：HTTP 生成 Surge 成功
- `T-E2E-003`：非法 `format` 返回 `400`
- `T-E2E-004`：订阅拉取失败返回 `502`
- `T-E2E-005`：配置非法或内部错误返回 `500` 或启动失败
- `T-E2E-006`：`/healthz` 返回 `200`

### 对应需求

- `REQ-01`
- `REQ-10`
- `REQ-11`
- `REQ-12`

### 回溯点

- 里程碑记录：`M5-http-e2e`
- 发布候选版本：`rc1`

### 风险

- 错误映射与用户预期不一致
- HTTP 层吸收过多业务逻辑，破坏模块边界

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
