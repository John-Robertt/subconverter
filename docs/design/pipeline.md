# 管道设计

> v1.0 管道文档已归档至 docs/v1.0/design/pipeline.md
>
> 状态提示：本文描述 v2.0 当前管道契约；当前可用能力见 docs/README.md 状态矩阵。

## 目标

本文件说明用户配置怎样一步步变成客户端配置。前两阶段读取并准备配置；请求到来后，再根据远程节点构建引用关系，按输出格式整理这些关系，最后写成文本。预览复用所需的阶段，并在得到结果时停止。

---

## 总体流程

```text
启动期 / 热重载期:
读取并解析 YAML
  -> Prepare（产出 RuntimeConfig）
     ↑ POST /api/reload 可重新触发

运行时生成（/generate, GET /api/generate/preview）:
RLock()
  -> 取得 *RuntimeConfig 指针
RUnlock()
  -> Build(Source -> Filter -> Group -> Route -> ValidateGraph)
  -> Target
  -> Render

运行时节点 / 分组预览（GET /api/preview/*）:
读取当前配置指针快照并释放锁
  -> 所需阶段，见下方预览管道表

草稿预览期（POST /api/preview/*, POST /api/generate/preview）:
草稿 Config
  -> Prepare（产出本次请求的临时 RuntimeConfig）
  -> 所需预览阶段
```

设计原则：

- 每个阶段尽量只负责一类转换
- 阶段之间通过稳定的中间表示传递数据
- I/O 与纯转换逻辑尽量分离

---

### 本地文件缺失的启动路径

