# 能力注册表设计

> 状态：v3.0 目标契约。本文定义来源、协议和目标格式能力的集中入口。

## 目标

能力注册表用于收敛新增来源、代理协议和目标格式时的分派点。开发者应能从一个入口理解：

- 某个来源产生什么 Proxy Kind。
- 某个协议允许出现在哪些 Kind 上。
- 某个协议支持哪些目标格式。
- 某个目标格式支持哪些协议。

能力注册表分为两层：

- 静态能力矩阵：Product Core 可依赖，只包含来源、协议、目标格式和支持关系。
- Engine Binding Registry：组合层或 engine 层维护，把 source key、target format 绑定到 parser、projector、renderer 等具体实现。

Product Core 不持有 parser、projector 或 renderer，避免反向依赖具体实现层。

## 核心类型

```go
type SourceKindSpec struct {
    Key       string
    ProxyKind string
    FetchKind bool
}

type ProtocolSpec struct {
    Type              string
    AllowedProxyKinds []string
    SupportedTargets  []string
    Chainable         bool
}

type TargetSpec struct {
    Format             string
    SupportedProtocols []string
}

type CapabilitySet struct {
    Sources   []SourceKindSpec
    Protocols []ProtocolSpec
    Targets   []TargetSpec
}
```

静态能力矩阵是内部机制，不是插件系统。它可以被 Config I/O、Prepare、Target Projection 和 Web UI DTO 共同读取。

实现绑定另设类型，位置在 composition 或 engine 层：

```go
type EngineBindingRegistry struct {
    SourceParsers    map[string]SourceParser
    TargetProjectors map[string]TargetProjector
    Renderers        map[string]Renderer
}
```

`EngineBindingRegistry` 不进入 Product Core；它只负责把已经通过静态能力矩阵校验过的 key 分派给具体实现。

## 初始能力矩阵

### 来源

| Source key | Proxy Kind | Fetch kind | 说明 |
|------------|------------|------------|------|
| `subscriptions` | `subscription` | 是 | SS / AnyTLS 订阅 |
| `snell` | `snell` | 是 | Surge 专属协议来源 |
| `vless` | `vless` | 是 | Clash 专属协议来源 |
| `custom_proxies` | `custom` / `chained` | 否 | 本地声明，不进入 `fetch_order` |

### 协议

| Protocol | Clash | Surge | Chainable |
|----------|-------|-------|-----------|
| `ss` | 是 | 是 | 是 |
| `anytls` | 是 | 是 | 是 |
| `snell` | 否 | 是 | 是 |
| `vless` | 是 | 否 | 是 |
| `socks5` | 是 | 是 | 否 |
| `http` | 是 | 是 | 否 |

`socks5` 和 `http` 仅允许来自不带 `relay_through` 的 custom proxy；链式节点是否可渲染由其模板协议和目标格式共同决定。

## 使用方式

- Config I/O 和 Prepare 使用 SourceKindSpec 判断 `sources` 合法 key 和 `fetch_order`。
- Source 阶段按 fetch kind source specs 调度拉取，再通过 Engine Binding Registry 查找 parser。
- Proxy invariant 校验使用 ProtocolSpec 判断 Kind / Type 组合。
- Target Projection 使用 TargetSpec 和 ProtocolSpec 判断需要过滤的协议，再通过 Engine Binding Registry 查找 projector。
- Render Adapter 只处理 TargetView，并通过 Engine Binding Registry 被调用；它不自行发明协议支持规则。
- Web UI 的“仅 Clash / 仅 Surge”提示来自同一能力矩阵。

## 设计约束

- 静态能力矩阵必须可 grep、可测试、无运行时动态加载。
- 新增来源、协议或目标格式必须先修改静态能力矩阵。
- 新增 parser、projector 或 renderer 时必须在 Engine Binding Registry 中显式绑定。
- Product Core 不导入 parser、projector、renderer 所在包。
- 不允许 app、target、render、前端分别维护协议支持矩阵。
- 能力矩阵变更必须同步文档和测试。

## 验收标准

- 新增 fetch-kind 来源时，不需要在 app 层维护第二份来源顺序枚举。
- 新增格式专属协议时，Target Projection 和 UI 提示来自同一能力定义。
- `core` 包不导入 source parser、target projector 或 render adapter。
- Render 测试只验证序列化，不验证协议支持矩阵。
- 能力矩阵至少被单测断言覆盖。
