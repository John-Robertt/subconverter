# v2.0 开发计划

> v1.0 开发计划（M0-M5）已归档至 `docs/v1.0/implementation/implementation-plan.md`。

## 目标

在 v1.0 已完成的核心管道基础上，扩展 Web 管理后台和配置热重载能力。v1.0 提供了从 YAML 配置到 Clash Meta / Surge 输出的完整管道（Source → Filter → Group → Route → ValidateGraph → Target → Render）；v2.0 在此之上增加配置 CRUD API、运行时热重载、管道中间阶段预览，以及由单个 `subconverter` 服务同源托管的 React SPA 管理界面——使用户能通过浏览器完成配置编辑、预览和生成操作，无需手动修改 YAML 文件。

本计划是 `docs/architecture.md`、`docs/design/*` 和测试策略之间的执行连接层。

---

## 计划原则

### 渐进式原则

- 先稳定 API 层（读写、校验、热重载），再构建依赖 API 的前端
- 先完成后端并发安全改造，再暴露会触发写操作的接口
- 先建立 Web 镜像与 Compose 生产部署基础设施，再进入前端页面开发
- 每个阶段只解决一类主要问题，避免跨阶段耦合

### 可验收原则

- 每个阶段都要有明确产物
- 每个阶段都要有最小可执行验证
- 每个阶段都要说明已覆盖范围与未覆盖风险

### 可回溯原则

- 需求、设计、里程碑、测试统一编号
- 所有实现项都能追溯到需求来源
- 所有验收结果都能追溯到具体阶段和测试项

---

## 编号体系

### v1.0 需求基线（已完成）

| 编号 | 需求 |
|------|------|
| `REQ-01` | 支持单用户、单配置文件运行模式 |
| `REQ-02` | 支持多个 SS 订阅源、Snell 来源与 VLESS 来源 |
| `REQ-03` | 支持自定义代理节点 |
| `REQ-04` | 支持链式节点与链式组生成 |
| `REQ-05` | 链式组属于节点组，且策略显式声明 |
| `REQ-06` | 支持地区节点组，且保留书写顺序 |
| `REQ-07` | 支持服务组，且保留书写顺序 |
| `REQ-08` | `@all` 仅展开原始节点，不含链式节点 |
| `REQ-09` | 支持 rulesets、rules、fallback |
| `REQ-10` | 同一中间表示输出 Clash Meta 与 Surge |
| `REQ-11` | 提供 `/generate` 与 `/healthz` |
| `REQ-12` | 配置、引用和循环依赖等错误可校验和报告 |
| `REQ-13` | `@auto` 自动补充 routing 成员（节点组+包含 `@all` 的服务组+DIRECT） |

### v2.0 需求

| 编号 | 需求 |
|------|------|
| `REQ-14` | 配置读取 API（`GET /api/config`，返回 Config JSON + `config_revision`） |
| `REQ-15` | 配置写入 API（`PUT /api/config`，本地可写配置源下基于 `config_revision` 条件写回） |
| `REQ-16` | 静态配置校验 API（`POST /api/config/validate`，结构化错误/警告/提示，含定位信息） |
| `REQ-17` | 运行时热重载（`POST /api/reload`，RWMutex 保护 RuntimeConfig） |
| `REQ-18` | 节点预览 API（`GET/POST /api/preview/nodes`，运行时与草稿双模式） |
| `REQ-19` | 分组预览 API（`GET/POST /api/preview/groups`，含服务组与宏展开结果） |
| `REQ-20` | 生成预览 API（`GET/POST /api/generate/preview`，返回文本不下载） |
| `REQ-21` | 系统状态 API（`GET /api/status`，含配置源能力与运行时状态）与订阅链接生成 API（`GET /api/generate/link`） |
| `REQ-22` | Web 静态资源容器化发布（生产镜像嵌入 SPA，Go 服务同源托管） |
| `REQ-23` | Docker Compose 生产部署（单个 `subconverter` 服务） |
| `REQ-24` | Web 管理后台 -- 配置编辑（A1-A8 页面） |
| `REQ-25` | Web 管理后台 -- 运行时预览（B1-B3 页面） |
| `REQ-26` | Web 管理后台 -- 系统状态（C 页面） |
| `REQ-27` | Web 管理后台 -- 独立管理员登录、首次 setup 与 Cookie session |

### 里程碑编号

- `M6`：Admin API 基线
- `M7`：预览与状态 API
- `M8`：Web 镜像与 Compose 集成
- `M9`：前端工程与核心页面
- `M10`：前端完善与端到端验收

### 测试编号扩展

v1.0 测试编号（`T-CFG-*`、`T-SRC-*`、`T-FLT-*`、`T-GRP-*`、`T-RTE-*`、`T-VAL-*`、`T-RND-*`、`T-E2E-*`）继续保留。v2.0 新增以下编号域：

- `T-ADM-*`：Admin API 测试（配置 CRUD、鉴权）
- `T-RLD-*`：热重载测试（并发安全、失败回滚）
- `T-PRV-*`：预览 API 测试（部分管道执行、状态查询）
- `T-SPA-*`：Web 与 Compose 测试（嵌入式静态资源、Go fallback、单服务 Compose）
- `T-WEB-*`：前端组件/集成测试（页面渲染、交互流程）

---

## 里程碑总览

