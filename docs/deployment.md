# 构建与部署

> 状态提示：当前发布可用能力以 `/generate`、`/healthz` 和核心生成管道为主；Web 管理后台、`/api/*` 与 `api + web` Compose 部署仍是 v2.0 规划能力。完整状态见 docs/README.md。

## 目标

本文档定义项目的 GitHub 构建、Release 发布、GHCR 镜像发布和手动部署方式。

---

## 发布产物

项目维护三类发布产物：

- GitHub Release 二进制压缩包
- GHCR 后端 Docker 镜像（`api` 服务）
- Web Docker 镜像（当前托管设计原型；正式管理后台属于 M8-M10 规划能力）

二进制平台矩阵：

- `linux/amd64`
- `linux/arm64`

Docker 镜像平台矩阵：

- `linux/amd64`
- `linux/arm64`

---

## GitHub Actions

### Release

文件：`.github/workflows/release.yml`

触发条件：

- 推送 `v*` tag，例如 `v0.1.0`

执行内容：

- 运行 lint（golangci-lint）、格式检查（gofmt）、测试（go test）和 vet（go vet）
- 用 GoReleaser 发布二进制和 `checksums.txt` 到 GitHub Release
- 构建并推送 GHCR 多架构后端镜像
- 构建 Web 镜像以验证静态托管链路；正式 `api + web` 生产部署需等待 M8-M10 完成

---

## GitHub Release 二进制

Release 包内包含：

- `subconverter` 可执行文件
- `configs/base_config.yaml`
- `configs/base_clash.yaml`
- `configs/base_surge.conf`

当前 GitHub Release 二进制仅发布 Linux 包。

这样解压后即可直接使用默认模板路径：

```yaml
templates:
  clash: "configs/base_clash.yaml"
  surge: "configs/base_surge.conf"
```

注意：这些相对路径是相对于进程工作目录解析的。最稳妥的用法是在解压目录下启动程序。

---

## GHCR 后端镜像

镜像地址：

```text
ghcr.io/john-robertt/subconverter      # api 服务
ghcr.io/john-robertt/subconverter-web  # web 服务
```

发布 tag：

- `vX.Y.Z`
- `vX.Y`
- `latest`

容器内约定：

- 工作目录：`/app`
- 二进制：`/app/subconverter`
- 内置模板：`/app/configs/*`
- 外部配置挂载路径：`/config/config.yaml`

镜像默认启动命令：

```shell
/app/subconverter -config /config/config.yaml
```

镜像同时内置 `SUBCONVERTER_LISTEN=:8080`，因此默认仍监听 `:8080`。

如需为当前可用的 `/generate` 启用访问控制，可额外设置 `SUBCONVERTER_TOKEN`。客户端下载配置文件时仍需在 URL 上附带 `token` 查询参数。v2.0 Admin API 规划中会复用同一 token，并要求 Web 后台通过 `Authorization: Bearer ...` header 访问 `/api/*`。

因此如果配置文件继续使用：

```yaml
templates:
  clash: "configs/base_clash.yaml"
  surge: "configs/base_surge.conf"
```

容器内也能正常解析到镜像内置模板。

---

## 手动部署

### Docker 部署（只读配置）

```bash
docker run -d \
  --name subconverter \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/config/config.yaml:ro \
  ghcr.io/john-robertt/subconverter:latest
```

这种模式适合当前发布以及 GitOps / 外部系统管理配置文件的部署方式。v2.0 Web 后台实现后，`api` 服务会把只读挂载识别为不可写配置源，`PUT /api/config` 返回 `409`，配置保存按钮应禁用。

### Docker 部署（可写目录挂载，为 v2.0 写回预留）

当前发布不会通过 Web 后台写回配置。若要为后续 v2.0 可写配置源预留部署形态，可挂载整个配置目录，并确保容器内运行用户对该目录有写权限：

```bash
mkdir -p ./config
cp config.yaml ./config/config.yaml

docker run -d \
  --name subconverter \
  -p 8080:8080 \
  -v $(pwd)/config:/config \
  -e SUBCONVERTER_TOKEN=your-token \
  ghcr.io/john-robertt/subconverter:latest
```

挂载目录而不是单个文件，可以让后续 `PUT /api/config` 使用“写临时文件 + rename 覆盖”的方式原子写回 YAML，避免保存中断时留下半文件。

如果需要额外挂载自定义模板，可以在配置文件中改成绝对路径，并将模板文件挂载进容器。

### Docker Compose 部署规划（Web 后台只读配置）

以下是 v2.0 M8-M10 的目标部署形态，当前发布不可把它视作已可用的生产后台。正式 Web 后台完成后，浏览器只访问 `web` 服务端口；`web` 容器用 nginx 托管 SPA，并同源反向代理 `/api/*`、`/generate`、`/healthz` 到 `api:8080`。

```yaml
services:
  api:
    image: ghcr.io/john-robertt/subconverter:latest
    environment:
      SUBCONVERTER_LISTEN: :8080
      SUBCONVERTER_TOKEN: your-token
    volumes:
      - ./config.yaml:/config/config.yaml:ro
    expose:
      - "8080"

  web:
    image: ghcr.io/john-robertt/subconverter-web:latest
    depends_on:
      - api
    ports:
      - "8080:80"
```

这种模式适合 GitOps 或外部系统管理配置文件的部署方式。v2.0 实现后，`api` 服务会把配置源标记为不可写，Web 后台应禁用保存入口，`PUT /api/config` 返回 `409`。

