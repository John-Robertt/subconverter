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
- `POST /api/config/validate`
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

- `config_revision` 表示配置源当前已保存内容。
- `runtime_config_revision` 表示当前 `RuntimeConfig` 对应的配置。
- `config_dirty = config_revision != runtime_config_revision`。
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
- `sources.fetch_order` 保存 `subscriptions` / `snell` / `vless` 的拉取顺序，写回时必须保留。
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

- 节点预览需要展示 Kind、来源、过滤状态和格式限定。
- 分组预览需要展示节点组、链式组、服务组和展开成员。
- `expanded_members` 需要区分用户显式声明、`@auto` 展开和 `@all` 展开。

## 订阅链接结果

`GET /api/generate/link` 返回：

- `url`：可复制给 Clash Meta 或 Surge 客户端的完整订阅链接。
- `token_included`：链接是否包含订阅访问 token。

当前端复制 `token_included=true` 的链接时，必须先展示确认。

## HTTP 状态

前端必须单独处理：

- `400`：请求或配置语义错误。
- `401`：管理接口缺少、无效或已过期的 session；登录凭据错误；或 `/generate` 缺少 / 不匹配订阅访问 token。
- `409`：按 `error.code` 分流处理。
- `429`：reload 正在执行，前端短间隔退避重试。
- `502`：远程主配置源或订阅等上游拉取失败，按接口上下文展示。

API client 归一化错误对象至少包含：

- `status`
- `code`
- `message`
- `details`

409 的前端行为：

- `config_revision_conflict`：保留草稿，提示外部修改，提供重新加载配置或手动合并入口。
- `config_source_readonly`：进入只读模式，禁用保存、新增、删除和排序。
- `config_file_not_writable`：展示文件权限或部署挂载问题，并保留草稿。
- 未知 409 code：按未知保存失败处理，不覆盖草稿，展示可重试或查看详情入口。

认证错误前端行为：

- `auth_required` / `session_expired`：跳转 `/login?next=<当前路径>`。
- `invalid_credentials`：登录页展示用户名或密码错误，并显示剩余尝试次数。
- `auth_locked`：登录页展示锁定截止时间并禁用提交。
- `setup_token_required` / `setup_token_invalid`：setup 模式展示 bootstrap token 错误，不创建管理员凭据。
- `/generate` 不返回 JSON `error.code`；缺少或错误订阅 token 时返回 `401 text/plain`，只表示客户端订阅访问 token 问题，不触发额外的后台凭据输入。
