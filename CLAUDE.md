# CLAUDE.md — subconverter 项目协作指南

**项目成功标准**：每次修改在保持管道不变量（节点名唯一、组名互斥、保序、链式上游约束、@all 语义）的前提下，正确扩展或修复功能，且两种输出格式（Clash Meta / Surge）的 golden 测试通过。

本文件补充而非覆盖用户全局 `~/.claude/CLAUDE.md`。全局文件定义**协作哲学**（决策、工程、透明），本文件沉淀**本项目的具体语境**——架构边界、扩展模板、已知局限。

**决策路由**：架构/流程/依赖取舍 → 全局文件两大锚点；管道阶段行为/扩展步骤/格式兼容性 → 本文件对应章节；灰区 → 回到决策锚（需要更多理解？）与工程锚（需要更清晰结构？）判断。

阅读顺序：速读 → 要改什么类型的东西 → 对应扩展模板 → 代码约定 → 已知局限。

---

## 项目速读

**定位**：Go 单二进制 HTTP 服务。读取一份 YAML 配置 → 拉取订阅 → 生成 Clash Meta / Surge 配置文件。

**管道阶段**：

```
LoadConfig → ValidateConfig → Build(Source → Filter → Group → Route → ValidateGraph) → Target → Render
```

**包边界**（依赖单向）：

```
cmd/subconverter → internal/{config,fetch,generate,server}
internal/server → internal/{generate,errtype}
internal/generate → internal/{config,fetch,pipeline,target,render}
internal/pipeline → internal/{config,fetch,model,errtype,proxyparse,ssparse}
internal/render → internal/{model,errtype}
internal/target → internal/{model,errtype}
```

`model` 和 `errtype` 不依赖其他业务包。`render` 不反向依赖 `config`。

**核心术语**（下游章节会反复使用）：

- **拉取类节点** = `KindSubscription` / `KindSnell` / `KindVLess`（Source 阶段从外部拉取或解析得到）
- **原始节点** = 拉取类节点 + 不带 `relay_through` 的 custom_proxy
- **链式节点** = `KindChained`（由带 `relay_through` 的 custom_proxy 作为模板派生）

**关键不变量**（修改时必须维持）：

- 节点名全局唯一（跨订阅两轮去重）
- 节点组名和服务组名共享命名空间，互斥
- 链式节点上游只能是拉取类节点
- `@all` 仅含原始节点，不含链式节点；带 `relay_through` 的 custom_proxy 仅派生 `KindChained` + 同名链式组，不产出 `KindCustom`，不进入 `@all`
- `groups` / `routing` / `rulesets` 三段**保序**

**详细设计**：`docs/architecture.md` + `docs/design/*.md`。修改前先读对应文档——该项目已把多数决策写进文档，文档与代码的偏离本身是 bug。

---

## 扩展操作模板

### 新增 `ProxyKind` 枚举值

因为 `ProxyKind` 是全系统的分派键，新增值必须**同步审视所有 Kind 敏感点**。单纯加常量不足以完成扩展。

1. `internal/model/model.go` 加常量
2. `grep -rn "p\.Kind\|model\.Kind" --include="*.go" internal/` 列出全部分派点
3. **判定原则**：每个分派点的本质问题是——新 Kind 在此处的行为与既有 Kind 是否相同？不同则必须处理。按此原则对以下**已知**分派点逐个评估（非完备清单，遇到未列出的 Kind 敏感点时用同一原则判断）：

   | 分派点                                                                     | 关注问题                                                              |
   | -------------------------------------------------------------------------- | --------------------------------------------------------------------- |
   | `internal/pipeline/filter.go` 的 `isFetchedKind`                           | 新 Kind 是否参与 `filters.exclude` 过滤？                             |
   | `internal/pipeline/group.go` 的 `fetchedProxies`                           | 是否参与区域组 regex 匹配？                                           |
   | `internal/pipeline/group.go` 的 `computeAllProxies`                        | 是否进入 `@all`？                                                     |
   | `internal/pipeline/group.go` 的 `buildChainedNodesAndGroups`               | 是否可作为链式上游？                                                  |
   | `internal/pipeline/source.go` 的 `deduplicateNames` / `checkNameConflicts` | 是否与其他 Kind 共享去重池？冲突检测范围？                            |
   | `internal/pipeline/validate.go`                                            | 是否纳入命名空间检查？                                                |
   | `internal/target/filter_cascade.go` 的 `filterByDroppedTypes`（经 `target.ForClash`）| 新 Kind 是否需在 Clash Target 投影前剔除？级联影响 fallback / 规则 / ruleset |
   | `internal/render/surge.go` 的 `renderSurgeProxy` switch                    | 新 Kind 是否支持 Surge？不支持时走 `RenderError` 还是提前过滤         |

