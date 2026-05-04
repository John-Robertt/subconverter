# 前端架构契约

## 技术栈

正式 SPA 使用：

- React + TypeScript
- Vite
- React Router
- React Query
- 原生 CSS、CSS Modules 或项目内轻量样式方案

当前 `subconverter Admin.html` 是高保真原型，不是正式工程结构。

## 目录建议

```text
web/
├── src/
│   ├── app/
│   ├── api/
│   ├── pages/
│   ├── components/
│   ├── features/
│   ├── state/
│   └── styles/
├── package.json
├── vite.config.ts
└── Dockerfile
```

## 状态边界

- 服务端状态由 React Query 管理。
- 前端草稿由页面或 feature 级本地 state 管理。
- 管理后台登录态由服务端 `session_id` Cookie 管理；前端不保存密码、session id 或订阅访问 token。
- URL 路由只表达当前页面，不把完整草稿塞入 URL。

## Query 与 mutation

建议 query key：

- `status`
- `authStatus`
- `config`
- `previewNodes(runtime_config_revision)`
- `previewGroups(runtime_config_revision)`
- `generatePreview(runtime_config_revision, format)`

草稿预览是 mutation 或显式触发的 query，避免每次字段变化都自动打满远程来源。

运行时预览必须以 `GET /api/status` 返回的 `runtime_config_revision` 作为配置快照缓存边界。`POST /api/reload` 成功后先刷新 `status`；当 status poll 或 reload 后发现 `runtime_config_revision` 变化时，B1/B2/B3 当前运行时预览通过 query key 变化重新拉取。`config_revision` 只用于已保存配置草稿和 revision 冲突判断，不作为运行时预览 key 的替代。

B1/B2/B3 的结果还依赖订阅、Snell、VLESS 来源和远程模板的 TTL 缓存。远程资源可能在 `runtime_config_revision` 不变时变化，因此运行时预览页必须提供“刷新预览”操作，调用 React Query `refetch` 主动重新请求。页面首次进入时自动请求；后续不做后台轮询，也不把远程资源变化伪装为 revision 变化。query 的 `staleTime` 应保持较短或为 `0`，避免隐藏手动刷新入口。

## API client

API client 统一负责：

- 使用同源 Cookie 发起请求（`credentials: "include"` 或等价封装）。
- 解析 JSON 与文本响应。
- 将 HTTP 状态映射到页面可消费的错误对象，至少包含 `status`、`code`、`message`、`details`。
- 对 `401 auth_required` / `401 session_expired` 触发登录跳转，并保留 `next`。
- 对 `401 invalid_credentials` 和 `423 auth_locked` 交给登录页展示字段错误或锁定状态。
- 对 `401 setup_token_required` / `401 setup_token_invalid` 交给 setup 模式展示 bootstrap token 错误。
- 对 409 按 `code` 区分 revision 冲突、只读配置源和文件不可写。
- 对 429 reload 响应执行短间隔退避策略。
- 复制订阅链接使用 `GET /api/generate/link`，不得从前端状态拼接 `SUBCONVERTER_TOKEN`。

## 构建与部署

- 开发模式使用 Vite dev server，推荐 proxy `/api/*`、`/generate`、`/healthz` 到 Go 后端；后台登录依赖 Cookie session，应优先保持同源调试。
- 生产主路径由仓库根 Dockerfile 使用 pnpm 构建 `dist/`，再嵌入 Go 二进制；同一个 `subconverter` 进程提供 SPA、`/api/*`、`/generate` 和 `/healthz`。
- 生产模式不依赖 CORS。
