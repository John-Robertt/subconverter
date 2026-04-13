# 校验设计

## 目标

本文件定义配置校验与引用校验的边界。校验分为三类：字段级静态校验、M2/M3 构建期校验和图级语义校验。

---

## 静态校验

静态校验发生在配置加载后、管道执行前。

检查范围：

- 必填字段是否存在
- 枚举值是否合法
- 条件字段是否成对出现
- 正则表达式是否可编译
- URL 是否满足基本格式要求

说明：

- 访问 token 不属于用户 YAML 配置，因此不在本层做字段校验；其存在性和匹配性由 HTTP 层在请求进入时校验

重点规则：

- `subscriptions[].url` 必填
- `custom_proxies[].name/type/server` 必填
- `custom_proxies[].port` 必须为正整数
- `custom_proxies` 之间名称不能重复
- `groups[*].match` 必填
- `groups[*].strategy` 必填且只能是 `select` 或 `url-test`
- `relay_through.type` 必填
- `relay_through.strategy` 必填且只能是 `select` 或 `url-test`
- `relay_through.type=group` 时必须提供 `name`
- `relay_through.type=select` 时必须提供 `match`
- `fallback` 必填
- `base_url` 可选；若非空，必须以 `http://` 或 `https://` 开头
- `routing` 中同一 entry 内 `@auto` 最多出现一次
- `routing` 中同一 entry 内 `@all` 与 `@auto` 不能同时出现

---

## 构建期校验

构建期校验发生在 `Source`、`Filter`、`Group`、`Route` 等管道阶段内部，用于尽早拦截依赖运行期输入的数据问题。

检查范围：

- 订阅拉取后是否得到至少一个有效节点
- 运行期名称冲突是否出现
- `relay_through.type=group` 的局部引用是否存在
- SS URI 与 plugin query 是否能按 SIP002 基本结构解析

重点规则：

- 订阅拉取结果不得为空（0 个有效节点视为上游订阅内容错误）
- 自定义代理名称不得与订阅节点名称冲突
- `relay_through.type=group` 引用的节点组必须存在
- SS URI 中端口值必须在 1-65535 范围内
- 非法 SS URI、非法 fragment 编码、非法 query 编码、损坏的 plugin 参数都属于 Source 阶段构建期错误

---

## 图级校验

图级校验发生在 Group 和 Route 阶段之后。

检查范围：

- 名称引用是否存在
- 共享命名空间是否无冲突
- 规则集绑定是否存在目标服务组
- fallback 是否引用有效服务组
- 服务组之间是否存在循环引用
- 链式展开后结果是否为空
- `@all` 是否正确排除了链式节点

重点规则：

- 跨订阅重名节点自动追加递增后缀
- 代理名、节点组名、服务组名在引用体系中共享命名空间，不允许重名或重复声明
- 节点组名与服务组名不得重复（共享同一命名空间）
- `routing` 中原始声明的成员只允许引用节点组、服务组、`DIRECT`、`REJECT`、`@all`、`@auto`
- `routing` 中 `@all` 展开后的具体代理名可出现在中间表示里，但用户配置不能直接声明这些代理名
- `routing` 中引用的节点组、服务组、保留字都必须可解析
- `rulesets` 的 key 必须存在于 `routing`
- 自动生成的链式组必须至少包含一个成员
- 地区节点组匹配结果不得为空

---

## 错误分层

按来源区分错误：

- 配置错误（`ConfigError`）
- 远程拉取错误（`FetchError`）
- 本地资源读取错误（`ResourceError`）
- 构建错误（`BuildError`）
- 渲染错误（`RenderError`）

HTTP 层映射：

- `400`：请求参数错误、静态配置错误、图级语义错误、可归因于用户配置的构建错误
- `401`：服务端启用访问 token 时，请求缺少 token 或 token 不匹配
- `502`：远程资源拉取失败，或远程订阅内容不可用（如 0 个有效节点）
- `500`：本地资源读取失败、渲染错误或未分类内部错误

目标：

- 让错误能明确定位到配置、网络还是内部逻辑
- 保持 HTTP 层错误码映射简单稳定