| 里程碑 | 主题 | 依赖 | 预估复杂度 |
|--------|------|------|-----------|
| `M6` | Admin API 基线 | v1.0 完成 | 中 |
| `M7` | 预览与状态 API | M6 | 中 |
| `M8` | Web 镜像与 Compose 集成 | M6 | 中 |
| `M9` | 前端工程与核心页面 | M7, M8 | 高 |
| `M10` | 前端完善与端到端验收 | M9 | 高 |

执行顺序：

```text
M6 -> M7 (顺序)
M6 -> M8 (与 M7 并行)
M7 + M8 -> M9 -> M10
```

关键依赖：

- `M6` 建立配置读写和热重载能力，是所有后续 API 和前端的基础
- `M7` 和 `M8` 互不依赖，可并行推进；`M7` 提供预览数据，`M8` 提供前端托管基础设施
- `M9` 同时依赖 `M7`（预览 API）和 `M8`（Web 容器托管基础设施），需两者均完成后启动
- `M10` 是 `M9` 的功能延续和验收收口

---

## M6: Admin API 基线

### 目标

建立配置管理的 API 基础设施，包括配置读写、校验和热重载。这是 v2.0 的地基——所有后续 API 和前端功能都依赖配置可读、可写、可校验、可热重载的能力。

### 工作项

**基础设施前置（后续工作项的地基）**：

- `OrderedMap` 完整序列化：在现有 `UnmarshalYAML` 基础上新增 `MarshalJSON`、`UnmarshalJSON`、`MarshalYAML` 三个方法。`MarshalJSON` 输出 `[{key,value}]` 数组；`UnmarshalJSON` 从该数组恢复；`MarshalYAML` 用于 `PUT /api/config` 写回 YAML 时保持键序。四个方法 round-trip 幂等性测试必须在此阶段完成
- `Config` 及嵌套结构体全部添加 `json` tag（与 `yaml` tag 保持同名 lowercase），使 `encoding/json` 序列化的 key 名与 API 契约一致
- `Sources` JSON 序列化：添加 `FetchOrder` 的 `json:"fetch_order"` tag（替代当前 `yaml:"-"`），新增 `Sources.MarshalYAML` 按 `FetchOrder` 排列子键顺序。`fetch_order` 非空时必须完整包含 `subscriptions`、`snell`、`vless` 且三项各出现一次；重复、未知或缺项返回 `invalid_fetch_order`。详见 `config-schema.md` §sources 的 JSON 表示
- `CachedFetcher` 新增 `Invalidate(rawURL string)` 方法，供 reload 在 `LoadConfig` 前失效远程主配置缓存。详见 `caching.md` §CachedFetcher API 扩展
- `ConfigError` 结构化路径：将 collector 的 `add(field, message)` 改为 `add(section, key, index, valuePath, message)`，使 `ConfigError` 携带结构化路径字段（`Section`、`Key`、`Index`、`ValuePath`），替代当前的 dot-separated `Field` 字符串。app 层基于此计算 `json_pointer`，组装 `DiagnosticItem`。详见 `app-service.md` §ConfigError → DiagnosticItem 翻译
- 在 `internal/errtype` 新增 `ErrConfigSourceReadonly`、`ErrConfigFileNotWritable`、`ErrReloadInProgress` sentinel error，以及携带 `CurrentConfigRevision` 的 `RevisionConflictError`。详见 `app-service.md` §错误处理

**包与服务搭建**：

- 新建 `internal/app` 包承接 v2.0 应用服务逻辑；新建 `internal/admin` 包承接 `/api/*` handler 逻辑
- 新增配置源能力模型：本地文件为 writable，HTTP(S) URL 为 read-only
- 将 `generate.Service` 改为无状态设计：移除 `cfg *config.RuntimeConfig` 字段，`Generate` 方法改为接收 `*config.RuntimeConfig` 参数；`app.Service` 在每次 `/generate` 和 `GET /api/generate/preview` 请求时取快照后传入（迁移方案 A）
- 在 `app.Service` 中管理并发安全配置访问（`sync.RWMutex`：读路径只在锁内复制 `*RuntimeConfig` 快照，`/api/reload` 写路径 WLock 替换指针；独立 `reloadMu sync.Mutex` 保护 reload 互斥，TryLock 失败返回 `ErrReloadInProgress`/429）

**API 端点实现**：

- 实现 `GET /api/config`（读取配置源中的 YAML，计算 `config_revision=sha256:<hex>`，转换为 JSON 格式返回；保序字段使用 `OrderedMap` 的 JSON 表示，`sources` 含 `fetch_order` 字段）
- 实现 `PUT /api/config`（仅本地可写配置源支持；接收 `{config_revision, config}`，写入前重读当前文件并校验 revision；冲突返回 `409 config_revision_conflict`；校验通过后临时文件 + rename 原子写回 YAML）
- 实现 `POST /api/config/validate`（仅执行 JSON 反序列化 + `Prepare` 静态校验，不写回、不拉取远程源；请求体合法时始终返回 200，配置无效返回 `valid=false` + errors / warnings / infos；请求体无法解析、缺少 `config` 或 `config` 非对象返回 400）
- 实现 `POST /api/reload`（触发热重载：TryLock reloadMu → Invalidate 主配置缓存 → `LoadConfig` → re-`Prepare` → WLock swap `*RuntimeConfig` pointer + runtime revision）
- 实现 `/api/auth/status`、`/api/auth/login`、`/api/auth/setup`、`/api/auth/logout`（管理员登录、首次 setup、session 状态和注销）

