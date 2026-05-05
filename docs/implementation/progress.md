# v2.0 开发进度与证据

本文件是 v2.0 开发进度确认的唯一主入口。`implementation-plan.md` 定义要做什么，`testing-strategy.md` 定义如何验证，本文件记录做到哪里、证据在哪里、还有什么风险。

## 维护规则

追踪粒度为“里程碑 + 工作包”。工作包是一个可独立验收的实现切片，通常对应一组 `REQ-*` 和 `T-*`。

状态枚举：

| 状态 | 含义 |
|------|------|
| 未开始 | 未进入实现，允许继续细化文档和测试夹具 |
| 进行中 | 已开始实现，但尚未完成本工作包验收 |
| 阻塞 | 发现需要外部决策、上游能力或方案回退的问题 |
| 待验收 | 实现完成，等待测试、评审或证据补齐 |
| 已验收 | 验收项通过，证据已记录，状态矩阵可按需更新 |

每次里程碑收口必须更新：

- 里程碑状态和下一步
- 已完成工作包状态
- 实现的 `REQ-*`
- 新增或通过的 `T-*`
- 测试命令和结果摘要
- 关键证据链接或文件路径
- 已知限制与未覆盖风险

`docs/README.md` 的能力状态矩阵只在对应里程碑“已验收”后更新，不用作日常开发进度看板。

## 里程碑总览

| 里程碑 | 状态 | 对应需求 | 对应测试 | 当前结论 | 下一步 |
|--------|------|----------|----------|----------|--------|
| M6 Admin API 基线 | 已验收 | `REQ-14` - `REQ-17`, `REQ-27` | `T-ADM-*`, `T-RLD-*`, `T-CCH-*` | 配置 CRUD、静态校验、热重载、Admin auth/session 已实现并通过验证 | 启动 M7 预览与状态 API |
| M7 预览与状态 API | 已验收 | `REQ-18` - `REQ-21` | `T-PRV-*` | 预览、生成预览、订阅链接与状态 API 已实现并通过验证 | 启动 M8 Web 镜像与 Compose 集成；M9 可依赖 M7 API |
| M8 Web 镜像与 Compose 集成 | 已验收 | `REQ-22`, `REQ-23` | `T-SPA-*` | 正式 Vite SPA 工程、`pnpm web:embed` 嵌入式 Web 发布链路、单服务 Compose 示例和 CI Web 产物校验已实现 | 已启动并完成 M9 前端核心页面 |
| M9 前端工程与核心页面 | 已验收 | `REQ-24` 部分, `REQ-25` 部分, `REQ-26`, `REQ-27` | `T-WEB-001` - `T-WEB-010`, `T-WEB-021` | Shell、API client、登录/setup、A1-A4、B1、C、主题、保存与独立 reload 工作流已实现并通过验证 | 启动 M10 前端完善与端到端验收 |
| M10 前端完善与端到端验收 | 已验收 | `REQ-24` 剩余, `REQ-25` 剩余 | `T-WEB-011` - `T-WEB-016`, `T-E2E-010`, `T-E2E-014`, `T-E2E-015` | A5-A8、B2/B3、正式 E2E 与桌面浏览器验收已通过 | M10 已完成；可进入发布前整理 |

## M6 Admin API 基线

目标：建立配置读取、写入、静态校验和热重载能力，作为后续预览 API 与 Web 后台的后端地基。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M6-WP1 基础序列化与诊断路径 | 已验收 | `OrderedMap` JSON/YAML round-trip、`Config` json tag、`Sources.fetch_order`、结构化 `ConfigError` | `design/config-schema.md`, `design/validation.md`, `design/app-service.md` | config 层序列化与诊断 DTO 基础 | `internal/config/*_test.go`, `internal/app/service_test.go`, `go test ./...` | 无 |
| M6-WP2 错误类型、缓存失效与无状态生成 | 已验收 | errtype sentinel、`RevisionConflictError`、`CachedFetcher.Invalidate`、`generate.Service` 无状态化 | `design/caching.md`, `design/app-service.md`, `implementation/project-structure.md` | fetch / errtype / generate 基础能力 | `internal/app/service_test.go`, `internal/generate/service_test.go`, `go test ./...` | 无 |
| M6-WP3 app/admin 服务与路由鉴权 | 已验收 | `internal/app`、`internal/admin`、`/api/config`、`/api/config/validate`、`/api/reload`、`/api/auth/*`、Admin session 鉴权 | `design/api.md`, `design/app-service.md` | M6 API 端点可用 | `internal/admin/*_test.go`, `internal/auth/service_test.go`, `internal/server/server_e2e_test.go`, `go test ./...` | 无 |
| M6-WP4 M6 收口验收 | 已验收 | 文档同步、错误路径、进度证据、状态矩阵评估 | `implementation/implementation-plan.md`, `implementation/testing-strategy.md` | M6 验收记录 | 本文件更新，测试结果记录，已知限制记录 | 无 |

