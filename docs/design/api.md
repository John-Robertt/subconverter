# HTTP API 设计

> v1.0 API 文档已归档至 docs/v1.0/design/api.md
>
> 状态提示：本文描述 v2.0 Admin API 当前契约；`/generate`、`/healthz` 与 `/api/*` 均已实现。能力状态以 docs/README.md 状态矩阵和 docs/implementation/progress.md 验收记录为准。

## 目标

本文件定义系统对外暴露的全部 HTTP 接口。v2.0 在原有生成接口基础上，新增后台认证、配置管理、热重载、运行时预览和系统状态接口，为 Web 管理后台提供数据支撑。

---

## 接口概览

| 方法 | 路径 | 用途 | 版本 |
|------|------|------|------|
| GET | `/generate` | 生成并下载目标格式配置 | v1.0 |
| GET | `/healthz` | 进程健康检查 | v1.0 |
| GET | `/api/auth/status` | 查询登录态、setup 与锁定状态 | v2.0 |
| POST | `/api/auth/login` | 管理员密码登录并创建 session | v2.0 |
| POST | `/api/auth/setup` | 首次启动创建管理员账号并登录 | v2.0 |
| POST | `/api/auth/logout` | 注销当前 session | v2.0 |
| GET | `/api/config` | 读取当前配置与 revision | v2.0 |
| PUT | `/api/config` | 条件保存配置（JSON → YAML 写回） | v2.0 |
| POST | `/api/config/validate` | 静态校验配置（Prepare 阶段） | v2.0 |
| POST | `/api/reload` | 触发配置热重载 | v2.0 |
| GET | `/api/preview/nodes` | 预览当前运行时节点列表 | v2.0 |
| POST | `/api/preview/nodes` | 预览草稿配置的节点列表 | v2.0 |
| GET | `/api/preview/groups` | 预览当前运行时分组与服务组 | v2.0 |
| POST | `/api/preview/groups` | 预览草稿配置的分组与服务组 | v2.0 |
| GET | `/api/generate/preview` | 预览当前运行时生成结果（不下载） | v2.0 |
| POST | `/api/generate/preview` | 预览草稿配置生成结果（不下载） | v2.0 |
| GET | `/api/generate/link` | 由服务端生成客户端订阅链接 | v2.0 |
| GET | `/api/status` | 系统状态信息 | v2.0 |

---

## 生成接口（v1.0）

### `GET /generate`

用途：生成目标客户端配置文件。

查询参数：

- `format=clash|surge`（必填）
- `token=<access-token>`（仅当服务端配置了订阅访问 token 且请求没有有效管理员 session 时必填）
- `filename=<custom-name>`（可选；未传时默认 `clash.yaml` / `surge.conf`；仅允许 ASCII 字母、数字、`.`、`-`、`_`，长度不超过 255；未带扩展名时自动补 `.yaml` / `.conf`，已有扩展名必须匹配目标格式）

说明：

- Clash / Surge 等客户端自动更新订阅时通过 query token 访问 `/generate`
- Web 管理后台内的下载按钮可通过同源 `session_id` Cookie 访问 `/generate`，不需要把订阅 token 暴露给前端代码

成功响应：

- Clash Meta：`Content-Type: text/yaml; charset=utf-8`
- Surge：`Content-Type: text/plain; charset=utf-8`
- 两种格式都输出 `Content-Disposition: attachment; ...`

### `GET /healthz`

用途：进程健康检查。

成功响应：HTTP `200`，响应体为 `ok`

---

## 后台认证接口（v2.0）

后台认证独立于 `/generate` 订阅访问 token。所有 `/api/*` 管理接口（除本节认证接口外）都要求有效 `session_id` Cookie。

### `GET /api/auth/status`

用途：查询当前浏览器是否已登录，以及后端是否需要首次 setup。

成功响应：

```json
{
  "authed": false,
  "setup_required": true,
  "setup_token_required": true,
  "locked_until": ""
}
```

字段说明：

- `authed`：当前请求携带的 `session_id` 是否有效
- `setup_required`：auth state 中尚无管理员凭据时为 `true`
- `setup_token_required`：需要首次 setup 且必须提交 bootstrap setup token 时为 `true`
- `locked_until`：登录失败锁定截止时间，未锁定时为空字符串；格式为 ISO 8601

### `POST /api/auth/login`

用途：管理员登录并创建 session。

请求体：

```json
{
  "username": "admin",
  "password": "password",
  "remember": false
}
```

成功响应：

```json
{
  "redirect": "/sources"
}
```

错误响应：