4. 更新相关函数/变量的 doc comment（历史注释可能写 "只含 KindSubscription"，必须同步改）
5. 写 group-level 回归测试（`TestGroup_<NewKind>ParticipatesInRegionMatch` 之类），防止 `isFetchedKind` 等 helper 被改窄导致静默回归

### 新增 `Proxy.Type`（代理协议）

因为协议字段决定渲染阶段的 switch 分派和格式兼容性判断，新增协议必须在**每个渲染器中明确其行为**（渲染 / 报错 / 过滤），并确保 golden 测试覆盖两种格式的输出确定性。

1. `internal/render/clash.go` 的 `buildClashProxy` switch 加 case
2. `internal/render/surge.go` 的 `renderSurgeProxy` switch 加 case
3. 若协议**只支持某一输出格式**，选一种策略：
   - **A 报错**：不支持方的渲染器 case 返回 `RenderError`（参考 SS v2ray-plugin 在 Surge 的处理）
   - **B 过滤**：不支持方做"视图过滤"，参考 `internal/target/filter_cascade.go` 的级联剔除
4. 字段顺序：在 Surge 里用 `xxxKeyOrder` slice 固定（golden 比对依赖确定性输出）
5. Params 的 key 名称**保持目标格式原样**（如 Snell 的 `shadow-tls-password`），避免在解析/渲染两处做命名映射
6. 更新 `docs/design/rendering.md` 的映射表

### 新增来源类型（类比 `sources.snell`）

因为来源类型引入新的 `ProxyKind` 并贯穿整条管道（Source → Filter → Group → Target → Render），新增来源必须在**每个管道阶段验证其行为**，并同步文档与配置示例以防信息断层。

**最小测试集**（缺一项就不算完成）：

- [ ] 解析器单测：valid / invalid / skip（注释/空行）/ 重复键 / 边界（端口越界）
- [ ] Source 阶段：与其他来源**共享去重池**的测试
- [ ] Source 阶段：单行失败路径 + 整源为空路径，URL 必须脱敏
- [ ] Filter 阶段：若节点参与 `filters.exclude`，补 KindXxx 过滤用例
- [ ] Group 阶段：混合 Kind 的 region regex 匹配用例
- [ ] Surge 渲染：含 base template + 含 managed header 的共存测试
- [ ] Clash 渲染：若做过滤，测级联 + fallback 清空 + 规则/ruleset 清理
- [ ] Execute 端到端：`TestExecute_<Source>EndToEnd` 贯通管道 + 两种渲染器
- [ ] ValidateGraph happy path：含新 Kind 的 GroupResult 通过校验
- [ ] 配置校验：`sources.<new>[].url` 空 / 非 HTTP(S) 报错

**文档与示例同步**（与测试同等重要；已出现过 Snell/VLESS 扩展时 `configs/base_config.yaml` 被漏更新的先例）：

