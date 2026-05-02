# 系统架构设计

> v1.0 架构文档已归档至 docs/v1.0/architecture.md
>
> 状态提示：本文描述 v2.0 目标架构契约；当前可运行能力与规划能力的边界见 docs/README.md 状态矩阵。

## 概述

本系统是一个使用 Go 实现的单用户 HTTP 服务。它读取一份用户 YAML 配置，拉取 SS 订阅源、Snell 来源和 VLESS 来源，结合自定义代理、节点分组、服务路由和远程规则集，生成 Clash Meta 或 Surge 配置文件。

v2.0 在 v1.0 核心管道基础上新增：

- **Web 管理后台**：React SPA，通过 Docker Compose 中的 `web` 容器托管静态资源，并反向代理到 `api` 容器
- **配置热重载**：运行时重新加载配置，无需重启服务
- **Admin API**：配置 CRUD、校验、预览和系统状态接口

系统目标：

- 将用户关注点拆分为来源、节点组、服务组、规则集和兜底策略
- 用统一中间表示支撑 Clash Meta 与 Surge 两种输出
- 保持后端单二进制、低依赖部署；Web 前端通过独立静态容器发布
- 让配置书写顺序稳定映射到客户端面板顺序
- 提供 Web 管理后台，降低配置编辑门槛

系统非目标：

- 不维护内置规则库
- 不支持多用户、多租户或多设备 overlay
- 不支持 Shadowrocket、Quantumult X 等额外输出目标
- 不在服务端预拉取规则集内容

---

## 安全与信任模型

本系统按**单用户信任模型**设计：运行者 = 使用者，所有通过管理员登录态进入的 Admin API 输入视为可信。单用户不等于公网后台无认证；v2.0 Web 管理后台必须使用独立的管理员账号与 Cookie session 保护。

- 管理后台访问 `/login` 完成登录；未登录访问任意 SPA 页面或 `/api/*` 管理接口时返回登录要求或重定向到 `/login?next=<原路径>`
- 后端首次启动且没有管理员凭据时进入 setup 流程；setup 必须提交 bootstrap setup token，创建单一管理员账号并写入独立 auth state 文件。若未显式配置 `SUBCONVERTER_SETUP_TOKEN`，服务启动时生成一次性 32-byte URL-safe token 并仅打印到日志。若 auth state 不可写，服务必须 fail closed，不允许自动放开管理接口
- 管理后台登录成功后写入 `session_id` Cookie（HttpOnly、SameSite=Lax、HTTPS 下 Secure）；未选择“记住我”时最长 24 小时，选择后最长 7 天
- auth state 只保存 PBKDF2 密码哈希和 session token 的 SHA-256 哈希，不保存明文密码或明文 session id；文件权限必须收紧到 `0600`
- `SUBCONVERTER_TOKEN` / `-access-token` 只表示 `/generate` 订阅访问 token，用于 Clash / Surge 等客户端自动更新订阅；它不作为 `/api/*` 或后台页面的权限凭据
- `POST /api/preview/*` 接受草稿配置中的任意订阅 URL 并实际拉取——这是设计意图（允许用户预览新增来源的效果），而非安全疏忽
- `/generate?token=...` 的 token 会出现在 URL 中（nginx access log、浏览器历史记录等），使用者应知悉此泄漏路径；同理，若 `-config` 参数是含 token 的私有远程 URL（如 GitHub Raw `?token=...`），该 URL 同样可能出现在 nginx access log 或进程日志中
- 本系统不提供多用户、RBAC、审计系统、IP 白名单或公网 WAF 能力——若需要这些能力，由前置反向代理（nginx、Cloudflare Tunnel 等）承担

---

## 运行模型

系统按单用户模式设计，运行期分三个阶段：

### 启动

- 加载 YAML 配置文件，执行静态校验和预计算（`LoadConfig → Prepare`），产出不可变的 `RuntimeConfig`
- 创建服务实例，`*RuntimeConfig` 指针由 `sync.RWMutex` 保护；请求只在锁内读取指针快照

### 热重载

- `POST /api/reload` 触发：重新加载配置源 → 重新执行 `Prepare` → 校验通过后 `WLock` 原子替换 `RuntimeConfig` 指针
- 校验失败时保留旧配置不变，返回错误
- 已取得快照的请求继续使用旧配置完成；新请求在替换后读取新配置

### 请求期