- `400`：请求体无法解析或字段缺失
- `401 invalid_credentials`：用户名或密码错误；响应可携带 `remaining`
- `423 auth_locked`：失败次数达到上限；响应携带 `until`

登录成功后服务端设置 `session_id` Cookie：HttpOnly、SameSite=Lax、Path=/；HTTPS 下必须设置 Secure。未选择 `remember` 时 session 最长 24 小时，选择后最长 7 天。

密码校验成功后，若 auth state 中的密码哈希参数低于当前推荐参数，服务端应在本次登录成功路径中重新计算并原子写回新哈希。

### `POST /api/auth/setup`

用途：首次启动且 auth state 中无管理员凭据时创建管理员并登录。setup 成功后不得再次调用。

首次 setup 必须具备 bootstrap setup token 防抢占边界：

- 若显式配置 `-setup-token` / `SUBCONVERTER_SETUP_TOKEN`，请求体中的 `setup_token` 必须与其匹配
- 若未显式配置 setup token，服务启动且检测到无管理员凭据时必须生成一次性 32-byte URL-safe token，只打印到服务日志，不通过 HTTP 返回
- setup token 只用于首次创建管理员；管理员凭据存在后，`POST /api/auth/setup` 无论 token 是否正确都返回 `409 setup_not_allowed`

请求体：

```json
{
  "username": "admin",
  "password": "password",
  "setup_token": "one-time-bootstrap-token"
}
```

约束：

- 密码至少 12 位
- 服务端只保存密码哈希，不保存明文密码
- 管理员密码使用 `PBKDF2-HMAC-SHA256`，当前参数为 `600000` iterations、32-byte random salt、32-byte derived key；存储格式为 `pbkdf2-sha256$600000$<base64url-salt>$<base64url-hash>`
- 密码哈希比较必须使用 constant-time compare
- session id 必须由 CSPRNG 生成；auth state 只保存 session token 的 SHA-256 哈希，不保存明文 session id
- auth state 目录自动创建时权限为 `0700`，文件权限为 `0600`
- auth state 写入使用同目录临时文件 → 写入 → fsync 文件 → rename → 尽力 fsync 目录；若不可写，返回部署配置错误并保持未初始化状态

错误响应：

- `400`：请求体无法解析、字段缺失或密码不满足策略
- `401 setup_token_required`：需要 setup token 但请求未提供
- `401 setup_token_invalid`：setup token 不匹配
- `409 setup_not_allowed`：管理员凭据已存在
- `409 auth_state_not_writable`：auth state 文件或目录不可写

### `POST /api/auth/logout`

用途：注销当前 session。无论当前 session 是否有效，服务端都应返回成功并清除浏览器 Cookie。

成功响应：

```json
{
  "success": true
}
```

---

## 配置管理接口（v2.0）

### 配置源能力

`-config` 仍支持本地文件路径或 HTTP(S) URL，但两类配置源的管理能力不同：

| 配置源 | `GET /api/config` | `PUT /api/config` | `POST /api/reload` | 说明 |
|--------|-------------------|-------------------|--------------------|------|
| 本地文件 | 支持 | 支持；文件或目录不可写时返回 `409 config_file_not_writable` | 支持 | Web 后台可编辑并写回 YAML 文件 |
| HTTP(S) URL | 支持 | 不支持，返回 `409` | 支持 | 远程配置视为只读，只能重新拉取并热重载 |

状态接口会暴露 `config_source.type`、`config_source.writable` 和 `capabilities.config_write`，前端据此隐藏或禁用保存入口。本地配置文件或所在目录不可写时，`config_source.writable=false` 且 `capabilities.config_write=false`。

### `GET /api/config`

用途：读取当前配置源中的已保存配置（saved config）。

成功响应：

- `200`，`Content-Type: application/json`
- Body：包含当前 YAML 配置的 JSON 表示和内容版本
- `config_revision` 格式为 `sha256:<hex>`，基于配置源原始字节计算
- `config` 中的保序字段（`groups` / `routing` / `rulesets`）以 `[{key, value}]` 数组形式返回

示例：

```json
{
  "config_revision": "sha256:7d9c...",
  "config": {
    "sources": {},
    "groups": [],
    "routing": [],
    "rulesets": []
  }
}
```

说明：

- `GET /api/config` 读取配置源内容，不等同于当前生效的 `RuntimeConfig`
- 本地配置通过 `PUT /api/config` 保存后，若尚未调用 `POST /api/reload`，保存配置与运行时配置可能短暂不同步
- 当前生效配置的加载时间、配置源能力和 dirty 状态通过 `GET /api/status` 查看
- 本地与 HTTP(S) 配置源都会返回 `config_revision`；HTTP(S) 配置源虽然不可写，revision 仍用于前端判断内容是否变化