- [ ] `configs/base_config.yaml`：为新 `sources.<kind>` 增加注释示例块（即使整块注释掉也要写，向用户声明该来源存在）
- [ ] `configs/base_config.yaml`：核对既有注释中"订阅节点"等窄化措辞——`filters` / `groups.match` / `custom_proxies.name` 唯一性 / `relay_through` 的 `select`/`all` 描述均应表述为"拉取类节点（订阅 / Snell / VLESS...）"
- [ ] `docs/design/config-schema.md`：顶层 `sources` 段、字段约束、校验规则
- [ ] `docs/design/pipeline.md`：Source 阶段子步骤、解析规则、错误格式、`FetchOrder` 顺序语义
- [ ] `docs/design/domain-model.md`：`Proxy.Kind` 表、命名冲突策略
- [ ] `docs/design/validation.md`：静态 / 构建期校验条目
- [ ] `docs/design/rendering.md`：若协议只支持某格式，补"XX 过滤"小节
- [ ] `docs/architecture.md`：若影响 `@all` / `@auto` 语义或"原始节点"定义，同步关键决策表
- [ ] **依赖图同步**：若新增或变更了包间 import，同步更新 `docs/architecture.md` 模块边界图和 `docs/implementation/project-structure.md` 依赖方向图（已出现过 Snell/VLESS 扩展引入 `config -> fetch/model/ssparse` 等依赖但未同步文档的先例）

### 新增错误码

1. 位置：`internal/errtype/errors.go`
2. 命名：`Code<Domain><Scene>`，Domain ∈ {Config, Fetch, Build, Render, Resource}
3. 使用对应 `*Error` 结构体：`ConfigError` / `FetchError` / `ResourceError` / `BuildError` / `RenderError`
4. `errors_test.go` 加覆盖（现有测试对每个新 Code 至少有一条断言其 Error() 输出格式）

---

## 代码约定

### 错误消息

- 用户可见文案用**中文**；code 注释、Error Code 常量名用英文
- **URL 必须脱敏**：拼进 `FetchError.URL` 前用 `fetch.SanitizeURL(rawURL)`。订阅 URL 常含 token
- 错误消息应含**定位信息**（field path、upstream id、proxy name），而非只说"失败"
- **级联失败**（如 Clash fallback 被清空）的消息应附带**根因路径**——"FINAL 为空 ← SVC_X 为空 ← GRP_SG 被过滤"，而非仅给终态

### 测试

- 命名：`Test<Package>_<Scenario>`；顶部注释加 `T-<DOMAIN>-<NN>` id（规则见 `docs/implementation/testing-strategy.md`）
- **表驱动测试优先**：cases slice + `t.Run(tt.name, ...)`
- 断言含上下文：失败消息带 got/want + 相关 state，便于 debug
- **YAML 断言避坑**：astral-plane 字符（如 🇭🇰 国旗 U+1F1ED+U+1F1F0，或 🔗 U+1F517）被 go-yaml 转成 `\UXXXX`；emoji 组名用 ASCII 前缀（`GRP_`/`SVC_`）测试更稳定。YAML 路径上的比对尤其敏感；纯 Go fixture（直接构造结构体 + 字符串字面量断言）可安全使用任意 emoji
- 新 feature 的测试文件命名与现有风格保持一致：`<feature>_test.go`
- **测试不跨架构边界导入**：测试文件的 import 应只用被测包的直接依赖（`model`、`errtype` 等叶包），不应导入同层或上层业务包（如 `render` 测试不应导入 `pipeline`）。需要复杂测试输入时，用 `model.Proxy` 等字面量手工构造而非调用其他业务包的解析器——这既保持边界清晰，也让测试输入的含义一目了然

### Plan 文件闭环

**本项目先例**（为何放项目级而非全局）：Snell / VLESS 扩展时 plan 含"可选的测试补充"类条目未二次决策，连带导致 `configs/base_config.yaml` 示例被漏更新。

实现完成后必须回读 `~/.claude/plans/<plan-id>.md`：

- 每个标记为"可选/推荐"的条目必须**二次决策**，收敛为"已做"或"不做 + 理由"
- 决定"不做"时，在 plan 文件"范围削减"小节写明理由，便于后续审查

### CI 流水线检查