### M6 验收记录（2026-05-03）

- 状态：已验收。
- 实现的 REQ：`REQ-14`、`REQ-15`、`REQ-16`、`REQ-17`、`REQ-27`。
- 完成的工作包：`M6-WP1`、`M6-WP2`、`M6-WP3`、`M6-WP4`。
- 依赖的设计文档：`docs/design/api.md`、`docs/design/app-service.md`、`docs/design/config-schema.md`、`docs/design/validation.md`、`docs/design/caching.md`、`docs/implementation/project-structure.md`。
- 新增或通过的测试：`OrderedMap` JSON/YAML round-trip、`Sources.fetch_order` JSON/YAML 写回、配置 snapshot/save/revision conflict/只读源/不可写文件、validate 结构化诊断、远程主配置 reload 缓存失效、auth setup/login/session/hash/lock、Admin API session 边界、`admin` 禁止直接导入 `pipeline` / `model`。
- 测试命令与结果：
  - `GOCACHE=/private/tmp/subconverter-gocache go test ./...`：通过。
  - `GOCACHE=/private/tmp/subconverter-gocache go vet ./...`：通过。
  - `git diff --check -- cmd/subconverter/main.go cmd/subconverter/main_test.go internal`：通过。
- 关键响应：
  - `GET /api/config`：返回 `{config_revision, config}`，保序字段为 `[{key,value}]`，`sources.fetch_order` 补全三项。
  - `PUT /api/config`：成功返回 `{config_revision}`；revision 冲突返回 `409 config_revision_conflict`；只读远程配置返回 `409 config_source_readonly`。
  - `POST /api/config/validate`：请求体合法时返回 `200`，配置错误返回 `valid=false` 与结构化 `locator.json_pointer`。
  - `POST /api/reload`：成功替换运行时快照；远程主配置 reload 前执行 cache invalidate；并发 reload 返回 `429 reload_in_progress`。
  - `/api/auth/*`：setup token、PBKDF2 密码哈希、session Cookie、失败锁定与 logout 已接入；受保护 `/api/*` 不接受 Bearer/query token。
- 已知错误案例：缺少 `config_revision` 返回 `400`；陈旧 revision 不写入并返回当前 revision；缺少或错误 setup token 返回 `401`；登录缺字段返回 `400 invalid_request`；跨站或缺失 `Origin` / `Referer` 的非安全 `/api/*` 请求被拒绝。
- 已知限制：M6 不包含 `GET /api/status`、`GET/POST /api/preview/*`、`GET/POST /api/generate/preview`、`GET /api/generate/link`；这些归属 M7。YAML 写回仍不保留注释、引号和原格式；`SaveConfig` 不提供外部多写者线性一致性保证。
- 下一步：启动 M7，先实现部分管道执行入口，再接入预览与状态 API。

## M7 预览与状态 API

目标：暴露管道中间阶段和运行状态，支持前端运行时预览和草稿预览。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M7-WP1 部分管道执行入口 | 已验收 | `SourceAndFilter`、`SourceFilterGroupRouteValidate`、`FilterResult` | `design/pipeline.md`, `design/app-service.md` | 可复用的预览阶段入口 | `internal/pipeline/preview_test.go`, `go test ./...` | 无 |
| M7-WP2 预览与生成预览 API | 已验收 | nodes/groups/generate preview 的 GET/POST 双模式、服务端订阅链接生成 | `design/api.md`, `design/web-ui.md` | `/api/preview/*`、`/api/generate/preview`、`/api/generate/link` | `internal/app/service_test.go`, `internal/admin/handler_test.go`, `go test ./...` | 无 |
| M7-WP3 状态与错误映射 | 已验收 | `/api/status`、dirty、TargetError HTTP 分码、并发锁边界 | `design/api.md`, `design/validation.md`, `design/app-service.md` | status API 与错误语义 | `internal/app/service_test.go`, `internal/server/errors_test.go`, `go test ./...` | 无 |
| M7-WP4 M7 收口验收 | 已验收 | 文档同步、M7 测试、进度证据 | `implementation/testing-strategy.md` | M7 验收记录 | 本文件更新，测试结果记录 | 无 |

