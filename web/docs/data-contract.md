# 数据契约索引

本文件只列前端必须消费的关键数据形态。完整结构以顶层设计文档为准。

## API

正式管理接口统一使用 `/api/*`：

- `GET /api/auth/status`
- `POST /api/auth/login`
- `POST /api/auth/setup`
- `POST /api/auth/logout`
- `GET /api/config`
- `PUT /api/config`
- `POST /api/config/import`
- `GET /api/config/saved.zip`
- `GET /api/config/effective.zip`
- `GET /api/config/effective.yaml`（后端已实现，`api/client.ts` 已导出路径常量，当前 UI 未直接暴露入口）
- `POST /api/config/validate`
- `POST /api/resources/read`
- `POST /api/subscriptions/check`
- `POST /api/reload`
- `GET/POST /api/preview/nodes`
- `GET/POST /api/preview/groups`
- `GET/POST /api/generate/preview`
- `GET /api/generate/link`
- `GET /api/status`

生成接口继续使用 `/generate`。
健康检查继续使用 `GET /healthz`，成功时返回 HTTP `200` 与纯文本 `ok`，不返回 JSON。

## 认证状态

`GET /api/auth/status` 返回：

- `authed`：当前 session 是否有效。
- `setup_required`：是否需要首次创建管理员账号。
- `setup_token_required`：首次 setup 是否要求输入 bootstrap setup token。
- `locked_until`：登录锁定截止时间，未锁定时为空。

登录成功由服务端设置 `session_id` Cookie。前端只读取状态结果，不读取 Cookie 内容。

`POST /api/auth/setup` 请求体必须包含 `username`、`password`、`setup_token`。缺少 setup token 返回 `401 setup_token_required`；setup token 不匹配返回 `401 setup_token_invalid`；管理员凭据已存在返回 `409 setup_not_allowed`。

## 配置 revision

- `config_revision` 表示当前已保存内容；本地文件缺失使用 `missing` 并返回空草稿，前端将其作为不透明版本传回。
- `runtime_config_revision` 表示当前 `RuntimeConfig` 对应的配置。
- 没有文件且没有运行快照时 `config_dirty=false`，其余按保存版本和运行版本是否不同判断。
- `PUT /api/config` 必须携带最近一次拿到的 `config_revision`。
- 409 revision 冲突时，前端不得静默覆盖用户草稿。
- `config_revision` 是乐观并发令牌，用于防止旧页面或旧 revision 覆盖已观测到的新配置；不表示服务端提供外部多写者线性一致写入。

## 校验响应

- `POST /api/config/validate` 请求体合法时返回 `200 ValidateResult`；配置语义无效时 `valid=false`。
- `PUT /api/config` 静态校验失败时返回 `400 ValidateResult`，结构同上，`valid=false`。
- `POST /api/reload` 静态校验失败时返回 `400 ValidateResult`，结构同上，`valid=false`。
- 请求体无法解析、缺少字段、revision 冲突、只读配置源和文件不可写等非配置语义错误返回 `{ "error": { ... } }`。

## 保序字段

前端必须保留三类顺序语义：

- `groups`、`routing`、`rulesets` 在 JSON API 中使用 `[{key,value}]` 数组表示。
- `sources.fetch_order` 保存 `subscriptions` / `snell` / `vless` 的拉取顺序，写回时必须保留；AnyTLS 与 SS 共用 `subscriptions`。
- `sources.fetch_order` 缺失或为空时服务端使用默认顺序；非空时必须完整包含 `subscriptions`、`snell`、`vless` 且三项各出现一次，否则返回 `invalid_fetch_order` 诊断。
- `rules` 是普通数组，A6 拖拽排序直接改变数组顺序。

保序映射示例：

```json
[
  { "key": "HK", "value": { "match": "(HK)", "strategy": "select" } }
]
```

前端拖拽排序直接改变对应数组顺序，写回时保持该顺序。`sources.fetch_order` 的权威定义见 [`../../docs/design/config-schema.md`](../../docs/design/config-schema.md)。

## 诊断定位

配置诊断项必须通过 `locator.json_pointer` 定位字段。`display_path` 只用于展示，不作为程序反查依据。

前端需消费：

- `severity`
- `code`
- `message`
- `display_path`
- `locator.section`
- `locator.index`
- `locator.key`
- `locator.value_path`
- `locator.json_pointer`

## 预览结果