Release workflow 按以下顺序执行，任一步失败则阻断后续 job（binaries / docker）：

1. **`golangci-lint`**（含 `unused` linter）：检测**包括 `_test.go` 在内**的未使用函数/变量——Go 编译器不报错，但 CI 会拒绝
2. **`gofmt -l .`**：严格格式检查，文件末尾多余空行即失败
3. **`go test ./...`** + **`go vet ./...`**

**本地预检命令**：`gofmt -l . && go vet ./...`——在 `git commit` 前跑一次可避免大部分 CI 格式类失败。

### 文档术语一致性

**本项目先例**（为何放项目级而非全局）：`relay_through` 语义扩展后，`pipeline.md` 和 `config-schema.md` 的"原始节点"定义已更新为"不带 `relay_through` 的自定义节点"，但 `architecture.md`、`domain-model.md`、`testing-strategy.md`、`implementation-plan.md` 仍沿用旧定义"自定义节点"——概要层文档滞后于细节层是最常见的漂移模式。

核心术语（如"原始节点""拉取类节点"）的**定义变更**时，必须全文搜索所有出现处：

- `grep -rn "原始节点\|拉取类节点\|@all" docs/` 列出全部引用点
- **概要层文档**（`architecture.md`、决策表）最易遗漏——它们在首次撰写时正确，但后续扩展时通常只改细节层
- 检查范围包括：`docs/`、`CLAUDE.md`、`configs/base_config.yaml` 注释

### 语义变更优先 rename

**本项目先例**（为何放项目级而非全局）：`isFetchedKind` 的语义从"仅订阅"扩展到"订阅 / Snell / VLESS"，若只改注释而不 rename，grep 到该函数的新成员会错误推断其仅处理订阅。

函数/变量的**行为语义**变化时，优先 rename 函数/变量名：

- 函数名是调用点的锚点，注释会随时间漂移
- 正例：新增同名 helper 同时覆盖新老语义，或改名为更贴合新语义的名称

---

## 参考资料

- `docs/architecture.md` — 系统架构与管道模型
- `docs/design/config-schema.md` — 用户 YAML 配置字段规约
- `docs/design/domain-model.md` — 中间表示（Proxy / ProxyGroup / Ruleset / Rule / Pipeline）
- `docs/design/pipeline.md` — 各阶段输入输出与不变量
- `docs/design/rendering.md` — Clash Meta / Surge 渲染映射
- `docs/design/validation.md` — 配置与图校验规则
- `docs/implementation/testing-strategy.md` — 测试编号与覆盖策略
- `~/.claude/CLAUDE.md` — 用户全局规则（协作哲学、沟通、自检）

---

## 已知架构局限

记录当前设计的已知权衡。修改前先看是否触及——触及时**保持该局限的边界不变**；需要改变边界的，先与用户确认并在本节补记。

### 1. ValidateGraph 不感知输出格式

> **触及时行动**：新增 format-specific 过滤时 → 重新评估是否引入 per-format validation hook（让 ValidateGraph 接受 `formatHint` 参数）。

- **现象**：Build 阶段"合法"的配置，在某一输出格式的 Target 阶段可能失败（如 Snell 节点让 Clash fallback 被级联清空）
- **报错路径**：`target.ForClash`（内部调用 `filterByDroppedTypes`）在 Target 阶段返回 `CodeRenderClashFallbackEmpty`
- **影响**：错误被"晚报"；调试时用户看到 render 错而非 build 错
- **缓解方案（未实施）**：引入 per-format validation hook，让 ValidateGraph 接受 `formatHint` 参数，对每种输出格式跑一次图校验。当前规模下不必处理，但**新增 format-specific 过滤时应重新评估**

### 2. 中间表示对"格式限定字段"的宽松包容

> **触及时行动**：考虑严格模式时 → 优先扩充 `xxxKeyOrder` 而非引入白名单拒绝；需同时改解析器 + 渲染器 + 错误码。