**路由与鉴权**：

- 在 `internal/server` 层注册 `/api/*` 路由
- 新增管理员 auth state 文件：保存单一管理员 PBKDF2 密码哈希和持久 session 哈希；无凭据时进入需要 bootstrap setup token 的 setup，auth state 不可写时 fail closed
- `/api/*` 管理接口使用 `session_id` HttpOnly Cookie；除 `/api/auth/status`、`/api/auth/login`、`/api/auth/setup`、`/api/auth/logout` 外均要求有效 session
- `SUBCONVERTER_TOKEN` / `-access-token` 只保护 `/generate` 客户端订阅更新；`/api/*` 不接受 Bearer token 作为后台权限
- Cookie session 下，非安全方法需要校验同源 `Origin` 或 `Referer`

### 工序依赖图

M6 工作项之间存在严格的执行顺序约束。以下标注每个工作项的前置条件，用于编排开发顺序：

```text
基础设施前置（全部 API 的地基，必须最先完成）
  │
  ├─ OrderedMap MarshalJSON / UnmarshalJSON / MarshalYAML + round-trip 测试
  ├─ Config 及嵌套结构体添加 json tag
  ├─ Sources: FetchOrder json tag + MarshalYAML（按 fetch_order 排列子键）
  ├─ CachedFetcher.Invalidate 方法
  ├─ ConfigError 结构化路径（collector 改造）
  └─ errtype errors（ErrConfigSourceReadonly / ErrConfigFileNotWritable / RevisionConflictError / ErrReloadInProgress）
       │
       ├─► generate.Service 无状态化改造
       │     └─► POST /api/reload（依赖无状态 Generate + CachedFetcher.Invalidate + reloadMu）
       │
       ├─► 配置源能力模型 ──► GET /api/config（依赖 OrderedMap JSON + Config json tag + Sources fetch_order）
       │     │                └─► PUT /api/config（依赖 OrderedMap JSON + MarshalYAML + 原子写回）
       │     │
       │     └─► POST /api/config/validate（依赖 ConfigError 结构化路径 + DiagnosticItem 翻译）
       │
       └─► 管理员 session 鉴权与 auth state（可与上述并行；不依赖 OrderedMap）
              │
              └─► server 路由注册（前置：所有 handler 均已实现）
```

关键约束：
- **基础设施前置**：OrderedMap 四方法、Config json tag、Sources MarshalYAML、CachedFetcher.Invalidate、ConfigError 结构化路径、errtype 错误类型——这些是所有后续工作项的地基，必须在 M6 最初期完成并通过独立单测验证
- **无状态化改造优先**：`generate.Service` 必须移除 `cfg` 字段后才能被 `app.Service.Reload` 正确调用
- **Validate 依赖 ConfigError 结构化路径**：collector 改造后 `Prepare` 产出结构化 `ConfigError`，app 层翻译为 `DiagnosticItem`（含 `json_pointer`）
- **Reload 依赖 CachedFetcher.Invalidate**：远程主配置 URL 必须先 invalidate 缓存再拉取
- **路由注册收尾**：所有 handler 实现完成后再注册路由

### 产物

- `internal/config/`：
  - `orderedmap.go`：新增 `MarshalJSON`、`UnmarshalJSON`、`MarshalYAML` 三方法
  - `config.go`：全部结构体添加 `json` tag；`Sources` 新增 `FetchOrder` json tag + `MarshalYAML`
  - `prepare.go`：collector 改为结构化路径接口（`Section`/`Key`/`Index`/`ValuePath`）
- `internal/errtype/`：
  - `errors.go`：新增 `ErrConfigSourceReadonly`、`ErrConfigFileNotWritable`、`ErrReloadInProgress` sentinel errors，以及 `RevisionConflictError`
- `internal/fetch/`：
  - `cache.go`：新增 `CachedFetcher.Invalidate(rawURL string)` 方法
- `internal/app/`：
  - `service.go`：配置快照、条件写回、热重载和运行时快照入口（含 `reloadMu` 互斥）
  - `preview.go`：运行时 / 草稿预览与草稿生成入口
  - `generate_link.go`：服务端订阅链接生成
  - `diagnostic.go`：ConfigError → DiagnosticItem 翻译逻辑（计算 `json_pointer`）
  - `validation.go`：Admin API 额外校验边界（如 `sources.fetch_order`）
  - `status.go`：状态、配置源能力和运行环境 DTO
  - `errors.go` / `generate_input.go`：app 层请求错误与生成输入别名
- `internal/admin/`：
  - `handler.go`：集中注册认证、配置、校验、reload、预览、生成预览、订阅链接和状态 handler；调用 `internal/app` / `internal/auth`
- `internal/generate/`：
  - 移除 `Service.cfg` 字段，`Generate` 改为接收 `*config.RuntimeConfig` 参数（无状态化迁移）
- `internal/server/`：
  - 路由注册扩展：`/api/auth/*`、`/api/config`、`/api/config/validate`、`/api/reload`
- `internal/auth/`：
  - `state.go` / `session.go`：auth state 文件、PBKDF2 密码哈希、session token 哈希、失败计数与锁定逻辑