- 每个 `/generate` 或 `/api/preview/*` 请求短暂持有 `RLock` 读取当前 `RuntimeConfig` 快照，随后释放锁
- 使用快照执行 `Build → Target → Render` 管道（或部分阶段用于预览）
- 请求之间无共享可变状态

```text
config.yaml + remote sources
        │
        ▼
   subconverter
        │
        ├─► /generate?format=clash|surge    （生成配置）
        ├─► /api/config                     （配置管理）
        ├─► /api/preview/*                  （运行时预览）
        ├─► /api/reload                     （热重载）
        ├─► /api/status                     （系统状态）
        └─► /healthz                        （健康检查）
```

v2.0 正式 Web 部署时，浏览器不直接访问 `subconverter` 后端端口，而是访问 Docker Compose 中的 `web` 容器：

```text
browser
  │
  ▼
web container (nginx)
  ├─► /                 SPA 静态资源 + 前端路由 fallback
  ├─► /api/*      ───┐
  ├─► /generate   ───┼─► api container (subconverter:8080)
  └─► /healthz    ───┘
```

---

## 核心对象

系统围绕三类对象组织：

- 节点：订阅节点、Snell 节点（仅 Surge 输出）、VLESS 节点（仅 Clash 输出）、自定义节点、链式节点
- 组：节点组、服务组
- 路由规则：远程规则集、内联规则、fallback

其中：

- 节点组用于选择"某一地区或某一链路使用哪个具体节点"
- 服务组用于选择"某个服务走哪个出口"
- 链式组属于节点组，与地区组同层
- 所有节点组都必须显式声明 `select` 或 `url-test`
- 服务组统一为 `select`

---

## 管道模型

```text
启动期 / 热重载期:
LoadConfig
  -> Prepare (produces RuntimeConfig)
     ↑ POST /api/reload 可重新触发

请求期:
RLock()
  -> snapshot *RuntimeConfig
RUnlock()
  -> Build(Source -> Filter -> Group -> Route -> ValidateGraph)
  -> Target
  -> Render
```

对应职责：

- `LoadConfig`：读取并解析用户 YAML，产出 `Config`
- `Prepare`：校验字段合法性、编译正则、解析自定义代理 URL、展开 `@auto`、检测命名冲突与路由环路，产出 `RuntimeConfig`
- `Build`：构建格式无关 IR，其中 `Source/Filter/Group/Route/ValidateGraph` 仍按阶段拆分
- `Target`：按目标格式做协议能力裁剪和格式相关图校验
- `Render`：把已投影的目标格式视图序列化为 Clash Meta 或 Surge 文本

预览请求（`/api/preview/*`）执行管道的部分阶段：

- `/api/preview/nodes`：执行 `Source + Filter`，返回节点列表
- `/api/preview/groups`：执行 `Source + Filter + Group + Route + ValidateGraph`，返回节点组、链式组、服务组与宏展开结果；图级错误返回 400 结构化诊断
- `POST /api/preview/*` 使用草稿配置生成临时 `RuntimeConfig`，不替换当前运行时配置

---

## 模块边界

完整的包依赖方向图见 `docs/implementation/project-structure.md` §依赖方向（唯一权威来源）。以下仅列核心约束。

模块职责：

- `config`：配置加载、保序解析、静态校验、启动期预计算（`Prepare` 产出 `RuntimeConfig`）
- `model`：格式无关的中间表示
- `fetch`：订阅拉取、缓存、统一资源加载
- `ssparse`：Shadowsocks URI 解析
- `proxyparse`：自定义代理 URL 解析
- `pipeline`：Build 内的 Source / Filter / Group / Route / ValidateGraph 编排
- `target`：目标格式投影与格式相关级联校验
- `render`：Clash Meta 与 Surge 渲染器，只做序列化
- `generate`：单一"生成配置"服务，承接 `Build -> Target -> Render`。v2.0 起改为无状态设计：`Generate` 方法接收 `*RuntimeConfig` 参数，不再通过结构体字段持有配置指针；`app.Service` 在每次请求时取快照后传入
- `app`：v2.0 应用服务层，统一承接配置快照、条件写回、热重载、运行时预览、草稿预览、订阅链接生成、状态查询；用 `RWMutex` 保护 `*RuntimeConfig` 指针快照与替换。包内按文件拆分职责（`service.go` / `config_revision.go` / `config_source.go` / `preview.go` / `status.go`），保持包级别统一入口而非引入不必要的子包抽象
- `auth`：v2.0 管理后台认证层，承接 bootstrap setup token、管理员 PBKDF2 密码哈希、auth state 文件、session 创建/校验/注销和登录失败锁定；不依赖配置生成管道
- `admin`：Admin API 处理器，只做 JSON 解析、调用 `app.Service` 或 `auth`、错误映射；不直接编排管道或渲染逻辑
- `server`：HTTP 接口、路由注册、session middleware、同源校验、参数校验和错误映射

