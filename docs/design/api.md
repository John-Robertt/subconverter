# HTTP API 设计

## 目标

本文件定义系统对外暴露的 HTTP 接口、输入参数和错误语义。系统按单用户模式运行，请求不携带配置文件。

---

## 接口列表

### `GET /generate`

用途：

- 生成目标客户端配置文件

查询参数：

- `format=clash|surge`

成功响应：

- `format=clash` 时返回 YAML 文本
- `format=surge` 时返回 conf 文本

响应头：

- Clash Meta：`Content-Type: text/yaml; charset=utf-8`
- Surge：`Content-Type: text/plain; charset=utf-8`

### `GET /healthz`

用途：

- 进程健康检查

成功响应：

- `200 OK`

---

## 错误语义

| 状态码 | 场景 |
|------|------|
| `400` | 请求参数非法，或配置校验失败 |
| `502` | 订阅拉取失败 |
| `500` | 内部处理或渲染失败 |

设计意图：

- 请求本身错误与远端依赖错误分开表达
- 配置结构错误视为服务当前不可正确生成配置，但仍归类为可识别输入错误

---

## 请求处理流程

`/generate` 的典型处理顺序：

1. 校验 `format`
2. 读取内存中的配置对象
3. 执行管道生成中间表示
4. 根据 `format` 选择渲染器
5. 若 `format=surge` 且配置了 `base_url`，将 `base_url` + 请求路径拼接为 managed URL 传入渲染器
6. 返回配置文本

---

## 运行参数

系统支持以下启动参数：

- `-config`：YAML 配置文件路径或 HTTP(S) URL（必填，除 `-healthcheck` / `-version` 模式外）
- `-listen`：HTTP 监听地址（默认 `:8080`）
- `-cache-ttl`：订阅、模板和远程配置文件的缓存 TTL（默认 `5m`）
- `-timeout`：拉取订阅的 HTTP 超时时间（默认 `30s`）
- `-healthcheck`：按监听地址解析规则向本地 `/healthz` 发起探活请求并退出（退出码 0 = 健康，1 = 异常）
- `-version`：打印版本信息并退出

监听地址解析规则：显式 `-listen` > `SUBCONVERTER_LISTEN` > `:8080`。`-healthcheck` 与主服务启动共用这套规则。

这些参数属于进程运行时控制，不属于用户 YAML 配置的一部分。