- 测试：`T-ADM-001` ~ `T-ADM-022`（含 OrderedMap round-trip、CachedFetcher.Invalidate、ConfigError 结构化路径、409 error code 判断和 session 鉴权）

### 验收项

- `GET /api/config` 返回与配置源中已保存 YAML 等价的 JSON，并包含 `config_revision`
- `PUT /api/config` → `GET /api/config` round-trip 幂等（JSON → YAML → JSON 内容一致），成功后返回新的 `config_revision`
- `PUT /api/config` 缺少 revision 返回 400；revision 与当前文件不一致返回 `409 config_revision_conflict` 且不写入
- `PUT /api/config` 成功响应仅返回 `{config_revision}`；YAML 注释/格式丢失提示由 Web UI 首次保存确认承担，后端不自动创建 `.bak` 文件，也不返回 warning 字段
- HTTP(S) 配置源下 `PUT /api/config` 返回 409，且不尝试写回远端
- `PUT` 配置语义校验失败返回 400 + `ValidateResult`（`valid=false`，诊断项含 `code`、`display_path`、`locator.json_pointer`）；请求体格式错误、revision 冲突、只读配置源等非配置语义错误返回 `{error}`
- `POST /api/config/validate` 请求体合法时返回 `{ valid, errors, warnings, infos }` 结构；配置无效返回 `200 valid=false`，请求体格式错误返回 400
- `POST /api/config/validate` 不拉取订阅、不执行 Source / Group / Target / Render；生成可用性由预览和生成预览接口覆盖
- `POST /api/reload` 成功路径：新 `RuntimeConfig` 生效，后续 `/generate` 使用新配置；远程主配置 URL 在 TTL 未过期时仍能读到最新内容
- `POST /api/reload` 失败路径：旧 `RuntimeConfig` 不变，返回错误详情
- 慢速 `/generate` 请求不会因持有配置读锁而阻塞 `/api/reload` 获取写锁；`/api/preview/*` 的同类验证归属 M7
- 保序字段（`groups` / `routing` / `rulesets`）JSON round-trip 顺序不变
- 未携带有效 `session_id` Cookie 的受保护 `/api/*` 请求返回 `401 auth_required`；过期或注销 session 返回 `401 session_expired`
- 登录失败返回 `401 invalid_credentials` 并携带剩余次数；连续失败锁定返回 `423 auth_locked`
- 无管理员凭据时 `GET /api/auth/status` 返回 `setup_required=true` 与 `setup_token_required=true`；setup 缺少或错误 token 时返回 401；auth state 不可写时 setup 返回 `409 auth_state_not_writable`
- `/generate?token=...` 保持 v1.0 兼容；后台内下载可凭管理员 session 调用 `/generate`
- `go test ./...` 全部通过

### 对应需求

- `REQ-14`、`REQ-15`、`REQ-16`、`REQ-17`、`REQ-27`

### 已知限制

- YAML 写回可能丢失用户注释（`gopkg.in/yaml.v3` 的 `Marshal` 不保留原始注释节点；若需保留注释，需改用 `yaml.Node` 级别的 patch-merge 策略，但复杂度显著上升）；当前策略是 Web UI 首次保存前确认，不做后端自动备份
- 热重载成功前已经取得配置快照的请求会继续使用旧配置完成；当前不提供严格线性一致性保证
- 条件写回只保护本地文件配置源，并只承诺防止旧页面或旧 revision 覆盖已观测到的新配置；HTTP(S) 配置源仍为只读
- `SaveConfig` 的 revision 检查存在理论上的 TOCTOU 窗口（外部进程在 revision 比对和 rename 之间修改文件），单用户场景下可接受

### 风险

- `OrderedMap` 完整序列化（`MarshalJSON` / `UnmarshalJSON` / `MarshalYAML`）是 M6 全部 API 的地基——round-trip 不一致会破坏保序不变量。应在 M6 最初期用独立单测验证 YAML→JSON→YAML 和 JSON→YAML→JSON 两个方向的幂等性
- `Sources.FetchOrder` 在 JSON round-trip 中必须保留——`PUT /api/config` 写回 YAML 时需按 `fetch_order` 排列 `sources` 子键，否则代理排列顺序会变化
- `ConfigError` 结构化路径改造涉及全部 `Prepare` 校验代码的调用点——需确保改造后每条校验都携带完整路径信息，否则 `json_pointer` 会空缺
- `sources.fetch_order` 必须覆盖默认、完整排列、重复值、未知值和缺项测试；否则写回 YAML 时可能改变拉取顺序或接受不可解释的前端输入
- YAML 写回格式与原始文件可能有差异（缩进、引号风格、注释丢失），前端需在本地可写配置首次保存前提示并要求确认
- `admin` 若直接依赖 `pipeline` / `model` 会破坏 HTTP 薄层边界；需用依赖检查或包导入测试锁定边界

---

## M7: 预览与状态 API

### 目标

暴露管道中间阶段的数据，支持前端运行时预览。预览 API 让用户在不实际生成配置文件的情况下查看节点列表、分组结果和生成输出，降低配置调试的试错成本。

### 工作项

