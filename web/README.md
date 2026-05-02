# subconverter Admin

本目录是 v2.0 Web 管理后台的正式前端工程。M8 阶段交付 Vite + React + TypeScript 骨架、nginx 静态托管和 Docker Compose 同源反向代理；完整页面和业务工作流在 M9/M10 继续实现。

## 开发

```bash
npm ci
npm run dev
```

Vite dev server 默认监听 `localhost:5173`，并把 `/api/*`、`/generate`、`/healthz` 代理到 `http://localhost:8080`。后台登录依赖同源 Cookie，本地调试时优先通过该 proxy 访问后端。

## 验证

```bash
npm test
npm run build
docker build -t subconverter-web .
```

`npm test` 在 M8 只覆盖最小 SPA 路由烟测；正式页面组件、API client 和工作流测试归属 M9/M10。

## 生产容器

`web/Dockerfile` 使用 Node 22 执行 `npm ci` 和 `npm run build`，再用 nginx 托管 `dist/`。`web/nginx.conf` 负责：

- `/` 和前端路由：静态资源与 SPA fallback
- `/api/*`：反向代理到 `api:8080`
- `/generate`：反向代理到 `api:8080`
- `/healthz`：反向代理到 `api:8080`

生产 Compose 示例位于仓库根目录的 `deploy/`：

```bash
docker compose -f deploy/compose.readonly.yaml up -d --build
docker compose -f deploy/compose.writable.yaml up -d --build
```

## 原型

旧高保真原型已迁移到 `web/prototype/`，仅作为视觉和交互参考，不参与正式 Vite 构建，也不会被 Web 镜像复制。

## 文档

| 文件 | 说明 |
|------|------|
| [docs/README.md](docs/README.md) | 正式 v2.0 Web 管理后台文档契约入口 |
| [docs/product.md](docs/product.md) | 产品目标、范围和非目标 |
| [docs/pages.md](docs/pages.md) | 页面、路由、数据来源和页面状态 |
| [docs/workflows.md](docs/workflows.md) | 草稿、保存、reload、revision 和 dirty 工作流 |
| [docs/frontend-architecture.md](docs/frontend-architecture.md) | 前端架构、状态边界和 API client 约束 |
