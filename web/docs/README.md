# Web 管理后台文档契约

本目录定义正式 v2.0 Web 管理后台的前端产品、交互、数据和验收契约。当前 `web/` 是正式 Vite SPA 工程；旧高保真原型位于 `web/prototype/`，只作为参考。

## 权威来源

| 主题 | 权威文档 |
|------|----------|
| Admin API 请求、响应、状态码 | [`../../docs/design/api.md`](../../docs/design/api.md) |
| 用户 YAML 与 JSON API 结构 | [`../../docs/design/config-schema.md`](../../docs/design/config-schema.md) |
| Web 页面总览与集成模型 | [`../../docs/design/web-ui.md`](../../docs/design/web-ui.md) |
| 配置校验边界与错误类型 | [`../../docs/design/validation.md`](../../docs/design/validation.md) |
| 部署与 nginx 反向代理 | [`../../docs/deployment.md`](../../docs/deployment.md) |

`web/docs/` 不重复定义完整后端 schema。当前端文档与顶层 `docs/design/*` 冲突时，以顶层设计文档为准。

## 阅读顺序

| 文件 | 用途 |
|------|------|
| [`product.md`](product.md) | 产品目标、范围、非目标 |
| [`pages.md`](pages.md) | A/B/C 页面、路由、数据来源和状态 |
| [`page-specs.md`](page-specs.md) | 逐页字段、动作、状态和测试矩阵 |
| [`data-contract.md`](data-contract.md) | 前端消费的数据契约索引 |
| [`workflows.md`](workflows.md) | 草稿、保存、reload、dirty、revision 工作流 |
| [`frontend-architecture.md`](frontend-architecture.md) | 正式 SPA 技术栈和目录建议 |
| [`auth-and-security.md`](auth-and-security.md) | 后台登录、session、setup 和订阅链接安全 |
| [`interaction.md`](interaction.md) | 交互模式与反馈规范 |
| [`design-system.md`](design-system.md) | 视觉 token 和组件约束 |
| [`acceptance.md`](acceptance.md) | 正式 SPA 验收与测试场景 |

## 当前状态

- `web/` 当前是 Vite + React + TypeScript + React Query 工程骨架。
- `web/prototype/` 保留旧高保真原型，不参与正式构建。
- 正式管理接口统一使用 `/api/*`；生成接口继续使用 `/generate`。
- Web 管理后台使用管理员登录态和 `session_id` Cookie；`SUBCONVERTER_TOKEN` 只用于 `/generate` 订阅链接。
- v2.0 当前不承诺内建配置版本恢复、多用户权限系统或服务端节点测速图表。
