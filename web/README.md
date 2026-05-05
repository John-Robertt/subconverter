# subconverter Admin

本目录是 v2.0 Web 管理后台的正式前端工程。前端使用 Vite + React + TypeScript；生产主路径由本地 `pnpm web:embed` 生成 `internal/webui/dist`，再由 Go 二进制嵌入并托管 SPA 与 API。

## 开发

```bash
pnpm install
pnpm --filter subconverter-web dev
```

Vite dev server 默认监听 `localhost:5173`，并把 `/api/*`、`/generate`、`/healthz` 代理到 `http://localhost:8080`。后台登录依赖同源 Cookie，本地调试时优先通过该 proxy 访问后端。

## 验证

```bash
pnpm --filter subconverter-web test
pnpm --filter subconverter-web build
pnpm web:embed
pnpm --filter subconverter-web test:e2e:int   # Playwright 集成测试（route-mock + Vite dev server）
pnpm --filter subconverter-web test:e2e       # Playwright 端到端测试（带真后端，需要 Go 工具链）
docker build -t subconverter:local ..
```

- `pnpm --filter subconverter-web test`：vitest 单元/组件测试。
- `pnpm --filter subconverter-web test:e2e:int`：`web/e2e/integration/` 下的 53 个集成测试。Vite dev server 由 `playwright.integration.config.ts` 自动启动，所有 `/api/*`、`/generate`、`/healthz` 通过 `page.route` 拦截 mock，无需 Go 后端。覆盖登录/setup/锁定/注销、A1–A8 编辑页、B1/B2/B3 运行时预览、保存工作流（首次确认 / revision 冲突 / reload 失败 / 只读源）、C 系统状态，以及跨页边界（session 失效跳转、reload 429 自动重试、后端不可达条幅、保序、只读全局禁用）。
- `pnpm --filter subconverter-web test:e2e`：`web/scripts/e2e-stack.mjs` 启动真实的 Go 后端 + Vite + 跑 `web/e2e/m10.spec.ts`，用于回归 M10 端到端不变量。

## 生产容器

生产主路径使用仓库根目录 `Dockerfile`：

- 本地 `pnpm web:embed` 构建并同步 `internal/webui/dist`
- Docker/Release 只用已提交的 `internal/webui/dist` 和 Go 工具链
- runtime 镜像中只有 `/app/subconverter` 和底版模板，不需要 nginx

生产 Demo Compose 示例位于仓库根目录，使用单个 `subconverter` 服务：

```bash
pnpm web:embed
pnpm docker:build
pnpm compose:up
```

## 文档

| 文件 | 说明 |
|------|------|
| [docs/README.md](docs/README.md) | 正式 v2.0 Web 管理后台文档契约入口 |
| [docs/product.md](docs/product.md) | 产品目标、范围和非目标 |
| [docs/pages.md](docs/pages.md) | 页面、路由、数据来源和页面状态 |
| [docs/workflows.md](docs/workflows.md) | 草稿、保存、reload、revision 和 dirty 工作流 |
| [docs/frontend-architecture.md](docs/frontend-architecture.md) | 前端架构、状态边界和 API client 约束 |