### `PUT /api/config`

用途：保存修改后的配置。

请求体：

- `Content-Type: application/json`
- Body：`{ "config_revision": "...", "config": { ... } }`
- `config_revision` 必须来自最近一次 `GET /api/config` 或成功 `PUT /api/config` 的响应

处理流程：

1. 确认配置源是本地可写文件
2. 重新读取当前配置文件原始字节，计算当前 `config_revision`
3. 请求中的 `config_revision` 与当前 revision 不一致 → 不写入，返回 `409 config_revision_conflict`
4. JSON 反序列化为 `Config`
5. 执行 `Prepare` 校验
6. 校验通过 → 序列化为 YAML → 写入同目录临时文件 → 原子 rename 覆盖原配置文件 → 重新计算新的 `config_revision` → 同步更新 `app.Service` 内缓存的 `config_revision` → 返回新的 `config_revision`
7. 校验失败 → 不写入，返回 `400` + `ValidateResult`（`valid=false`）

成功响应：

```json
{
  "config_revision": "sha256:9a21..."
}
```

错误响应：

- `400`：缺少 `config_revision`、请求 JSON 无法解析，或校验失败
- `409`：只读配置源、本地文件不可写，或 `config_revision` 与当前文件不一致；响应必须携带可分流的 `error.code`

配置语义校验失败响应体与 `POST /api/config/validate` 复用同一结构，但 HTTP 状态为 `400`：

```json
{
  "valid": false,
  "errors": [],
  "warnings": [],
  "infos": []
}
```

缺少字段、请求 JSON 无法解析、只读配置源、文件不可写和 revision 冲突等非配置语义错误继续使用 `{ "error": { ... } }` 响应体。

409 错误码：

| code | 场景 |
|------|------|
| `config_revision_conflict` | 请求 revision 与当前配置源 revision 不一致 |
| `config_source_readonly` | 当前配置源是 HTTP(S) URL 等只读来源 |
| `config_file_not_writable` | 本地配置文件或所在目录不可写 |

revision 冲突响应示例：

```json
{
  "error": {
    "code": "config_revision_conflict",
    "message": "配置文件已被其他来源修改，请重新读取后再保存",
    "current_config_revision": "sha256:0b31..."
  }
}
```

说明：

- 仅本地文件配置源支持 `PUT /api/config`
- 当 `-config` 是 HTTP(S) URL，返回 `409 config_source_readonly`，不尝试写入
- 当本地文件或所在目录不可写时，返回 `409 config_file_not_writable`，不尝试写入
- `PUT /api/config` 仅写回文件，不自动触发热重载
- 前端需在保存后显式调用 `POST /api/reload` 使新配置生效
- YAML 写回可能改变格式细节并丢失原始文件中的注释
- 本地可写配置源首次保存前，Web UI 必须弹出确认，提示注释、引号和格式风格可能丢失；`PUT /api/config` 成功响应只返回新的 `config_revision`
- `config_revision` 是乐观并发令牌，用于防止旧页面或旧 revision 静默覆盖已观测到的新配置；它不提供外部多写者的线性一致性保证
- 多标签页、外部编辑器或 GitOps 进程在保存前已改写配置时，revision 校验会拒绝陈旧请求；若外部进程恰好在 revision 比对和 `rename` 之间改写文件，当前单用户设计不额外加文件锁或备份

### `POST /api/config/validate`

用途：静态校验配置，不写入文件、不重载。

边界：

- 本接口只执行 JSON 反序列化与 `Prepare` 阶段校验
- 本接口不拉取订阅、不执行 Source / Filter / Group / Route / Target / Render
- 因此它能提前发现字段、正则、URL 基本格式、命名冲突、跨段引用和环路等静态问题
- 它不能发现订阅/Snell/VLESS 来源不可用、远程来源为空、过滤后组为空、目标格式级联过滤后 fallback 清空等生成期问题
- 生成可用性由 `POST /api/preview/nodes`、`POST /api/preview/groups` 和 `POST /api/generate/preview?format=...` 覆盖

请求体：

- `Content-Type: application/json`
- Body：`{ "config": { ... } }`，与 `PUT /api/config` 的 `config` 字段结构一致；校验作用于 `config` 键下的完整配置 JSON

响应语义：

- `200`：请求体形状合法，Body 为校验结果 JSON；配置语义无效时也返回 `200`，但 `valid=false` 且 `errors` 非空
- `400`：请求 JSON 无法解析、缺少 `config` 字段，或 `config` 字段不是对象

有效配置响应示例：

```json
{
  "valid": true,
  "errors": [],
  "warnings": [],
  "infos": []
}
```