- 节点预览通过 `kind` 区分来源（订阅 / Snell / VLESS / 自定义），并展示类型、地址、端口、过滤状态与格式限定。
- 分组预览展示节点组、链式组、服务组和展开成员，前端优先消费带身份信息的 `member_details`、`expanded_member_details`、`all_proxy_details`，缺省时回退 `members`、`expanded_members`、`all_proxies`。
- `expanded_member_details` / `expanded_members` 区分用户显式声明（`literal`）、`@auto` 展开（`auto_expanded`）和 `@all` 展开（`all_expanded`）。

## 订阅链接结果

`GET /api/generate/link` 接受 `format`（必填）、`filename`（可选）与 `include_token`（`true`/`false`，默认 `true`；下载页卡片展示传 `false`，点击复制传 `true`），返回：

- `url`：可复制给 Clash Meta、Surge 或 Loon 客户端的完整订阅链接。
- `token_included`：链接是否包含订阅访问 token。

当前端复制 `token_included=true` 的链接时，必须先展示确认。

## HTTP 状态

前端需要处理的 HTTP 状态码：

- `400`：请求或配置语义错误。
- `401`：管理接口缺少、无效或已过期的 session；登录凭据错误；首次 setup 缺少/错误 setup token；或 `/generate` 缺少 / 不匹配订阅访问 token。
- `403`：非安全 `/api/*` 请求同源校验失败（`csrf_check_failed`）。
- `409`：按 `error.code` 分流处理。
- `423`：登录失败次数达上限临时锁定（`auth_locked`）。
- `429`：reload 正在执行（`reload_in_progress`），前端短间隔退避重试一次。
- `502`：远程主配置源或订阅等上游拉取失败，按接口上下文展示。

API client 归一化错误对象至少包含：

- `status`
- `code`
- `message`
- `details`

409 的前端行为：

- `config_revision_conflict`：保留草稿，提示外部修改，在提示中提供“重新加载配置”入口。
- `config_source_readonly`：进入只读模式，禁用保存、新增、删除和排序。
- `config_file_not_writable`：展示文件权限或部署挂载问题，并保留草稿。
- `resource_content_write_failed`：展示正文目录写入权限问题，保留草稿与编辑能力。
- `config_not_initialized`：本地主配置尚未创建或尚未热重载生效，引导初始化与热重载。
- 未知 409 code：按未知保存失败处理，不覆盖草稿，展示持久错误提示。

认证错误前端行为：

- `auth_required` / `session_expired`：跳转 `/login?next=<当前路径>`。
- `invalid_credentials`：登录页展示用户名或密码错误，并显示剩余尝试次数。
- `auth_locked`：登录页展示锁定截止时间并禁用提交。
- `setup_token_required` / `setup_token_invalid`：setup 模式展示 bootstrap token 错误，不创建管理员凭据。
- `/generate` 不返回 JSON `error.code`；缺少或错误订阅 token 时返回 `401 text/plain`，只表示客户端订阅访问 token 问题，不触发额外的后台凭据输入。

## Loon 来源与模板

`GenerateFormat` 包含 `loon`。来源编辑器保留 `sources.subscriptions[].format`（枚举见配置规约），设置页保留 `templates.loon`；读取、修改 URL、保存和导入导出均保持这些字段。下载页以相同运行时 revision 请求 Loon 预览与订阅链接。字段权威定义见 [配置规约](../../docs/design/config-schema.md#subscriptions)，输出语义见 [Loon 渲染](../../docs/design/rendering.md#loon)。


三类来源正文编辑共用 `name`、`content`、`content_file`；模板使用 `templates.inline`，正文均与相应 URL／路径互斥。完整字段定义见 [配置规约](../../docs/design/config-schema.md#拉取来源的输入与展示)。模板读取使用 `api.readResource(location)`；节点来源使用 `api.checkSource` 并携带 `kind`，契约见 [来源检查 API](../../docs/design/api.md#post-apisubscriptionscheck)。

`GET /api/config` 展开可读正文供编辑；不可读的 `content_file` 或模板路径保留原引用，编辑器允许修复或删除。保存正文时，后端生成时间戳文件并写入引用。正文目录写入错误使用 `409 resource_content_write_failed`，前端保留草稿与编辑能力。

`SourceEditor` 检查链接来源时先调用 `api.readResource(address)` 获取原文，再携带该原文与 `source_location` 调用 `api.checkSource`；点击“编辑为本地副本”沿用已读取的正文，保留原链接直到用户切换并保存。`api.checkSource` 返回 `recognized` 或 `ambiguous`，过期响应被丢弃，输入变更后清除检查结果；歧义未解决或检查失败时禁用“应用到草稿”（无配置改动或只读查看时按钮显示“完成”直接关闭弹窗）。
