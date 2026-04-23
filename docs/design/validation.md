# 校验设计

## 目标

本文件定义配置校验与引用校验的边界。校验分为三类：字段级静态校验、构建期校验和图级语义校验。

---

## 静态校验（Prepare 阶段）

静态校验在 `config.Prepare()` 中完成，发生在配置加载后、请求期管道执行前。Prepare 同时执行正则编译、URL 解析和 `@auto` 展开，产出不可变的 `RuntimeConfig`。

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
- `snell[].url` 必填，必须为 HTTP(S) URL
- `vless[].url` 必填，必须为 HTTP(S) URL
- `custom_proxies[].name` 必填，且互不重复
- `custom_proxies[].url` 必填；scheme 必须是 `ss://`、`socks5://` 或 `http://`；URL 解析失败（host/port 缺失、port 越界、SS userinfo 解码失败、SS plugin 选项格式错误等）记为字段错误
- `custom_proxies[].url` 为 SS 时还需含 `cipher` + `password`（base64/明文 userinfo 解码后必须形如 `method:password`）
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
- Snell 来源拉取结果不得为空（0 个有效节点报同类错误）
- VLESS 来源拉取结果不得为空（0 个有效节点报同类错误）
- Snell 来源中单行解析失败整源报错（与 SS 订阅的静默跳过不同，详见 `pipeline.md`）；错误消息附带脱敏后的来源 URL 和 1-based 物理行号
- VLESS 来源中单行解析失败整源报错（与 Snell 一致，详见 `pipeline.md`）；错误消息附带脱敏后的来源 URL 和 1-based 物理行号
- 自定义代理名称不得与订阅、Snell 或 VLESS 节点名称冲突（错误消息会指明冲突源的 kind）
- `relay_through.type=group` 引用的节点组必须存在
- SS URI 中端口值必须在 1-65535 范围内；Snell / VLESS 节点的 port 字段同理
- 非法 SS URI、非法 fragment 编码、非法 query 编码、损坏的 plugin 参数都属于 Source 阶段构建期错误
- Snell 行格式错误（缺 `=`、type 非 snell、缺必填 psk 等）属于 Source 阶段构建期错误
- VLESS URI 中非法 UUID、非法 `security`、非法端口等属于 Source 阶段构建期错误；未知 `type` 不报错，而是按 Mihomo 兼容语义回落到 `tcp`

---

## 图级校验

图级校验发生在 Group 和 Route 阶段之后。

检查范围：

- 共享命名空间是否无冲突、重复声明
- `@all` 展开是否正确排除了链式节点
- 空节点组（地区组和链式组）
- 路由成员引用合法性（区分原始声明 vs 展开后的成员溯源）
- 服务组之间是否存在循环引用

说明：ruleset/rule 策略存在性和 fallback 存在性由启动期 Prepare 保证，ValidateGraph 不再重复检查。

重点规则：

- 跨订阅重名节点自动追加递增后缀
- 代理名、节点组名、服务组名在引用体系中共享命名空间，不允许重名或重复声明
- 节点组名与服务组名不得重复（共享同一命名空间）
- `routing` 中原始声明的成员只允许引用节点组、服务组、`DIRECT`、`REJECT`、`@all`、`@auto`
- `routing` 中 `@all` 展开后的具体代理名可出现在中间表示里，但用户配置不能直接声明这些代理名
- `routing` 中引用的节点组、服务组、保留字都必须可解析
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

补充说明：

- `BuildError` 的 `Error()` 文本格式保持稳定；需要补充来源上下文时，可通过 `Cause` / `Unwrap()` 保留内层根因链
- 当前 Snell 单行失败路径采用“外层 `BuildError` 提供来源 URL + 物理行号，内层解析错误保留具体语法原因”的模式
- 当前 VLESS 单行失败路径采用与 Snell 相同的“外层 `BuildError` 提供来源 URL + 物理行号，内层解析错误保留具体语法原因”的模式

HTTP 层映射：

- `400`：请求参数错误、静态配置错误、图级语义错误、可归因于用户配置的构建错误
- `401`：服务端启用访问 token 时，请求缺少 token 或 token 不匹配
- `502`：远程资源拉取失败，或远程订阅内容不可用（如 0 个有效节点）
- `500`：本地资源读取失败、渲染错误或未分类内部错误

目标：

- 让错误能明确定位到配置、网络还是内部逻辑
- 保持 HTTP 层错误码映射简单稳定

---

## 目标格式投影校验

`ValidateGraph` 仍然保持格式无关，只负责统一 IR 上的引用与图结构正确性。目标格式差异则前移到 `Target` 阶段：

- `target.ForClash`：剔除 Snell 节点及其级联影响
- `target.ForSurge`：剔除 VLESS 节点及其级联影响
- 若 `fallback` 在目标格式视图中被清空，立即返回带清空路径的错误
- 级联过滤的诊断路径格式（`(snell)`、`(chained)`、`(cycle)` 等标记）详见 `rendering.md` §级联过滤

这样分层后：

- `Build` 只产出格式无关 IR
- `Target` 承接 format-specific 过滤与校验
- `Render` 只负责文本序列化

当前错误类型仍沿用 `RenderError` 以保持外部 HTTP 错误语义稳定；其中 fallback 被清空仍使用 `CodeRenderClashFallbackEmpty` / `CodeRenderSurgeFallbackEmpty`，而 Target 阶段的内部不变量异常使用独立 projection 错误码，避免与用户配置触发的 fallback 清空混淆。