无效配置响应示例（HTTP 状态仍为 `200`）：

```json
{
  "valid": false,
  "errors": [
    {
      "severity": "error",
      "code": "invalid_regex",
      "message": "正则表达式无效：...",
      "display_path": "groups.🇭🇰 Hong Kong.match",
      "locator": {
        "section": "groups",
        "key": "🇭🇰 Hong Kong",
        "index": 0,
        "value_path": "match",
        "json_pointer": "/config/groups/0/value/match"
      }
    }
  ],
  "warnings": [],
  "infos": []
}
```

每条诊断项的结构：

```json
{
  "severity": "error",
  "code": "invalid_regex",
  "message": "正则表达式无效：...",
  "display_path": "groups.🇭🇰 Hong Kong.match",
  "locator": {
    "section": "groups",
    "key": "🇭🇰 Hong Kong",
    "index": 0,
    "value_path": "match",
    "json_pointer": "/config/groups/0/value/match"
  }
}
```

- `severity`：`error`（阻塞保存）、`warning`（建议修改）、`info`（提示）
- `code`：稳定错误码，用于前端分类展示和测试断言
- `display_path`：面向用户展示的 YAML 风格路径，不作为程序定位依据
- `locator`：面向前端定位的结构化字段
- `locator.section`：顶层配置段，如 `sources` / `groups` / `routing` / `rulesets` / `rules` / `fallback` / `base_url` / `templates`
- `locator.key`：保序映射项的 key；仅适用于 `groups` / `routing` / `rulesets` 等 key-value 段
- `locator.index`：数组下标；适用于保序映射的数组表示和普通列表
- `locator.value_path`：定位到 value 内部字段，如 `match`、`strategy`、`url`
- `locator.json_pointer`：指向 API JSON 请求体的 JSON Pointer；前端跳转和高亮必须优先使用它

定位规则：

- `groups` / `routing` / `rulesets` 在 API JSON 中是 `[{key,value}]` 数组，因此诊断必须携带 `json_pointer`，不能依赖 `display_path` 反查
- 当保序映射项的 key 包含空格、点号或 emoji 时，前端仍通过 `json_pointer` 和 `index` 定位
- 若错误无法定位到具体字段，`locator` 可只包含 `section`，`json_pointer` 指向最接近的父节点

---

## 热重载接口（v2.0）

### `POST /api/reload`

用途：触发配置热重载，使最新 YAML 文件内容生效。

处理流程：

1. 读取配置源（`LoadConfig`；本地文件或 HTTP(S) URL）
2. 执行 `Prepare` 校验
3. 校验通过 → `WLock` 替换 `RuntimeConfig` 指针，同时将重读主配置源得到的 `config_revision`（`sha256:<hex>`）写入 `runtime_config_revision`，此后 `GET /api/status` 的 `config_dirty` 变为 `false` → 返回 `200`
4. 校验失败 → 不替换，返回 `400` + `ValidateResult` 结构的静态诊断（与 `POST /api/config/validate` 的 Body 结构一致，但 HTTP 状态不同）

成功响应：

```json
{
  "success": true,
  "duration_ms": 12
}
```

边界：reload 只执行 `LoadConfig + Prepare`，不拉取订阅/Snell/VLESS 来源，不执行 Source / Filter / Group / Target / Render，因此不证明生成一定可用。订阅源不可用、过滤后空组、目标格式级联过滤和渲染错误由预览或生成路径发现。

并发行为：同一时刻只允许一次 reload 操作。若 reload 正在执行时收到第二个 `POST /api/reload` 请求，立即返回 `429 Too Many Requests`，不排队等待。429 响应不携带 `Retry-After` header；客户端应短间隔退避后重试，避免循环重放请求。

错误响应：

- `400`：配置校验失败（静态诊断）
- `429`：另一个 reload 正在执行中（不携带 `Retry-After`）
- `502`：远程主配置源拉取失败（仅当 `-config` 为 HTTP(S) URL 时；不表示订阅源拉取失败）

---

## 运行时预览接口（v2.0）

预览接口分为两类：

- `GET /api/preview/*` 与 `GET /api/generate/preview`：读取当前生效的 `RuntimeConfig` 快照，展示运行时状态
- `POST /api/preview/*` 与 `POST /api/generate/preview`：接收草稿配置，执行 `Prepare` 与相应管道阶段，但不写文件、不替换 `RuntimeConfig`，也不改变 `config_dirty` 或 `last_reload`

### `GET /api/preview/nodes`

用途：基于当前运行时配置，拉取并返回全部来源的节点列表（执行管道的 Source + Filter 阶段）。