本地主配置路径尚无文件时，`app.Service` 启动管理员后台并保留空运行快照；此时不执行 Prepare 或 Build。导入或编辑产生草稿，首次保存创建主配置，再通过显式热重载执行 Prepare、发布运行快照。已存在但无效的配置仍按启动校验失败处理；完整状态与首次创建契约见[首次配置 API](api.md#首次配置)。

## 热重载生命周期

`POST /api/reload` 重新读取并准备主配置。加载、解析和 Prepare 都在配置写锁外完成；只有替换生效状态时才短暂加写锁：

```text
POST /api/reload
  │
  ├─ reloadMu.TryLock()     已有重载在执行时，返回 429 reload_in_progress
  ├─ readSavedConfig        本地先检查是否存在（缺失则记录失败并返 409 config_not_initialized）；远程主配置先 Invalidate 再经 readConfigBytes 读取
  ├─ setObservedConfigRevision  短暂 WLock 更新观测 revision
  ├─ parseConfigYAML        解析配置
  ├─ prepareAdminConfig     应用层检查 + config.Prepare
  │
  ├─ 失败？ ──► 短暂 WLock 记录 lastReload 失败状态，返回错误，生效 RuntimeConfig 不变
  │
  └─ 成功：
       WLock()
       替换配置指针、runtime/observed revision、生效 YAML、加载时间和重载状态
       WUnlock()
```

关键保证：

- 热重载校验失败时，旧 `RuntimeConfig`、`runtimeConfigRevision` 与 `effectiveConfigYAML` 保持不变，所有请求继续使用旧配置（仅会短暂加写锁更新 `observedConfigRevision` 与 `lastReload` 记录）
- 文件读取、网络拉取、解析和 Prepare 均在配置锁外执行，写锁仅用于上述极短的内存状态替换/记录
- 读请求只在读取 `*RuntimeConfig` 指针快照时短暂持有 `RLock`，随后释放锁再执行 Source / Filter / Group / Render 等可能耗时阶段
- 已经取得快照的请求会继续使用旧配置完成；热重载成功后，新请求会读取新配置快照
- 重载只验证主配置的静态内容，不拉取节点来源、不加载模板，也不执行 Build、Target 或 Render。要确认生成可用，需继续运行对应预览或生成请求

例如，新配置中的地区组正则可以正常编译，但可能匹配不到实际节点。这次热重载可以成功，分组预览或生成时才会报告空组。缓存刷新范围见 [缓存设计](caching.md#热重载与预览如何使用缓存)。

---

## 预览管道

预览请求执行管道的部分阶段，返回中间数据而非最终配置文本：

| 端点 | 执行阶段 | 输出 |
|------|---------|------|
| `GET /api/preview/nodes` | Source + Filter | 节点列表（含来源标记和过滤状态） |
| `POST /api/preview/nodes` | Prepare + Source + Filter | 草稿配置的节点列表 |
| `GET /api/preview/groups` | Source + Filter + Group + Route + ValidateGraph | 节点组列表 + 链式组列表 + 服务组列表 + @all / @auto 展开成员；图级错误返回 400 诊断 |
| `POST /api/preview/groups` | Prepare + Source + Filter + Group + Route + ValidateGraph | 草稿配置的分组与服务组结果；图级错误返回 400 诊断 |
| `GET /api/generate/preview` | 完整管道（Build + Target + Render） | 配置文本（不触发下载） |
| `POST /api/generate/preview` | Prepare + 完整管道（Build + Target + Render） | 草稿配置文本（不触发下载） |

预览管道复用与生成管道相同的阶段实现，不存在独立的过滤或分组语义；差异只在于预览 API 会暴露阶段诊断数据。

GET 预览只读取当前生效的 `RuntimeConfig`；POST 预览只使用请求体中的草稿配置，不写文件、不替换运行时配置。`preview/groups` 不返回部分成功结果：一旦 ValidateGraph 发现空组、非法引用或循环引用等图级错误，HTTP 层返回 `400` 结构化诊断。

---

## Stage 1: LoadConfig

这里的 `LoadConfig` 是“读取并解析配置”的阶段名称。当前启动和热重载入口由 `app.Service` 的 `readSavedConfig`（内部按需调用 `readConfigBytes`）和 `parseConfigYAML` 完成，再调用 `prepareAdminConfig`；`config.Load` 仍是配置包提供的加载函数，并非当前应用入口的调用链。

职责：

- 读取 YAML 文件（支持本地路径或 HTTP(S) URL）
- 反序列化为用户配置对象
- 保留 `groups`、`routing`、`rulesets` 的书写顺序

输出：

- 配置对象

---

## Stage 2: Prepare

职责：

- 校验字段是否存在、值是否合法、互斥和依赖关系
- 编译正则表达式（`groups[*].match`、`filters.exclude`、`relay_through.match`）
- 解析自定义代理 URL（`custom_proxies[].url`），校验 SS 参数
- 展开 `@auto`（存入 `ExpandedMembers`），`@all` 保留到请求期 Route 阶段展开
- 构建策略命名空间：注册 DIRECT/REJECT、节点组名、服务组名和链式组名，检测组/保留策略之间的命名冲突
- 检测路由环路
- 校验 ruleset/rule 策略引用合法性、解析内联规则并提取目标策略、校验 fallback 存在性

输出：

- `RuntimeConfig`（含编译后的正则、解析后的自定义代理、展开后的路由成员）

并发语义：

- 启动时执行一次，热重载时重新执行
- 草稿校验和草稿预览也执行 Prepare，生成只供本次请求使用的临时配置
- 产出的 `RuntimeConfig` 在请求期按只读契约消费

应用入口的 `prepareAdminConfig` 还要求至少有一个地区节点组；对 Admin API 的 JSON 配置，另行检查 `sources.fetch_order`。这两项应用层检查与 `config.Prepare` 的通用校验分开，见 [校验设计](validation.md#静态校验prepare-阶段)。

---

## Stage 3: Source

职责：

- 按来源类型顺序、同类来源的声明顺序逐个拉取；单次 Build 内当前采用顺序拉取
- 解析订阅节点（TXT / Clash YAML / Surge CONF / Loon）、Snell 节点、VLESS 节点
- 将自定义代理转换为原始节点对象或链式模板
- 按来源与声明顺序为每个原始节点分配非零、Build 内唯一的 `ProxyID`

输入：

- `sources`

输出：

- `SourceResult`（原始节点集合 + 链式模板 + `NextProxyID`；返回前调用 `model.ValidateProxyInvariant` 校验单节点结构不变量，`NextProxyID` 确保 Group 阶段不复用被 Filter 排除的原始节点 ID）

说明：

- `subscriptions`、`snell`、`vless` 均使用 `config.Source` 表示 URL、正文或本地文件输入。`loadSourceBody` 统一选择来源：`content` 直接使用，`content_file` 读取本地文件，两者均跳过远程 Fetcher；输入约束见 [配置规约](config-schema.md#拉取来源的输入与展示)
- 本阶段不生成链式节点
- 同名节点保留各自候选显示名和独立 ID；本阶段不修改节点名
- `InspectSource` 复用 Source 正文解析以检查单个来源；订阅通过 `InspectSubscription` 按内容提供识别结果或候选，Snell / VLESS 通过各自严格解析器检查，不改变 Build 中的解析规则；订阅识别规则见 [配置规约](config-schema.md#web-订阅检查)
- 订阅格式由 `format`、URL 后缀及内容识别决定，规则以 [配置规约](config-schema.md#subscriptions) 为准
- Clash YAML 复制 `proxies` 中的完整节点映射；Surge CONF / Loon 复制 `[Proxy]` 中每条声明的完整右值（本地 `content` 或 `content_file` 在显式 `format: surge` / `loon` 且无 `[Section]` 头时自动补 `[Proxy]` 接受裸节点片段）。这些节点都保留为 `KindSubscription`，并用 `NativeProxy.Format` 记录其唯一目标格式
- TXT 响应体支持 Base64 编码列表和明文列表，两者解码后共用统一行解析器，兼容 SS（SIP002 `ss://`、Quantumult X `shadowsocks = ...`、Surge `<name> = ss/shadowsocks, ...`；仅 SIP002 解析 `plugin`）和 AnyTLS（`anytls://`、Quantumult X `anytls=...`、Surge `<name> = anytls, ...`）行格式
- AnyTLS 输入按“宽进严出”归一化：Surge / Quantumult X 方言字段可被接受，但只有目标客户端官方支持字段会在 Render 阶段输出
- TXT 订阅中的空行、整行注释和单行解析失败会被跳过；结构化订阅声明错误整源报错（Clash YAML 附带 `proxies[index]`，Surge CONF / Loon 附带 `[Proxy]` 物理行号）；任一格式最终 0 个有效节点都报错
- Snell 来源单行解析失败整源报错
- VLESS 来源单行解析失败整源报错
- 来源错误对 URL 脱敏；本地订阅正文与节点池正文使用字段路径定位（如 `sources.subscriptions[index].content` / `sources.snell[index].content`），文件使用本地路径定位，Snell / VLESS 错误均保留物理行号

来源遍历顺序：

- `sources` 下的 `subscriptions` / `snell` / `vless` 三类 key 按 YAML 书写顺序记录到 `Sources.FetchOrder`（为空时默认按 `["subscriptions", "snell", "vless"]`），Source 阶段按此顺序调度拉取；`custom_proxies` 不进入 `FetchOrder`，统一在拉取类来源之后转换并追加到 `Proxies` 末尾再分配 `ProxyID`

---

## Stage 4: Filter

职责：

- 对拉取类节点执行名称过滤

输入：

- 原始节点集合
- `filters`

输出：

- `FilterResult`（供预览入口 `SourceAndFilter` / `SourceFilterGroupRouteValidate` 使用；生成管道内部的 `filterCompiled` 直接产出保留节点切片）
  - `Included`：未被过滤的节点集合，供后续 Group / Route / Render 使用
  - `Excluded`：被 `filters.exclude` 命中的拉取类节点，用于统计过滤数量
  - `All`：按 Source 原始顺序保留的全部 `FilteredProxy{Proxy, Filtered}`，供节点预览输出

说明：

- 过滤对象是拉取类节点（`KindSubscription`，含 TXT / Clash YAML / Surge CONF / Loon；加上 `KindSnell` + `KindVLess`）
- 自定义代理和链式节点不参与过滤
- 生成管道只消费未过滤节点（`Included`）
- `/api/preview/nodes` 按 Source 原始顺序输出 `FilterResult.All`，用 `filtered` 字段标记是否被排除，并分别以 `Included` / `Excluded` 计数 `active_count` / `filtered_count`

---

## Stage 5: Group

职责：

- 根据 `groups` 生成地区节点组
- 根据 `relay_through` 生成链式节点和链式组
- 计算 `@all` 展开列表

输入：

- `SourceResult`（其中 `Proxies` 已经过 Filter 阶段处理）
- 地区节点组定义

输出：

- `GroupResult`：全部节点（原始 + 链式，返回前调用 `model.ValidateProxyInvariant` 校验）、`RegionGroups`、`ChainedGroups`、`NodeGroups`（地区组在前、链式组在后）、`@all` 展开列表

子步骤执行顺序：

1. **构建地区节点组**：正则匹配过滤后的拉取类节点
2. **构建链式节点和链式组**：按 `relay_through.type` 利用已构建的地区组（`group`）、正则匹配（`select`）或全部过滤后的拉取类节点（`all`）确定上游节点
3. **计算 `@all`**：收集全部原始节点（不含链式节点）

关键规则：

- `@all` 只包含原始节点
- 地区组、链式组和 `@all` 都保存 `ProxyID`，不保存节点名
- 原始节点 = 订阅节点（TXT / Clash YAML / Surge CONF / Loon）+ Snell 节点 + VLESS 节点 + 不带 `relay_through` 的自定义节点
- 链式展开的上游只来自拉取类节点，并通过 `DialerID` 引用

---

## Stage 6: Route

职责：

- 根据 Prepare 产出的 `PreparedRouteGroup` 生成服务组
- 将预计算后的 `@auto` 成员和 `@all` token 解析为类型化引用
- 将 `PreparedRuleset` 转换为 IR 规则集集合
- 将 Prepare 阶段已解析的 `PreparedRule` 转换为 IR 内联规则列表（不再重复解析规则文本）
- 记录 `fallback`

输入：

- Prepare 产出的 `routing`（`[]PreparedRouteGroup`）、`rulesets`（`[]PreparedRuleset`）、`rules`（`[]PreparedRule`）、`fallback`
- `GroupResult`

输出：

- `RouteResult`：服务组集合（`RouteGroups`）、规则集集合、内联规则集合、fallback，以及用于 ValidateGraph 和分组预览溯源的 `PreparedRouteGroups` 与 `ResolvedRouteGroups`（含成员来源 `Origin`）

说明：

- 服务组统一为 `select`
- `@auto` 在启动期 Prepare 阶段已展开；Route 阶段将组名/保留策略解析为对应 `PolicyRef`，并将 `@all` 展开为 `PolicyRefProxy`

---

## Stage 7: ValidateGraph

职责：

- 检查代理 ID 唯一性、组/保留策略命名空间冲突和重复声明
- 检查 `DialerID`、节点组成员 ID、服务组类型化引用的存在性，并保证链式上游是拉取类节点
- 检查 `@all` 展开排除链式节点
- 检查空节点组
- 检查路由成员引用合法性
- 检查服务组之间是否存在循环引用

输入：

- 全部节点、节点组、服务组

输出：

- 可进入 Target 投影的完整 ID 中间表示

---

## Stage 8: Target

职责：

- 将格式无关 IR 投影为目标格式视图
- 做目标格式协议能力过滤
- 做目标格式特有的级联校验
- 在过滤完成后为本目标独立分配唯一节点输出名，并把所有 ID / 类型化引用转换为最终名称

目标名称分配遵循以下顺序：

1. 预留节点组名、服务组名、`DIRECT`、`REJECT`
2. 按管道顺序处理非链式节点；冲突时追加 `②`…`⑩`、`(11)`… 后缀
3. 使用上游在本目标中的最终名称组装链式候选名，再用同一分配器消歧
4. 产出 `TargetPipeline`；Render 只消费最终名称，不再观察内部 ID

说明：

- Clash 视图（`ForClash`）剔除 Snell 节点及所有非 Clash 原生订阅节点（Surge CONF / Loon），并执行级联清理（见 [Snell 过滤](rendering.md#snell-过滤)）
- Surge 视图（`ForSurge`）剔除 VLESS 节点及所有非 Surge 原生订阅节点（Clash YAML / Loon），并执行级联清理（见 [VLESS 过滤](rendering.md#vless-过滤)）
- Loon 视图（`ForLoon`）剔除 Snell、所有非 Loon 原生订阅节点（Clash YAML / Surge CONF），以及规范化 VLESS 中不支持的传输（非 `tcp`/`ws`/`http`）、未适配的 `encryption`（非空且非 `none`）或 `flow`（非空且非 `xtls-rprx-vision`）；通用名称分配完成后追加执行 `projectLoonChains`，在预留组/保留策略/已有代理与链路名后，为每个带 `Dialer` 的链式节点分配 `<链路名>-exit` 辅助出口名（冲突按 `②`…`⑩`、`(11)`… 消歧），写入 `TargetPipeline.ProxyChains`（含 `Members: [Dialer, exitName]` 与 `UDP`），并将出口代理改名为 `exitName`、清空 `Dialer`（见 [Loon 渲染](rendering.md#loon)）
- 原生格式剔除与协议剔除共用同一级联：依赖被剔除上游的链式节点、空节点组/服务组、对应规则集、内联规则与 `@all` 成员同步清理
- 级联过滤算法由共享引擎 `filterUnsupportedProxies`（`internal/target/filter_cascade.go`）提供
- 若 `fallback` 在目标格式视图中被清空，本阶段返回 `TargetError`

---

## Stage 9: Render

职责：

- 将已投影的目标格式视图序列化为目标客户端格式；Loon 的规则来源优先级见 [渲染设计](rendering.md#loon)

输出目标：

- Clash Meta YAML
- Surge conf
- Loon lcf

要求：

- 三种渲染器使用相同的分组和路由声明，分别输出各自 Target 保留下来的内容
- Target 过滤后，各目标输出的节点、组和规则可以不同；Render 负责目标语法、协议字段输出和底版模板合并

---

## 并发模型

```text
运行时生成 / GET 预览             POST 草稿预览
  RLock → 取配置指针 → RUnlock      解析草稿 → Prepare 临时配置
              │                              │
              └──────── 执行所需阶段 ──────────┘

POST /api/reload
  reloadMu.TryLock → readSavedConfig（本地缺失返 409）→ 短暂 WLock 更新观测 revision → 解析 / Prepare → 成功则 WLock 替换生效状态，失败则短暂 WLock 记录 lastReload
```

配置锁、重载互斥和文件保存的边界见 [app.Service 并发安全](app-service.md#并发安全)。请求各自构建结果，共享的 `RuntimeConfig` 按只读约定使用；远程资源缓存另由 `CachedFetcher` 的锁保护。

---

## 阶段划分理由

这样拆分的原因：

- 便于把"获取数据""重塑结构""检查引用""输出文本"四类问题隔离
- 便于单测直接覆盖单个阶段
- 便于后续扩展新的输出格式
- 便于预览请求复用部分阶段
- 热重载只需重新执行启动期阶段（LoadConfig + Prepare），请求期阶段不受影响
