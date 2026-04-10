# SubConverter

将 Shadowsocks 订阅转换为 Clash Meta 和 Surge 代理配置的单二进制 HTTP 服务。

SubConverter 读取声明式 YAML 配置，拉取 SS 订阅源，执行过滤与分组，定义服务路由策略，最终渲染为 Clash Meta YAML 或 Surge conf——两种格式语义等价，仅语法不同。

## 功能特性

- **多订阅源** — 支持多个 SS 订阅源合并，跨订阅自动去重
- **SIP002 兼容** — 支持常见 SS URI 变体，包括 base64/plain userinfo、query 参数和 plugin 声明
- **自定义代理** — 在订阅之外定义 socks5/http 代理节点（如专线、ISP 代理）
- **链式代理** — 通过 `relay_through` 配置上游中转，支持三种选择模式：`group`、`select`、`all`
- **节点过滤** — 正则排除订阅节点（如到期提醒、流量统计等信息性节点）
- **地区分组** — 按正则将节点组织为地区组，支持 `select`（手选）或 `url-test`（自动选延迟最低）策略
- **服务路由** — 为每个服务定义出口偏好顺序（Telegram、Netflix、YouTube 等），`@auto` 自动补充节点组、`@all` 服务组和 `DIRECT`，`REJECT` 需手动声明
- **远程规则集** — 将规则集 URL 绑定到路由组（URL 透传，由客户端运行时拉取）
- **内联规则** — 直接编写路由规则，与远程规则集并用
- **双格式输出** — 同一份配置生成 Clash Meta YAML 和 Surge conf
- **模板合并** — 将生成内容注入底版模板，保留 DNS/TUN 等通用设置
- **缓存** — 基于 TTL 的内存缓存，加速订阅和模板拉取
- **优雅关闭** — 正确处理 SIGINT/SIGTERM 信号

## 快速开始