### M7 验收记录（2026-05-03）

- 状态：已验收。
- 实现的 REQ：`REQ-18`、`REQ-19`、`REQ-20`、`REQ-21`。
- 完成的工作包：`M7-WP1`、`M7-WP2`、`M7-WP3`、`M7-WP4`。
- 依赖的设计文档：`docs/design/api.md`、`docs/design/app-service.md`、`docs/design/pipeline.md`、`docs/design/validation.md`、`docs/design/web-ui.md`。
- 新增或通过的测试：`FilterResult` Included/Excluded/All、预览 groups 执行到 ValidateGraph、运行时与草稿 nodes/groups 预览隔离、草稿 generate preview 不改变运行时状态、status 本地重算 sha256 与远程不主动拉取、`/api/generate/link` 服务端 token 生成、generate preview 无下载响应头、TargetError fallback 清空映射 400。
- 测试命令与结果：
  - `GOCACHE=/private/tmp/subconverter-gocache go test ./...`：通过。
  - `GOCACHE=/private/tmp/subconverter-gocache go vet ./...`：通过。
  - `git diff --check -- cmd/subconverter internal docs`：通过。
- 关键响应：
  - `GET/POST /api/preview/nodes`：返回节点列表、`total`、`active_count`、`filtered_count`，并保留 filtered 标记。
  - `GET/POST /api/preview/groups`：返回地区节点组、链式组、服务组、`expanded_members` 与 `all_proxies`；图级错误返回 `400 valid=false` 结构化诊断，不返回部分成功结果。
  - `GET/POST /api/generate/preview`：返回完整 Clash/Surge 文本，设置 `Cache-Control: no-store`，不设置 `Content-Disposition`。
  - `GET /api/generate/link`：要求管理员 session；使用当前运行时 `base_url` 和服务端订阅访问 token 生成 `/generate` URL。
  - `GET /api/status`：返回版本、配置源能力、当前配置 revision、运行时 revision、dirty、加载时间和最近 reload 结果。
- 已知错误案例：缺少管理员 session 返回 `401 auth_required`；缺少 `base_url` 返回 `400 base_url_required`；`include_token` 非 `true/false` 返回 `400 invalid_request`；远程拉取失败返回 `502`；目标格式 fallback 清空返回 `400 target_*_fallback_empty`。
- 已知限制：预览会实际拉取远程来源，响应时间受上游和缓存状态影响；M7 仅交付后端 API，不包含前端页面、Web 镜像或 Compose 反向代理。
- 下一步：启动 M8 Web 镜像与 Compose 集成；M9 前端页面可依赖 M7 API 契约。

## M8 Web 镜像与 Compose 集成

目标：建立正式 SPA 工程、嵌入式 Web 发布链路、Go SPA fallback 和单服务同源 Compose 路径。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M8-WP1 正式前端工程骨架 | 已验收 | Vite + React + TypeScript、最小 SPA、脚本 | `web/docs/frontend-architecture.md` | `web/src` 工程骨架 | `pnpm web:test`、`pnpm web:build` | 无 |
| M8-WP2 嵌入式 Web 与 Compose 路径 | 已验收 | `pnpm web:embed`、根 `Dockerfile`、`internal/webui`、Compose 示例、Go fallback | `docs/deployment.md`, `docs/design/web-ui.md` | 单服务 Web 镜像与同源路径 | `test -s internal/webui/dist/index.html`、`docker build -t subconverter:local .` | 无 |
| M8-WP3 M8 收口验收 | 已验收 | Web 镜像构建、代理验证、文档同步 | `implementation/implementation-plan.md` | M8 验收记录 | 本文件更新，前端/Go/Docker 测试结果记录 | 无 |

### M8 当前契约记录（2026-05-05）

