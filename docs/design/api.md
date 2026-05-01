# HTTP API 设计

> v1.0 API 文档已归档至 docs/v1.0/design/api.md

## 目标

本文件定义系统对外暴露的全部 HTTP 接口。v2.0 在原有生成接口基础上，新增配置管理、热重载、运行时预览和系统状态接口，为 Web 管理后台提供数据支撑。

---

## 接口概览

| 方法 | 路径 | 用途 | 版本 |
|------|------|------|------|
| GET | `/generate` | 生成并下载目标格式配置 | v1.0 |
| GET | `/healthz` | 进程健康检查 | v1.0 |
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
| GET | `/api/status` | 系统状态信息 | v2.0 |

---

## 生成接口（v1.0）

### `GET /generate`

用途：生成目标客户端配置文件。

查询参数：

- `format=clash|surge`（必填）
- `token=<access-token>`（仅当服务端配置了访问 token 时必填）
- `filename=<custom-name>`（可选；未传时默认 `clash.yaml` / `surge.conf`；仅允许 ASCII 字母、数字、`.`、`-`、`_`）

成功响应：

- Clash Meta：`Content-Type: text/yaml; charset=utf-8`
- Surge：`Content-Type: text/plain; charset=utf-8`
- 两种格式都输出 `Content-Disposition: attachment; ...`

### `GET /healthz`

用途：进程健康检查。

成功响应：`200 OK`

---

## 配置管理接口（v2.0）

### 配置源能力

`-config` 仍支持本地文件路径或 HTTP(S) URL，但两类配置源的管理能力不同：

| 配置源 | `GET /api/config` | `PUT /api/config` | `POST /api/reload` | 说明 |
|--------|-------------------|-------------------|--------------------|------|
| 本地文件 | 支持 | 支持 | 支持 | Web 后台可编辑并写回 YAML 文件 |
| HTTP(S) URL | 支持 | 不支持，返回 `409` | 支持 | 远程配置视为只读，只能重新拉取并热重载 |

状态接口会暴露 `config_source.type`、`config_source.writable` 和 `capabilities.config_write`，前端据此隐藏或禁用保存入口。

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
6. 校验通过 → 序列化为 YAML → 写入同目录临时文件 → 原子 rename 覆盖原配置文件 → 重新计算新的 `config_revision` → 同步更新 `app.Service` 内缓存的 `config_revision` 和 `(mtime, size)` 快照（避免自身写回被误判为外部修改） → 返回新的 `config_revision`
7. 校验失败 → 不写入，返回 `400` + 结构化错误

成功响应：

```json
{
  "config_revision": "sha256:9a21..."
}
```

错误响应：

- `400`：缺少 `config_revision`、请求 JSON 无法解析，或校验失败
- `409`：只读配置源、本地文件不可写，或 `config_revision` 与当前文件不一致

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
- 当 `-config` 是 HTTP(S) URL，或本地文件权限不可写时，返回 `409`，不尝试写入
- `PUT /api/config` 仅写回文件，不自动触发热重载
- 前端需在保存后显式调用 `POST /api/reload` 使新配置生效
- YAML 写回可能改变格式细节并丢失原始文件中的注释
- 多标签页、外部编辑器或 GitOps 进程改写配置时，revision 校验会阻止后台静默覆盖外部修改

### `POST /api/config/validate`

用途：静态校验配置，不写入文件、不重载。

边界：

- 本接口只执行 JSON 反序列化与 `Prepare` 阶段校验
- 本接口不拉取订阅、不执行 Source / Filter / Group / Route / Target / Render
- 因此它能提前发现字段、正则、URL 基本格式、命名冲突、跨段引用和环路等静态问题
- 它不能发现远程源不可用、远程源为空、过滤后组为空、目标格式级联过滤后 fallback 清空等生成期问题
- 生成可用性由 `POST /api/preview/nodes`、`POST /api/preview/groups` 和 `POST /api/generate/preview?format=...` 覆盖

请求体：

- `Content-Type: application/json`
- Body：完整配置 JSON

成功响应：`200`，Body 为校验结果 JSON：