- 实现 `GET /api/preview/nodes`（基于当前 `RuntimeConfig` 执行 Source + Filter 阶段，返回节点列表 JSON，含 Kind / Type / 来源标记 / filtered 标记）
- 实现 `POST /api/preview/nodes`（接收 `{config}` 草稿，Prepare 后执行 Source + Filter，不写文件、不替换 RuntimeConfig）
- 实现 `GET /api/preview/groups`（基于当前 `RuntimeConfig` 执行到 ValidateGraph 阶段，返回节点组、链式组、服务组、`@all` / `@auto` 展开结果；图级错误返回 400 结构化诊断）
- 实现 `POST /api/preview/groups`（接收 `{config}` 草稿，Prepare 后执行到 ValidateGraph 阶段；图级错误返回 400 结构化诊断）
- 实现 `GET/POST /api/generate/preview?format=clash|surge`（复用生成逻辑，返回生成文本但不触发下载——无 `Content-Disposition` header；POST 使用草稿配置；用于验证草稿在目标格式下的生成可用性）
- 实现 `GET /api/generate/link?format=clash|surge`（要求管理员 session，由服务端按 `base_url`、filename 和订阅访问 token 生成客户端订阅链接）
- 实现 `GET /api/status`（进程信息、版本号、配置源位置与可写性、配置加载状态、dirty 状态、上次热重载时间和结果）
- `/generate` 与 `/api/generate/preview` 的 TargetError HTTP 映射已统一：fallback 清空返回 400，projection invariant 保持 500
- 在 `app` / `pipeline` 之间新增部分执行入口（`SourceAndFilter`、`SourceFilterGroupRouteValidate`），复用现有阶段函数但在指定阶段截断返回

### 产物

- `internal/admin/`：
  - `preview_handler.go`：节点预览和分组预览 handler
  - `generate_preview_handler.go`：生成预览 handler
  - `generate_link_handler.go`：订阅链接生成 handler
  - `status_handler.go`：系统状态 handler
- `internal/app/`：
  - `preview.go`：运行时与草稿预览编排
  - `status.go`：系统状态查询
  - `generate_link.go`：订阅链接生成
- `internal/pipeline/`：
  - `FilterResult{Included, Excluded}` 与必要的部分执行入口函数（基于现有 `Build` 的阶段拆分，不改变现有 `Build` 对外行为）；groups 预览入口必须执行到 ValidateGraph
- `internal/generate/`：
  - 继续提供生成能力；状态查询由 `app.Service` 承接
- 测试：`T-PRV-001` ~ `T-PRV-014`，其中 `T-PRV-014` 覆盖后台 session 下载 Surge 时 managed URL 的 token 来源必须来自服务端订阅访问 token

### 验收项

- `/api/preview/nodes` 返回全部源的节点列表，每个节点含 `name`、`type`、`kind`、`server`、`port`、`filtered` 等基础字段；生成管道只消费 `FilterResult.Included`
- `/api/preview/groups` 成功时返回节点组、链式组、服务组和宏展开结果，顺序与 Group / Route 阶段输出一致；ValidateGraph 失败时返回 400 结构化诊断，不返回部分成功结果
- `POST /api/preview/*` 使用草稿配置，GET 预览仍使用旧 `RuntimeConfig`
- `/api/generate/preview?format=clash` 返回与 `/generate?format=clash` 相同内容和 `Content-Type`，但无 `Content-Disposition`；POST 草稿预览不影响运行时
- `POST /api/generate/preview` 能发现静态校验无法覆盖的 Source / Group / Target / Render 问题，例如远程源为空、过滤后空组、fallback 级联清空或模板渲染错误
- `/api/status` 返回版本号、配置源位置、配置源可写性、上次加载时间、dirty 状态、上次重载结果、当前配置 revision 与运行时 revision
- `/api/generate/link` 在 `base_url` 缺失时返回 `400 base_url_required`；服务端配置订阅 token 时返回含 token 链接，未配置时返回 `token_included=false`；链接 token 来源是服务端订阅访问 token，不依赖当前请求鉴权方式
- 本地配置源 status 每次重算 sha256；HTTP(S) 配置源 status 不主动拉取远程配置，dirty 基于最近一次 config/reload 观测结果
- `CodeTargetClashFallbackEmpty` / `CodeTargetSurgeFallbackEmpty` 映射为 400，projection invariant 仍映射为 500
- 预览 API 使用当前 `RuntimeConfig` 快照（RLock 复制指针后立即释放），不影响并发安全
- 慢速 `/api/preview/*` 请求不会因持有配置读锁而阻塞 `/api/reload` 获取写锁
- `go test ./...` 全部通过

### 对应需求

- `REQ-18`、`REQ-19`、`REQ-20`、`REQ-21`

### 已知限制

- `preview/nodes` 需要实际拉取订阅，响应时间受上游网络影响（通过现有 `CachedFetcher` 的 TTL 缓存缓解）
- 部分管道执行入口（`SourceAndFilter`、`SourceFilterGroupRouteValidate`）需在不破坏现有阶段封装的前提下截断——实现方式为组合调用现有阶段函数，不引入新的阶段间耦合
- `/api/generate/preview` 与 `/generate` 共享相同的生成逻辑和潜在错误路径

### 风险

- 部分执行入口可能暴露阶段间的中间状态结构体（`GroupResult` 等）——JSON 序列化需选择性暴露字段，避免内部实现细节泄漏到 API 契约
- 预览 API 的错误响应格式需与 M6 的结构化错误保持一致

---

