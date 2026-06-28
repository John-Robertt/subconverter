# 渲染设计

> 状态：v3.0 目标契约。本文定义 TargetView 到 Clash Meta / Surge 文本的序列化边界。

## 渲染边界

Render Adapter 的输入是 `RenderInput`：

```go
type RenderInput struct {
    Target   TargetView
    Template RenderTemplate
    Managed  ManagedConfig
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

Render 负责：

- 字段排序。
- 目标格式语法。
- 底版模板合并。
- Surge managed header。
- 文本编码。

它不负责：

- 拉取订阅。
- 读取 ConfigStore 或 Resource Adapter。
- 构造订阅链接或持有原始 token。
- 过滤目标格式不支持的协议。
- 级联清空组或 fallback。
- 修改 RuntimeSnapshot、Pipeline 或 TargetView。

`Target` 是 Render 的唯一业务图输入。模板内容和 managed URL 由 ArtifactService 或 PreviewService 在调用 Render 前准备。

## Clash Meta

TargetView 映射：

- `Proxies` -> `proxies`
- `NodeGroups` + `RouteGroups` -> `proxy-groups`
- `Rulesets` -> `rule-providers`
- `Rules` + ruleset 引用 + fallback -> `rules`

规则：

- 链式节点输出 `dialer-proxy`。
- fallback 输出为 `MATCH,<fallback>`。
- `url-test` 默认参数：`url=http://www.gstatic.com/generate_204`、`interval=300`、`tolerance=100`。
- rule-provider 命名必须全局去重并保持确定性。

协议：

- SS、AnyTLS、VLESS 可输出到 Clash。
- Snell 不应出现在 Clash TargetView 中。
- 若 Render 看到 Snell，返回内部不变量错误。

## Surge

TargetView 映射：

- `Proxies` -> `[Proxy]`
- `NodeGroups` + `RouteGroups` -> `[Proxy Group]`
- `Rulesets`、`Rules`、fallback -> `[Rule]`

规则：

- 链式节点输出 `underlying-proxy`。
- fallback 输出为 `FINAL,<fallback>`。
- 配置了 `base_url` 时输出 `#!MANAGED-CONFIG` header。
- 底版模板中的旧 managed header 必须剥离。

协议：

- SS、AnyTLS、Snell 可输出到 Surge。
- VLESS 不应出现在 Surge TargetView 中。
- 若 Render 看到 VLESS，返回内部不变量错误。

## 协议字段原则

- 字段顺序必须固定，保证 golden 测试稳定。
- `Params` 中未知键默认不输出。
- 目标格式支持的新字段应通过固定 key order 显式加入。
- 协议支持矩阵来自 Capability Registry，不由 Render Adapter 自行判断。

## 底版模板合并

### Clash

- 使用 yaml.v3 Node API 解析模板。
- 替换根 mapping 中的 `proxies`、`proxy-groups`、`rule-providers`、`rules`。
- 其他 key 原样保留。
- 模板为空、非 YAML 或非 mapping document 时返回 render diagnostic。

### Surge

- 按 section header 切分模板。
- 替换 `[Proxy]`、`[Proxy Group]`、`[Rule]`。
- 其他 section 原样保留。
- 缺失 section 时追加到末尾。
- preamble 中旧 managed header 必须移除。

## Managed URL

Surge managed URL 由 ArtifactService 构造后通过 `RenderInput.Managed.URL` 传入：

- base URL 来自 RuntimeSnapshot 中的 Prepared Runtime Config。
- 订阅访问 token 由服务端持有，只能在需要时嵌入最终 URL，不得作为独立字段传给 Render。
- filename 已由 HTTP 层或 ArtifactService 规范化为安全 ASCII 文件名。
- Render 不能依赖当前请求使用管理员 session 还是 query token。
- `Managed.URL` 可能包含敏感 token，除最终产物外不得进入 Diagnostic metadata、日志或测试 golden。

## 错误语义

Render 阶段错误分两类：

- 用户可修复：模板内容不合法、不支持的 Surge SS plugin 参数、managed URL 缺失或非法等，返回 render diagnostic。
- 内部不变量：TargetView 含不支持协议、缺少 required graph 字段等，返回 500 类错误。

Render 失败不得修改快照或缓存半成品。

## 测试要求

- Clash / Surge golden 输出保持稳定。
- Render 测试手工构造 RenderInput，不跨层调用 Build Engine。
- 模板合并保留非托管段。
- Snell 不进入 Clash TargetView，VLESS 不进入 Surge TargetView；若强行输入，Render 返回内部不变量错误。
