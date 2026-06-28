# 校验与错误语义

> 状态：v3.0 目标契约。本文定义校验阶段、诊断边界和 HTTP 映射原则。

## 校验分层

| 层级 | 阶段 | 处理问题 |
|------|------|----------|
| 请求层 | HTTP Adapter | JSON 无法解析、缺字段、鉴权、同源、revision 冲突 |
| 配置 I/O | Config I/O | 配置格式 decode/encode、导入导出、配置源只读 |
| 静态层 | Prepare | 字段、URL、正则、命名空间、静态引用 |
| 构建层 | Source/Filter/Group/Route/Graph | 远程来源、动态名称冲突、空组、图引用 |
| 目标层 | Target/Render | 格式能力过滤、级联影响、模板合并、序列化 |

请求层错误使用单 error 响应；产品层问题使用 DiagnosticBundle。

## Config I/O 校验

检查范围：

- 输入格式是否可 decode 为 Config。
- ConfigStore 是否可读或可写。
- revision 是否匹配。
- 导入内容是否完整。
- 导出 codec 是否可用。

失败示例：

- `config_decode_failed`
- `config_encode_failed`
- `config_revision_conflict`
- `config_source_readonly`
- `config_store_write_failed`

## Prepare 静态校验

检查范围：

- 必填字段。
- 枚举值。
- HTTP(S) URL 和代理 URL。
- 正则可编译。
- `groups` 不为空。
- `fallback` 存在。
- `routing` 中 `@auto` / `@all` 的静态约束。
- `sources.fetch_order` 完整、无重复、无未知项。
- 自定义代理名称和静态命名空间冲突。

Prepare 输出 PreparedConfig；失败输出 prepare diagnostics。

## 构建期校验

Source 阶段：

- 远程拉取失败。
- 拉取结果为空。
- 订阅内容格式错误。
- Snell / VLESS 单行解析失败。
- 拉取类节点去重后与自定义节点冲突。

Group / Route / Graph 阶段：

- 地区组匹配为空。
- 链式组上游为空。
- 服务组引用非法。
- 服务组循环引用。
- `@all` 展开后导致空成员风险。

构建期错误若能归因于用户配置或远程内容，返回 Diagnostic；远程不可用可映射为 502。

## 目标格式校验

Target Projection 负责格式相关问题：

- Clash 过滤 Snell。
- Surge 过滤 VLESS。
- 过滤导致链式节点失效。
- 过滤导致组、ruleset、rule 被移除。
- fallback 被级联清空。

要求：

- 格式相关诊断必须包含 `format`。
- 级联失败必须包含 `cause_path`。
- 可修复的目标投影失败映射为 400。
- 内部不变量错误映射为 500。

## Render 校验

Render 只处理序列化和模板问题：

- Clash 模板无法合并。
- Surge 模板 section 无法处理。
- 目标格式编码失败。
- managed URL 缺失或非法。

如果 TargetView 含有目标格式不支持的协议，这是 Target Projection 的 bug。

## HTTP 状态码映射

| 状态码 | 场景 |
|--------|------|
| 200 | 成功；校验查询可返回 `valid=false` |
| 400 | 请求体错误、配置语义错误、图错误、可修复 target 错误 |
| 401 | 未认证、session 过期、订阅 token 错误 |
| 403 | 同源校验失败 |
| 409 | 只读源、文件不可写、revision 冲突 |
| 429 | reload 互斥 |
| 502 | 远程配置、订阅或模板不可用 |
| 500 | 本地资源异常、内部不变量错误、渲染内部错误 |

## 诊断要求

- 每条 Diagnostic 必须包含 `severity`、`phase`、`code`、`message`。
- 字段级问题能定位时必须包含 `locator.json_pointer`。
- 面向用户展示应使用 `display_path` 或 message，不解析 JSON Pointer。
- 格式相关问题必须包含 `format`。
- 级联问题必须包含 `cause_path`。
- URL 必须先脱敏再进入 message 或 diagnostic metadata。