依赖原则：

- 依赖方向单向
- `model` 和 `errtype` 是叶子包，不依赖其他业务包
- 渲染层不反向依赖配置层实现细节
- HTTP 层不直接依赖管道与渲染细节：受保护 `/api/*` 先经 `auth` 校验 session，再通过 `admin` 调用 `app.Service`；`/generate` 也通过 `app.Service` 获取当前快照后再进入生成逻辑

---

## 前端架构

### 技术栈

- React SPA（TypeScript），Vite 构建
- 状态管理：React Query（服务端状态）+ React 本地状态
- 构建产物输出到 `web/dist/`

### 生产部署方式（v2.0 目标）

- 前端构建产物输出到 `web/dist/`
- `web/Dockerfile` 使用 Node 构建前端，并用 nginx 托管静态资源
- nginx 对 `/` 执行 SPA fallback，未命中静态文件时返回 `index.html`
- nginx 将 `/api/*`、`/generate`、`/healthz` 反向代理到 Compose 内的 `api:8080`
- 后端 `api` 容器只负责接口和配置生成，不托管 Web 静态资源
- 生产模式下 SPA 与 API 对浏览器同源，不需要启用 CORS

### 开发模式

- 前端：Vite dev server（默认 `localhost:5173`）
- 后端：Go 服务（默认 `localhost:8080`）
- 推荐通过 Vite proxy 将 `/api/*`、`/generate` 和 `/healthz` 转发到后端；若不使用 proxy，可在后端启动时加 `-cors` 标志用于本地跨域调试

### YAML 真相源原则

- UI 是 YAML 配置的可视化外壳，不存在"前端独有"持久状态
- 前端通过 `/api/config` 读写配置，不直接操作 YAML 文件
- 保序字段（groups / routing / rulesets）在 JSON 中用 `[{key, value}]` 数组表示
- 当 `-config` 是本地文件时，后台可写回 YAML；当 `-config` 是 HTTP(S) URL 时，后台以只读模式运行

---

## 热重载机制

```text
POST /api/reload
  │
  ├─ re-LoadConfig(source)      读取最新 YAML（远程主配置 bypass / invalidate 缓存）
  ├─ re-Prepare(config)         静态校验 + 预计算
  │
  ├─ 校验失败？ ──► 返回错误，不替换
  │
  └─ 校验通过：
       WLock()
       swap *RuntimeConfig pointer + runtime config revision
       WUnlock()
       返回成功 + 耗时
```

并发模型：

- `app.Service` 内部持有 `sync.RWMutex` 保护 `*RuntimeConfig` 指针
- `/generate` 和 `/api/preview/*` 请求只在复制配置指针时短暂使用 `RLock`
- `/api/reload` 使用 `WLock` 替换配置指针
- 慢速订阅拉取、模板加载或渲染不持有配置锁，因此不会阻塞热重载获取写锁
- 无需请求排队或版本号机制；请求以取得快照的时间点决定使用旧配置或新配置

---

## 日志策略

当前使用 Go 标准库 `log`，不引入结构化日志框架。理由：单用户工具、部署规模小、不需要日志聚合或查询。日志输出到 stderr，由 Docker / systemd 等外层基础设施管理持久化和轮转。

---

## 优雅停止

进程收到 `SIGTERM` 或 `SIGINT` 后按以下顺序停止：

1. 停止接受新连接
2. 等待 in-flight 请求完成（超时由 `http.Server.Shutdown` 的 context 控制，建议 30s）
3. 超时后强制关闭剩余连接并退出

Docker 环境下，`docker stop` 先发 `SIGTERM`，默认 10s 后 `SIGKILL`。建议 Dockerfile 中设置 `STOPSIGNAL SIGTERM`（Go 默认已处理），并确保 shutdown timeout ≤ Docker 的 stop grace period。

---

## 数据流