- 状态：已验收。
- 实现的 REQ：`REQ-22`、`REQ-23`。
- 完成的工作包：`M8-WP1`、`M8-WP2`、`M8-WP3`。
- 依赖的设计文档：`docs/deployment.md`、`docs/design/web-ui.md`、`web/docs/frontend-architecture.md`、`web/docs/acceptance.md`。
- 新增或覆盖的测试/验证入口：
  - `web/src/App.test.tsx`：正式 SPA shell、登录/setup、配置编辑、预览、下载和错误分流相关组件/流程测试。
  - Web build：Vite production build 输出 `dist/index.html` 和 hashed assets。
  - Web embed：`pnpm web:embed` 本地构建 `web/dist` 并同步到 `internal/webui/dist`。
  - Docker build：根 Dockerfile 不运行 pnpm，Go 阶段使用 `-tags webui` 嵌入已同步的 `internal/webui/dist`，runtime 只包含 Go 二进制和底版模板。
  - Compose smoke test：`subconverter` 是唯一浏览器入口，验证 SPA fallback、`/healthz`、`/api/status` 与 `/generate` 均由同一服务处理。
  - 后端回归：`go test ./...`、`go vet ./...`。
- 当前标准测试命令与结果：
  - `go test ./...`：通过。
  - `go test -tags webui ./...`：通过。
  - `go vet ./...`：通过。
  - `pnpm web:test`：通过，1 个测试文件 / 21 个测试。
  - `pnpm web:typecheck`：通过。
  - `pnpm web:build`：通过。
  - `test -s internal/webui/dist/index.html`：通过。
  - `git diff --check`：通过。
- 关键证据：
  - `package.json` / `pnpm-lock.yaml` / `pnpm-workspace.yaml`：pnpm workspace 工程与锁文件。
  - 根 `Dockerfile`：使用已同步的 `internal/webui/dist`，Go `webui` build tag 嵌入静态资源。
  - `internal/server/webui.go`：`/api/*`、精确 `/generate`、`/healthz` 优先交给后端；其他路径 fallback 到 `index.html`。
  - `docker-compose.demo.yaml`：只引用已构建镜像，单个 `subconverter` 服务映射 `8080:8080`。
- 示例输入或 fixture：Compose 验证使用临时只读配置 `/private/tmp/subconverter-m8/config/config.yaml`，订阅源由本地临时 HTTP 服务提供，包含 1 个 `HK-01` 节点。
- 关键响应：`/download` 不触发下载、返回 SPA；`/generate?format=clash|surge` 触发后端下载响应；`/api/status` 未登录时返回 `auth_required`；`/healthz` 返回响应体 `ok`。
- 已知错误案例：未登录访问 `/api/*` 返回 `401 auth_required`；`/generate/path` 非精确生成端点，按 SPA fallback 处理。
- 已知限制：M8 只交付工程、嵌入式发布链路和部署入口；登录、配置编辑、API client、主题切换、保存/预览工作流归属 M9/M10。
- 下一步：启动 M9 前端工程与核心页面，基于 M7 API 和 M8 单服务 SPA/Compose 基础实现正式后台交互。

## M9 前端工程与核心页面

目标：实现核心配置编辑、运行时节点预览、系统状态和保存生效工作流。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M9-WP1 Shell、API client 与草稿状态 | 已验收 | 布局、导航、登录态、React Query、草稿管理、错误归一化 | `web/docs/frontend-architecture.md`, `web/docs/auth-and-security.md`, `web/docs/workflows.md` | 前端应用基础 | `T-WEB-001` - `T-WEB-003`, `T-WEB-007`, `T-WEB-010`, `T-WEB-021` | 无 |
| M9-WP2 A1-A4 核心编辑页 | 已验收 | 来源、过滤器、节点分组、路由策略、拖拽保序、草稿预览 | `web/docs/page-specs.md`, `web/docs/data-contract.md` | A1-A4 可用 | `T-WEB-004`, `T-WEB-006`, `T-WEB-008` | 无 |
| M9-WP3 B1/C 与保存、reload 工作流 | 已验收 | 节点预览、系统状态、validate-save、独立 reload、dirty 提示、主题 | `web/docs/workflows.md`, `web/docs/acceptance.md` | B1、C 与核心工作流 | `T-WEB-009`, `pnpm web:test` | 无 |
| M9-WP4 M9 收口验收 | 已验收 | 核心页面验收、视觉状态、文档同步 | `web/docs/page-specs.md` | M9 验收记录 | 本文件更新，前端测试结果 | 无 |

测试命令结果：

