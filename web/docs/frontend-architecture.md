# 前端架构契约

## 技术栈

正式 SPA 使用：

- React + TypeScript
- Vite
- React Router
- React Query
- 原生 CSS（当前实现为 `web/src/styles.css`）

当前正式工程位于 `web/src/`；历史高保真原型已从仓库中移除，不参与生产构建。

## 当前主要目录

```text
web/
├── index.html
├── README.md
├── docs/
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── App.test.tsx
│   ├── app/
│   ├── api/
│   ├── components/
│   ├── features/
│   ├── layout/
│   ├── pages/
│   ├── state/
│   └── styles.css
├── e2e/
├── scripts/
├── package.json
├── tsconfig.json
├── tsconfig.app.json
├── tsconfig.node.json
├── playwright.config.ts
├── playwright.integration.config.ts
└── vite.config.ts
```

## 状态边界

- React Query 保存从服务端读取的配置、状态和预览结果，负责请求与刷新。
- `state/config.tsx` 中的 `ConfigProvider` 统一保存配置草稿、已加载基线和 `baseRevision`，让用户跨编辑页切换时保留草稿。页面只更新这份草稿，表单展开状态等界面状态由页面自己保存。
- `features/useSaveWorkflow.ts` 负责校验草稿、保存确认、调用保存接口，以及独立的热重载操作。保存更新 YAML，热重载才更新生效配置；两者的版本字段见 [app.Service 契约](../../docs/design/app-service.md#statusresult)。
- 管理后台登录态由服务端 `session_id` Cookie 管理；前端不保存密码、session id 或订阅访问 token。
- URL 路由只表达当前页面，不把完整草稿塞入 URL。

## Query 与 mutation

`app/queryKeys.ts` 定义核心 query key，即 React Query 区分各份缓存数据所用的键：

- `status`
- `authStatus`
- `config`
- `previewNodes(runtime_config_revision)`
- `previewGroups(runtime_config_revision)`
- `generatePreview(runtime_config_revision, format)`

另有两处页面内联 key：`StatusPage` 使用 `["healthz"]`，`DownloadPage` 使用 `["generateLink", format, runtime_config_revision]`。

编辑页草稿预览是 mutation 或显式触发的 query，避免每次字段变化都自动打满远程来源。当前 B3 页面只暴露运行时生成预览；后端 `POST /api/generate/preview` 仍保留为 API 能力。

运行时预览必须以 `GET /api/status` 返回的 `runtime_config_revision` 作为配置快照缓存边界。`POST /api/reload` 成功后先刷新 `status`；当 status poll 或 reload 后发现 `runtime_config_revision` 变化时，B1/B2/B3 当前运行时预览通过 query key 变化重新拉取。`config_revision` 只用于已保存配置草稿和 revision 冲突判断，不作为运行时预览 key 的替代。

B1/B2/B3 的结果还依赖订阅、Snell、VLESS 来源和远程模板的 TTL 缓存。远程资源可能在 `runtime_config_revision` 不变时变化；B1 通过“重新拉取”调用 React Query `refetch` 主动重新请求，B2/B3 通过页面进入、query key 变化或错误重试重新请求。页面首次进入时自动请求；后续不做后台轮询，也不把远程资源变化伪装为 revision 变化。query 的 `staleTime` 应保持较短或为 `0`。

## API client 与错误处理分层

`api/client.ts` 统一负责：

- 使用同源 Cookie 发起请求（`credentials: "include"`）。
- 解析 JSON 与文本响应。
- 将 HTTP 失败响应归一化为 `ApiError`（包含 `status`、`code`、`message`、`payload` / 校验诊断等详情）。
- 对受保护接口返回的 `401 auth_required` / `401 session_expired` 派发 `subconverter:auth-required` 全局事件。
- 复制订阅链接调用 `GET /api/generate/link`，不在前端状态拼接 `SUBCONVERTER_TOKEN`。

上层模块分别承接对应错误恢复：

- `app/ProtectedRoute.tsx` 监听 `subconverter:auth-required` 事件，重定向至 `/login?next=<当前路径>`。
- `pages/LoginPage.tsx` 处理 `401 invalid_credentials`、`423 auth_locked`、`401 setup_token_required` / `401 setup_token_invalid` 与认证网络错误。
- `features/useSaveWorkflow.ts` 处理 `409` 按 `code` 分流（`config_revision_conflict`、`config_source_readonly`、`config_file_not_writable`、`resource_content_write_failed` 与未知 409），以及重载 `429 reload_in_progress` 退避（提示用户已有重载在执行，等待 1.5 秒后重试一次）。

## 构建与部署

- 开发模式使用 Vite dev server，推荐 proxy `/api/*`、`/generate`、`/healthz` 到 Go 后端；后台登录依赖 Cookie session，应优先保持同源调试。
- 生产主路径先运行仓库根 `pnpm web:embed` 构建 `dist/` 并同步到 `internal/webui/dist`，再由根 Dockerfile 使用 Go `webui` build tag 嵌入二进制；同一个 `subconverter` 进程提供 SPA、`/api/*`、`/generate` 和 `/healthz`。
- 生产模式不依赖 CORS。
