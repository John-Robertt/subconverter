# 构建与部署

> 状态提示：当前源码已包含 `/generate`、`/healthz`、`/api/*` 后端接口和正式 Web 管理后台。生产主路径是单镜像单服务：先运行 `pnpm web:embed` 生成并同步 Web 嵌入产物，再由 Dockerfile 使用 Go 工具链构建托管 SPA 与 API 的单个二进制。

## 目标

本文档定义项目的 GitHub 构建、Release 发布、GHCR 镜像发布和手动部署方式。

---

## 发布产物

项目维护两类发布产物：

- GitHub Release 二进制压缩包
- GHCR Docker 镜像（单个 `subconverter` 服务，内嵌正式 Vite SPA）

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

发布按 `test → binaries → docker` 三个 job 顺序执行，前一步失败，后一步就不会开始：

1. `test`：依次运行 `go mod verify`、golangci-lint、gofmt 检查、`go test ./...`、`go test -tags webui ./...`、`go vet ./...`，检查 `internal/webui/dist/index.html` 非空，再执行 Docker 构建。
2. `binaries`：用 GoReleaser 将带 Web 资源的二进制压缩包和 `checksums.txt` 发布到 GitHub Release。
3. `docker`：构建并推送内嵌 Web SPA 的 GHCR 多架构镜像。

