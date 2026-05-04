# SubConverter

将 Shadowsocks 订阅、Snell 节点源与 VLESS 节点源转换为 Clash Meta 和 Surge 代理配置的单二进制 HTTP 服务。

SubConverter 读取声明式 YAML 配置，拉取 SS 订阅源、Snell 来源和 VLESS 来源，执行过滤与分组，定义服务路由策略，最终渲染为 Clash Meta YAML 或 Surge conf。两种输出尽量共享同一语义；若目标格式不支持某类协议，则按格式能力处理（如 Snell 仅进入 Surge 输出，VLESS 仅进入 Clash 输出）。

## 功能特性

- **多订阅源** — 支持多个 SS 订阅源合并，跨订阅自动去重
- **Snell 来源** — 支持拉取纯文本 Snell 节点清单；Snell 节点仅进入 Surge 输出，Clash 会做级联过滤
- **VLESS 来源** — 支持拉取纯文本 VLESS URI 清单；VLESS 节点仅进入 Clash 输出，Surge 会做级联过滤；`type` 缺失或未知值回落到 `tcp`，`encryption` 非空时透传
- **SIP002 兼容** — 支持常见 SS URI 变体，包括 base64/plain userinfo、query 参数和 plugin 声明
- **自定义代理** — 在订阅之外定义 socks5/http 代理节点（如专线、ISP 代理）
- **链式代理** — 通过 `relay_through` 配置上游中转，支持三种选择模式：`group`、`select`、`all`
- **节点过滤** — 正则排除拉取类节点（订阅 + Snell + VLESS，如到期提醒、流量统计等信息性节点）
- **地区分组** — 按正则将拉取类节点组织为地区组，支持 `select`（手选）或 `url-test`（自动选延迟最低）策略
- **服务路由** — 为每个服务定义出口偏好顺序（Telegram、Netflix、YouTube 等），`@auto` 自动补充节点组、包含 `@all` 的服务组和 `DIRECT`，`REJECT` 需手动声明
- **远程规则集** — 将规则集 URL 绑定到路由组（URL 透传，由客户端运行时拉取）
- **内联规则** — 直接编写路由规则，与远程规则集并用
- **双格式输出** — 同一份配置生成 Clash Meta YAML 和 Surge conf；格式不支持的协议按能力差异处理
- **Web 管理后台** — 通过同源 SPA 管理配置、预览节点/分组、生成订阅链接
- **单镜像部署** — Docker 构建时嵌入 `web/dist`，由 Go 二进制直接托管 Web、`/api/*`、`/generate` 和 `/healthz`
- **模板合并** — 将生成内容注入底版模板，保留 DNS/TUN 等通用设置
- **缓存** — 基于 TTL 的内存缓存，加速订阅和模板拉取
- **优雅关闭** — 正确处理 SIGINT/SIGTERM 信号

## 安装

