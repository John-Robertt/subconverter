# 核心模型设计

> 状态：v3.0 目标契约。本文定义 Product Core 中长期稳定的数据结构。

## 模型主线

```text
Config
  -> PreparedConfig
  -> RuntimeSnapshot
  -> Pipeline
  -> TargetView
  -> RenderInput
  -> Artifact
```

核心模型只表达产品语义，不表达 HTTP、文件格式、存储路径或渲染实现。

## Config

`Config` 是用户配置的语义模型，也是 API DTO、Web 草稿和保存请求的共同结构。

```go
type Config struct {
    Sources       Sources
    Filters       Filters
    Groups        []Named[Group]
    CustomProxies []CustomProxy
    Routing       []Named[RouteGroup]
    Rulesets      []Named[RulesetConfig]
    Rules         []string
    Fallback      string
    Templates     Templates
    BaseURL       string
    Options       Options
}

type Named[T any] struct {
    Name  string
    Value T
}

type RulesetConfig struct {
    Policy string
    URLs   []string
}
```

约束：

- `Groups`、`CustomProxies`、`Routing`、`Rulesets`、`Rules` 必须保序。
- 所有持久字段必须能通过稳定 JSON Pointer 定位。
- `Config` 不包含注释、缩进、主配置存储路径或渲染中间状态。
- `Rules` 保存用户声明的原始 rule 字符串；Build 阶段再解析出可校验的 policy。

## PreparedConfig

`PreparedConfig` 是 reload 或草稿预览时从 `Config` 派生出的只读预计算结构。

内容包括：

- 编译后的正则。
- 解析后的自定义代理。
- 展开的 `@auto`。
- 静态命名空间。
- 模板引用。
- 运行时所需常量。

`PreparedConfig` 不用于保存写回。

## RuntimeSnapshot

`RuntimeSnapshot` 表达一次成功生效的不可变运行态。

```go
type RuntimeSnapshot struct {
    ID               string
    ConfigRevision   string
    SnapshotRevision string
    LoadedAt         time.Time
    Prepared         PreparedConfig
    Capabilities     CapabilitySet
    ExportSource     RuntimeExportSource
}

type RuntimeExportSource struct {
    Config         Config
    TemplateRefs   Templates
    ConfigRevision string
}
```

规则：

- 保存配置不改变快照。
- reload 成功才替换快照。
- reload 失败保留旧快照。
- 请求期代码不得修改快照内部数据。
- 生效配置导出只读取 `ExportSource`，不得重新读取工作配置。

## Pipeline

`Pipeline` 是格式无关生成图。

```go
type Pipeline struct {
    Proxies     []Proxy
    NodeGroups  []ProxyGroup
    RouteGroups []ProxyGroup
    Rulesets    []Ruleset
    Rules       []Rule
    Fallback    string
}
```

Build Engine 只产出格式无关图，不判断 Clash / Surge 协议差异。

## Proxy

```go
type Proxy struct {
    Name   string
    Kind   ProxyKind
    Type   Protocol
    Server string
    Port   int
    Params map[string]string
    Dialer string
}
```

`Kind` 表达来源语义：`subscription`、`snell`、`vless`、`custom`、`chained`。

`Type` 表达代理协议：`ss`、`anytls`、`snell`、`vless`、`socks5`、`http`。

`Dialer` 仅对 chained proxy 有意义。

## ProxyGroup

```go
type ProxyGroup struct {
    Name    string
    Kind    GroupKind
    Members []string
    Mode    string
}
```

`Kind`：

- `node_group`：节点组。
- `route_group`：服务组。

`Members` 引用 proxy name 或 group name。合法性由 Build 校验和 Target Projection 校验共同保证。

## Ruleset / Rule

```go
type Ruleset struct {
    Name   string
    URLs   []string
    Policy string
}

type Rule struct {
    Raw    string
    Policy string
}
```

`Ruleset.Policy` 和 `Rule.Policy` 必须引用节点组或服务组。Target Projection 可因 policy 被过滤而级联移除 ruleset 或 rule。

## TargetView

```go
type TargetView struct {
    Format      TargetFormat
    Proxies     []Proxy
    NodeGroups  []ProxyGroup
    RouteGroups []ProxyGroup
    Rulesets    []Ruleset
    Rules       []Rule
    Fallback    string
    Diagnostics []Diagnostic
}
```

TargetView 是某个目标格式下可渲染的视图。它不修改 Pipeline，只复制或过滤必要内容。

## RenderInput

`RenderInput` 是 TargetView 进入渲染器时的完整上下文。

```go
type RenderInput struct {
    Target    TargetView
    Template  RenderTemplate
    Managed   ManagedConfig
}

type RenderTemplate struct {
    Source  string
    Content []byte
}

type ManagedConfig struct {
    Enabled bool
    URL     string
}
```

规则：

- `Target` 是 Render 的唯一业务图输入。
- `Template` 由 ArtifactService 或 PreviewService 通过资源端口读取后传入，Render 不读取 ConfigStore 或 Resource Adapter。
- `Managed.URL` 由 ArtifactService 基于快照中的 `BaseURL`、服务端订阅访问 token 和安全文件名构造；Render 只负责注入。
- `Managed.URL` 可能包含敏感 token，除最终产物外不得进入 Diagnostic metadata 或日志。

## DiagnosticBundle

```go
type DiagnosticBundle struct {
    Valid       bool
    Diagnostics []Diagnostic
}
```

所有产品层失败都必须能转换为 Diagnostic。请求层错误可以使用普通 error 响应。

## 共享不变量

- 节点名全局唯一。
- 节点组名与服务组名互斥。
- 链式节点上游只能是拉取类节点。
- `@all` 只含原始节点，不含链式节点。
- 带 `relay_through` 的 custom proxy 不产出普通 custom proxy。
- `groups`、`routing`、`rulesets` 保序。
- TargetView 不得反向修改 Pipeline。
- RuntimeSnapshot 创建后请求期只读。