## M8: Web 镜像与 Compose 集成

### 目标

让生产部署通过 Docker Compose 启动单个 `subconverter` 服务：根 Dockerfile 先构建 React SPA，再用 `webui` build tag 将 `web/dist` 嵌入 Go 二进制，由同一个进程同源提供 SPA、`/api/*`、`/generate` 和 `/healthz`。M8 建立前端工程、嵌入式 Web 发布链路和生产入口，使 M9/M10 的前端开发可以独立迭代。

### 工作项

- 建立 `web/` 目录结构（React + Vite + TypeScript）
- 根 `Dockerfile` 使用 pnpm 构建 `web/dist/`，再复制到 `internal/webui/dist` 并以 `-tags webui` 构建 Go 二进制
- `internal/server` 支持嵌入式 SPA fallback；`/api/*`、精确 `/generate`、`/healthz` 保持后端优先处理
- 新增单服务 Demo Compose 示例：`docker-compose.demo.yaml` 只启动 `subconverter`
- 不维护独立 Web 静态镜像或 nginx 路径配置夹具
- 编写 SPA 路由与静态资源测试，确认 Go fallback 和后端路径边界可工作

### 产物

- `web/`：正式 React + Vite + TypeScript SPA 工程
- `internal/webui/`：可选嵌入式 Web 静态资源入口；普通 Go 构建返回空 FS，生产 Docker 构建使用 `webui` tag
- 根 `Dockerfile`：pnpm Web build + Go embed build + distroless runtime
- `docker-compose.demo.yaml`：单服务 Demo Compose，挂载 config 与 auth 目录
- `docs/deployment.md`：单镜像、单服务部署说明
- 测试：`T-SPA-001` ~ `T-SPA-008`

### 验收项

- Compose 启动后，浏览器访问 `subconverter` 服务端口可打开 SPA
- SPA 路由刷新不 404（`/any/path` 在无匹配静态文件时由 Go fallback 到 `index.html`）
- `/generate?format=clash` 与 `/generate?format=surge` 由同一 Go 服务处理成功，且不被 SPA fallback 接管
- `/healthz` 由同一 Go 服务处理成功
- `/api/status` 由同一 Go 服务处理成功
- 生产路径不依赖 CORS；浏览器看到的 Web 页面与 API 为同源
- 根 Dockerfile 单服务镜像构建通过
- `go test ./...` 全部通过

### 对应需求

- `REQ-22`、`REQ-23`

### 已知限制

- 前端代码更新后需要重新构建生产镜像，使新的 `web/dist` 嵌入 Go 二进制
- Go fallback 只处理 Web 前端路由；`/api/*`、`/generate`、`/healthz` 必须优先由后端处理
- 开发模式的 Vite dev server 可单独配置 proxy；生产 Compose 路径不依赖 CORS

### 风险

- Go fallback 路径若配置错误，会导致后端请求被 SPA fallback 吞掉；测试需锁定 `/api/*`、`/generate` 和 `/healthz` 的优先级
- 单镜像构建会引入 Node/pnpm 依赖下载时间；CI 可通过 Docker build cache 或 pnpm store cache 缓解

---

## M9: 前端工程与核心页面

### 目标

实现前端核心功能页面，覆盖最常用的配置编辑和运行时预览。M9 完成后用户可通过浏览器完成日常配置修改和节点预览操作。

### 工作项

- 前端项目工程化（TypeScript 严格模式、React Query 数据层、React Router 路由、React 本地状态 + `useReducer`/Context 管理配置草稿）
- API 客户端层（封装 `/api/*` 调用，统一错误处理）
- 布局与导航（侧边栏 + 主内容区 + 顶栏状态指示器）
- A1 订阅来源页面（四类来源卡片：subscription / snell / vless / custom_proxy；含 `relay_through` 子表单，URL 输入脱敏显示）
- A2 过滤器页面（`exclude` 正则编辑 + 草稿匹配预览——调用 `POST /api/preview/nodes` 显示当前草稿会过滤哪些节点）
- A3 节点分组页面（分组卡片 + 拖拽排序 + 正则匹配预览——调用 `POST /api/preview/groups` 显示草稿分组结果）
- A4 路由策略页面（服务组列表 + 拖拽排序 + 成员选择器，支持 `@auto` / `@all` 特殊成员）
- B1 节点预览页面（调用 `/api/preview/nodes`，表格展示全部节点，支持按 Kind / Type / 名称筛选）
- C 系统状态页面（调用 `/api/status`，展示版本、配置源位置与可写性、上次加载时间、重载状态）
- 保存/静态校验/热重载工作流集成（编辑 → 静态校验 → 本地可写配置首次保存确认 → 保存 → 重载，每步反馈明确）
- 主题系统（浅色/深色，跟随系统偏好 + 手动切换；手动选择刷新后保持）

### 产物

- `web/src/`：
  - `api/`：API 客户端封装
  - `components/`：通用组件（拖拽排序、正则输入、卡片、表格等）
  - `pages/`：A1-A4、B1、C 页面组件
  - `layouts/`：布局组件
  - `hooks/`：自定义 hooks（useConfig、usePreview、useReload 等）
  - `theme/`：主题配置
- 测试：`T-WEB-001` ~ `T-WEB-010` + `T-WEB-021`（组件渲染测试 + API 集成测试 + 登录/setup 流程）

