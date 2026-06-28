# 目标格式投影设计

> 状态：v3.0 目标契约。本文定义格式无关 Pipeline 到 TargetView 的转换。

## 目标

Clash Meta 与 Surge 支持的协议集合不同。v3.0 将格式差异显式放在 Target Projection 阶段：Build Engine 产出格式无关图，Target Projection 产出某个格式的可渲染 TargetView 和诊断。

## 流程

```text
Pipeline(format-agnostic)
  -> load TargetSpec(format)
  -> compute unsupported protocols
  -> drop unsupported proxies
  -> drop chained proxies with dropped dialer
  -> cascade groups / rulesets / rules / fallback
  -> validate projected graph
  -> TargetView(format) + Diagnostics
```

## TargetView

```go
type TargetView struct {
    Format      string
    Proxies     []Proxy
    NodeGroups  []ProxyGroup
    RouteGroups []ProxyGroup
    Rulesets    []Ruleset
    Rules       []Rule
    Fallback    string
    Diagnostics []Diagnostic
}
```

约束：

- TargetView 是 Render Adapter 的唯一业务输入。
- TargetView 必须满足目标格式协议能力。
- TargetView 中不出现被过滤协议。
- TargetView 不修改原 Pipeline。

## 初始投影规则

### Clash

- 支持 SS、AnyTLS、VLESS、custom socks5/http、可渲染链式节点。
- 过滤 Snell。
- 过滤以上游 Snell 为 dialer 的链式节点。
- 级联移除空组、失效 ruleset、失效 rule。
- fallback 清空返回 target diagnostic。

### Surge

- 支持 SS、AnyTLS、Snell、custom socks5/http、可渲染链式节点。
- 过滤 VLESS。
- 过滤以上游 VLESS 为 dialer 的链式节点。
- 级联移除空组、失效 ruleset、失效 rule。
- fallback 清空返回 target diagnostic。

## 级联诊断

格式投影诊断必须回答：

- 哪个目标格式受影响。
- 哪些协议或节点被过滤。
- 哪些链式节点因上游被过滤而失效。
- 哪些组因此为空。
- 哪些 ruleset 或 rule 因 policy 失效被移除。
- fallback 是否因此不可用。

fallback 清空必须包含 `cause_path`，例如：

```json
[
  { "name": "FINAL", "kind": "route_group", "reason": "empty_group" },
  { "name": "SG", "kind": "node_group", "reason": "empty_group" },
  { "name": "SG-Snell", "kind": "proxy", "reason": "unsupported_protocol:snell" }
]
```

## 预览输出

`/api/preview/targets/{format}` 返回：

- TargetView 摘要。
- dropped proxies。
- dropped groups。
- dropped rulesets / rules。
- fallback 状态。
- generatable 布尔值。
- diagnostics。

预览和实际生成必须使用同一 Target Projection 实现。

## 渲染器边界

Render Adapter 只处理 TargetView 序列化和模板合并。若 TargetView 不满足目标格式前置条件，这是 Target Projection 的 bug；Render 可返回内部不变量错误，但不能自行修正图。

## 测试要求

- Clash 过滤 Snell 的级联行为。
- Surge 过滤 VLESS 的级联行为。
- 过滤后 fallback 清空的 cause path。
- 原 Pipeline 不被投影修改。
- TargetView 不包含目标格式不支持的协议。
