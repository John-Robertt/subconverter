# 系统架构设计

## 概述

本系统是一个使用 Go 实现的单用户 HTTP 服务。它读取一份用户 YAML 配置，拉取 SS 订阅源，结合自定义代理、节点分组、服务路由和远程规则集，生成 Clash Meta 或 Surge 配置文件。

系统目标：

- 将用户关注点拆分为来源、节点组、服务组、规则集和兜底策略
- 用统一中间表示支撑 Clash Meta 与 Surge 两种输出
- 保持单二进制、低依赖、无状态部署
- 让配置书写顺序稳定映射到客户端面板顺序

系统非目标：

- 不维护内置规则库
- 不支持多用户、多租户或多设备 overlay
- 不支持 Shadowrocket、Quantumult X 等额外输出目标
- 不在服务端预拉取规则集内容
- 不支持运行时配置热重载（修改配置后需重启服务）

---

## 运行模型

系统按单用户模式设计：

- 启动时加载一份 YAML 配置文件
- 服务运行期间使用同一份内存配置处理所有请求
- 客户端通过 HTTP 请求指定输出格式，不在请求中上传配置
- 若服务端启用了访问 token，请求需附带 `token` 查询参数

```text
config.yaml + subscriptions
        │
        ▼
   subconverter
        │
        ├─► /generate?format=clash[&token=...][&filename=...]
        └─► /generate?format=surge[&token=...][&filename=...]
```

---

## 核心对象

系统围绕三类对象组织：

- 节点：订阅节点、自定义节点、链式节点
- 组：节点组、服务组
- 路由规则：远程规则集、内联规则、fallback

其中：

- 节点组用于选择“某一地区或某一链路使用哪个具体节点”
- 服务组用于选择“某个服务走哪个出口”
- 链式组属于节点组，与地区组同层
- 所有节点组都必须显式声明 `select` 或 `url-test`
- 服务组统一为 `select`

---

## 管道模型

系统采用声明式管道架构：

```text
LoadConfig
  -> ValidateConfig
  -> Source
  -> Filter
  -> Group
  -> Route
  -> ValidateGraph
  -> Render
```

对应职责：

- `LoadConfig`：读取并解析用户 YAML
- `ValidateConfig`：校验字段合法性和配置结构完整性
- `Source`：拉取订阅并解析原始节点
- `Filter`：对订阅节点执行过滤
- `Group`：构建地区组和链式组，产出节点组层
- `Route`：构建服务组、规则集与 fallback
- `ValidateGraph`：检查引用关系、循环依赖和展开结果
- `Render`：根据目标格式输出 Clash Meta 或 Surge 配置

该模型的核心特点：

- 各阶段输入输出清晰，便于单测和定位问题
- 渲染器只依赖统一中间表示，不直接依赖配置原文
- 链式节点在分组阶段生成，避免污染源数据获取逻辑

---

## 模块边界

```text
cmd/subconverter
  └─► internal/server
        ├─► internal/config
        ├─► internal/pipeline
        │     └─► internal/fetch
        ├─► internal/render
        └─► internal/model
```

模块职责：

- `config`：配置加载（支持本地/远程）、保序解析、静态校验
- `model`：格式无关的中间表示
- `fetch`：订阅拉取、缓存、统一资源加载（`LoadResource`：按前缀分发本地文件读取或 HTTP 拉取）
- `pipeline`：Source / Filter / Group / Route / ValidateGraph 编排
- `render`：Clash Meta 与 Surge 渲染器（支持底版模板合并）
- `server`：HTTP 接口和错误映射

依赖原则：

- 依赖方向单向
- `model` 只承载数据，不依赖其他业务包
- 渲染层不反向依赖配置层实现细节

---

## 数据流

```text
用户 YAML 配置
    │
    ├─► sources      ──► 节点来源
    ├─► groups       ──► 节点组定义
    ├─► routing      ──► 服务组定义
    ├─► rulesets     ──► 远程规则集绑定
    ├─► rules        ──► 内联规则
    └─► fallback     ──► 兜底出口

订阅节点 + 自定义节点
    │
    ▼
统一中间表示
    │
    ├─► Clash Meta 渲染
    └─► Surge 渲染
```

设计要求：