```json
{
  "valid": true,
  "errors": [],
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
  "page": "A3",
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
- `page`：对应前端页面标识（A1-A8）；此为当前前端布局的辅助提示，非稳定 API 契约——前端定位必须以 `locator.json_pointer` 为准
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
3. 校验通过 → `WLock` 替换 `RuntimeConfig` → 返回 `200`
4. 校验失败 → 不替换，返回 `400` + 与 `POST /api/config/validate` 相同结构的静态诊断

成功响应：

```json
{
  "success": true,
  "duration_ms": 12
}
```

错误响应：`400`，Body 为静态诊断或加载错误；远程配置拉取失败按 `502` 返回

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

注意：此端点会实际拉取草稿配置中的全部订阅 URL。如果草稿包含新增来源且不在 TTL 缓存中，响应时间受上游网络影响（通常 10-30s）。前端应显示"正在拉取订阅..."等明确提示，而非仅显示 spinner。

请求体：

```json
{
  "config": {}
}
```

### `GET /api/preview/groups`

用途：基于当前运行时配置返回分组与服务组匹配结果（执行管道的 Source + Filter + Group + Route 阶段）。

成功响应：`200`，Body 为分组结果 JSON：

```json
{
  "node_groups": [
    {
      "name": "🇭🇰 Hong Kong",
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

### `POST /api/preview/groups`

用途：基于草稿配置返回分组与服务组匹配结果，响应结构与 `GET /api/preview/groups` 相同。

注意：此端点执行完整的 Source + Filter + Group + Route 阶段，包括实际拉取草稿配置中的全部订阅 URL。延迟与 `POST /api/preview/nodes` 相同，前端应提供明确的加载反馈。

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
  }
}
```

字段说明：

- `config_source.type`：`local` 或 `remote`
- `config_source.writable`：当前配置源是否支持 `PUT /api/config`
- `config_revision`：配置源当前已保存内容的 revision
- `runtime_config_revision`：当前 `RuntimeConfig` 对应的配置 revision
- `config_dirty`：`config_revision != runtime_config_revision` 时为 `true`。检测策略：服务维护配置文件的 `(mtime, size)` 快照，`GET /api/status` 时先比对 mtime 和 size，仅在变化时才重读文件并重算 `config_revision`；未变化时直接使用缓存的 revision 比较。因此外部进程改写文件后，下一次 status 请求即可检测到变更
- `capabilities.config_write`：前端是否应启用保存入口

---

## 错误语义

| 状态码 | 场景 |
|------|------|
| `400` | 请求参数非法；配置语义 / 图校验失败；热重载校验失败 |
| `401` | 缺少 token，或 token 不匹配 |
| `409` | 当前配置源不支持该操作；配置 revision 冲突 |
| `502` | 远程资源拉取失败，或远程订阅内容不可用 |
| `500` | 本地资源读取失败，或内部处理 / 渲染失败 |

错误响应格式：

- `/generate`：`text/plain; charset=utf-8`，中文纯文本
- `/api/*`：`application/json`，结构化 JSON（含 `error` / `errors` 字段）

---

## 鉴权

- 所有 `/api/*` 和 `/generate` 共享同一 token 值
- `/api/*` 仅通过 `Authorization: Bearer ...` header 传递 token；后台 SPA 不把 token 放入 API query
- `/generate` 继续支持 `token=...` query 参数，用于 Clash / Surge 订阅链接兼容；也可接受 `Authorization: Bearer ...` header 供浏览器内下载调用
- 服务端未配置 token 时（`-access-token` 为空），所有请求免鉴权
- 前端复制订阅链接时，必须显式确认是否把当前 token 写入 query 参数

---

## CORS

- 生产 Docker Compose 模式不启用 CORS：浏览器访问 `web` 容器，nginx 同源反向代理 `/api/*`、`/generate`、`/healthz` 到 `api` 容器
- 开发模式优先使用 Vite proxy；仅在不使用 proxy、需要浏览器跨域直连 Go 后端时，通过 `-cors` 标志或 `SUBCONVERTER_CORS=true` 启用
- 启用后仅允许本机开发来源：`http://localhost:*`、`http://127.0.0.1:*`、`http://[::1]:*`
- CORS middleware 必须处理 preflight：允许 `GET`、`POST`、`PUT`、`OPTIONS`；允许 `Authorization`、`Content-Type`
- 响应需设置 `Vary: Origin`，避免不同来源的缓存污染

---

## 运行参数

系统支持以下启动参数：

- `-config`：YAML 配置文件路径或 HTTP(S) URL（必填；HTTP(S) URL 为只读配置源）
- `-listen`：HTTP 监听地址（默认 `:8080`）
- `-cache-ttl`：订阅、模板和远程配置的缓存 TTL（默认 `5m`）
- `-timeout`：拉取订阅的 HTTP 超时时间（默认 `30s`）
- `-access-token`：为全部接口启用访问 token（默认空 = 不鉴权）
- `-cors`：启用 CORS 中间件（默认 `false`，仅开发模式使用）
- `-healthcheck`：健康检查模式
- `-version`：打印版本信息

环境变量：`SUBCONVERTER_LISTEN`、`SUBCONVERTER_TOKEN`、`SUBCONVERTER_CORS`

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
2. 若服务端配置了 token，校验 `token`
3. 校验并规范化 `filename`
4. 通过 `app.Service` 获取当前 `RuntimeConfig` 快照（内部 `RLock` 复制指针后立即释放）
5. 将快照传入 `generate.Generate(ctx, cfg, req)`（无状态调用）执行 `Build → Target → Render`
6. 若 `format=surge` 且有 `base_url`，组装 managed URL
7. 返回配置文本（带 `Content-Disposition`）

### `/api/config` 流程

- GET：读取配置源中的已保存 YAML → 解析为 JSON → 返回
- PUT：确认配置源可写 → 接收 JSON → 反序列化 → Prepare 校验 → 原子写回 YAML

### `/api/reload` 流程

1. 重新加载配置源（`LoadConfig`；远程主配置 URL 必须 bypass / invalidate 缓存）
2. 执行 `Prepare`
3. 成功 → `WLock` 替换 `RuntimeConfig` 与 `runtime_config_revision` → 返回
4. 失败 → 返回错误

### `GET /api/preview/*` 流程

1. `RLock` 读取当前 `RuntimeConfig` 指针快照并立即释放锁
2. 使用快照执行管道部分阶段（nodes: Source+Filter；groups: Source+Filter+Group+Route）
3. 返回 JSON

### `POST /api/preview/*` 流程

1. 接收 `{ "config": ... }` 草稿配置
2. 执行 `Prepare` 得到临时 `RuntimeConfig`
3. 使用临时配置执行对应预览阶段
4. 返回 JSON，不写文件，不替换服务当前配置