运行前需准备一份 YAML 配置文件，格式参见下方[配置](#配置)章节或 [`configs/base_config.yaml`](configs/base_config.yaml)。`-config` 同时支持本地路径和 HTTP(S) URL。

### Docker

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

验证服务：

```bash
curl "http://localhost:8080/generate?format=clash"
curl "http://localhost:8080/generate?format=surge"
curl "http://localhost:8080/generate?format=surge&filename=my-profile"
curl "http://localhost:8080/healthz"
```

### Docker Compose

```yaml
services:
  subconverter:
    image: ghcr.io/john-robertt/subconverter:latest
    environment:
      SUBCONVERTER_LISTEN: :8080
    ports:
      - 8080:8080
    restart: unless-stopped
    volumes:
      - ./config.yaml:/config/config.yaml:ro
    healthcheck:
      test: ["CMD", "/app/subconverter", "-healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 20
```

```bash
docker compose up -d
```

使用远程配置时，去掉 `volumes` 挂载并覆盖启动命令：

```yaml
services:
  subconverter:
    image: ghcr.io/john-robertt/subconverter:latest
    command: ["-config", "https://example.com/config.yaml"]
    environment:
      SUBCONVERTER_LISTEN: :8080
    ports:
      - 8080:8080
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/app/subconverter", "-healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 20
```

修改容器监听端口时，推荐只改一次 `SUBCONVERTER_LISTEN`。主服务和内置 `-healthcheck` 都会按相同优先级解析监听地址：显式 `-listen` > `SUBCONVERTER_LISTEN` > `:8080`。

### 预编译二进制

从 [GitHub Releases](https://github.com/John-Robertt/subconverter/releases) 下载（支持 linux/amd64、linux/arm64）：

```bash
tar xzf subconverter_*.tar.gz
./subconverter -config ./config.yaml -listen :8080
```

发布包中包含 `configs/` 目录下的底版模板。

### 从源码构建

需要 Go 1.24+。

```bash
git clone https://github.com/John-Robertt/subconverter.git
cd subconverter
make build
./subconverter -config ./configs/base_config.yaml
```

## 命令行参数

| 参数           | 默认值     | 说明                                                                                        |
| -------------- | ---------- | ------------------------------------------------------------------------------------------- |
| `-config`      | _（必填）_ | YAML 配置文件路径或 HTTP(S) URL                                                             |
| `-listen`      | `:8080`    | HTTP 监听地址；未显式传入时可由 `SUBCONVERTER_LISTEN` 提供                                  |
| `-cache-ttl`   | `5m`       | 订阅和模板缓存的 TTL                                                                        |
| `-timeout`     | `30s`      | 拉取订阅的 HTTP 超时时间                                                                    |
| `-access-token` | _空_      | `/generate` 访问 token；未显式传入时可由 `SUBCONVERTER_TOKEN` 提供                          |
| `-healthcheck` |            | 按 `-listen` > `SUBCONVERTER_LISTEN` > `:8080` 解析监听地址后，对本地 `/healthz` 探活并退出 |
| `-version`     |            | 打印版本信息并退出                                                                          |

## API

### `GET /generate?format=clash|surge`

生成代理配置文件。

- **查询参数**：`format`（必填）— `clash` 或 `surge`；`token`（当服务端配置 `-access-token` / `SUBCONVERTER_TOKEN` 时必填）；`filename`（可选，自定义下载文件名）
- **默认文件名**：`clash.yaml`、`surge.conf`；未显式传入 `filename` 时也会作为下载文件名返回
- **文件名约束**：`filename` 仅允许 ASCII 字母、数字、`.`、`-`、`_`；非法值直接返回 `400`
- **成功**：返回生成的配置文本
- **响应头**：返回 `Content-Disposition: attachment; ...`，浏览器和下载器会使用最终文件名
- **SS plugin 支持**：Clash Meta 通用透传 SS plugin；Surge 仅支持可映射的 obfs 类 SS plugin，不支持的 plugin 会返回 `500`
- **错误码**：`400` — 参数无效，或配置语义 / 图校验失败；`401` — 缺少 token 或 token 不匹配；`502` — 订阅拉取失败，或订阅内容无效（如 0 个有效节点）；`500` — 内部处理或渲染错误

如果配置了 `base_url`，Surge 输出中的 `#!MANAGED-CONFIG` 会自动继承当前请求的 `token` 和最终 `filename`，保证客户端后续自动更新仍能访问同一 URL。

### `GET /healthz`

返回 `200 OK`。用于容器健康检查和负载均衡器探针。

在容器场景下，内置 `-healthcheck` 与主服务启动共用监听地址解析规则：显式 `-listen` > `SUBCONVERTER_LISTEN` > `:8080`。

## 配置

SubConverter 使用单个 YAML 文件声明全部输入：订阅源、自定义代理、节点分组、服务路由、规则集和兜底策略。

```yaml
base_url: "https://my-server.com" # 可选：用于 Surge 托管配置头

templates:
  clash: "configs/base_clash.yaml" # Clash Meta 输出的底版模板
  surge: "configs/base_surge.conf" # Surge 输出的底版模板

sources:
  subscriptions:
    - url: "https://sub.example.com/api/v1/client/subscribe?token=xxx" # 订阅体应为 base64 编码的 SS URI 列表（SIP002 兼容）
  custom_proxies:
    - name: HK-ISP
      type: socks5
      server: 154.197.1.1
      port: 45002
      username: user
      password: pass
      relay_through: # 可选：链式代理
        type: group
        name: "🇭🇰 Hong Kong"
        strategy: select

filters:
  exclude: "(过期|剩余流量)" # 正则排除订阅节点

groups:
  🇭🇰 Hong Kong: { match: "(港|HK|Hong Kong)", strategy: select }
  🇯🇵 Japan: { match: "(日本|JP|Japan)", strategy: url-test }

routing:
  🚀 快速选择: ["@auto"]          # @auto 自动补充：全部节点组 + @all 服务组 + DIRECT（每个 entry 最多一次）
  🚀 手动切换: ["@all"]           # @all 展开为全部原始节点（不含链式节点）
  📲 Telegram: [🇭🇰 Hong Kong, 🚀 快速选择, "@auto", REJECT]  # REJECT 不在 @auto 中，按需要手动添加
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
| `sources`   | 订阅 URL 和自定义代理定义           | 是   |
| `filters`   | 正则排除节点                        | 否   |
| `groups`    | 地区节点组，含匹配正则和策略        | 是   |
| `routing`   | 服务路由组，含出口偏好顺序          | 是   |
| `rulesets`  | 远程规则集 URL 绑定到路由组         | 否   |
| `rules`     | 内联路由规则                        | 否   |
| `fallback`  | 兜底路由组，处理未匹配流量          | 是   |

完整注释示例参见 [`configs/base_config.yaml`](configs/base_config.yaml)，字段详细说明参见 [`docs/design/config-schema.md`](docs/design/config-schema.md)。

## 架构

SubConverter 通过 8 阶段流水线处理配置：

```
YAML 配置
    |
    v
LoadConfig --> ValidateConfig --> Source --> Filter
                                               |
                                               v
Clash Meta <-- Render <-- ValidateGraph <-- Route <-- Group
Surge conf
```

每个阶段变换或充实一个共享的中间表示（`model.Pipeline`）。渲染器仅依赖这一格式无关的模型，不依赖配置类型，从而保证 Clash Meta 和 Surge 输出的语义一致性。

详细架构说明参见 [`docs/architecture.md`](docs/architecture.md)。

## 项目结构

```
subconverter/
  cmd/subconverter/      入口、命令行参数、优雅关闭
  internal/
    config/              YAML 解析、有序 Map、静态校验
    errtype/             类型化错误（Config、Fetch、Build、Render）
    fetch/               HTTP 拉取器、TTL 缓存、资源加载
    model/               格式无关的中间表示
    pipeline/            处理阶段：source、filter、group、route、validate
    render/              Clash Meta YAML 和 Surge conf 渲染器
    server/              HTTP 处理器和错误映射
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