当前仓库中的 workflow 只在推送 tag 时触发，没有分支推送或 PR 检查。它也不运行 pnpm，不重新生成 Web 产物。入口文件非空只能证明产物存在，不能证明它与 `web/src` 一致；发布者需要先完成下方 [Web 构建](#web-构建)，并提交同步后的产物。

---

## GitHub Release 二进制

Release 包内包含：

- `subconverter` 可执行文件
- `configs/base_config.yaml`
- `configs/base_clash.yaml`
- `configs/base_surge.conf`
- `configs/base_loon.conf`

当前 GitHub Release 二进制仅发布 Linux 包。

这样解压后即可直接使用默认模板路径：

```yaml
templates:
  clash: "configs/base_clash.yaml"
  surge: "configs/base_surge.conf"
  loon: "configs/base_loon.conf"
```

注意：这些相对路径是相对于进程工作目录解析的。最稳妥的用法是在解压目录下启动程序。

---

## GHCR 镜像

镜像地址：

```text
ghcr.io/john-robertt/subconverter
```

发布 tag：

- `X.Y.Z`
- `X.Y`
- `latest`

容器内约定：

- 工作目录：`/app`
- 二进制：`/app/subconverter`
- 内置模板：`/app/configs/*`
- 内嵌 Web SPA：由 Go 二进制在 `/`、`/assets/*` 和前端路由上托管
- 外部配置挂载路径：`/config/config.yaml`

镜像默认启动命令：

```shell
/app/subconverter -config /config/config.yaml
```

镜像同时内置 `SUBCONVERTER_LISTEN=:8080`，因此默认仍监听 `:8080`。

如需为当前可用的 `/generate` 启用访问控制，可额外设置 `SUBCONVERTER_TOKEN`。客户端下载配置文件时仍需在 URL 上附带 `token` 查询参数。v2.0 Web 管理后台不复用该 token 作为后台权限；后台使用独立管理员账号和 `session_id` Cookie。正式 Web 部署需要提供可写 auth state，用于保存管理员密码哈希和持久 session；首次 setup 还需要一次性 bootstrap setup token，防止公网首次启动时被抢先初始化。

因此如果配置文件继续使用：

```yaml
templates:
  clash: "configs/base_clash.yaml"
  surge: "configs/base_surge.conf"
  loon: "configs/base_loon.conf"
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

这种模式适合 GitOps / 外部系统管理配置文件的部署方式。`api` 服务会把只读挂载识别为本地配置文件不可写，`GET /api/status` 返回 `config_source.writable=false` 与 `capabilities.config_write=false`；正式 Web 页面据此禁用保存入口。若绕过前端直接调用 `PUT /api/config`，会返回 `409 config_file_not_writable`。

注意：该示例未挂载 `/auth`，不适合作为首次 Web setup 的完整配置。需要 Web 管理后台时，请额外挂载可写 auth state（如 `-v $(pwd)/auth:/auth`）并设置 `SUBCONVERTER_SETUP_TOKEN`，或直接使用下方 Compose 示例。

### Docker 部署（后台可保存配置）

若要允许 Web 后台写回配置，可挂载整个配置目录，并确保容器内运行用户对该目录有写权限：

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

挂载目录而不是单个文件，可以让 `PUT /api/config` 使用“写临时文件 + rename 覆盖”的方式原子写回 YAML，避免保存中断时留下半文件。配置文件和目录都要允许容器运行用户写入。后台认证文件还需要独立的可写 `/auth` 挂载，完整示例见下方 Compose 部署。

如果需要额外挂载自定义模板，可以在配置文件中改成绝对路径，并将模板文件挂载进容器。

### Docker Compose Demo 部署（Web 后台可编辑配置）

若 Web 后台需要保存配置，应挂载整个配置目录，使 `PUT /api/config` 能以“临时文件 + rename”的方式原子写回。

仓库内示例文件：

```bash
pnpm web:embed
pnpm docker:build
mkdir -p config auth
cp configs/base_config.yaml config/config.yaml
pnpm compose:up
```

核心结构如下：

```yaml
services:
  subconverter:
    image: "${SUBCONVERTER_IMAGE:-subconverter:local}"
    environment:
      SUBCONVERTER_LISTEN: ":8080"
      SUBCONVERTER_TOKEN: your-token
      SUBCONVERTER_AUTH_STATE: /auth/auth.json
      SUBCONVERTER_DATA_DIR: /config
      SUBCONVERTER_SETUP_TOKEN: change-this-bootstrap-token
    volumes:
      - ./config:/config
      - ./auth:/auth
    ports:
      - "8080:8080"
```

`SUBCONVERTER_TOKEN` 只用于 `/generate` 订阅访问控制。v2.0 Web 页面访问 `/api/*` 时使用 `session_id` Cookie；复制 Clash / Surge 订阅链接时，前端调用 `GET /api/generate/link`，由后端按需把 token 写入 `/generate?token=...` query 并返回完整链接。

若 auth state 中没有管理员凭据，首次访问 `/login` 会进入 setup 流程。setup 请求必须携带 bootstrap setup token：生产部署推荐显式设置 `SUBCONVERTER_SETUP_TOKEN`，完成 setup 后移除该环境变量并重启；若未设置，服务启动时会生成一次性 32-byte URL-safe token 并只打印到服务日志，不通过 HTTP 返回。setup 会把管理员 PBKDF2 密码哈希和 session token 哈希写入 `SUBCONVERTER_AUTH_STATE` 指向的文件；该文件权限必须为 `0600`，所在目录建议只允许运行用户访问。如果该路径不可写，后台保持关闭并展示部署配置错误。Demo Compose 额外挂载 `./auth:/auth`，避免把 auth state 混入配置文件写回目录。

示例文件只引用已构建镜像，不在 Compose 中执行源码构建。默认镜像名是 `subconverter:local`，与 `pnpm docker:build` 的输出保持一致；如果要演示发布镜像，可用 `SUBCONVERTER_IMAGE=ghcr.io/john-robertt/subconverter:latest docker compose -f docker-compose.demo.yaml up -d` 覆盖。


### 健康检查

服务提供 `/healthz` 端点和 `-healthcheck` 标志两种探活方式：

- `/healthz`：HTTP 端点，成功时返回 HTTP `200` 与纯文本 `ok`，用于负载均衡器或外部监控
- `-healthcheck`：二进制自检模式，按 `-listen` > `SUBCONVERTER_LISTEN` > `:8080` 解析监听地址后，对本地 `/healthz` 发请求并退出（退出码 0 = 健康，1 = 异常）

`-healthcheck` 不依赖容器内的 curl 等外部工具，适用于 distroless 镜像。Dockerfile 已内置 `HEALTHCHECK` 指令，Docker Compose 部署时也可显式声明：

```yaml
healthcheck:
  test: ["CMD", "/app/subconverter", "-healthcheck"]
  interval: 10s
  timeout: 3s
  retries: 20
```

镜像默认设置了 `SUBCONVERTER_LISTEN=:8080` 与 `SUBCONVERTER_AUTH_STATE=/auth/auth.json`。后者意味着只要把 `/auth` 挂出到持久卷（如 demo Compose 的 `./auth:/auth`），管理员凭据和 session token 哈希就能跨容器重建保留；如需自定义路径，再在 `environment` 段显式覆盖即可。

若服务监听非默认端口，推荐只改环境变量，让主服务和内置 `HEALTHCHECK` 自动保持一致：

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

若不显式传入 `-access-token`，进程会按 `SUBCONVERTER_TOKEN` > 空值解析订阅访问 token；空值表示 `/generate` 不启用 token 鉴权。受保护的 `/api/*` 管理接口要求有效管理员 session，使用自己的认证边界。

若不显式传入 `-auth-state`，进程会按 `SUBCONVERTER_AUTH_STATE` > 默认路径解析 auth state 文件。生产部署应把该路径放在可写持久卷中；若没有管理员凭据且 auth state 不可写，首次 setup 无法完成，管理后台保持关闭。

若不显式传入 `-setup-token`，进程会按 `SUBCONVERTER_SETUP_TOKEN` > 自动生成一次性 token 解析首次 setup bootstrap token。显式配置 token 适合自动化部署；自动生成 token 适合手工首次初始化，但必须从服务日志读取。

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

1. 在发布提交上完成本地检查。涉及前端时先运行 `pnpm web:test`、`pnpm web:typecheck` 和 `pnpm web:embed`，确认同步产物已提交；当前 workflow 不替发布者执行这一步。
2. 创建并推送 tag，触发上方 [Release](#release) 流程。

```bash
git tag v0.1.0
git push origin v0.1.0
```

3. 等待 GitHub Actions 完成：

- GitHub Release 二进制上传完成
- GHCR 单镜像推送完成

4. 在目标环境手动拉取并部署对应版本

---

## GHCR 页面描述

Release workflow 会为镜像写入 OCI 元数据：

- `org.opencontainers.image.source=https://github.com/John-Robertt/subconverter`
- 镜像：`org.opencontainers.image.description=Single-user HTTP service that converts proxy subscriptions into Clash Meta and Surge configs.`

对多架构镜像，workflow 还会把描述写入 manifest index annotation，确保 GHCR 包页面可以显示描述信息。

---

## Web 构建

Web 管理后台前端源码位于 `web/` 目录。生产路径不发布独立 Web 服务；`pnpm web:embed` 会构建 `web/dist` 并同步到 `internal/webui/dist`，根 Dockerfile 只使用该已同步产物和 Go 工具链构建镜像。

### 构建流程

```bash
pnpm install
pnpm web:test
pnpm web:embed
docker build -t subconverter:local .
```

根 `Dockerfile` 的目标职责为：

- 使用已提交或已同步的 `internal/webui/dist`
- Go 构建阶段使用 `-tags webui` 将 Web 静态资源嵌入二进制
- distroless runtime 仅包含 `/app/subconverter` 和底版模板

缓存头策略应与 API 契约保持一致：

- `/api/*`、`/api/generate/preview` 和 `/generate` 属于敏感响应，必须由 Go 服务设置 `Cache-Control: no-store`
- SPA 入口 `index.html` 使用 `Cache-Control: no-cache` 或等价重验证策略，确保发布后能及时发现新构建
- Vite 生成的带 hash 静态资源可使用长期缓存，例如 `Cache-Control: public, max-age=31536000, immutable`

### 构建与发布的关系

Web 源码先在本地构建并同步，Go 构建再将 `internal/webui/dist` 嵌入二进制。Release workflow 使用提交中的这份产物，具体检查顺序见上方 [Release](#release)。因此，修改 `web/src` 后只提交源码，会让发布构建继续使用此前提交的 Web 产物。

Docker Compose 只启动一个 `subconverter` 服务。默认 Go 二进制构建不要求 Web 静态资源；生产 Docker 构建使用 `webui` build tag 嵌入资源。

---

## 开发模式

前后端分离开发：

- **前端**：`pnpm --filter subconverter-web dev`（启动 Vite dev server，默认 `localhost:5173`）
- **后端**：`go run ./cmd/subconverter -config ...`

Vite 配置已把 `/api/*`、`/generate`、`/healthz` 代理到 Go 后端。Cookie session 依赖同源语义，本地调试后台登录也应优先使用 Vite proxy；正式生产 Compose 部署不需要 CORS。

本地开发若不想预先准备管理员凭据，可删除临时 auth state 后重新走 `/login` setup，并从启动日志读取自动生成的 setup token；不要在公网部署中使用临时或弱密码。


## Web 正文版本文件

正文默认直接写入 `/config`，复用配置目录挂载，不额外创建正文子目录。每次保存生成带 UTC 时间戳的新文件并保留旧版，主配置记录引用。容器运行用户需要对 `/config` 有创建、写入和删除文件的权限；目录写入失败时 Web 提示具体路径并保留草稿。文件格式、权限及版本清理规则见 [配置规约](design/config-schema.md#templates)。

本地开发可用 `-data-dir ./bin/acceptance`，或设置 `SUBCONVERTER_DATA_DIR`；命令行参数优先，默认值为 `/config`。该目录需要随主配置一起持久化。导出 ZIP 会嵌入当前正文，适合跨机器迁移；单独的生效 YAML 仍引用版本文件。

主配置可写性契约保持原样：HTTP(S) 主配置只读，本地配置文件或其目录不可写时 Web 禁用编辑。仅正文存储目录不可写而主配置可写时，保存返回 `409 resource_content_write_failed`，修复目录权限后重试。