### 验收项

- 配置编辑 → 保存 → 热重载 → 再读取 round-trip 正确（前端发起完整流程，数据一致）
- 拖拽排序后保存，`groups` / `routing` 顺序不变（保序不变量在前端操作后仍成立）
- 过滤器正则预览与实际过滤结果一致（前端预览结果 = 后端 `preview/nodes` 结果）
- 节点预览显示正确的来源分类（Kind）和协议标注（Type）
- 浅色/深色主题切换正常，跟随系统偏好；手动选择刷新后保持；两套主题在 1280x800 下均无文本重叠、按钮溢出或关键状态不可读
- 保存流程中的内联静态校验错误可通过 `locator.json_pointer` 映射到 A1-A4 已实现字段；不要求 M9 提供 A8 路由或校验 Drawer
- 所有 API 调用错误在 UI 上有明确反馈（非静默失败）
- 本地可写配置首次保存前显示 YAML 注释、引号和格式风格可能丢失的确认；用户确认后才发起 `PUT /api/config`
- API 请求使用同源 Cookie session
- `go test ./...` 全部通过
- 前端测试通过（`pnpm --filter subconverter-web test`）

### 对应需求

- `REQ-24`（A1-A4）、`REQ-25`（B1）、`REQ-26`

### 已知限制

- 拖拽排序依赖前端 `OrderedMap` 表示的 JSON 数组结构——前后端需约定一致的保序 JSON 格式（如 `[{ key: "...", value: {...} }, ...]`）
- 草稿预览需调用后端 API，网络延迟会影响"实时"体验（可通过 debounce + 骨架屏缓解）
- `relay_through` 子表单的 `type=group` 需引用已定义的节点组名——前端需从当前编辑中的配置读取组名列表
- 默认不引入额外前端状态库；只有当 M9 实现中证明 Context/useReducer 导致明显性能或维护问题时，才评估 Zustand，并需先说明替代方案与新增依赖成本

### 风险

- 保序 JSON 格式的前后端约定是 M9 最大的接口风险——若 M6 的 `OrderedMap` JSON 格式设计不当，M9 的拖拽排序会遇到序列化问题。应在 M6 完成时即固化 JSON 格式契约
- 前端状态管理复杂度：配置编辑涉及多个嵌套结构（sources、groups、routing、rulesets），需用 reducer action 按配置段拆分更新路径，避免每次局部编辑都触发全量重建

---

## M10: 前端完善与端到端验收

### 目标

补齐剩余页面，完成全流程端到端验证。M10 完成后 v2.0 所有功能就绪。

### 工作项

- A5 规则集页面（rulesets 列表编辑，支持 URL 和 Policy 绑定）
- A6 内联规则页面（rules 列表编辑，支持自由文本 + Policy 选择器）
- A7 其他配置页面（`fallback`、`base_url`、`templates` 字段编辑）
- A8 静态配置校验页面（Drawer 展示校验结果——errors / warnings / infos 分级显示，错误项通过 `locator.json_pointer` 跳转到对应页面的对应字段）
- B2 分组预览页面（调用 `GET /api/preview/groups`，树形展示节点组 / 链式组 / 服务组 → 成员映射关系；图级错误时显示诊断，不展示部分成功结果）
- B3 生成下载页面（调用 `GET /api/generate/preview`、`/generate`、`GET /api/generate/link`，自动展示 Clash / Surge 双格式运行时生成文本 + 下载按钮 + 复制订阅链接；后端 `POST /api/generate/preview` 草稿生成能力保留为 API 能力）
- 端到端测试覆盖全流程（空配置 → 逐步编辑 → 保存 → 重载 → 预览 → 生成 → 下载 → 验证内容）

### 产物

- `web/src/pages/`：A5-A8、B2-B3 页面组件
- `web/src/components/`：校验结果 Drawer、代码预览器、链接生成器等
- 测试：`T-WEB-011` ~ `T-WEB-020` + `T-E2E-010` ~ `T-E2E-015`

### 验收项

- 全部 12 个路由页面（A1-A8 + B1-B3 + C）和校验 Drawer 组件可用
- A8 静态配置校验错误可点击跳转到对应页面的对应字段，前端不得解析 `display_path`
- 运行时生成预览与实际 `/generate` 下载内容一致
- 订阅链接复制功能正确：前端调用 `GET /api/generate/link`，复制含 `token_included=true` 的链接前显式确认；服务端配置订阅 token 时链接可包含 `token` 和 `filename` 参数
- 端到端：空配置 → 编辑全部字段 → 保存 → 重载 → 生成 → 下载 → 验证内容（两种格式均覆盖）
- 错误路径：静态校验失败 → 不可保存 → 修复后可保存；分组预览图级错误 → UI 显示诊断并保留草稿；运行时生成预览失败 → UI 显示生成期错误并可重试；重载失败 → UI 显示错误 → 旧配置不变
- `go test ./...` 全部通过
- 前端测试通过（`pnpm --filter subconverter-web test`）
- 端到端测试通过

### 对应需求

- `REQ-24`（A5-A8）、`REQ-25`（B2-B3）

### 已知限制

- 校验跳转依赖错误响应中的 `locator.json_pointer` 与前端路由的映射关系；`display_path` 仅用于展示，不能作为程序定位依据
- B3 复制订阅链接功能依赖 `base_url` 配置——未配置 `base_url` 时应提示用户先配置

