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
| M6 Admin API 基线 | 未开始 | `REQ-14` - `REQ-17`, `REQ-27` | `T-ADM-*`, `T-RLD-*`, `T-CCH-*` | 设计契约已定义，代码仍未实现 `/api/*` | 从基础序列化与结构化诊断路径开始 |
| M7 预览与状态 API | 未开始 | `REQ-18` - `REQ-21` | `T-PRV-*` | 依赖 M6 的 `app.Service` 和运行时快照模型 | M6 验收后启动 |
| M8 Web 镜像与 Compose 集成 | 未开始 | `REQ-22`, `REQ-23` | `T-SPA-*` | 当前 `web/` 是静态原型，不是正式 SPA 工程 | M6 路由和鉴权模型稳定后启动 |
| M9 前端工程与核心页面 | 未开始 | `REQ-24` 部分, `REQ-25` 部分, `REQ-26`, `REQ-27` | `T-WEB-001` - `T-WEB-010`, `T-WEB-021` | 依赖 M7 预览 API 与 M8 前端工程基础 | M7 + M8 验收后启动 |
| M10 前端完善与端到端验收 | 未开始 | `REQ-24` 剩余, `REQ-25` 剩余 | `T-WEB-011` - `T-WEB-020`, `T-E2E-*` | 端到端场景已在测试策略中定义 | M9 验收后启动 |

## M6 Admin API 基线

目标：建立配置读取、写入、静态校验和热重载能力，作为后续预览 API 与 Web 后台的后端地基。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M6-WP1 基础序列化与诊断路径 | 未开始 | `OrderedMap` JSON/YAML round-trip、`Config` json tag、`Sources.fetch_order`、结构化 `ConfigError` | `design/config-schema.md`, `design/validation.md`, `design/app-service.md` | config 层序列化与诊断 DTO 基础 | `T-ADM-007`, `T-ADM-014`, `go test ./...` | 无 |
| M6-WP2 错误类型、缓存失效与无状态生成 | 未开始 | errtype sentinel、`RevisionConflictError`、`CachedFetcher.Invalidate`、`generate.Service` 无状态化 | `design/caching.md`, `design/app-service.md`, `implementation/project-structure.md` | fetch / errtype / generate 基础能力 | `T-ADM-011`, `T-CCH-001` - `T-CCH-003`, `go test ./...` | 无 |
| M6-WP3 app/admin 服务与路由鉴权 | 未开始 | `internal/app`、`internal/admin`、`/api/config`、`/api/config/validate`、`/api/reload`、`/api/auth/*`、Admin session 鉴权 | `design/api.md`, `design/app-service.md` | M6 API 端点可用 | `T-ADM-001` - `T-ADM-022`, `T-RLD-*`, `go test ./...` | 无 |
| M6-WP4 M6 收口验收 | 未开始 | 文档同步、错误路径、进度证据、状态矩阵评估 | `implementation/implementation-plan.md`, `implementation/testing-strategy.md` | M6 验收记录 | 本文件更新，测试结果记录，已知限制记录 | 无 |

测试命令结果：未执行，能力尚未实现。

已知限制：当前 `/api/*` 仍是设计能力，不应作为发布能力使用。

## M7 预览与状态 API

目标：暴露管道中间阶段和运行状态，支持前端运行时预览和草稿预览。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M7-WP1 部分管道执行入口 | 未开始 | `SourceAndFilter`、`SourceFilterGroupRouteValidate`、`FilterResult` | `design/pipeline.md`, `design/app-service.md` | 可复用的预览阶段入口 | `T-PRV-001`, `T-PRV-002`, `T-PRV-009` | 依赖 M6 |
| M7-WP2 预览与生成预览 API | 未开始 | nodes/groups/generate preview 的 GET/POST 双模式、服务端订阅链接生成 | `design/api.md`, `design/web-ui.md` | `/api/preview/*`、`/api/generate/preview`、`/api/generate/link` | `T-PRV-003`, `T-PRV-005` - `T-PRV-008`, `T-PRV-013`, `T-PRV-014` | 依赖 M6 |
| M7-WP3 状态与错误映射 | 未开始 | `/api/status`、dirty、TargetError HTTP 分码、并发锁边界 | `design/api.md`, `design/validation.md`, `design/app-service.md` | status API 与错误语义 | `T-PRV-004`, `T-PRV-010` - `T-PRV-012` | 依赖 M6 |
| M7-WP4 M7 收口验收 | 未开始 | 文档同步、M7 测试、进度证据 | `implementation/testing-strategy.md` | M7 验收记录 | 本文件更新，测试结果记录 | 依赖 M6 |