SubConverter 可以通过 Docker 镜像、预编译二进制或源码构建安装。运行前需准备一份 YAML 配置文件，格式参见下方[配置](#配置)章节或 [`configs/base_config.yaml`](configs/base_config.yaml)。`-config` 同时支持本地路径和 HTTP(S) URL。

### Docker 镜像

使用已发布镜像：

```bash
docker pull ghcr.io/john-robertt/subconverter:latest
```

从源码构建完整 Web 发布镜像：

```bash
pnpm docker:build
```

根 Dockerfile 会在 Docker 内使用 pnpm 构建 `web/dist`，再用 `webui` build tag 将静态资源嵌入 Go 二进制。构建产物默认命名为 `subconverter:local`。

### 预编译二进制

从 [GitHub Releases](https://github.com/John-Robertt/subconverter/releases) 下载（支持 linux/amd64、linux/arm64）：

```bash
tar xzf subconverter_*.tar.gz
./subconverter -config ./config.yaml -listen :8080
```

发布包中包含 `configs/` 目录下的底版模板。

### 源码构建

Go-only 二进制只需要 Go 1.24+，不会嵌入 Web 管理后台，适合本地开发、测试和命令行服务验证：

```bash
git clone https://github.com/John-Robertt/subconverter.git
cd subconverter
make build
./subconverter -config ./configs/base_config.yaml
```

前端开发、测试和生产镜像构建脚本由 pnpm workspace 管理：

```bash
pnpm install
pnpm web:dev
pnpm web:test
pnpm docker:build
```

## 部署

### Docker Compose（推荐）

v2.0 Web 部署使用单服务单镜像。浏览器、Admin API、`/generate` 和 `/healthz` 都访问同一个 `:8080`，生产路径不需要 CORS。

Demo Compose 使用可写配置目录，适合 Web 后台保存完整配置：

```bash
pnpm docker:build
mkdir -p config auth
cp configs/base_config.yaml config/config.yaml
pnpm compose:up
```

`docker-compose.demo.yaml` 只引用已构建好的镜像，默认镜像名与 `pnpm docker:build` 产物一致：`subconverter:local`。需要演示发布镜像时，可用 `SUBCONVERTER_IMAGE=ghcr.io/john-robertt/subconverter:latest docker compose -f docker-compose.demo.yaml up -d` 覆盖。

`SUBCONVERTER_TOKEN` 只保护 `/generate` 客户端订阅访问；Web 管理后台使用管理员账号和 `session_id` Cookie。首次 setup 需要 `SUBCONVERTER_SETUP_TOKEN`，生产环境应改掉示例中的默认值。

### Docker Run

单容器运行适合只提供 `/generate` 和 `/healthz`，或使用远程配置的只读部署：

```bash
docker run -d \
  --name subconverter \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/config/config.yaml:ro \
  ghcr.io/john-robertt/subconverter:latest
```

容器默认读取 `/config/config.yaml`，内置底版模板位于 `/app/configs/`。也可以跳过挂载，直接使用远程配置：

```bash
docker run -d \
  --name subconverter \
  -p 8080:8080 \
  ghcr.io/john-robertt/subconverter:latest \
  -config https://example.com/config.yaml
```

## 使用

### Web 管理后台

部署后访问 `http://localhost:8080/login`。首次进入时使用 `SUBCONVERTER_SETUP_TOKEN` 完成管理员 setup，然后用管理员账号登录。

登录后可以在 Web 管理后台编辑配置、保存到挂载的 `./config/config.yaml`、预览节点和分组，并复制 Clash / Surge 订阅链接。

### 订阅生成接口

直接生成 Clash Meta 或 Surge 配置：

```bash
curl "http://localhost:8080/generate?format=clash"
curl "http://localhost:8080/generate?format=surge"
curl "http://localhost:8080/generate?format=surge&filename=my-profile"
```

如果配置了 `SUBCONVERTER_TOKEN` 或 `-access-token`，客户端请求 `/generate` 时必须带上 `token` query：

```bash
curl "http://localhost:8080/generate?format=clash&token=your-token"
```

### 健康检查

```bash
curl "http://localhost:8080/healthz"
```

## 命令行参数

| 参数            | 默认值     | 说明                                                                                        |
| --------------- | ---------- | ------------------------------------------------------------------------------------------- |
| `-config`       | _（必填）_ | YAML 配置文件路径或 HTTP(S) URL                                                             |
| `-listen`       | `:8080`    | HTTP 监听地址；未显式传入时可由 `SUBCONVERTER_LISTEN` 提供                                  |
| `-cache-ttl`    | `5m`       | 订阅和模板缓存的 TTL                                                                        |
| `-timeout`      | `30s`      | 拉取订阅的 HTTP 超时时间                                                                    |
| `-access-token` | _空_       | `/generate` 访问 token；未显式传入时可由 `SUBCONVERTER_TOKEN` 提供                          |
| `-auth-state`   | 系统配置目录 | 管理员密码哈希与 session 状态文件；未显式传入时可由 `SUBCONVERTER_AUTH_STATE` 提供          |
| `-setup-token`  | 自动生成   | 首次 setup bootstrap token；未显式传入时可由 `SUBCONVERTER_SETUP_TOKEN` 提供                |
| `-cors`         | `false`    | 仅本地开发调试使用；生产 Compose 由同一个 Go 服务同源托管 SPA 和 API，不需要 CORS           |
| `-healthcheck`  |            | 按 `-listen` > `SUBCONVERTER_LISTEN` > `:8080` 解析监听地址后，对本地 `/healthz` 探活并退出 |
| `-version`      |            | 打印版本信息并退出                                                                          |

## API

### `GET /generate?format=clash|surge`

生成代理配置文件。

- **查询参数**：`format`（必填）— `clash` 或 `surge`；`token`（当服务端配置 `-access-token` / `SUBCONVERTER_TOKEN` 时必填）；`filename`（可选，自定义下载文件名）
- **默认文件名**：`clash.yaml`、`surge.conf`；未显式传入 `filename` 时也会作为下载文件名返回
- **文件名约束**：`filename` 仅允许 ASCII 字母、数字、`.`、`-`、`_`；非法值直接返回 `400`
- **成功**：返回生成的配置文本
- **响应头**：返回 `Content-Disposition: attachment; ...`，浏览器和下载器会使用最终文件名
- **错误响应**：返回中文纯文本；已分类错误会直接说明问题，未分类内部错误统一返回 `内部错误`
- **SS plugin 支持**：Clash Meta 通用透传 SS plugin；Surge 仅支持可映射的 obfs 类 SS plugin，不支持的 plugin 会返回 `500`
- **错误码**：`400` — 参数无效，或配置语义 / 图校验失败；`401` — 缺少 token 或 token 不匹配；`502` — 远程资源拉取失败，或远程订阅内容无效（如 0 个有效节点）；`500` — 本地资源读取失败、内部处理或渲染错误

如果配置了 `base_url`，Surge 输出中的 `#!MANAGED-CONFIG` 会自动继承当前请求的 `token` 和最终 `filename`，保证客户端后续自动更新仍能访问同一 URL。

### `GET /healthz`

返回 `200 OK`。用于容器健康检查和负载均衡器探针。

在容器场景下，内置 `-healthcheck` 与主服务启动共用监听地址解析规则：显式 `-listen` > `SUBCONVERTER_LISTEN` > `:8080`。

## 配置

SubConverter 使用单个 YAML 文件声明全部输入：订阅源、Snell 来源、VLESS 来源、自定义代理、节点分组、服务路由、规则集和兜底策略。

```yaml
base_url: "https://my-server.com" # 可选：用于 Surge 托管配置头

templates:
  clash: "configs/base_clash.yaml" # Clash Meta 输出的底版模板
  surge: "configs/base_surge.conf" # Surge 输出的底版模板

sources:
  subscriptions:
    - url: "https://sub.example.com/api/v1/client/subscribe?token=xxx" # 订阅体应为 base64 编码的 SS URI 列表（SIP002 兼容）
  snell:
    - url: "https://my-server.com/snell-nodes.txt" # 纯文本 Snell 节点清单；单行失败会报脱敏 URL + 物理行号
  vless:
    - url: "https://my-server.com/vless-nodes.txt" # 纯文本 VLESS URI 清单；单行失败会报脱敏 URL + 物理行号
  custom_proxies:
    - name: 🔗 HK-ISP # 声明 relay_through 时 name 即链式组名（原样使用，需要视觉前缀自行写入）
      url: socks5://user:pass@154.197.1.1:45002 # 支持 ss:// / socks5:// / http://；SS URI 中的 #fragment 会被忽略
      relay_through: # 可选：声明后 cp 仅作链式模板，不产生独立代理
        type: group
        name: "🇭🇰 Hong Kong"
        strategy: select

filters:
  exclude: "(过期|剩余流量)" # 正则排除拉取类节点（订阅 + Snell + VLESS）

groups:
  🇭🇰 Hong Kong: { match: "(港|HK|Hong Kong)", strategy: select }
  🇯🇵 Japan: { match: "(日本|JP|Japan)", strategy: url-test }

routing:
  🚀 快速选择: ["@auto"] # @auto 自动补充：全部节点组 + 包含 @all 的服务组 + DIRECT（每个 entry 最多一次）
  🚀 手动切换: ["@all"] # @all 展开为全部原始节点（订阅 + Snell + VLESS + 不带 relay_through 的自定义代理；不含链式节点）
  📲 Telegram: [🇭🇰 Hong Kong, 🚀 快速选择, "@auto", REJECT] # REJECT 不在 @auto 中，按需要手动添加
  🐟 FINAL: ["@auto"]

rulesets:
  📲 Telegram:
    - "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Telegram.list"
  📺 Netflix:
    - "https://example.com/Netflix.list"

# rulesets 只接受字符串 URL
# 远程内容必须是纯文本规则集，而不是 Clash payload YAML

rules:
  - "GEOIP,CN,🎯 China"

fallback: 🐟 FINAL # 未匹配任何规则的流量走这里
```

### 配置段说明

| 配置段      | 用途                                | 必填 |
| ----------- | ----------------------------------- | ---- |
| `base_url`  | 外部服务地址，用于 Surge 托管配置头 | 否   |
| `templates` | Clash Meta / Surge 输出的底版模板   | 否   |
| `sources`   | 订阅 URL、Snell/VLESS 来源和自定义代理定义 | 是   |
| `filters`   | 正则排除节点                        | 否   |
| `groups`    | 地区节点组，含匹配正则和策略        | 是   |
| `routing`   | 服务路由组，含出口偏好顺序          | 是   |
| `rulesets`  | 远程规则集 URL 绑定到路由组         | 否   |
| `rules`     | 内联路由规则                        | 否   |
| `fallback`  | 兜底路由组，处理未匹配流量          | 是   |

完整注释示例参见 [`configs/base_config.yaml`](configs/base_config.yaml)，字段详细说明参见 [`docs/design/config-schema.md`](docs/design/config-schema.md)。

## 架构

SubConverter 的发布镜像、HTTP 路由和配置生成管道保持单向依赖：

```
发布期:
pnpm --filter subconverter-web build --> web/dist --> go build -tags webui --> single binary

HTTP:
Go server --> /, /login, /sources, /assets/*
          --> /api/*
          --> /generate
          --> /healthz

启动期:
LoadConfig --> Prepare (produces RuntimeConfig)

请求期:
Build(Source --> Filter --> Group --> Route --> ValidateGraph)
  --> Target --> Render --> Clash Meta / Surge conf
```

生产镜像在构建期嵌入 Web 静态资源，运行时由同一个 Go 进程托管 SPA、Admin API、订阅生成和健康检查。启动期通过 `Prepare` 完成静态校验、正则编译、URL 解析和 `@auto` 展开，产出不可变的 `RuntimeConfig`。请求期管道在此基础上拉取订阅、构建格式无关的中间表示（`model.Pipeline`），再由 Target/Render 投影为目标格式输出。

详细架构说明参见 [`docs/architecture.md`](docs/architecture.md)。

## 项目结构

```
subconverter/
  Dockerfile             单镜像发布构建：pnpm 构建 Web，再用 Go 嵌入并输出 distroless runtime
  docker-compose.demo.yaml
                         运行已构建镜像的单服务 Demo Compose
  package.json           pnpm workspace 脚本入口
  pnpm-workspace.yaml    pnpm workspace 定义
  cmd/subconverter/      入口、命令行参数、优雅关闭
  internal/
    admin/               Web 管理后台 API handler
    app/                 配置读写、状态、预览和生成应用服务
    auth/                管理员 setup、登录、session 状态
    config/              YAML 解析、有序 Map、静态校验、启动期预计算（Prepare → RuntimeConfig）
    errtype/             类型化错误（Config、Fetch、Resource、Build、Render）
    fetch/               HTTP 拉取器、TTL 缓存、资源加载
    generate/            生成订阅配置的应用用例
    model/               格式无关的中间表示
    pipeline/            处理阶段：source、filter、group、route、validate
    render/              Clash Meta YAML 和 Surge conf 渲染器
    server/              HTTP 处理器和错误映射
    webui/               可选嵌入 Web 静态资源；默认 Go 构建为空，Docker 发布用 `webui` tag
  web/                   Vite + React + TypeScript 管理后台
  configs/               示例配置和底版模板
  testdata/              测试夹具（配置、订阅、期望输出）
  docs/                  架构和设计文档
```

## 文档

| 文档                                        | 说明                             |
| ------------------------------------------- | -------------------------------- |
| [架构总览](docs/architecture.md)            | 系统概览、模块边界、关键决策     |
| [产品规格](docs/product-spec.md)            | 需求定义、输入输出设计、使用场景 |
| [部署指南](docs/deployment.md)              | Docker、二进制、CI/CD 和发布流程 |
| [配置 Schema](docs/design/config-schema.md) | 完整字段参考                     |
| [领域模型](docs/design/domain-model.md)     | 核心实体和不变量                 |
| [流水线](docs/design/pipeline.md)           | 逐阶段处理细节                   |
| [渲染](docs/design/rendering.md)            | Clash Meta 和 Surge 输出映射     |
| [API](docs/design/api.md)                   | HTTP 端点和错误语义              |
| [校验](docs/design/validation.md)           | 配置校验和图校验规则             |
| [缓存](docs/design/caching.md)              | 订阅和模板缓存行为               |