- `pnpm web:test`：通过，1 个测试文件 / 21 个测试。
- `pnpm web:build`：通过，Vite 生产构建成功。
- 发布前 Docker 环境验证项：`docker build -t subconverter:local .` 使用已同步 Web embed 产物并嵌入 Go 二进制。
- `go test ./...`：通过，15 个 Go package 均通过。
- `pnpm --filter subconverter-web dev -- --host 127.0.0.1` + 本地 mock API：通过，Chrome 冒烟验证 `/sources`、`/filters` 草稿节点预览、`/nodes` 运行时节点预览可渲染且无明显首屏溢出。

关键产出：

- `web/src/api/`：统一 API client、错误归一化和前端消费类型；`401 auth_required/session_expired` 触发登录跳转，409 按稳定 `error.code` 分流。
- `web/src/state/`、`web/src/features/`：主题、Toast、Confirm、配置草稿 Context、validate-save 与独立 reload 工作流、保序/脱敏工具。
- `web/src/layout/`、`web/src/components/`、`web/src/pages/`：正式 Shell、导航、基础 UI 组件、DND 排序、登录/setup、A1-A4、B1、C，并为后续 M10 页面保留受保护路由入口。

示例输入或 fixture：前端测试使用 mock backend，包含本地可写配置源、1 个 SS 订阅 URL、1 个 `HK` 节点组、1 个 `Proxy` 路由服务组和 1 个 `HK-01` 节点预览结果。

关键响应：

- A2 草稿节点预览调用 `POST /api/preview/nodes`，B1 运行时节点预览调用 `GET /api/preview/nodes`。
- 首次保存前展示 YAML 注释、引号和格式风格可能丢失的确认；确认后才调用 `PUT /api/config`。热重载由全局“热重载”按钮单独调用 `POST /api/reload`。
- `PUT /api/config` 成功但尚未 reload 时，前端更新已保存 revision，并通过 dirty 状态提示运行时仍使用旧 RuntimeConfig。
- `409 config_revision_conflict`、`config_source_readonly`、`config_file_not_writable` 与未知 409 已按 `error.code` 分流；`reload_in_progress` 展示退避重试提示。
- M9 阶段仅保留 M10 页面受保护路由入口；完整业务功能已在后续 M10 验收中完成。

已知限制：M9 不包含 A5-A8、B2、B3 的完整功能和端到端真实后端场景，这些归属 M10；M9 的浏览器视觉验收使用本地 mock API 冒烟，不替代 M10 的正式 E2E。

## M10 前端完善与端到端验收

目标：补齐全部页面并完成正式 SPA 的端到端验收。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M10-WP1 A5-A8 编辑与校验收口 | 已验收 | 规则集、内联规则、其他配置、静态校验 Drawer | `web/docs/page-specs.md`, `web/docs/interaction.md` | A5-A8 可用 | `T-WEB-011` - `T-WEB-014`，桌面浏览器验收 | 无 |
| M10-WP2 B2/B3 预览与下载 | 已验收 | 分组预览、生成预览、下载、订阅链接确认 | `web/docs/page-specs.md`, `web/docs/auth-and-security.md` | B2/B3 可用 | `T-WEB-015`, `T-WEB-016`，桌面浏览器验收 | 无 |
| M10-WP3 端到端验收 | 已验收 | 本地可写、双格式、setup/login/logout、订阅链接生成 | `implementation/testing-strategy.md`, `web/docs/acceptance.md` | E2E 场景通过 | `T-E2E-010`, `T-E2E-014`, `T-E2E-015` | 无 |
| M10-WP4 M10 收口验收 | 已验收 | 进度证据、CI E2E、已知限制 | `docs/README.md`, `docs/deployment.md` | v2.0 可交付记录 | 本文件与 `docs/README.md` 更新，所有测试结果记录 | 无 |

### M10 验收记录（2026-05-03）