```text
用户 YAML 配置
    │
    ├─► sources      ──► 节点来源
    ├─► groups       ──► 节点组定义
    ├─► routing      ──► 服务组定义
    ├─► rulesets     ──► 远程规则集绑定
    ├─► rules        ──► 内联规则
    └─► fallback     ──► 兜底出口

订阅节点 + Snell 节点 + VLESS 节点 + 自定义节点
    │
    ▼
统一中间表示
    │
    ├─► Clash 目标投影 ──► Clash Meta 渲染
    └─► Surge 目标投影 ──► Surge 渲染

Web 管理后台
    │
    ├─► GET /api/config        ──► 读取配置 JSON + config_revision
    ├─► PUT /api/config        ──► 条件写回 YAML（revision 匹配才覆盖）
    ├─► POST /api/reload       ──► 热重载 RuntimeConfig
    ├─► GET /api/preview/*     ──► 查看当前运行时数据
    ├─► POST /api/preview/*    ──► 查看草稿配置数据
    ├─► GET /api/generate/preview  ──► 当前运行时生成预览
    └─► POST /api/generate/preview ──► 草稿配置生成预览
```

设计要求：

- `groups`、`routing`、`rulesets` 都要保留书写顺序
- `@all` 只展开原始节点，不包含链式节点
- 原始节点 = 订阅节点 + Snell 节点 + VLESS 节点 + 不带 `relay_through` 的自定义节点
- `@auto` 展开为自动补充池（节点组 → 包含 `@all` 的服务组 → DIRECT），自动去重且排除自身
- `REJECT` 不在 `@auto` 补充池中；如需使用，必须显式声明
- 链式组由自定义代理派生，但在节点组层中与地区组平级

---

## 关键决策

| 决策 | 结论 | 原因 |
|------|------|------|
| 部署模型 | 单用户、单配置文件 | 与产品草案一致，运行模型最简单 |
| 用户配置风格 | 声明式分层 YAML | 关注点分离，便于维护 |
| 核心架构 | 管道模型 | 阶段清晰，易测试和调试 |
| 输出目标 | Clash Meta + Surge | 满足当前目标，避免过早扩展 |
| 规则集策略 | 仅透传 URL | 客户端运行时自行拉取 |
| 节点组策略 | 所有节点组必须显式声明策略 | 面板行为一致，避免隐式默认值 |
| 链式组建模 | 属于节点组，由 `relay_through` 派生 | 与用户心智一致 |
| `@all` 范围 | 仅原始节点，不含链式节点 | 控制节点膨胀 |
| `@auto` 语义 | 自动补充节点组 + 包含 `@all` 的服务组 + DIRECT | 消除 routing 冗余 |
| 缓存范围 | 缓存订阅和模板的远程拉取结果 | 规则集由客户端消费 |
| 资源加载模型 | 配置文件和模板均支持本地路径或 HTTP(S) URL | 统一 `LoadResource` |
| 渲染器合并策略 | Clash 用 yaml.Node 替换；Surge 用 section 切分替换 | 保留底版用户设置 |
| **热重载并发模型** | `sync.RWMutex` 保护 `RuntimeConfig` | 单进程无需分布式锁，读多写少场景最优 |
| **前端部署方式** | Docker Compose：`web` nginx 静态站点 + 反向代理到 `api` | 避免 Go 嵌入目录边界问题，生产同源访问 |
| **开发模式** | Vite proxy 优先；`-cors` 仅作本地调试兜底 | 前后端分离开发 |
| **Admin API 前缀** | `/api/*` | 与 `/generate` 平行，不冲突 |
| **YAML 真相源** | UI 是 YAML 的可视化外壳 | 数据一致性，修改可追溯 |
| **配置源写入边界** | 本地文件可写，HTTP(S) 配置只读 | 远程配置源无法可靠写回，需显式暴露能力 |
| **条件写回** | `config_revision = sha256:<hex>` | 乐观并发令牌，防止旧页面或旧 revision 覆盖已观测到的新配置；不承诺外部多写者线性一致 |
| **应用服务边界** | `admin -> app -> pipeline/generate` | HTTP 层保持薄边界，管道编排不泄漏到 handler |

---

## 已知架构局限

### 1. ValidateGraph 不感知输出格式

> **触及时行动**：新增 format-specific 过滤时 → 重新评估是否引入 per-format validation hook。

