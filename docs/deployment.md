# 构建与部署

## 目标

本文档定义项目的 GitHub 构建、Release 发布、GHCR 镜像发布和手动部署方式。

---

## 发布产物

项目维护两类发布产物：

- GitHub Release 二进制压缩包
- GHCR Docker 镜像

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
- 构建并推送 GHCR 多架构镜像

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

## GHCR 镜像

镜像地址：

```text
ghcr.io/john-robertt/subconverter
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

因此如果配置文件继续使用：

```yaml
templates:
  clash: "configs/base_clash.yaml"
  surge: "configs/base_surge.conf"
```

容器内也能正常解析到镜像内置模板。

---

## 手动部署

### Docker 部署

```bash
docker run -d \
  --name subconverter \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/config/config.yaml:ro \
  ghcr.io/john-robertt/subconverter:latest
```

如果需要额外挂载自定义模板，可以在配置文件中改成绝对路径，并将模板文件挂载进容器。

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
- GHCR 镜像推送完成

4. 在目标环境手动拉取并部署对应版本

---

## GHCR 页面描述

Release workflow 会为镜像写入 OCI 元数据：

- `org.opencontainers.image.source=https://github.com/John-Robertt/subconverter`
- `org.opencontainers.image.description=Single-user HTTP service that converts SS subscriptions into Clash Meta and Surge configs.`

对多架构镜像，workflow 还会把描述写入 manifest index annotation，确保 GHCR 包页面可以显示描述信息。
