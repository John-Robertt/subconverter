# 项目结构建议

## 目标

本文件定义代码目录和包职责，作为实现阶段的结构约束。

---

## 推荐目录

```text
subconverter/
├── cmd/subconverter/
│   └── main.go
├── internal/
│   ├── config/
│   ├── errtype/
│   ├── fetch/
│   ├── app/
│   ├── admin/
│   ├── auth/
│   ├── generate/
│   ├── model/
│   ├── pipeline/
│   ├── proxyparse/
│   ├── render/
│   ├── ssparse/
│   ├── server/
│   └── target/
├── configs/
│   ├── base_config.yaml
│   ├── base_clash.yaml
│   └── base_surge.conf
├── testdata/
└── docs/
```

---

## 包职责

`internal/config`

- 配置结构（含 `Templates` 底版模板声明）
- YAML 加载（支持本地路径和远程 URL）
- 保序映射
- 静态校验与启动期预计算（`Prepare` 产出启动期准备好的 `RuntimeConfig`，含编译后正则、解析后自定义代理、展开后路由成员、静态命名空间；请求期按只读契约消费）
- 原始配置层（`Config`）不持有运行期派生的代理字段；预计算层（`RuntimeConfig`）存储启动期产物

`internal/errtype`

- 六类错误定义（ConfigError、FetchError、ResourceError、BuildError、TargetError、RenderError）
- 横跨所有业务包的共享错误类型；`BuildError` / `TargetError` / `RenderError` 等可通过 `Cause` 保留内层根因链

`internal/model`

- 统一中间表示
- `ValidateProxyInvariant`：集中校验单个 Proxy 的结构不变量（Kind/Type 一致性、Dialer 约束、必填参数），供 pipeline 阶段调用以避免各阶段重复解释部分重叠的校验规则

`internal/fetch`

- 远程来源拉取
- TTL 缓存
- 统一资源加载（`LoadResource`：按 URL 前缀分发本地/远程）

`internal/pipeline`

- SS / Snell / VLESS 来源解析、各阶段转换
- 图级校验
- `Build` 编排（格式无关 IR）

`internal/proxyparse`

- 解析 `custom_proxies[].url`
- 返回运行期中立结构（type/server/port/params/plugin）
- 隔离 `config` 与 `model`

`internal/render`

- Clash Meta / Surge 序列化
- 底版模板合并
- 不承担协议级裁剪与图改写

`internal/target`

- Clash / Surge 目标格式投影
- 协议支持过滤（Snell / VLESS）
- fallback 清空等格式相关级联校验

`internal/generate`

- 单一”生成配置”服务
- 统一承接 `Build -> Target -> Render`
- 装配模板加载与 Surge managed URL
- v2.0 起改为无状态设计：`Generate` 方法接收 `*config.RuntimeConfig` 参数，不通过结构体字段持有配置指针；`app.Service` 在每次请求时取快照后传入

`internal/app`

- v2.0 应用服务层
- 持有并发安全的 `RuntimeConfig` 快照与配置 revision 状态
- 承接配置读取、条件写回、热重载、运行时预览、草稿预览和状态查询
- 可以编排 `config` / `pipeline` / `generate`，但不处理 HTTP 请求细节

`internal/auth`

- v2.0 管理后台认证服务
- bootstrap setup token、管理员 PBKDF2 密码哈希、auth state 文件读写、session 创建 / 校验 / 注销
- 登录失败计数与临时锁定
- 不依赖 `config` / `pipeline` / `model`，避免权限逻辑与配置生成逻辑耦合

`internal/server`

- HTTP 路由装配
- `/generate` 和 `/healthz` handler
- Web SPA fallback、CORS 和请求计数 middleware
- `/generate` 参数校验、订阅 token / 管理员 session 放行判断和错误映射
- Admin API 作为注入的 `http.Handler` 挂载到 `/api/`

`internal/ssparse`

- Shadowsocks URI 解析（SIP002 body 解析、plugin query 解析）
- 被 `proxyparse`（自定义代理 URL 解析）和 `pipeline`（SS 订阅 URI 解析）共享

---

## 依赖方向

