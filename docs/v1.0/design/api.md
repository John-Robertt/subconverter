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
- `token=<access-token>`（仅当服务端配置了访问 token 时必填）
- `filename=<custom-name>`（可选；未传时默认 `clash.yaml` / `surge.conf`；仅允许 ASCII 字母、数字、`.`、`-`、`_`）

成功响应：

- `format=clash` 时返回 YAML 文本
- `format=surge` 时返回 conf 文本

响应头：

- Clash Meta：`Content-Type: text/yaml; charset=utf-8`
- Surge：`Content-Type: text/plain; charset=utf-8`
- 两种格式都会输出 `Content-Disposition: attachment; ...`，默认文件名分别为 `clash.yaml`、`surge.conf`

错误响应：

- 统一返回 `text/plain; charset=utf-8`
- 错误正文为中文纯文本
- 已分类错误返回可定位问题的说明
- 未分类内部错误统一返回 `内部错误`

### `GET /healthz`

用途：

- 进程健康检查

成功响应：

- `200 OK`

---

## 错误语义

| 状态码 | 场景 |
|------|------|
| `400` | 请求参数非法，或配置语义 / 图校验失败 |
| `401` | 缺少 token，或 token 不匹配 |
| `502` | 远程资源拉取失败，或远程订阅内容不可用 |
| `500` | 本地资源读取失败，或内部处理 / 渲染失败 |

设计意图：

- 请求本身错误与远端依赖错误分开表达
- 用户可修复的配置错误（静态校验、图校验、路由语义错误）统一归为 `400`
- 远端资源抓取失败或内容不可解析归为 `502`
- 本地模板等资源读取失败归为 `500`

---

## 请求处理流程

`/generate` 的典型处理顺序：

1. 校验 `format`
2. 若服务端配置了访问 token，校验 `token`
3. 校验并规范化 `filename`
4. 调用生成服务执行 `Build -> Target -> Render`
5. 若 `format=surge` 且配置了 `base_url`，由生成服务组装 managed URL：`<base_url>/generate?format=surge[&token=...][&filename=...]`
6. 返回配置文本

---

## 运行参数

系统支持以下启动参数：

- `-config`：YAML 配置文件路径或 HTTP(S) URL（必填，除 `-healthcheck` / `-version` 模式外）
- `-listen`：HTTP 监听地址（默认 `:8080`）
- `-cache-ttl`：订阅、模板和远程配置文件的缓存 TTL（默认 `5m`）
- `-timeout`：拉取订阅的 HTTP 超时时间（默认 `30s`）
- `-access-token`：为 `/generate` 启用访问 token（默认空；空值表示不鉴权）
- `-healthcheck`：按监听地址解析规则向本地 `/healthz` 发起探活请求并退出（退出码 0 = 健康，1 = 异常）
- `-version`：打印版本信息并退出

监听地址解析规则：显式 `-listen` > `SUBCONVERTER_LISTEN` > `:8080`。`-healthcheck` 与主服务启动共用这套规则。

这些参数属于进程运行时控制，不属于用户 YAML 配置的一部分。