成功响应：`200`，Body 为节点列表 JSON：

```json
{
  "nodes": [
    {
      "name": "HK-01",
      "type": "ss",
      "kind": "subscription",
      "server": "hk.example.com",
      "port": 8388,
      "filtered": false
    }
  ],
  "total": 42,
  "active_count": 40,
  "filtered_count": 2
}
```

- `kind` 标记来源类型：`subscription` / `snell` / `vless` / `custom`
- `filtered` 标记该节点是否被 `filters.exclude` 排除
- `nodes` 包含 `FilterResult.Included` 与 `FilterResult.Excluded` 的合并视图；生成管道只消费 `Included`
- 节点列表利用现有 TTL 缓存，响应时间受上游影响

### `POST /api/preview/nodes`

用途：基于前端编辑中的草稿配置预览节点列表，响应结构与 `GET /api/preview/nodes` 相同。

注意：此端点会实际拉取草稿配置中的全部订阅 URL。如果草稿包含新增来源且不在 TTL 缓存中，响应时间受上游网络影响（通常 10-30s）。前端应显示"正在拉取订阅..."等明确提示，而非仅显示 spinner。订阅拉取超时继承 `-timeout` 参数（默认 30s）；超时返回 `502`（`FetchError`），与配置校验失败的 `400` 可通过 HTTP 状态码区分。

请求体：

```json
{
  "config": {}
}
```

### `GET /api/preview/groups`

用途：基于当前运行时配置返回分组与服务组匹配结果（执行管道的 Source + Filter + Group + Route + ValidateGraph 阶段）。

成功响应：`200`，Body 为分组结果 JSON：

```json
{
  "node_groups": [
    {
      "name": "🇭🇰 Hong Kong",
      "match": "(港|HK|Hong Kong)",
      "strategy": "select",
      "members": ["HK-01", "HK-02"]
    }
  ],
  "chained_groups": [
    {
      "name": "🔗 HK-ISP",
      "strategy": "select",
      "members": ["HK-01→🔗 HK-ISP", "HK-02→🔗 HK-ISP"]
    }
  ],
  "service_groups": [
    {
      "name": "🚀 快速选择",
      "strategy": "select",
      "members": ["🇭🇰 Hong Kong", "🔗 HK-ISP", "DIRECT"],
      "expanded_members": [
        { "value": "🇭🇰 Hong Kong", "origin": "auto_expanded" },
        { "value": "🔗 HK-ISP", "origin": "auto_expanded" },
        { "value": "DIRECT", "origin": "auto_expanded" }
      ]
    }
  ],
  "all_proxies": ["HK-01", "HK-02", "SG-01"]
}
```

- `service_groups` 展示 Route 阶段产出的服务组结果
- `expanded_members` 用于前端区分用户显式声明、`@auto` 展开和 `@all` 展开结果
- `origin` 可选值：`literal`、`auto_expanded`、`all_expanded`
- `node_groups[].match` 是该地区组在 YAML 中声明的原始正则字符串（透传，不做转义）；`chained_groups` 由 `relay_through` 派生，没有 match 概念，字段省略（JSON 中以 `omitempty` 形式不输出）
- 若 ValidateGraph 发现空节点组、空链式组、非法成员引用或循环引用等图级错误，本接口返回 `400` + 结构化诊断，不返回部分成功的分组结果

### `POST /api/preview/groups`

用途：基于草稿配置返回分组与服务组匹配结果，响应结构与 `GET /api/preview/groups` 相同。

注意：此端点执行完整的 Source + Filter + Group + Route + ValidateGraph 阶段，包括实际拉取草稿配置中的全部订阅 URL。延迟与 `POST /api/preview/nodes` 相同，前端应提供明确的加载反馈。

请求体：

```json
{
  "config": {}
}
```

### `GET /api/generate/preview`

用途：基于当前运行时配置预览生成内容（不触发浏览器下载）。

查询参数：

- `format=clash|surge`（必填）

成功响应：

- `200`
- `Content-Type: text/yaml; charset=utf-8`（Clash）或 `text/plain; charset=utf-8`（Surge）
- **不**设置 `Content-Disposition` header（区别于 `/generate`）
- Body 为完整配置文本，与 `/generate` 内容相同

### `POST /api/generate/preview`

用途：基于草稿配置预览生成内容，不写入配置文件，不替换当前运行时配置。

查询参数：

- `format=clash|surge`（必填）

请求体：

```json
{
  "config": {}
}
```

成功响应与 `GET /api/generate/preview` 相同。

### `GET /api/generate/link`

