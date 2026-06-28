# 统一诊断模型

> 状态：v3.0 目标契约。本文定义 Diagnostic 与 Diagnostic Bundle 的稳定字段。

## 目标

v3.0 使用统一 Diagnostic 表达配置读取、静态校验、来源拉取、图构建、目标格式投影和渲染阶段的问题。诊断同时服务 API、Web UI、测试和用户文档。

## Diagnostic Bundle

```go
type DiagnosticBundle struct {
    Valid       bool
    Diagnostics []Diagnostic
}
```

语义：

- `Valid=false` 表示存在阻塞当前动作的 `error`。
- `warning` 和 `info` 不阻塞保存、预览或生成，除非调用方明确要求严格模式。
- 请求层错误不包装成 Diagnostic Bundle，例如鉴权失败、revision 冲突、JSON 无法解析。

## Diagnostic

```go
type Diagnostic struct {
    Severity  DiagnosticSeverity
    Phase     DiagnosticPhase
    Code      string
    Message   string
    Locator   DiagnosticLocator
    Format    string
    CausePath []DiagnosticCause
    Metadata  map[string]string
}
```

字段语义：

- `Severity`：`error`、`warning`、`info`。
- `Phase`：`config_io`、`prepare`、`source`、`filter`、`group`、`route`、`graph`、`target`、`render`。
- `Code`：稳定 code，用于测试和前端分类。
- `Message`：中文用户可见描述。
- `Locator`：定位到配置字段、文档位置或运行时对象。
- `Format`：格式相关诊断使用，值为 `clash` 或 `surge`。
- `CausePath`：级联失败路径。
- `Metadata`：只读辅助信息，用于脱敏后的 URL、source id、target format 等前端展示或测试分类；不得放入未脱敏敏感值。

`Metadata` 不参与程序定位，前端跳转仍以 `Locator` 为准。

## Locator

```go
type DiagnosticLocator struct {
    JSONPointer string
    DisplayPath string
    Section     string
    Key         string
    Index       *int
    ValuePath   string
    RuntimeName string
}
```

定位原则：

- 能定位到配置字段时，必须提供 `JSONPointer`。
- `JSONPointer` 指向 API JSON DTO，统一以 `/config` 为根，例如 `/config/groups/0/value/match`。
- `DisplayPath` 面向用户展示，例如 `groups.HK.match`。
- `Section`、`Key`、`Index`、`ValuePath` 用于前端导航。
- 运行时对象问题使用 `RuntimeName` 表达节点名、组名或策略名。
- 无法精确定位时，至少提供 `Section` 或 `RuntimeName`。

对外 JSON 字段固定为：

```json
{
  "json_pointer": "/config/groups/0/value/match",
  "display_path": "groups.HK.match",
  "section": "groups",
  "key": "HK",
  "index": 0,
  "value_path": "match",
  "runtime_name": "HK"
}
```

字段为空时可以省略。即使内部校验函数接收裸 `Config`，进入 API wire shape 前也必须映射到 `/config/...`。

## Cause Path

```go
type DiagnosticCause struct {
    Name   string
    Kind   string
    Reason string
}
```

用途：

- 表达目标格式过滤造成的级联影响。
- 表达 fallback 清空的上游链路。
- 表达链式节点因上游被过滤而失效。

对外 JSON 字段名固定为 `cause_path`。

示例：

```json
[
  { "name": "FINAL", "kind": "route_group", "reason": "empty_group" },
  { "name": "SVC_STREAM", "kind": "route_group", "reason": "member_dropped" },
  { "name": "HK-Snell", "kind": "proxy", "reason": "unsupported_protocol:snell" }
]
```

## 阶段边界

| Phase | 职责 |
|-------|------|
| `config_io` | 配置读取、配置格式 decode/encode、导入导出 |
| `prepare` | 字段、URL、正则、静态命名空间、静态引用 |
| `source` | 远程来源拉取、来源解析、来源为空 |
| `filter` | 节点过滤结果与过滤后风险 |
| `group` | 节点组匹配、链式组构建 |
| `route` | 服务组展开、`@auto` / `@all` 解释 |
| `graph` | 格式无关图校验 |
| `target` | 目标格式能力过滤和格式相关图校验 |
| `render` | 模板合并和最终序列化 |

## HTTP 集成

- `POST /api/workspace/validate`：请求体合法时总是返回 `200` + Diagnostic Bundle。
- `PUT /api/workspace/config`：配置语义失败返回 `400` + Diagnostic Bundle。
- `POST /api/runtime/reload`：reload 失败返回 Diagnostic Bundle，旧快照保持不变。
- `GET/POST /api/preview/*`：预览失败返回 Diagnostic Bundle 或带 diagnostics 的部分解释结果。
- `GET /api/artifacts/{format}`：生成失败返回 Diagnostic Bundle 或请求层 error。

## 测试要求

- 每个稳定 code 至少有一条测试。
- 每个 phase 至少覆盖一类转换。
- 字段级错误必须包含 `locator.json_pointer`。
- 格式相关错误必须包含 `format`。
- 级联错误必须包含 `cause_path`。
- URL 在进入 message 或 metadata 前必须脱敏。
