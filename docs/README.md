# 文档状态矩阵

本目录顶层文档描述 subconverter v2.0 的当前设计契约。当前代码已完成 M6-M10，实现后端 Admin API、正式 Web 管理后台、Web 镜像与端到端验收；能力状态以本文矩阵和 `implementation/progress.md` 的最新验收记录为准。

## 文档导航

按问题领域速查：

| 你想知道 | 读这份文档 |
|----------|----------|
| 系统整体如何工作、模块边界、关键决策 | `architecture.md` |
| 用户配置文件的完整字段定义 | `design/config-schema.md` |
| 管道每个阶段做了什么、输入输出是什么 | `design/pipeline.md` |
| Proxy / ProxyGroup / Ruleset 等实体的字段语义 | `design/domain-model.md` |
| 每个 API 端点的请求/响应格式 | `design/api.md` |
| `app.Service` 的 Go 层接口契约与 DTO | `design/app-service.md` |
| Clash Meta / Surge 的输出映射与级联过滤 | `design/rendering.md` |
| 配置校验的三层边界与错误码 | `design/validation.md` |
| 远程资源拉取的缓存策略 | `design/caching.md` |
| Web 管理后台的前端页面设计与交互 | `design/web-ui.md` |
| Web 管理后台正式前端契约 | `../web/docs/README.md` |
| 怎么构建/部署/Docker Compose | `deployment.md` |
| v2.0 各里程碑的工作项与验收标准 | `implementation/implementation-plan.md` |
| v2.0 当前开发进度、证据、已知限制 | `implementation/progress.md` |
| Go 代码目录布局与包依赖约束 | `implementation/project-structure.md` |
| 测试编号体系与覆盖策略 | `implementation/testing-strategy.md` |
| v1.0 归档设计 | `v1.0/` |

## 能力状态矩阵

| 能力 | 状态 | 里程碑 | 当前发布可用 | 说明 |
|------|------|--------|--------------|------|
| `GET /generate` | 当前可用 | v1.0 | 是 | 生成 Clash Meta / Surge 配置。 |
| `GET /healthz` | 当前可用 | v1.0 | 是 | 进程健康检查。 |
| Snell / VLESS 来源、链式组、目标格式投影 | 当前可用 | v1.0 | 是 | 由当前 Go 后端实现并有测试覆盖。 |
| `/api/auth/*` | 当前可用 | M6 | 是 | 管理后台登录、首次 setup、session 状态和注销；M9 已接入前端登录/setup/logout 闭环。 |
| `GET/PUT /api/config` | 当前可用 | M6 | 是 | 配置 CRUD 与 JSON/YAML round-trip。 |
| `POST /api/config/validate` | 当前可用 | M6 | 是 | 静态配置校验与结构化诊断。 |
| `POST /api/reload` | 当前可用 | M6 | 是 | 运行时热重载与 `RuntimeConfig` 快照替换。 |
| `GET/POST /api/preview/nodes` | 当前可用 | M7 | 是 | 节点预览 API，支持运行时与草稿双模式。 |
| `GET/POST /api/preview/groups` | 当前可用 | M7 | 是 | 分组预览 API，执行到 `ValidateGraph`，图级错误返回结构化诊断。 |
| `GET/POST /api/generate/preview` | 当前可用 | M7 | 是 | 页面内生成预览，不设置下载响应头。 |
| `GET /api/generate/link` | 当前可用 | M7/M10 | 是 | 已登录后台中由服务端生成客户端订阅链接；M10 前端复制确认流已通过验收。 |
| `GET /api/status` | 当前可用 | M7 | 是 | 系统状态、配置 revision、dirty 状态。 |
| Web 管理后台正式 SPA | 当前可用 | M9-M10 | 是 | 登录/setup、A1-A8、B1-B3、C、主题、保存-reload、诊断跳转、预览下载和订阅链接确认流已通过自动化与桌面浏览器验收。 |
| 单镜像 Docker Compose 生产部署 | 当前可用 | M8-M10 | 是 | Compose 示例使用单个 `subconverter` 服务；生产镜像嵌入 Web SPA，由 Go 同源提供 `/`、`/api/*`、`/generate` 和 `/healthz`。 |

状态定义：

- **当前可用**：当前代码和发布产物已经提供的能力。
- **设计中**：本文档锁定但尚未验收的能力，需等对应里程碑实现和验收后才可作为生产能力使用。
