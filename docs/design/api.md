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
| GET | `/api/config/effective.yaml` | 下载当前生效的源 YAML 配置 | v2.0 |
| GET | `/api/config/saved.zip` | 下载已保存的配置文件与正文模板包 | v2.0 |
| GET | `/api/config/effective.zip` | 下载当前生效的源 YAML 配置与模板包 | v2.0 |
| POST | `/api/subscriptions/check` | 检查订阅格式、节点数和适用输出 | v2.0 |
| POST | `/api/resources/read` | 读取节点订阅或模板正文为编辑副本 | v2.0 |
| POST | `/api/config/import` | 导入 YAML 或配置模板 ZIP 包为草稿配置 JSON | v2.0 |
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

- `format=clash|surge|loon`（必填）
- `token=<access-token>`（仅当服务端配置了订阅访问 token 且请求没有有效管理员 session 时必填）
- `filename=<custom-name>`（可选；未传时默认 `clash.yaml` / `surge.conf` / `loon.lcf`；仅允许 ASCII 字母、数字、`.`、`-`、`_`，长度不超过 255；未带扩展名时自动补 `.yaml` / `.conf` / `.lcf`，已有扩展名必须匹配目标格式）

说明：

- Clash / Surge / Loon 等客户端自动更新订阅时通过 query token 访问 `/generate`
- Web 管理后台内的下载按钮可通过同源 `session_id` Cookie 访问 `/generate`，不需要把订阅 token 暴露给前端代码

成功响应：

- Clash Meta：`Content-Type: text/yaml; charset=utf-8`
- Surge / Loon：`Content-Type: text/plain; charset=utf-8`
- 三种格式都输出 `Content-Disposition: attachment; ...`

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
| 本地路径尚无文件 | 返回空草稿与 `missing` 版本 | 父目录存在且可写时允许首次创建 | 保存后支持 | 后台先运行，导入或编辑后保存，再热重载 |
| 本地文件 | 支持 | 支持；文件或目录不可写时返回 `409 config_file_not_writable` | 支持 | Web 后台可编辑并写回 YAML 文件 |
| HTTP(S) URL | 支持 | 不支持，返回 `409` | 支持 | 远程配置视为只读，只能重新拉取并热重载 |

状态接口会暴露 `config_source.type`、`config_source.writable` 和 `capabilities.config_write`，前端据此隐藏或禁用保存入口。本地配置文件或所在目录不可写时，`config_source.writable=false` 且 `capabilities.config_write=false`。

### 首次配置

本地主配置路径尚无文件时，服务启动受管理员认证保护的后台，`/healthz` 返回健康；已有文件的 YAML 或语义错误、远程配置读取失败仍阻止启动。父目录需要已存在并可写，服务才允许首次保存；目录不可写时，状态和空草稿仍可读取，保存返回 `409 config_file_not_writable`。

管理员完成 setup 或登录后，可导入 YAML / ZIP 或手动编辑草稿。导入只返回内存草稿，不创建主配置或正文文件；空草稿必须补齐并通过校验才能保存。首次保存直接创建文件，不展示已有文件覆盖的格式丢失确认。保存后需要显式热重载，才启用当前配置的生成、预览和订阅链接。

未创建主配置时，相关状态字段如下：

```json
{
  "config_source": {"location":"/config/config.yaml","type":"local","exists":false,"writable":true},
  "config_revision":"missing",
  "runtime_config_revision":"",
  "config_loaded_at":"",
  "config_dirty":false,
  "capabilities":{"config_write":true,"reload":false}
}
```

`missing` 是文件缺失的专用版本标记，与空文件的 SHA-256 不同。首次保存携带读取到的该标记，按排他方式发布主配置；如果其他请求或外部进程已创建文件，返回 `409 config_revision_conflict`，保留现有文件并回收本次新增正文版本。已有文件的保存继续使用原版本校验和原子替换。