用途：在已登录后台中，由服务端根据 `base_url`、目标格式、文件名和服务端配置的订阅访问 token 生成客户端订阅链接。前端不得自行持有或拼接 `SUBCONVERTER_TOKEN`。

查询参数：

- `format=clash|surge`（必填）
- `filename=<custom-name>`（可选；校验、自动补扩展名和默认值规则与 `/generate` 相同）
- `include_token=true|false`（可选，默认 `true`；为 `false` 时返回不含 token 的链接）

成功响应：

```json
{
  "url": "https://example.com/generate?format=surge&token=xxx&filename=surge.conf",
  "token_included": true
}
```

说明：

- 本接口要求有效管理员 session
- 若配置未声明 `base_url`，返回 `400 base_url_required`
- 当 `include_token=true` 但服务端未配置订阅访问 token 时，返回不含 token 的链接并设置 `token_included=false`
- UI 在复制 `token_included=true` 的链接前仍必须展示确认，提示 token 会进入 URL、客户端配置和代理日志

---

## 系统状态接口（v2.0）

### `GET /api/status`

用途：返回系统运行状态。

成功响应：`200`，Body 为 JSON：

```json
{
  "version": "2.0.0",
  "commit": "abc1234",
  "build_date": "2026-05-01",
  "config_source": {
    "location": "/config/config.yaml",
    "type": "local",
    "writable": true
  },
  "config_revision": "sha256:9a21...",
  "runtime_config_revision": "sha256:7d9c...",
  "config_loaded_at": "2026-05-01T10:00:00Z",
  "config_dirty": false,
  "capabilities": {
    "config_write": true,
    "reload": true
  },
  "last_reload": {
    "time": "2026-05-01T12:30:00Z",
    "success": true,
    "duration_ms": 15
  },
  "runtime_environment": {
    "listen_addr": ":8080",
    "working_dir": "/app",
    "go_runtime": "go1.24.0 linux/amd64",
    "memory_alloc_mb": "12.3",
    "request_count_24h": 128,
    "uptime_seconds": 3600
  }
}
```

字段说明：

- `config_source.type`：`local` 或 `remote`
- `config_source.writable`：当前配置源是否支持 `PUT /api/config` 且当前本地文件/目录权限允许保存；HTTP(S) 配置源和本地不可写配置均为 `false`
- `config_revision`：配置源当前已保存内容的 revision
- `runtime_config_revision`：当前 `RuntimeConfig` 对应的配置 revision
- `config_dirty`：`config_revision != runtime_config_revision` 时为 `true`
- 本地配置源检测策略：每次 `GET /api/status` 都重新读取配置文件并计算 `sha256:<hex>`，不以 `(mtime, size)` 作为跳过 hash 的强判断；因此同大小改写或保留 mtime 的外部修改也能被下一次 status 请求发现
- HTTP(S) 配置源检测策略：status 不主动拉取远程配置；`config_revision` 与 dirty 状态基于最近一次 `GET /api/config` 或 `POST /api/reload` 观测到的内容
- `capabilities.config_write`：前端是否应启用保存入口；与当前后端实际写回能力保持一致
- `last_reload`：可选字段。仅在进程曾经触发过 `POST /api/reload`（无论成功或失败）时存在；从未发生过 reload 时该字段被省略（`omitempty`）。这避免用 zero value 同时表达"未发生"与"失败"两种语义——前端据此区分"运行中（未重载）"与"上次重载失败"
- `last_reload.error`：仅在 `success=false` 时填充，记录最近一次 reload 失败的错误消息（用于 UI 直接展示原因）；`success=true` 时省略
- `runtime_environment`：当前进程运行环境。`request_count_24h` 当前为进程内请求计数；进程启动未满 24 小时时，前端可按“自启动以来”展示

---

## 错误语义

本节描述当前错误语义。可归因于用户配置的目标格式 fallback 清空错误返回 `400`，内部投影不变量错误继续保持 `500`。

例外：`POST /api/config/validate` 是校验查询接口，请求体合法但配置语义无效时返回 `200 valid=false`；下表中的“配置语义 / 图校验失败”适用于 `PUT /api/config`、`POST /api/reload`、预览和生成路径。

| 状态码 | 场景 |
|------|------|
| `400` | 请求参数非法；配置语义 / 图校验失败；热重载校验失败；目标格式级联过滤导致 fallback 清空 |
| `401` | `/generate` 缺少或不匹配订阅访问 token；管理接口缺少、无效或已过期的管理员 session；登录凭据错误 |
| `409` | 当前配置源不支持该操作、本地配置文件不可写，或配置 revision 冲突 |
| `429` | reload 操作正在执行中，拒绝并发 reload 请求 |
| `502` | 远程资源拉取失败，或远程订阅内容不可用 |
| `500` | 本地资源读取失败、内部投影不变量失败，或内部处理 / 渲染失败 |

