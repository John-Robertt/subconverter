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

## 热重载生命周期

`POST /api/reload` 重新读取并准备主配置。加载、解析和 Prepare 都在配置写锁外完成；只有替换生效状态时才短暂加写锁：

```text
POST /api/reload
  │
  ├─ reloadMu.TryLock()     已有重载在执行时，返回 429 reload_in_progress
  ├─ readConfigBytes       读取 YAML；远程主配置先 Invalidate 对应缓存项
  ├─ parseConfigYAML       解析配置
  ├─ prepareAdminConfig    应用层检查 + config.Prepare
  │
  ├─ 失败？ ──► 返回错误，RuntimeConfig 不变
  │
  └─ 成功：
       WLock()
       替换配置指针、revision、生效 YAML 和重载状态
       WUnlock()
```

关键保证：

- 热重载校验失败时，旧 `RuntimeConfig` 保持不变，所有请求继续使用旧配置
- 配置写锁只保护上述内存状态的更新，文件读取、网络拉取、解析和 Prepare 均在锁外
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

这里的 `LoadConfig` 是“读取并解析配置”的阶段名称。当前启动和热重载入口由 `app.Service` 的 `readConfigBytes`、`parseConfigYAML` 完成，再调用 `prepareAdminConfig`；`config.Load` 仍是配置包提供的加载函数，并非当前应用入口的调用链。

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
- 校验 ruleset/rule 策略引用合法性、fallback 存在性

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
- 解析订阅节点（TXT / Clash YAML / Surge CONF）、Snell 节点、VLESS 节点
- 将自定义代理转换为原始节点对象或链式模板
- 按来源与声明顺序为每个原始节点分配非零、Build 内唯一的 `ProxyID`

输入：

- `sources`

输出：

- `SourceResult`（原始节点集合 + 链式模板）

说明：

- 本阶段不生成链式节点
- 同名节点保留各自候选显示名和独立 ID；本阶段不修改节点名
- 订阅格式按 `Clash YAML → Surge CONF → TXT/Base64` 的优先级识别；`.yaml` / `.yml` / `.conf` URL 后缀会强制对应格式，其他 URL 再按顶层 `proxies` 或 `[Proxy]` 内容识别
- Clash YAML 复制 `proxies` 中的完整节点映射；Surge CONF 复制 `[Proxy]` 中每条声明的完整右值。两类节点都保留为 `KindSubscription`，并用 `NativeProxy.Format` 记录其唯一目标格式
- TXT 响应体支持 Base64 编码列表和明文列表；明文列表兼容 SS（SIP002 `ss://`、Quantumult X `shadowsocks = ...`、Surge `<name> = ss, ...`）和 AnyTLS（`anytls://`、Quantumult X `anytls=...`、Surge `<name> = anytls, ...`）行格式
- AnyTLS 输入按“宽进严出”归一化：Surge / Quantumult X 方言字段可被接受，但只有目标客户端官方支持字段会在 Render 阶段输出
- TXT 订阅中的空行、整行注释和单行解析失败会被跳过；结构化订阅声明错误整源报错；任一格式最终 0 个有效节点都报错
- Snell 来源单行解析失败整源报错
- VLESS 来源单行解析失败整源报错
- 拉取失败时错误信息对 URL 做脱敏处理

来源遍历顺序：

- `sources` 下的 `subscriptions` / `snell` / `vless` 三类 key 按 YAML 书写顺序记录到 `Sources.FetchOrder`，Source 阶段按此顺序调度拉取

---

## Stage 4: Filter

职责：

- 对拉取类节点执行名称过滤

输入：

- 原始节点集合
- `filters`

输出：

- `FilterResult`
  - `Included`：未被过滤的节点集合，供后续 Group / Route / Render 使用
  - `Excluded`：被 `filters.exclude` 命中的拉取类节点，仅供预览和诊断使用

说明：

- 过滤对象是拉取类节点（`KindSubscription`，含 TXT / Clash YAML / Surge CONF；加上 `KindSnell` + `KindVLess`）
- 自定义代理和链式节点不参与过滤
- 生成管道只消费 `FilterResult.Included`
- `/api/preview/nodes` 同时返回 `Included` 与 `Excluded` 的合并视图，并用 `filtered` 字段标记节点是否被排除

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

- 全部节点、节点组集合、`@all` 展开列表

子步骤执行顺序：

1. **构建地区节点组**：正则匹配过滤后的拉取类节点
2. **构建链式节点和链式组**：利用已构建的地区组或正则匹配确定上游节点
3. **计算 `@all`**：收集全部原始节点（不含链式节点）

关键规则：

- `@all` 只包含原始节点
- 地区组、链式组和 `@all` 都保存 `ProxyID`，不保存节点名
- 原始节点 = 订阅节点（TXT / Clash YAML / Surge CONF）+ Snell 节点 + VLESS 节点 + 不带 `relay_through` 的自定义节点
- 链式展开的上游只来自拉取类节点，并通过 `DialerID` 引用

---

## Stage 6: Route

职责：

- 根据 `routing` 生成服务组
- 将预计算后的 `@auto` 成员和 `@all` token解析为类型化引用
- 根据 `rulesets` 生成服务组绑定关系
- 解析 `rules`
- 记录 `fallback`

输入：

- `routing`、`rulesets`、`rules`、`fallback`
- `GroupResult`

输出：

- 服务组集合、规则集集合、内联规则集合、fallback

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

- Clash 视图剔除 Snell 节点及级联影响（见 `rendering.md` §Snell 过滤）
- Surge 视图剔除 VLESS 节点及级联影响（见 `rendering.md` §VLESS 过滤）
- Clash 视图剔除 Surge CONF 原生订阅节点；Surge 视图剔除 Clash YAML 原生订阅节点
- 原生格式剔除与协议剔除共用同一级联：依赖被剔除上游的链式节点、空节点组/服务组、对应规则与 `@all` 成员同步清理
- 级联过滤算法由共享引擎 `filterUnsupportedProxies`（`internal/target/filter_cascade.go`）提供
- 若 `fallback` 在目标格式视图中被清空，本阶段返回 `TargetError`

---

## Stage 9: Render

职责：

- 将已投影的目标格式视图序列化为目标客户端格式

输出目标：

- Clash Meta YAML
- Surge conf

要求：

- 两种渲染器遵循相同的分组和路由规则，分别输出各自 Target 保留下来的内容
- Target 过滤后，两种输出的节点、组和规则可以不同；Render 负责目标语法、协议字段输出和底版模板合并

---

## 并发模型

```text
运行时生成 / GET 预览             POST 草稿预览
  RLock → 取配置指针 → RUnlock      解析草稿 → Prepare 临时配置
              │                              │
              └──────── 执行所需阶段 ──────────┘

POST /api/reload
  reloadMu.TryLock → 读取 / 解析 / Prepare → WLock → 替换状态 → Unlock
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