尚无生效配置时，运行时预览、生成、订阅链接和生效配置导出返回 `409 config_not_initialized`；未创建主配置时，已保存配置导出也返回该错误。草稿校验、来源检查和草稿预览仍可使用。

### `GET /api/config`

用途：读取当前配置源中的已保存配置（saved config）。

成功响应：

- `200`，`Content-Type: application/json`
- Body：包含当前 YAML 配置的 JSON 表示和内容版本
- 已有配置的 `config_revision` 为原始字节的 `sha256:<hex>`；本地文件不存在时为 `missing`，同时返回空草稿
- `config` 中的保序字段（`groups` / `routing` / `rulesets`）以 `[{key, value}]` 数组形式返回

示例：

```json
{
  "config_revision": "sha256:7d9c...",
  "config": {
    "sources": {"subscriptions": [{"name": "主订阅", "url": "https://example.com/sub.conf", "format": "loon"}]},
    "templates": {"loon": "configs/base_loon.lcf"},
    "groups": [],
    "routing": [],
    "rulesets": []
  }
}
```

说明：

- 三类来源的 `name`、订阅的 `sources.subscriptions[].format` 和 `templates.loon` 随 JSON/YAML 往返保留；字段约束见 [配置规约](config-schema.md#subscriptions)
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

1. 确认配置源是本地可写文件，或父目录可写的待创建路径
2. 读取当前配置文件和版本；文件缺失使用 `missing`
3. 请求中的 `config_revision` 与当前 revision 不一致 → 不写入，返回 `409 config_revision_conflict`
4. JSON 反序列化为 `Config`
5. 执行 `Prepare` 校验
6. 校验通过后复用未变正文，仅为新增或变化的正文生成版本文件
7. 序列化为 YAML；字节相同则返回原 revision；首次创建以同目录临时文件排他发布，已有文件以原子 rename 替换，返回保存后的 `config_revision`
8. 校验失败 → 不写入，返回 `400` + `ValidateResult`（`valid=false`）

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
- `PUT /api/config` 仅写回文件，不自动触发热重载；配置和正文均未变化时不重写文件，返回原 `config_revision`
- 前端需在保存后显式调用 `POST /api/reload` 使新配置生效
- YAML 写回可能改变格式细节并丢失原始文件中的注释
- 首次覆写已有本地配置前，Web UI 弹出确认，提示注释、引号和格式风格可能丢失；首次创建直接保存；`PUT /api/config` 成功响应只返回保存后的 `config_revision`（未变化时保留原值）
- `config_revision` 是乐观并发令牌，用于防止旧页面或旧 revision 静默覆盖已观测到的新配置；它不提供外部多写者的线性一致性保证
- 多标签页、外部编辑器或 GitOps 进程在保存前已改写配置时，revision 校验会拒绝陈旧请求；若外部进程恰好在 revision 比对和 `rename` 之间改写文件，当前单用户设计不额外加文件锁或备份

### `GET /api/config/effective.yaml`

用途：下载当前正在生效的源 YAML 配置。

成功响应：

- `200`，`Content-Type: application/x-yaml; charset=utf-8`
- `Content-Disposition: attachment; filename="config.yaml"; filename*=UTF-8''config.yaml`
- Body：上一次成功启动或 `POST /api/reload` 时进入 `RuntimeConfig` 的原始 YAML 字节

说明：

- 本接口返回“正在生效”的配置，而不是当前前端草稿
- 若 `PUT /api/config` 已保存但尚未 reload，导出的仍是旧的生效 YAML
- HTTP(S) 远程配置源同样导出上一次成功加载到运行时的远程 YAML 内容
- 本接口不读取当前配置源、不写文件、不改变 `config_dirty`

### `GET /api/config/saved.zip`

用途：导出当前配置源已保存的配置，包含 Web 编辑后通过 `PUT /api/config` 保存的正文。需要管理员会话；本地与远程只读配置源均可导出。

响应类型、文件名和包结构与 `GET /api/config/effective.zip` 相同。接口读取当前配置源，不导出浏览器中尚未保存的草稿；保存后无需热重载即可导出新内容。任一正文文件或模板读取失败时，整个导出失败。导出不写文件、不替换运行时配置。

### `GET /api/config/effective.zip`

用途：下载当前正在生效的源 YAML 配置、本地订阅正文文件与全部已配置底版模板。

成功响应：

- `200`，`Content-Type: application/zip`
- `Content-Disposition: attachment; filename="subconverter-config.zip"; filename*=UTF-8''subconverter-config.zip`
- ZIP 内容：
  - `config.yaml`：当前生效配置的可迁移副本；本地订阅使用包内相对 `content_file` 引用，模板引用改为包内相对路径，不保留服务器绝对路径或订阅正文字符串；远程订阅 URL 保持原样，不下载为副本
  - `subscriptions/<序号>.txt`：所有格式的本地订阅正文均使用 `.txt`，序号为该条来源在 `sources.subscriptions` 中的 1-based 位置。正文逐字保留，订阅名称单独保存在 `name` 中；解析格式保存在 `format` 中，包括依赖原文件后缀识别的格式
  - `snell/<序号>.txt`、`vless/<序号>.txt`：两类本地节点池的正文，序号为各自列表中的 1-based 位置；主配置的对应来源使用相对 `content_file` 引用，保留 `name`
  - `templates/clash.yaml`：Clash 底版正文（引用模板或本地编辑正文配置后存在）
  - `templates/surge.conf`：Surge 底版正文（引用模板或本地编辑正文配置后存在）
  - `templates/loon.lcf`：Loon 底版正文（引用模板或本地编辑正文配置后存在）

说明：

- 本接口返回“正在生效”的配置包，而不是当前前端草稿
- 若 `PUT /api/config` 已保存但尚未 reload，ZIP 中 `config.yaml` 仍是旧的生效 YAML
- 模板内容按当前生效模板位置读取；远程模板走现有 fetch/cache，本地模板走本地资源读取
- 任一已配置模板读取失败时，整个导出失败，不返回不完整 ZIP
- 本接口不写文件、不改变 `config_dirty`

例如，本地订阅导出后在 `config.yaml` 中表示为：

```yaml
sources:
  subscriptions:
    - name: 手动节点
      content_file: subscriptions/1.txt
      format: text
```

解压后以解压目录作为工作目录启动服务，即可读取这些相对路径。

### `POST /api/config/import`

用途：导入 YAML 或配置模板 ZIP 包为前端草稿 JSON。

YAML 请求体：

- `Content-Type: application/json`
- Body：`{ "yaml": "sources: ...\n" }`

ZIP 请求体：

- `Content-Type: application/zip`
- Body：ZIP 字节，大小不超过 10 MiB
- ZIP 必须包含非空 `config.yaml`
- 可选包含 `templates/clash.yaml`、`templates/surge.conf` 和 `templates/loon.lcf`

成功响应：

```json
{
  "config": {
    "sources": {},
    "groups": [],
    "routing": [],
    "rulesets": []
  }
}
```

说明：

- `config` 的 JSON 形状与 `GET /api/config` 相同，保序字段仍以 `[{key, value}]` 数组表示
- YAML 导入解析配置并读取已有的本地正文引用以还原编辑内容；不写文件、不替换运行时配置
- ZIP 导入将 `subscriptions`、`snell`、`vless` 中 `content_file` 引用的包内正文文件还原为对应来源的 `content`，保留 `name` 及订阅的 `format`，清除文件引用；包内模板还原为 `templates.inline` 草稿正文，清除对应模板路径
- 导入不创建目录、不写任何文件。用户编辑草稿后，通过 `PUT /api/config` 统一校验并保存配置与正文文件；保存失败由保存流程回收本次新增文件
- 既有 `.imports` 模板路径继续作为普通本地引用读取，新导入不再创建此目录
- ZIP 导入只读取 `config.yaml`、上述固定模板条目及 `subscriptions/`、`snell/`、`vless/` 下的直接文件，不按 ZIP 内路径解压，也不读取服务器上的同名文件。各来源正文必须通过 `content_file` 引用包内同类目录的文件，内容须为不含 NUL 的非空 UTF-8 文本；缺失或无效文件、目录外引用、重复条目及无效 ZIP 返回 `400 invalid_archive`。压缩包和所读取条目的解压总大小均不能超过 10 MiB
- HTTP(S) 远程配置源导入 ZIP 返回 `409 config_source_readonly`；本地配置文件不可写返回 `409 config_file_not_writable`
- 导入成功后，前端应只替换草稿；用户仍需显式保存和热重载才会生效

### `POST /api/config/validate`

用途：静态校验配置，不写入文件、不重载。

边界：

- 本接口只执行 JSON 反序列化、Admin API JSON 层校验（如 `sources.fetch_order`）与 `Prepare` 阶段校验
- 本接口不拉取订阅、不执行 Source / Filter / Group / Route / Target / Render
- 因此它能提前发现字段、正则、URL 基本格式、命名冲突、跨段引用和环路等静态问题
- 它不能发现订阅（TXT / Clash YAML / Surge CONF / Loon）/Snell/VLESS 来源不可用、远程来源为空、过滤后组为空、目标格式级联过滤后 fallback 清空等生成期问题
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

1. 读取并解析本地或远程 YAML；远程主配置先清除对应缓存项，再读取
2. 执行应用层检查和 `Prepare`（不消费 `sources.fetch_order` 等 Admin API JSON-only 字段）；读取、解析和校验均在配置写锁外完成
3. 校验通过 → 短暂加写锁，一起替换 `RuntimeConfig`、生效 YAML、revision 和重载状态 → 返回 `200`。若配置源随后没有被改动，`GET /api/status` 的 `config_dirty` 为 `false`
4. 校验失败 → 不替换，返回 `400` + `ValidateResult` 结构的静态诊断（与 `POST /api/config/validate` 的 Body 结构一致，但 HTTP 状态不同）

成功响应：

```json
{
  "success": true,
  "duration_ms": 12
}
```

边界：reload 只执行 `LoadConfig + Prepare`，不拉取订阅（TXT / Clash YAML / Surge CONF / Loon）/Snell/VLESS 来源，不执行 Source / Filter / Group / Target / Render，因此不证明生成一定可用。订阅源不可用、过滤后空组、目标格式级联过滤和渲染错误由预览或生成路径发现。

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
      "id": "proxy-1",
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
- `id` 是本次 Build 内的稳定调试标识；不会跨 reload / 配置 revision 持久化
- `name` 是显示名候选，允许重复
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
      "members": ["HK-01", "HK-02"],
      "member_details": [
        { "kind": "proxy", "id": "proxy-1", "name": "HK-01" },
        { "kind": "proxy", "id": "proxy-2", "name": "HK-02" }
      ]
    }
  ],
  "chained_groups": [
    {
      "name": "🔗 HK-ISP",
      "strategy": "select",
      "members": ["HK-01→🔗 HK-ISP", "HK-02→🔗 HK-ISP"],
      "member_details": [
        { "kind": "proxy", "id": "proxy-4", "name": "HK-01→🔗 HK-ISP" },
        { "kind": "proxy", "id": "proxy-5", "name": "HK-02→🔗 HK-ISP" }
      ]
    }
  ],
  "service_groups": [
    {
      "name": "🚀 快速选择",
      "strategy": "select",
      "members": ["🇭🇰 Hong Kong", "🔗 HK-ISP", "DIRECT"],
      "member_details": [
        { "kind": "node_group", "name": "🇭🇰 Hong Kong" },
        { "kind": "node_group", "name": "🔗 HK-ISP" },
        { "kind": "reserved", "name": "DIRECT" }
      ],
      "expanded_members": [
        { "value": "🇭🇰 Hong Kong", "origin": "auto_expanded" },
        { "value": "🔗 HK-ISP", "origin": "auto_expanded" },
        { "value": "DIRECT", "origin": "auto_expanded" }
      ],
      "expanded_member_details": [
        { "kind": "node_group", "name": "🇭🇰 Hong Kong", "origin": "auto_expanded" },
        { "kind": "node_group", "name": "🔗 HK-ISP", "origin": "auto_expanded" },
        { "kind": "reserved", "name": "DIRECT", "origin": "auto_expanded" }
      ]
    }
  ],
  "all_proxies": ["HK-01", "HK-02", "SG-01"],
  "all_proxy_details": [
    { "kind": "proxy", "id": "proxy-1", "name": "HK-01" },
    { "kind": "proxy", "id": "proxy-2", "name": "HK-02" },
    { "kind": "proxy", "id": "proxy-3", "name": "SG-01" }
  ]
}
```

- `service_groups` 展示 Route 阶段产出的服务组结果
- `members`、`expanded_members` 和 `all_proxies` 保留 v2.0 已有字符串/`{value, origin}` 结构，避免破坏既有调用方
- `member_details`、`expanded_member_details` 和 `all_proxy_details` 提供包含代理 ID 与引用种类的富身份视图
- `expanded_member_details` 用于前端区分用户显式声明、`@auto` 展开和 `@all` 展开结果
- `origin` 可选值：`literal`、`auto_expanded`、`all_expanded`
- 成员 `kind` 区分 `proxy`、`node_group`、`route_group`、`reserved`；只有代理成员携带 Build 内 `id`
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

- `format=clash|surge|loon`（必填）

成功响应：

- `200`
- `Content-Type: text/yaml; charset=utf-8`（Clash）或 `text/plain; charset=utf-8`（Surge）
- **不**设置 `Content-Disposition` header（区别于 `/generate`）
- Body 为完整配置文本，与 `/generate` 内容相同

### `POST /api/generate/preview`

用途：基于草稿配置预览生成内容，不写入配置文件，不替换当前运行时配置。

查询参数：

- `format=clash|surge|loon`（必填）

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

- `format=clash|surge|loon`（必填）
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
    "writable": true,
    "exists": true
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
- `config_source.exists`：本地为当前主配置文件是否存在；远程在成功加载后为 `true`，status 不主动探测远程 URL
- `config_source.writable`：当前配置源是否支持保存或首次创建；已有文件检查文件和父目录权限，缺失文件检查已存在的父目录，HTTP(S) 配置源始终为 `false`
- `config_revision`：当前已保存内容的 revision，本地文件缺失时为 `missing`
- `runtime_config_revision`：当前 `RuntimeConfig` 对应的配置 revision
- `config_dirty`：尚未保存且没有运行快照时为 `false`；其余状态按保存版本与运行版本是否不同判断
- 本地配置源检测策略：每次 `GET /api/status` 都重新读取配置文件并计算 `sha256:<hex>`，不以 `(mtime, size)` 作为跳过 hash 的强判断；因此同大小改写或保留 mtime 的外部修改也能被下一次 status 请求发现
- HTTP(S) 配置源检测策略：status 不主动拉取远程配置；`config_revision` 与 dirty 状态基于最近一次 `GET /api/config` 或 `POST /api/reload` 观测到的内容
- `capabilities.reload`：主配置存在时为 `true`；首次创建前为 `false`
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
| `409` | 配置尚未初始化或生效、当前配置源不支持该操作、本地配置文件不可写，或配置 revision 冲突 |
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
- 失败登录按 IP + 用户名联合计数，最近失败时间随认证状态持久化；连续 15 分钟没有新失败则清零，服务重启保留尚未过期的计数。第 5 次失败返回 `423 auth_locked` 并锁定 15 分钟，截止时间由后端返回；锁定期间的请求不延长锁定，锁定到期或同一 IP + 用户名成功登录后清零
- 读取认证状态时忽略已过期的失败记录，并在下一次写入时清理。没有失败时间的未锁定记录不继续累计；已有且未到期的锁定仍保持到原截止时间
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

流程与错误响应见本文 [热重载接口](#post-apireload)；Go 层的配置锁、重载互斥和保存边界见 [app.Service 并发安全](app-service.md#并发安全)。

### `GET /api/preview/*` 流程

1. `RLock` 读取当前 `RuntimeConfig` 指针快照并立即释放锁
2. 使用快照执行管道部分阶段（nodes: Source+Filter；groups: Source+Filter+Group+Route+ValidateGraph）
3. 返回 JSON

AnyTLS 节点从 `sources.subscriptions` 进入预览结果，`kind` 为 `subscription`，`type` 为 `anytls`。

### `POST /api/preview/*` 流程

1. 接收 `{ "config": ... }` 草稿配置
2. 执行 `Prepare` 得到临时 `RuntimeConfig`
3. 使用临时配置执行对应预览阶段
4. 返回 JSON，不写文件，不替换服务当前配置


## POST /api/resources/read

管理员读取现有订阅 URL 或模板路径后，在 Web 编辑副本。此接口要求有效 session 并执行同源检查；只读取，不保存配置、不写回资源，也不 reload。与已有模板引用相同，路径相对于服务进程工作目录，URL 使用共享 Fetcher 缓存。

请求：

```json
{"location":"https://example.com/nodes.lcf"}
```

响应 `200`：

```json
{"content":"[Proxy]\nHK = anytls, hk.example.com, 443, \"example-password\"\n","format":"loon"}
```

空 location 返回 `400 invalid_request`，非 UTF-8 或含 NUL 的资源返回 `400 invalid_resource`。远程和本地读取失败沿用既有 FetchError / ResourceError 映射。Web 在只读配置源下仍允许读取和查看正文，禁用编辑副本与保存；保存始终遵循 `PUT /api/config` 的可写性契约。

`format` 可选，表示按 Source 的自动识别规则会采用的结构化订阅格式，取 `clash`、`surge` 或 `loon`；TXT/Base64 不返回此字段。该提示仅表示旧版 Source 自动规则的选择，不表示节点检查成功。模板和订阅编辑器都使用此接口自动读取原文；订阅随后使用下述专用检查接口，格式提示不替代检查结果。

`GET /api/config` 和 `PUT /api/config` 的配置 JSON 支持三类来源的 `name`、`content`、`content_file` 与 `templates.inline`，字段及互斥约束见 [配置规约](config-schema.md#拉取来源的输入与展示)。例如：

```json
{
  "sources": {"subscriptions": [{"name":"手动 Loon 订阅", "format":"loon", "content":"HK = anytls, hk.example.com, 443, \"example-password\""}]},
  "templates": {"inline": {"loon":"[General]\ndns-server = system\n"}}
}
```

保存时复用内容未变的服务正文文件；仅为新增或变化的正文生成带时间戳的文件，默认直接放在 `/config`，主配置保存文件引用；`GET /api/config` 还原可读正文。单个文件读取失败时保留原引用，配置编辑仍可使用；ZIP 导出保持严格检查，避免生成缺少正文的迁移包。存储规则与文件名示例见 [配置规约](config-schema.md#templates)。

正文目录无法创建或写入时，`PUT /api/config` 返回 `409`：

```json
{"error":{"code":"resource_content_write_failed","message":"正文保存失败：目录 \"/config\" 无法写入。请检查目录权限、容器挂载和运行用户；当前草稿已保留。原因：permission denied"}}
```

失败不修改主配置和运行快照；修复目录权限后可用同一 revision 重试。此错误不改变主配置的 `config_source.writable` 或 `capabilities.config_write`；主配置自身只读时仍遵守原有 `config_file_not_writable` / `config_source_readonly` 契约。

配置包的正文文件及引用方式见[导出配置 API](#get-apiconfigeffectivezip)。YAML 导入会读取其中已有的本地正文引用以还原编辑内容；跨机器迁移使用 ZIP 包。


## POST /api/subscriptions/check

检查节点订阅、Snell 或 VLESS 节点池的链接、本地文件或粘贴正文，不保存配置、不创建版本文件、不 reload。要求管理员 session，并执行同源检查。远程地址沿用共享 Fetcher 和缓存；只读配置源也可通过此只读接口检查。

请求的 `location` 与非空 `content` 二选一。`kind` 接受 `subscriptions`、`snell`、`vless`，省略时为 `subscriptions`。只有节点订阅使用 `format`，可省略或为 `auto`，也可指定 `text`、`clash`、`surge`、`loon` 更正解析器；Snell / VLESS 由 `kind` 固定解析器，传入 `format` 返回 `400 invalid_request`。

```json
{"location":"https://example.com/sub.conf","format":"auto"}
```

```json
{"content":"anytls://example-password@hk.example.com:443#HK"}
```

节点池检查示例：

```json
{"kind":"snell","content":"HK = snell, hk.example.com, 443, psk=example-password, version=4"}
```

```json
{"kind":"vless","content":"vless://11111111-1111-1111-1111-111111111111@hk.example.com:443?type=tcp#HK"}
```

两类节点池成功时返回 `status="recognized"`、节点数量、`output_formats` 和原文，不返回订阅的 `format` 或候选客户端。单行无效则整源检查失败，返回 `400 build_snell_line_invalid` 或 `400 build_vless_source_line_invalid`，保留物理行号与脱敏来源位置。

已读取原文的检查可同时提供可选的 `source_location`，表示原文来自哪个 URL 或本地文件。它仅传给格式识别器，不触发读取，不改变 `location` 与 `content` 二选一的约束。使用 `content` 时，来源位置保留 `.lcf` 等现有识别提示，以及远程完整文档与本地节点片段的解析边界；直接提供 `location` 时以该实际读取位置为准。

```json
{"content":"[Proxy]\nHK = http, hk.example.com, 80","source_location":"https://example.com/nodes.lcf","format":"auto"}
```

成功识别响应 `200`：

```json
{
  "status":"recognized",
  "format":"loon",
  "node_count":1,
  "output_formats":["loon"],
  "content":"[Proxy]\nHK = anytls, hk.example.com, 443, \"example-password\"\n"
}
```

`content` 是本次检查的原文；Web 的正文展示不依赖检查成功，读取后即可查看或编辑本地副本，引用只有用户明确切换并保存后才改变。`format` 是确认后写入草稿的格式，节点列表使用 `text`。`output_formats` 表示全部提取节点通过现有 Target/Render 的输出格式，不代表全局配置或节点连通性已通过检查。

需要确认来源客户端时也返回 `200`，但不返回成功格式：

```json
{
  "status":"ambiguous",
  "node_count":1,
  "output_formats":[],
  "candidates":["surge","loon"],
  "content":"[Proxy]\nHK = http, hk.example.com, 80\n"
}
```

此时前端显示候选客户端，选择后携带明确 `format` 再检查。解析规则的权威定义见 [配置规约](config-schema.md#web-订阅检查)。

请求字段非法返回 `400 invalid_request`；无法提取节点、结构化声明错误或非 UTF-8 文本返回 `400`，沿用 `fetch_subscription_empty`、`fetch_subscription_format_invalid`、`fetch_subscription_base64_invalid` 等 Source 错误码。上游读取失败仍返回 `502`，本地文件读取失败仍返回 `500 resource_local_read_failed`；错误不包含订阅原文或 URL token。