### Docker Compose 部署规划（Web 后台可编辑配置）

以下同样是 v2.0 规划示例。若正式 Web 后台需要保存配置，应挂载整个配置目录：

```yaml
services:
  api:
    image: ghcr.io/john-robertt/subconverter:latest
    environment:
      SUBCONVERTER_LISTEN: :8080
      SUBCONVERTER_TOKEN: your-token
    volumes:
      - ./config:/config
    expose:
      - "8080"

  web:
    image: ghcr.io/john-robertt/subconverter-web:latest
    depends_on:
      - api
    ports:
      - "8080:80"
```

`SUBCONVERTER_TOKEN` 只配置在 `api` 服务上。v2.0 Web 页面访问 `/api/*` 时使用 `Authorization: Bearer ...` header；复制 Clash / Surge 订阅链接时，前端再按需把 token 放入 `/generate?token=...` query。

若需要在本地源码基础上构建 Web 镜像，可将 `web.image` 替换为：

```yaml
build:
  context: ./web
```

### 健康检查

服务提供 `/healthz` 端点和 `-healthcheck` 标志两种探活方式：

- `/healthz`：HTTP 端点，返回 `200 OK`，用于负载均衡器或外部监控
- `-healthcheck`：二进制自检模式，按 `-listen` > `SUBCONVERTER_LISTEN` > `:8080` 解析监听地址后，对本地 `/healthz` 发请求并退出（退出码 0 = 健康，1 = 异常）

`-healthcheck` 不依赖容器内的 curl 等外部工具，适用于 distroless 镜像。Dockerfile 已内置 `HEALTHCHECK` 指令，Docker Compose 部署时也可显式声明：

```yaml
healthcheck:
  test: ["CMD", "/app/subconverter", "-healthcheck"]
  interval: 10s
  timeout: 3s
  retries: 20
```

镜像默认设置了 `SUBCONVERTER_LISTEN=:8080`。若服务监听非默认端口，推荐只改环境变量，让主服务和内置 `HEALTHCHECK` 自动保持一致：

```yaml
environment:
  SUBCONVERTER_LISTEN: :9090
  SUBCONVERTER_TOKEN: your-token
ports:
  - 9090:9090
healthcheck:
  test: ["CMD", "/app/subconverter", "-healthcheck"]
```

### 二进制部署

```bash
./subconverter -config ./config.yaml -listen :8080
```

若不显式传入 `-listen`，进程会按 `SUBCONVERTER_LISTEN` > `:8080` 解析监听地址。

若不显式传入 `-access-token`，进程会按 `SUBCONVERTER_TOKEN` > 空值解析访问 token；空值表示当前 `/generate` 不启用鉴权。v2.0 Admin API 实现后，空值也表示 `/api/*` 不启用鉴权。

建议生产环境使用：

- `systemd`
- 非 root 运行用户
- `Restart=always`

---

## 版本信息

发布构建会注入以下元数据：

- `version`
- `commit`
- `date`

可以通过以下命令查看：

```bash
./subconverter -version
```

---

## 发布流程

1. 确保 `main` 分支 CI 通过
2. 创建并推送 tag

```bash
git tag v0.1.0
git push origin v0.1.0
```

3. 等待 GitHub Actions 完成：

- GitHub Release 二进制上传完成
- GHCR 后端镜像与 Web 镜像推送完成

4. 在目标环境手动拉取并部署对应版本

---

## GHCR 页面描述

Release workflow 会为镜像写入 OCI 元数据：

- `org.opencontainers.image.source=https://github.com/John-Robertt/subconverter`
- 后端镜像：`org.opencontainers.image.description=Single-user HTTP service that converts SS subscriptions into Clash Meta and Surge configs.`
- Web 镜像：`org.opencontainers.image.description=Web admin UI for subconverter.`

对多架构镜像，workflow 还会把描述写入 manifest index annotation，确保 GHCR 包页面可以显示描述信息。

---

## 前端镜像（v2.0 规划）

v2.0 目标是新增 Web 管理后台（React SPA），前端源码位于 `web/` 目录。当前 `web/` 是设计原型，可用于验证 nginx 静态托管和反向代理配置，不代表 Admin API 已可用。

### 构建流程

```bash
docker build -t subconverter-web ./web
```

`web/Dockerfile` 使用 Node 阶段构建前端产物（若尚无 `package.json`，则托管当前设计原型），再用 nginx 托管 `dist/`。nginx 配置位于 `web/nginx.conf`，目标职责为：

- `/`：静态资源与 SPA fallback
- `/api/*`：反向代理到 `api:8080`
- `/generate`：反向代理到 `api:8080`
- `/healthz`：反向代理到 `api:8080`

### Release 流程变更

Release workflow 需要分别发布后端镜像与 Web 镜像：

1. `go build` / 后端 Docker build（`Dockerfile`）
2. Web Docker build（`web/Dockerfile`）

v2.0 正式交付时通过 Docker Compose 把两个镜像组合起来。后端二进制和后端镜像不包含 Web 静态资源。

---

## 开发模式（v2.0 规划）

前后端分离开发：

- **前端**：`cd web && npm run dev`（启动 Vite dev server，默认 `localhost:5173`）
- **后端**：`go run ./cmd/subconverter -config ...`

v2.0 正式前端工程接入后，推荐在 Vite 配置中把 `/api/*`、`/generate`、`/healthz` 代理到 Go 后端。若不使用 Vite proxy，可由 Go 开启 CORS middleware 允许前端 dev server 跨域请求；正式生产 Compose 部署不需要 CORS。