- 状态：已验收；实现、自动化验证和桌面浏览器验收已完成，`docs/README.md` 能力状态矩阵已更新。
- 实现的 REQ：`REQ-24` 剩余 Web 后台页面能力、`REQ-25` 剩余预览/生成/订阅链接能力。
- 完成的工作包：`M10-WP1`、`M10-WP2`、`M10-WP3`、`M10-WP4` 均已验收。
- 依赖的设计文档：`docs/design/api.md`、`docs/design/web-ui.md`、`web/docs/frontend-architecture.md`、`web/docs/page-specs.md`、`web/docs/interaction.md`、`web/docs/auth-and-security.md`、`web/docs/acceptance.md`。
- 新增或通过的测试：
  - `T-WEB-011`：规则集 policy 与多 URL 编辑、顺序保持。
  - `T-WEB-012`：内联规则自由文本、policy selector 与顺序保持。
  - `T-WEB-013`：fallback、base_url、templates 编辑与只读禁用。
  - `T-WEB-014`：静态校验 errors/warnings/infos Drawer 与 `locator.json_pointer` 跳转高亮。
  - `T-WEB-015`：分组预览成功树形展示，ValidateGraph 失败只显示诊断。
  - `T-WEB-016`：Clash/Surge 生成预览、下载、订阅链接复制确认流。
  - `T-E2E-010`：本地可写配置的规则集保存、reload、校验、分组预览、下载。
  - `T-E2E-014`：Clash/Surge 双格式预览，前端不向 `/api/*` query/header 拼接订阅 token。
  - `T-E2E-015`：setup、login、logout、含 token 订阅链接复制确认。
- 测试命令与结果：
  - `go test ./...`：通过。
  - `pnpm web:typecheck`：通过。
  - `pnpm web:test`：通过，1 个测试文件 / 21 个测试。
  - `pnpm web:build`：通过。
  - `pnpm web:test:e2e`：正式 E2E 入口保留，当前标准验证未将其列为必跑项。
  - 发布前 Docker 环境验证项：`docker build -t subconverter:local .` 使用已同步 Web embed 产物；不在 Docker 内运行 pnpm。
  - `git diff --check -- .github web docs`：通过。
  - `Computer Use` + Chrome `127.0.0.1:15174` 桌面验收：通过；覆盖 setup 后登录态、A5 新增 URL 保存后手动 reload、A6 policy selector、A7 fallback/base_url/templates、A8 通过态与错误 Drawer 跳转、B2 分组预览、B3 Clash/Surge 预览、下载和含 token 链接复制确认。
- 示例输入或 fixture：E2E stack 使用临时 config/auth 目录、本地 fake 订阅源和模板源、Go 后端 `127.0.0.1:18080`、Vite 前端 `127.0.0.1:15173`、fake upstream `127.0.0.1:18081`；订阅源包含 `HK-01`，服务端订阅访问 token 为 `server-token`。
- golden 输出或关键响应：
  - A5/A6/A7 修改写回保存后，手动 reload 会更新运行时 revision。
  - A8 静态校验通过时页面正文和 toast 展示通过状态；诊断存在时 Drawer 可按 `locator.json_pointer` 跳转到父页面并高亮。
  - B2 成功返回 `All proxies` 与 `HK-01`；图级错误不渲染部分成功树。
  - B3 自动加载 Clash/Surge 双格式运行时预览，`/generate` 下载和 `/api/generate/link` 复制均可用；复制含 token 链接前弹出确认。后端 `POST /api/generate/preview` 草稿生成 API 保留，但当前页面不暴露独立草稿生成入口。
- 桌面验收补充证据：临时可写配置保存后 `config.yaml` 包含 `rules-extra.list`，未保存的 `MissingPolicy` 诊断草稿未写入配置文件；剪贴板链接由服务端生成并包含服务端 token，前端未拼接 token。
- 已知错误案例：Vitest 与 Playwright spec 已通过脚本分离；`/api/*` 请求中未出现服务端订阅 token，合法参数 `include_token=false` 不视为泄漏；桌面自动化首次复制链接时出现一次 Chrome Clipboard 焦点错误，重新聚焦并点击确认后成功，自动化 E2E 该流程稳定通过。
- 已知限制：M10 仍按 1280x800 桌面视口验收；A7 不实现订阅访问 token 编辑；A8 Drawer 只做诊断详情和跳转高亮，不复制 A1-A7 全量编辑表单。
- 下一步：发布前整理和版本发布流程。

## 证据记录模板

里程碑进入“待验收”或“已验收”时，复制并填写以下模板。

```markdown
### Mx 验收记录（YYYY-MM-DD）

- 状态：
- 实现的 REQ：
- 完成的工作包：
- 依赖的设计文档：
- 新增或通过的测试：
- 测试命令与结果：
- 示例输入或 fixture：
- golden 输出或关键响应：
- 已知错误案例：
- 已知限制：
- 下一步：
```