- **现象**：`Proxy.Params` 是 `map[string]string`，容纳任意键（如 Snell 的 `shadow-tls-*` 仅对 Surge 有意义）
- **包容方式**：解析阶段宽松存所有键；渲染阶段按固定列表（如 `surgeSnellKeyOrder`）输出，未知键静默丢弃
- **权衡**：好处是解析器不随目标格式版本迭代；代价是用户 typo 的键不会报错
- **如果你在考虑严格模式**：需同时改解析器 + 渲染器 + 错误码，以及决定向后兼容策略。优先扩充 `xxxKeyOrder` 而非引入白名单拒绝

### 3. 协议格式专属性导致 Target 阶段级联过滤

> **触及时行动**：新增格式专属协议 → 复用 `internal/target/filter_cascade.go` 的 `filterByDroppedTypes` 过滤策略；同步补跨格式过滤测试（至少覆盖级联效应）。

- **现象**：Snell 仅支持 Surge、VLESS 仅支持 Clash。不支持方在 Target 阶段做"视图过滤"（非 Build 阶段拒绝），导致只在特定输出格式下触发错误，而非 ValidateGraph 阶段
- **报错路径**：
  - Clash 走 `internal/target/filter_cascade.go` 的 `filterByDroppedTypes`（经 `target.ForClash`），可能级联清空 fallback / 规则 / ruleset，最终抛 `CodeRenderClashFallbackEmpty` 等错误
  - Surge 走 `internal/target/filter_cascade.go` 的 `filterByDroppedTypes`（经 `target.ForSurge`），可能级联清空 fallback / 规则 / ruleset，最终抛 `CodeRenderSurgeFallbackEmpty` 等错误
- **影响**：build 阶段"合法"的配置在 render 阶段失败；调试需读级联链（"FINAL 为空 ← SVC_X 为空 ← GRP_SG 被过滤 ← 仅含 Snell 节点"）
- **与局限 1 的关系**：同属"ValidateGraph 不感知格式"的具体表现。缓解方案共享：per-format validation hook
- **新增格式专属协议时**：复用 `internal/target/filter_cascade.go` 的 `filterByDroppedTypes` 过滤策略；同步补跨格式过滤测试（至少覆盖"该协议节点在另一格式下被过滤后 fallback / 规则的级联效应"）

---

## 提交前自检（本项目最易漏的检查）

按本项目历次偏差归纳的尾部自检清单。提交改动前逐条确认：

- [ ] **ProxyKind 枚举变更** → 见 §新增 ProxyKind：已按判定原则逐个评估所有 Kind 敏感分派点（pipeline 层 6 处 + render 层 2 处）
- [ ] **用户可见错误消息含 URL**：已用 `fetch.SanitizeURL` 脱敏（订阅/链路 URL 常含 token）
- [ ] **新增来源类型** → 见 §新增来源类型：`configs/base_config.yaml` 已加注释示例块；docs 已同步
- [ ] **函数行为语义变化** → 见 §语义变更优先 rename：已 rename 函数/变量名而非仅改注释（锚点一致性）
- [ ] **Plan 文件含"可选/推荐"条目** → 见 §Plan 文件闭环：已收敛为"已做"或"不做 + 理由"，不留模糊词汇
- [ ] **格式专属协议改动** → 见 §已知架构局限 #3：已测级联效应
- [ ] **包间依赖变更** → 见 §新增来源类型 · 文档与示例同步：`docs/architecture.md` 模块边界图和 `docs/implementation/project-structure.md` 依赖方向图已同步更新
- [ ] **测试跨边界导入** → 见 §测试：测试文件未导入同层或上层业务包（如 render 测试不导入 pipeline）
- [ ] **删除代码后的清理** → 见 §CI 流水线检查：已确认无未使用的 test helper 函数、无多余 import、无尾部空行（`gofmt -l .` 通过）
- [ ] **核心术语定义变更** → 见 §文档术语一致性：对变更术语全文搜索，确认所有出现处（含概要层文档）均已同步