- **现象**：Build 阶段"合法"的配置，在某一输出格式的 Target 阶段可能失败
- **报错路径**：`target.ForClash` / `target.ForSurge` 在 Target 阶段返回 `TargetError`
- **影响**：错误被"晚报"；调试时用户看到 render 错而非 build 错

### 2. 中间表示对"格式限定字段"的宽松包容

> **触及时行动**：考虑严格模式时 → 优先扩充 `xxxKeyOrder` 而非引入白名单拒绝。

- **现象**：`Proxy.Params` 是 `map[string]string`，容纳任意键
- **权衡**：解析器不随目标格式版本迭代；代价是用户 typo 不会报错

### 3. 协议格式专属性导致 Target 阶段级联过滤

> **触及时行动**：新增格式专属协议 → 复用 `filterByDroppedTypes` 策略；同步补跨格式过滤测试。

- **现象**：Snell 仅支持 Surge、VLESS 仅支持 Clash，不支持方在 Target 阶段做级联过滤

### 4. RuntimeConfig 快照一致性

> **触及时行动**：若未来需要保证 reload 返回后所有已在途请求都使用新配置 → 引入配置版本号或请求栅栏。

- **现象**：热重载成功前已经取得快照的请求仍会使用旧配置完成
- **权衡**：单用户服务更重视请求不中断和热重载低阻塞；严格线性一致性不是当前目标
- **锁边界**：`LoadConfig`、`Prepare`、订阅拉取、模板加载和渲染都不在写锁内，写锁仅保护指针替换

### 5. YAML 写回丢失注释与格式

> **触及时行动**：若用户反馈注释丢失影响过大 → 评估 `yaml.Node` 级 patch-merge 方案。

- **现象**：`PUT /api/config` 将 JSON 反序列化为 `Config` 后通过 `yaml.Marshal` 写回，`gopkg.in/yaml.v3` 的 `Marshal` 不保留原始文件中的注释节点和格式风格（缩进、引号选择）
- **影响**：每次通过 Web 后台保存配置后，YAML 文件中的注释永久丢失；原有缩进和引号风格可能改变
- **缓解**：Web UI 在本地可写配置源首次保存前弹出确认，明确提示注释、引号和格式风格可能丢失；用户确认后才发起 `PUT /api/config`
- **API 契约**：`PUT /api/config` 成功响应保持 `{ "config_revision": "sha256:<hex>" }`，不承担自动备份或 warning 响应语义
- **长期方案（未实施）**：用 `yaml.Node` 级 patch-merge 策略替代全量 Marshal——在 AST 层只替换变更的节点，保留其余注释和格式。复杂度显著上升，当前规模下不必处理

### 6. 用户正则表达式无执行复杂度限制

> **触及时行动**：若用户反馈正则匹配导致请求明显变慢 → 先用 profiling 确认瓶颈，再考虑限制节点名长度/节点数量、为请求路径增加超时或取消传播。

- **现象**：`groups[].match`、`filters.exclude`、`relay_through.match` 的正则由用户输入，`Prepare` 阶段仅校验"是否可编译"，不限制匹配时的输入规模
- **影响**：Go 标准库 `regexp` 使用 RE2 引擎，匹配时间随输入规模线性增长，不存在典型指数回溯 ReDoS；但在节点数量或节点名极大时，Filter/Group 阶段仍可能出现可感知延迟
- **缓解**：当前保持标准库 `regexp`，避免引入新正则引擎；若出现真实慢请求，优先用输入规模限制、请求级超时/取消和 profiling 定位处理
- **当前判断**：单用户工具，攻击面有限。无需为尚未验证的性能风险增加外部依赖或替换正则语义

---

## 文档导航

实现细节由后续文档承接：

- `docs/design/config-schema.md`：配置结构与字段约束
- `docs/design/domain-model.md`：领域模型与中间表示
- `docs/design/pipeline.md`：各阶段职责与数据约束
- `docs/design/api.md`：HTTP 接口约定
- `docs/design/rendering.md`：Clash Meta / Surge 渲染映射
- `docs/design/validation.md`：配置与图校验规则
- `docs/design/caching.md`：订阅拉取与缓存策略
- `docs/design/web-ui.md`：Web 管理后台设计
- `docs/implementation/project-structure.md`：代码目录与包边界
- `docs/implementation/implementation-plan.md`：v2.0 开发计划
- `docs/implementation/testing-strategy.md`：测试与验收策略
