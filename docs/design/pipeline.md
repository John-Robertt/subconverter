# 管道设计

> v1.0 管道文档已归档至 docs/v1.0/design/pipeline.md
>
> 状态提示：本文描述 v2.0 当前管道契约；当前可用能力见 docs/README.md 状态矩阵。

## 目标

本文件定义系统从配置输入到配置输出的阶段划分、输入输出和阶段职责。v2.0 在 v1.0 的 9 阶段管道基础上，新增热重载生命周期和预览管道。

---

## 总体流程

```text
启动期 / 热重载期:
LoadConfig
  -> Prepare (produces RuntimeConfig)
     ↑ POST /api/reload 可重新触发

请求期（/generate, /api/preview/*）:
RLock()
  -> snapshot *RuntimeConfig
RUnlock()
  -> Build(Source -> Filter -> Group -> Route -> ValidateGraph)
  -> Target
  -> Render

草稿预览期（POST /api/preview/*, POST /api/generate/preview）:
Draft Config
  -> Prepare (temporary RuntimeConfig)
  -> requested preview stages
```

设计原则：

- 每个阶段尽量只负责一类转换
- 阶段之间通过稳定的中间表示传递数据
- I/O 与纯转换逻辑尽量分离

---

## 热重载生命周期

`POST /api/reload` 触发完整的 LoadConfig + Prepare 流程：

```text
POST /api/reload
  │
  ├─ LoadConfig(path)       读取最新 YAML（远程主配置 bypass / invalidate 缓存）
  ├─ Prepare(config)        校验 + 编译正则 + 展开 @auto + 检测环路
  │
  ├─ 失败？ ──► 返回错误，RuntimeConfig 不变
  │
  └─ 成功：
       WLock()
       swap RuntimeConfig pointer + runtime config revision
       WUnlock()
```

关键保证：

- 热重载校验失败时，旧 `RuntimeConfig` 保持不变，所有请求继续使用旧配置
- `WLock` 仅保护指针替换，不包含 I/O 或计算
- 读请求只在读取 `*RuntimeConfig` 指针快照时短暂持有 `RLock`，随后释放锁再执行 Source / Filter / Group / Render 等可能耗时阶段
- 已经取得快照的请求会继续使用旧配置完成；热重载成功后，新请求会读取新配置快照

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

职责：

- 读取 YAML 文件（支持本地路径或 HTTP URL）
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
- 构建静态命名空间（`StaticNamespace`）：注册 DIRECT/REJECT、节点组名、服务组名、自定义代理名、链式组名，检测跨类别命名冲突
- 检测路由环路
- 校验 ruleset/rule 策略引用合法性、fallback 存在性

输出：

- `RuntimeConfig`（含编译后的正则、解析后的自定义代理、展开后的路由成员、静态命名空间）

并发语义：

- 启动时执行一次，热重载时重新执行
- 产出的 `RuntimeConfig` 在请求期按只读契约消费

---

## Stage 3: Source

职责：

- 拉取所有远程来源
- 解析 SS 节点、Snell 节点、VLESS 节点
- 将自定义代理转换为原始节点对象或链式模板

输入：

- `sources`

输出：

- `SourceResult`（原始节点集合 + 链式模板）

说明：

- 本阶段不生成链式节点
- 跨订阅重名通过两轮去重处理
- SS URI 按 SIP002 解析
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

- 过滤对象是拉取类节点（`KindSubscription` + `KindSnell` + `KindVLess`）
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
- 原始节点 = 订阅节点 + Snell 节点 + VLESS 节点 + 不带 `relay_through` 的自定义节点
- 链式展开的上游只来自拉取类节点

---

## Stage 6: Route

职责：

- 根据 `routing` 生成服务组
- 展开 `@auto` 和 `@all` token
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
- `@auto` 在启动期 Prepare 阶段已展开；Route 阶段仅展开 `@all`

---

## Stage 7: ValidateGraph

职责：

- 检查共享命名空间冲突和重复声明
- 检查 `@all` 展开排除链式节点
- 检查空节点组
- 检查路由成员引用合法性
- 检查服务组之间是否存在循环引用

输入：

- 全部节点、节点组、服务组

输出：

- 可渲染的完整中间表示

---

## Stage 8: Target

职责：

- 将格式无关 IR 投影为目标格式视图
- 做目标格式协议能力过滤
- 做目标格式特有的级联校验

说明：

- Clash 视图剔除 Snell 节点及级联影响（见 `rendering.md` §Snell 过滤）
- Surge 视图剔除 VLESS 节点及级联影响（见 `rendering.md` §VLESS 过滤）
- 级联过滤算法由共享引擎 `filterByDroppedTypes`（`internal/target/filter_cascade.go`）提供
- 若 `fallback` 在目标格式视图中被清空，本阶段返回 `TargetError`

---

## Stage 9: Render

职责：

- 将已投影的目标格式视图序列化为目标客户端格式

输出目标：

- Clash Meta YAML
- Surge conf

要求：

- 两种输出的面板结构和路由语义保持一致
- 差异仅体现在目标语法

---

## 并发模型

```text
	                    RuntimeConfig
	                         │
	            ┌────────────┼────────────┐
	            │            │            │
	      /generate    /api/preview   /api/reload
	     snapshot     snapshot       WLock()
          │            │              │
        Build        Source         swap ptr + revision
        Target       Filter
        Render       [Group/Route]
```

- `app.Service` 内部持有 `sync.RWMutex` 保护 `*RuntimeConfig` 指针
- 读请求（`/generate`、`/api/preview/*`）只在复制配置指针时短暂使用 `RLock`
- 写请求（`/api/reload`）使用 `WLock`，互斥
- `WLock` 仅保护指针替换，不包含 LoadConfig 或 Prepare 计算
- 慢速订阅拉取、模板加载或渲染不持有配置锁，因此不会阻塞热重载获取写锁

---

## 阶段划分理由

这样拆分的原因：

- 便于把"获取数据""重塑结构""检查引用""输出文本"四类问题隔离
- 便于单测直接覆盖单个阶段
- 便于后续扩展新的输出格式
- 便于预览请求复用部分阶段
- 热重载只需重新执行启动期阶段（LoadConfig + Prepare），请求期阶段不受影响
