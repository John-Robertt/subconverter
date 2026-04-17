# CLAUDE.md — subconverter 项目协作指南

本文件补充而非覆盖用户全局 `~/.claude/CLAUDE.md`。全局文件定义**协作哲学**（决策、工程、透明），本文件沉淀**本项目的具体语境**——架构边界、扩展模板、已知局限。

阅读顺序：速读 → 要改什么类型的东西 → 对应扩展模板 → 代码约定 → 已知局限。

---

## 项目速读

**定位**：Go 单二进制 HTTP 服务。读取一份 YAML 配置 → 拉取订阅 → 生成 Clash Meta / Surge 配置文件。

**管道阶段**：
```
LoadConfig → ValidateConfig → Source → Filter → Group → Route → ValidateGraph → Render
```

**包边界**（依赖单向）：
```
cmd/subconverter → internal/server → internal/{config,pipeline,render,model,fetch,errtype}
```
`model` 和 `errtype` 不依赖其他业务包。`render` 不反向依赖 `config`。

**关键不变量**（修改时必须维持）：
- 节点名全局唯一（跨订阅两轮去重）
- 节点组名和服务组名共享命名空间，互斥
- 链式节点上游只来自"拉取类"节点（`KindSubscription` / `KindSnell` / `KindVLess`）
- `@all` 仅含原始节点：订阅 / Snell / VLESS / **不带 `relay_through`** 的 custom_proxy；不含链式节点；带 `relay_through` 的 custom_proxy 是链式模板（仅派生 `KindChained` + 同名链式组），不产出 `KindCustom`，不进入 `@all`
- `groups` / `routing` / `rulesets` 三段**保序**

**详细设计**：`docs/architecture.md` + `docs/design/*.md`。修改前先读对应文档，不要凭直觉改动——该项目已把多数决策写进文档，文档与代码的偏离本身是 bug。

---

## 扩展操作模板

### 新增 `ProxyKind` 枚举值

因为 `ProxyKind` 是全系统的分派键，新增值必须**同步审视所有 Kind 敏感点**。单纯加常量不足以完成扩展。

1. `internal/model/model.go` 加常量
2. `grep -rn "p\.Kind\|model\.Kind" --include="*.go" internal/` 列出全部分派点
3. 逐个评估新 Kind 的行为是否与既有 Kind 相同：

   | 分派点 | 关注问题 |
   |-------|---------|
   | `internal/pipeline/filter.go` 的 `isFetchedKind` | 新 Kind 是否参与 `filters.exclude` 过滤？ |
   | `internal/pipeline/group.go` 的 `fetchedProxies` | 是否参与区域组 regex 匹配？ |
   | `internal/pipeline/group.go` 的 `computeAllProxies` | 是否进入 `@all`？ |
   | `internal/pipeline/group.go` 的 `buildChainedNodesAndGroups` | 是否可作为链式上游？ |
   | `internal/pipeline/source.go` 的 `deduplicateNames` / `checkNameConflicts` | 是否与其他 Kind 共享去重池？冲突检测范围？ |
   | `internal/pipeline/validate.go` | 是否纳入命名空间检查？ |
4. 更新相关函数/变量的 doc comment（历史注释可能写 "只含 KindSubscription"，必须同步改）
5. 写 group-level 回归测试（`TestGroup_<NewKind>ParticipatesInRegionMatch` 之类），防止 `isFetchedKind` 等 helper 被改窄导致静默回归

### 新增 `Proxy.Type`（代理协议）

1. `internal/render/clash.go` 的 `buildClashProxy` switch 加 case
2. `internal/render/surge.go` 的 `renderSurgeProxy` switch 加 case
3. 若协议**只支持某一输出格式**，选一种策略：
   - **A 报错**：不支持方的渲染器 case 返回 `RenderError`（参考 SS v2ray-plugin 在 Surge 的处理）
   - **B 过滤**：不支持方做"视图过滤"，参考 `internal/render/clash_filter.go` 的级联剔除
4. 字段顺序：在 Surge 里用 `xxxKeyOrder` slice 固定（golden 比对依赖确定性输出）
5. Params 的 key 名称**保持目标格式原样**（如 Snell 的 `shadow-tls-password`），避免在解析/渲染两处做命名映射
6. 更新 `docs/design/rendering.md` 的映射表

### 新增来源类型（类比 `sources.snell`）

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

### Plan 文件闭环

实现完成后必须回读 `~/.claude/plans/<plan-id>.md`：
- 每个标记为"可选/推荐"的条目必须**二次决策**：已做 / 不做+理由
- 不留模糊词汇。"可选"是规划安全词，实现阶段必须收敛
- 若决定"不做"，在 plan 文件"范围削减"小节里写明理由，便于后续审查

### 语义变更优于改注释

函数/变量的**行为语义**变化时，优先 rename，不要只改注释：
- 注释会随时间漂移，函数名是调用点的锚点
- 反例：语义扩展到"拉取类节点"后仍保留旧函数名，会让 grep 代码的人错误推断行为
- 正例：`isFetchedKind` 新加 helper 同时命中语义

---

## 已知架构局限

记录当前设计的已知权衡。修改前先看是否触及——触及时**不要扩大裂痕**，必要时在本节补记。

### 1. ValidateGraph 不感知输出格式

- **现象**：Build 阶段"合法"的配置，在某一输出格式的 render 阶段可能失败（如 Snell 节点让 Clash fallback 被级联清空）
- **报错路径**：`filterForClash` 在 render 入口返回 `CodeRenderClashFallbackEmpty`
- **影响**：错误被"晚报"；调试时用户看到 render 错而非 build 错
- **缓解方案（未实施）**：引入 per-format validation hook，让 ValidateGraph 接受 `formatHint` 参数，对每种输出格式跑一次图校验。当前规模下不必处理，但**新增 format-specific 过滤时应重新评估**

### 2. 中间表示对"格式限定字段"的宽松包容

- **现象**：`Proxy.Params` 是 `map[string]string`，容纳任意键（如 Snell 的 `shadow-tls-*` 仅对 Surge 有意义）
- **包容方式**：解析阶段宽松存所有键；渲染阶段按固定列表（如 `surgeSnellKeyOrder`）输出，未知键静默丢弃
- **权衡**：好处是解析器不随目标格式版本迭代；代价是用户 typo 的键不会报错
- **如果你在考虑严格模式**：需同时改解析器 + 渲染器 + 错误码，以及决定向后兼容策略。优先扩充 `xxxKeyOrder` 而非引入白名单拒绝

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