```text
cmd/subconverter
  -> admin
  -> config
  -> fetch
  -> app
  -> auth
  -> generate
  -> server
  -> webui

server
  -> errtype
  -> generate

admin
  -> app
  -> auth
  -> errtype

auth
  -> (leaf)

app
  -> config
  -> fetch
  -> generate
  -> pipeline
  -> model
  -> errtype

generate
  -> config
  -> fetch
  -> model
  -> pipeline
  -> target
  -> render

pipeline
  -> config
  -> fetch
  -> model
  -> errtype
  -> proxyparse
  -> ssparse

render
  -> model
  -> errtype

config
  -> errtype
  -> fetch
  -> proxyparse

target
  -> model
  -> errtype

proxyparse
  -> ssparse

fetch
  -> errtype

ssparse
  -> (leaf)
```

约束：

- `model` 不依赖其他业务包
- `errtype` 不依赖其他业务包
- `render` 不直接读取 YAML 配置
- `server` 不承担业务转换逻辑，`/api/*` 业务通过注入的 `admin.Handler` 处理
- `admin` 不直接依赖 `pipeline` / `model`，只调用 `app.Service`
- `auth` 不依赖其他业务包，避免权限逻辑与配置生成逻辑耦合
- `config` 不直接依赖 `model`
- `generate` 直接依赖 `model`（用于在 `pipeline.Build` 与 `target.ForXxx` 之间传递 `Pipeline`）

---

## v2.0 新增目录与包

### 新增目录

```text
subconverter/
├── web/                          # 前端 SPA 源码
│   ├── src/                      # React 组件与页面
│   ├── dist/                     # 构建产物（生产镜像嵌入 Go 二进制）
│   ├── package.json
│   ├── vite.config.ts
│   └── nginx.conf                # nginx 路径配置测试夹具
└── internal/
    ├── app/                      # v2.0 应用服务层
    ├── admin/                    # Admin API 处理器
    ├── auth/                     # 管理后台认证服务
    └── webui/                    # 可选嵌入式 Web SPA 资产
```

### `internal/app` 包职责

- `ConfigSnapshot`：读取配置源，返回 `{config_revision, config}`
- `SaveConfig`：基于 `config_revision` 做条件写回，revision 冲突返回 `409`
- `ValidateDraft`：校验草稿配置，返回结构化诊断
- `Reload`：强制刷新主配置源、Prepare 后原子替换 `RuntimeConfig`
- `PreviewNodes` / `PreviewGroups`：支持运行时 GET 预览与草稿 POST 预览；返回 `app` 包内定义的 DTO（如 `NodePreview` / `GroupPreview`），由 `app.Service` 负责从 `model.Proxy` / `model.ProxyGroup` 转换，使 `admin` 层无需导入 `model`
- `Generate` / `GenerateFromDraft`：分别基于当前 `RuntimeConfig` 快照或草稿配置，传入无状态的生成逻辑输出文本；“preview”只属于 HTTP route / handler 命名，`app.Service` 不新增薄封装方法
- `GenerateLink`：根据当前配置 `base_url`、格式、文件名和订阅访问 token 生成客户端订阅链接
- `Status`：返回配置源能力、当前 revision、运行时 revision、dirty 与最近 reload 信息

### `internal/admin` 包职责

- 认证 handler（`GET /api/auth/status`、`POST /api/auth/login`、`POST /api/auth/setup`、`POST /api/auth/logout`），调用 `internal/auth`
- 配置 CRUD handler（`GET /api/config`、`PUT /api/config`）
- 配置校验 handler（`POST /api/config/validate`）
- 热重载 handler（`POST /api/reload`）
- 运行时和草稿预览 handler（`GET/POST /api/preview/nodes`、`GET/POST /api/preview/groups`）
- 生成预览 handler（`GET/POST /api/generate/preview`）
- 订阅链接生成 handler（`GET /api/generate/link`）
- 系统状态 handler（`GET /api/status`）
- 不承担管道或渲染逻辑；不直接依赖 `internal/pipeline` 或 `internal/model`

依赖方向以本文 §依赖方向 的单一图为准。`admin` 通过 `app.Service` 间接访问 `RuntimeConfig`（RWMutex 保护），不直接持有配置引用，也不直接编排管道阶段。