错误响应格式：

- `/generate`：`text/plain; charset=utf-8`，中文纯文本
- `/api/*`：`application/json`，结构化 JSON（含 `error` / `errors` 字段）；单错误响应的 `error.code` 必须稳定可测试，供前端区分 409 等同状态码下的不同行为

响应缓存头：

- `/api/*` 管理响应默认设置 `Cache-Control: no-store`，包括认证状态、配置读取、校验、预览、订阅链接和系统状态接口
- `/api/generate/preview` 返回完整配置文本，必须设置 `Cache-Control: no-store`
- `/generate` 返回可下载配置文本，必须设置 `Cache-Control: no-store`
- Web 静态资源由 Go 服务分路径控制缓存：`index.html` 使用 `Cache-Control: no-cache` 或等价重验证策略；带 hash 的 Vite 静态资源可使用长期缓存（如 `public, max-age=31536000, immutable`）

---

## 鉴权

鉴权分为两条互不替代的边界：

- 管理后台和 `/api/*`：使用管理员用户名密码登录，登录成功后依赖 `session_id` Cookie
- `/generate` 客户端订阅更新：继续支持 `token=...` query 参数，兼容 Clash / Surge 等客户端

管理接口规则：

- `/api/auth/status`、`/api/auth/login`、`/api/auth/setup`、`/api/auth/logout` 是未登录可访问的认证入口；logout 无 session 时也返回成功并清 Cookie
- 其他 `/api/*` 管理接口必须携带有效 `session_id` Cookie；不接受 `Authorization: Bearer <SUBCONVERTER_TOKEN>` 作为管理后台权限
- 前端 API client 使用同源 Cookie，不把订阅访问 token 放入 `/api/*` query 或 header
- Session 失效后，所有受保护管理接口返回 `401 session_expired` 或 `401 auth_required`，前端全局拦截并跳转 `/login?next=<当前路径>`
- 失败登录按 IP + 用户名联合计数；第 5 次失败后返回 `423 auth_locked`，默认锁定 15 分钟，具体截止时间由后端返回
- 所有非安全 `/api/*` 请求必须先校验同源 `Origin` 或 `Referer`，包括未登录可访问的 `/api/auth/login`、`/api/auth/setup` 和 `/api/auth/logout`；这些认证入口不要求已有 session，但仍受同源校验约束

`/generate` 规则：

- `/generate` 保持 v1.0 兼容：服务端未配置订阅访问 token 时（`-access-token` 为空），外部客户端可无 token 访问
- 服务端配置订阅访问 token 时，外部客户端必须通过 `token=...` query 访问
- Web 管理后台内的下载按钮可以凭当前管理员 session 调用 `/generate`，无需向前端暴露订阅访问 token
- 前端复制订阅链接时，必须通过 `GET /api/generate/link` 让服务端生成 URL，并在复制含 token 的链接前显式确认

管理接口常见 401 / 423 错误码：

| code | 场景 |
|------|------|
| `auth_required` | 请求缺少管理员 session |
| `session_expired` | session 不存在、已过期或已被注销 |
| `invalid_credentials` | 登录用户名或密码错误 |
| `auth_locked` | 登录失败次数达到上限，临时锁定 |
| `setup_token_required` | 首次 setup 缺少 bootstrap setup token |
| `setup_token_invalid` | 首次 setup 的 bootstrap setup token 不匹配 |

Cookie session 下，所有会修改状态的管理请求必须校验同源 `Origin` 或 `Referer`；生产模式由同一个 Go 服务同源托管 SPA 与 API，开发模式优先使用 Vite proxy。

---

## CORS

- v2.0 正式生产 Docker Compose 模式不启用 CORS：浏览器访问单个 `subconverter` 服务，同源访问 SPA、`/api/*`、`/generate` 和 `/healthz`
- 开发模式优先使用 Vite proxy，以便 Cookie session 保持同源语义；仅在不使用 proxy、需要浏览器跨域直连 Go 后端调试非 Cookie 路径时，通过 `-cors` 标志或 `SUBCONVERTER_CORS=true` 启用
- 启用后仅允许本机开发来源：`http://localhost:*`、`http://127.0.0.1:*`、`http://[::1]:*`
- CORS middleware 必须处理 preflight：允许 `GET`、`POST`、`PUT`、`OPTIONS`；允许 `Content-Type`
- 响应需设置 `Vary: Origin`，避免不同来源的缓存污染

---

## 运行参数

系统支持以下启动参数：