### 风险

- 端到端测试的环境搭建复杂度（需同时运行后端 + 前端 + fake 订阅源）——可复用 v1.0 的 `httptest.Server` + fake fetcher 模式
- 全部 12 个路由页面和校验 Drawer 的交互一致性维护成本（需建立 UI 组件库避免各页面重复实现相似交互）

---

## 验收矩阵

| 里程碑 | 验收方式 | 核心证据 |
|--------|---------|---------|
| `M6` | API 测试 | Config CRUD round-trip 幂等、reload 并发安全、校验结构化输出、保序 JSON round-trip |
| `M7` | API 测试 | Preview 数据与管道阶段输出一致、status 信息完整 |
| `M8` | 集成测试 | Web 镜像可构建、SPA 可访问、fallback 正确、反向代理路径可用 |
| `M9` | 功能测试 | 核心页面可交互、保存-重载-读取一致、拖拽保序、主题切换 |
| `M10` | 端到端测试 | 全流程完成、全部页面可用、校验跳转正确、生成预览一致 |

---

## 每阶段通用完成标准

每个里程碑完成前都必须满足：

- `go test ./...` 通过
- 本阶段新增能力有对应测试
- 文档同步更新到对应设计或实施文档
- `docs/implementation/progress.md` 已记录本阶段状态、证据、已知限制和下一步
- 至少覆盖一个错误路径
- 不引入与当前里程碑无关的功能扩展
- 前端相关里程碑（M8-M10）额外要求 `pnpm --filter subconverter-web test` 通过

---

## v2.0 回溯机制

`docs/implementation/progress.md` 是 v2.0 开发进度和验收证据的唯一主入口。每个里程碑从“进行中”进入“待验收”或“已验收”时，必须更新该文件；日常实现中的局部进展也优先记录到对应工作包。

### 设计回溯

每个里程碑收口时应记录：

- 实现了哪些 `REQ-*`
- 依赖了哪些设计文档
- 完成了哪些工作包
- 新增或通过了哪些测试项
- 当前已知限制是什么

### 结果回溯

每个里程碑都应固化以下证据：

- 示例输入或 fixture
- 中间表示样本、关键响应或 golden 输出
- 测试命令和结果摘要
- 已知错误案例
- 未覆盖风险与下一步

### 状态矩阵更新规则

`docs/README.md` 的能力状态矩阵只在对应里程碑达到“已验收”后更新。实现进行中或待验收时，只更新 `progress.md`，避免把设计中或未验收能力误标为当前可用。

---

## 风险前置清单

正式编码前应优先验证这些高风险问题：

- **OrderedMap 完整序列化 round-trip**需在 M6 最初期验证——`MarshalJSON` / `UnmarshalJSON` / `MarshalYAML` 三方法的 YAML↔JSON 双向 round-trip 幂等性直接影响所有后续 API 和前端的保序假设
- **Sources.FetchOrder JSON round-trip**需在 M6 初期验证——`fetch_order` 字段在 JSON→YAML 写回后必须保持 `sources` 子键顺序不变，否则代理排列顺序会变化
- **ConfigError 结构化路径改造**需在 M6 初期验证——collector 改造后的结构化路径能否覆盖全部现有校验点，app 层能否正确计算 `json_pointer`
- **CachedFetcher.Invalidate**需在 M6 覆盖——reload 的远程主配置必须 bypass 未过期缓存
- **YAML 写回的注释丢失问题**需评估用户影响——若用户在 YAML 中维护了大量注释，API 写回导致注释丢失会造成负面体验。备选方案：仅在首次 API 写入时提示用户
- **配置快照锁边界**需在 M6/M7 用并发测试验证——慢速订阅拉取、模板加载、渲染以及本地 status 文件 hash 不得持有 `RuntimeConfig` 读锁；写锁只保护指针替换
- **配置 revision 条件写回**需在 M6 初期验证——缺 revision、过期 revision、保存前已观测到的外部文件改写都必须拒绝覆盖；不验证外部多写者线性一致
- **本地 status revision 重算**需在 M7 验证——`GET /api/status` 不能因同大小或保留 mtime 的外部改写漏报 dirty
- **TargetError HTTP 分码**需在 M7 验证——fallback 清空返回 400，projection invariant 返回 500
- **Web 嵌入构建链路**需在 M8 验证——Node/pnpm 构建阶段、Go `webui` build tag 和单服务 Compose 路径都必须可重复构建
- **preview API 的订阅拉取延迟**需验证 TTL 缓存效果——首次调用会触发实际网络请求，响应时间可能超过用户预期

---

## 最终交付定义

v2.0 达到可交付状态至少满足：

- v1.0 全部功能继续正常工作（`/generate`、`/healthz`、管道不变量）
- 能通过 `/api/config` 读写配置，JSON round-trip 保序幂等，并用 `config_revision` 防止静默覆盖
- 能通过 `/api/reload` 热重载配置，失败时旧配置不变
- 能通过 `GET /api/preview/*` 查看当前运行时数据，通过 `POST /api/preview/*` 查看草稿数据
- 能通过 Docker Compose 中的单个 `subconverter` 服务在浏览器中完成配置编辑、预览和生成
- 能通过测试证明并发安全、保序不变量和全流程端到端正确性