- `groups`、`routing`、`rulesets` 都要保留书写顺序
- `@all` 只展开原始节点，不包含链式节点
- `@auto` 展开为自动补充池（节点组 → @all 服务组 → DIRECT），自动去重且排除自身
- `REJECT` 不在 `@auto` 补充池中；如需使用，必须显式声明
- 链式组由自定义代理派生，但在节点组层中与地区组平级

---

## 关键决策

| 决策 | 结论 | 原因 |
|------|------|------|
| 部署模型 | 单用户、单配置文件 | 与产品草案一致，运行模型最简单 |
| 用户配置风格 | 声明式分层 YAML | 关注点分离，便于维护 |
| 核心架构 | 管道模型 | 阶段清晰，易测试和调试 |
| 输出目标 | Clash Meta + Surge | 满足当前目标，避免过早扩展 |
| 规则集策略 | 仅透传 URL | 客户端运行时自行拉取，服务端无需维护规则内容 |
| 节点组策略 | 所有节点组必须显式声明策略 | 面板行为一致，避免隐式默认值 |
| 链式组建模 | 属于节点组，由 `custom_proxies[].relay_through` 派生 | 与用户心智一致，配置归属清晰 |
| 链式组策略声明 | 写在 `relay_through.strategy` | 派生关系就近声明，避免 `groups` 出现异类结构 |
| `@all` 范围 | 仅原始节点，不含链式节点 | 控制节点膨胀 |
| `@auto` 语义 | 自动补充节点组 + @all 服务组 + DIRECT，去重、排除自身 | 消除 routing 冗余，链式组自动可用 |
| `REJECT` 声明方式 | 不在 `@auto` 中，必须由用户显式声明 | `REJECT` 是策略选择，不应被隐式补入 |
| `@auto` 次数限制 | 同一 routing entry 中最多出现一次 | 避免多次替换带来的歧义 |
| `@auto` 与 `@all` 互斥 | 同一 routing entry 中不能同时使用 | 语义不同，混用会产生歧义 |
| 缓存范围 | 缓存订阅和模板的远程拉取结果 | 规则集内容不由服务端消费；模板与订阅共享 CachedFetcher |
| `base_url` 用途 | 声明服务外部地址，用于 Surge Managed Profile | 用户显式声明，避免反向代理下自动推导不可靠 |
| 访问 token 存放位置 | 运行时参数（flag/env），不进入 YAML | 访问控制属于服务部署语义，而非配置生成语义 |
| 下载文件名 | 由请求参数 `filename` 决定；未传时按格式使用默认名；仅接受安全 ASCII 文件名 | 文件名是 HTTP 传输层行为，不属于配置模型本身 |
| 通用设置来源 | 用户提供底版模板文件（`templates.clash` / `templates.surge`） | 通用设置因用户环境而异，不可硬编码 |
| 资源加载模型 | 配置文件和模板均支持本地路径或 HTTP(S) URL | 统一 `LoadResource` 按前缀分发，复用已有 Fetcher |
| 渲染器合并策略 | Clash 用 yaml.Node 树替换；Surge 用 section header 切分替换 | 保留底版全部用户自定义设置 |

---

## 风险与边界

主要风险点：

- 节点名称匹配完全依赖用户正则，错误正则会导致分组为空或误分组
- 服务组和节点组存在引用关系，需要显式做图校验
- 链式展开可能造成节点数量快速增长，需要限制其只来源于订阅节点
- 若部署时启用了 token，客户端必须稳定携带同一 token；否则 Surge 自动更新会失效

边界约束：

- 本系统不负责校验远程规则集 URL 的内容格式是否适配客户端
- 本系统不负责客户端运行期的探测、测速和规则下载错误
- 本系统只保证生成结果在结构上自洽、在语义上可渲染

---

## 文档导航

实现细节由后续文档承接：

- `docs/design/config-schema.md`：配置结构与字段约束
- `docs/design/domain-model.md`：领域模型与中间表示
- `docs/design/pipeline.md`：各阶段职责与数据约束
- `docs/design/api.md`：HTTP 接口约定
- `docs/design/rendering.md`：Clash Meta / Surge 渲染映射
- `docs/design/validation.md`：配置与图校验规则
- `docs/design/caching.md`：订阅拉取与缓存策略
- `docs/implementation/project-structure.md`：代码目录与包边界
- `docs/implementation/implementation-plan.md`：实施顺序与阶段产出
- `docs/implementation/testing-strategy.md`：测试与验收策略