- `-config`：YAML 配置文件路径或 HTTP(S) URL（必填；HTTP(S) URL 为只读配置源）
- `-listen`：HTTP 监听地址（默认 `:8080`）
- `-cache-ttl`：订阅、模板和远程配置的缓存 TTL（默认 `5m`）
- `-timeout`：拉取订阅的 HTTP 超时时间（默认 `30s`）
- `-access-token`：订阅访问 token，只用于 `/generate` 客户端自动更新订阅；空值表示 `/generate` 外部客户端不启用 token 鉴权
- `-auth-state`：管理员凭据与持久 session 状态文件路径。Docker Compose Web 后台部署应挂载到可写目录；若无凭据且 auth state 不可写，setup 不可完成，管理接口保持关闭
- `-setup-token`：首次 setup bootstrap token；未配置且 auth state 无管理员凭据时，服务启动生成一次性 32-byte URL-safe token 并仅打印到日志
- `-cors`：启用 CORS 中间件（默认 `false`，仅开发模式使用）
- `-healthcheck`：健康检查模式
- `-version`：打印版本信息

环境变量：`SUBCONVERTER_LISTEN`、`SUBCONVERTER_TOKEN`、`SUBCONVERTER_AUTH_STATE`、`SUBCONVERTER_SETUP_TOKEN`、`SUBCONVERTER_CORS`

---

## 版本化策略

当前不使用 API 版本前缀（如 `/api/v1/`）。理由：

- 单用户工具，前端与后端版本绑定（同一 Docker Compose 声明同一 release tag）
- API 消费方仅有自有前端 SPA，无第三方集成

若未来需要破坏性变更（如 OrderedMap JSON 格式调整），采用以下策略：

- 在 Release Notes 中标注 breaking change
- 前端与后端同版本发布，确保兼容
- 不引入多版本共存——直接修改端点行为，旧版本不保留

---

## 请求处理流程

### `/generate` 流程

1. 校验 `format`
2. 若请求携带有效管理员 session，允许后台下载；否则在服务端配置订阅访问 token 时校验 query `token`
3. 校验并规范化 `filename`
4. 通过 `app.Service` 获取当前 `RuntimeConfig` 快照（内部 `RLock` 复制指针后立即释放）
5. 将快照传入 `generate.Generate(ctx, cfg, req)`（无状态调用）执行 `Build → Target → Render`
6. 若 `format=surge` 且有 `base_url`，组装 managed URL；请求鉴权方式只决定是否允许下载，managed URL 中的 token 始终来自服务端订阅访问 token（若启用），不依赖当前请求是否携带 query token 或管理员 session；`filename` 使用本次请求规范化后的最终文件名
7. 返回配置文本（带 `Content-Disposition`）

### `/api/auth/*` 流程

- status：读取 auth state 与当前 Cookie session，返回登录态、setup、setup token 要求和锁定状态
- login：校验账号密码 → 成功后创建 session 并写入 Cookie → 失败时更新失败计数
- setup：确认 auth state 尚无管理员凭据 → 校验 setup token → 校验密码策略 → 写入 PBKDF2 密码哈希 → 创建 session
- logout：删除当前 session 并清除 Cookie

### `/api/config` 流程

- GET：读取配置源中的已保存 YAML → 解析为 JSON → 返回
- PUT：确认配置源可写 → 接收 JSON → 反序列化 → Prepare 校验 → 原子写回 YAML

所有非安全 `/api/*` 请求进入业务 handler 前，都先校验同源 `Origin` / `Referer`；受保护管理接口随后校验 `session_id` Cookie。安全方法只校验各自需要的 session 语义，例如受保护 `GET /api/config` 要求有效 session，而未登录可访问的 `GET /api/auth/status` 只读取当前 Cookie session 状态。

### `/api/reload` 流程

1. 重新加载配置源（`LoadConfig`；远程主配置 URL 必须 bypass / invalidate 缓存）
2. 执行 `Prepare`
3. 成功 → `WLock` 替换 `RuntimeConfig` 与 `runtime_config_revision` → 返回
4. 失败 → 返回错误

### `GET /api/preview/*` 流程

1. `RLock` 读取当前 `RuntimeConfig` 指针快照并立即释放锁
2. 使用快照执行管道部分阶段（nodes: Source+Filter；groups: Source+Filter+Group+Route+ValidateGraph）
3. 返回 JSON

### `POST /api/preview/*` 流程

1. 接收 `{ "config": ... }` 草稿配置
2. 执行 `Prepare` 得到临时 `RuntimeConfig`
3. 使用临时配置执行对应预览阶段
4. 返回 JSON，不写文件，不替换服务当前配置