测试命令结果：未执行，能力尚未实现。

已知限制：预览会实际拉取远程来源，响应时间受上游和缓存状态影响。

## M8 Web 镜像与 Compose 集成

目标：建立正式 SPA 工程、Web 镜像、nginx fallback 和同源反向代理路径。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M8-WP1 正式前端工程骨架 | 未开始 | Vite + React + TypeScript、最小 SPA、脚本 | `web/docs/frontend-architecture.md` | `web/src` 工程骨架 | `T-SPA-001`, `npm test` | 依赖 M6 路由稳定 |
| M8-WP2 nginx 与 Compose 代理 | 未开始 | `web/Dockerfile`、`web/nginx.conf`、Compose 示例、fallback | `docs/deployment.md`, `docs/design/web-ui.md` | Web 镜像与反代路径 | `T-SPA-002` - `T-SPA-008` | 依赖 M6；`T-SPA-007` 依赖 M7 |
| M8-WP3 M8 收口验收 | 未开始 | Web 镜像构建、代理验证、文档同步 | `implementation/implementation-plan.md` | M8 验收记录 | 本文件更新，构建和代理测试结果 | 依赖 M6 |

测试命令结果：未执行，能力尚未实现。

已知限制：当前 `web/` 仍是高保真原型，不代表正式管理后台。

## M9 前端工程与核心页面

目标：实现核心配置编辑、运行时节点预览、系统状态和保存生效工作流。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M9-WP1 Shell、API client 与草稿状态 | 未开始 | 布局、导航、登录态、React Query、草稿管理、错误归一化 | `web/docs/frontend-architecture.md`, `web/docs/auth-and-security.md`, `web/docs/workflows.md` | 前端应用基础 | `T-WEB-001` - `T-WEB-003`, `T-WEB-007`, `T-WEB-010`, `T-WEB-021` | 依赖 M7/M8 |
| M9-WP2 A1-A4 核心编辑页 | 未开始 | 来源、过滤器、节点分组、路由策略、拖拽保序、草稿预览 | `web/docs/page-specs.md`, `web/docs/data-contract.md` | A1-A4 可用 | `T-WEB-004`, `T-WEB-006`, `T-WEB-008` | 依赖 M7/M8 |
| M9-WP3 B1/C 与保存-reload 工作流 | 未开始 | 节点预览、系统状态、validate-save-reload、dirty 提示、主题 | `web/docs/workflows.md`, `web/docs/acceptance.md` | B1、C 与核心工作流 | `T-WEB-009`, `npm test` | 依赖 M7/M8 |
| M9-WP4 M9 收口验收 | 未开始 | 核心页面验收、视觉状态、文档同步 | `web/docs/page-specs.md` | M9 验收记录 | 本文件更新，前端测试结果 | 依赖 M7/M8 |

测试命令结果：未执行，能力尚未实现。

已知限制：M9 不包含 A5-A8、B2、B3 的完整功能，这些归属 M10。

## M10 前端完善与端到端验收

目标：补齐全部页面并完成正式 SPA 的端到端验收。

| 工作包 | 状态 | 范围 | 依赖文档 | 交付物 | 验收证据 | 阻塞项 |
|--------|------|------|----------|--------|----------|--------|
| M10-WP1 A5-A8 编辑与校验收口 | 未开始 | 规则集、内联规则、其他配置、静态校验 Drawer | `web/docs/page-specs.md`, `web/docs/interaction.md` | A5-A8 可用 | `T-WEB-011` - `T-WEB-014` | 依赖 M9 |
| M10-WP2 B2/B3 预览与下载 | 未开始 | 分组预览、生成预览、下载、订阅链接确认 | `web/docs/page-specs.md`, `web/docs/auth-and-security.md` | B2/B3 可用 | `T-WEB-015`, `T-WEB-016` | 依赖 M9 |
| M10-WP3 端到端验收 | 未开始 | 本地可写、错误路径、双格式、HTTP(S) 只读模式 | `implementation/testing-strategy.md`, `web/docs/acceptance.md` | E2E 场景通过 | `T-WEB-017` - `T-WEB-020`, `T-E2E-010` - `T-E2E-015` | 依赖 M9 |
| M10-WP4 M10 收口验收 | 未开始 | 最终状态矩阵、部署文档、已知限制、发布前证据 | `docs/README.md`, `docs/deployment.md` | v2.0 可交付记录 | 本文件更新，所有测试结果记录 | 依赖 M9 |

测试命令结果：未执行，能力尚未实现。

已知限制：无新增限制；正式限制以 M10 验收记录为准。

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
