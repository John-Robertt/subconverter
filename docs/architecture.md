# 系统架构设计

> v1.0 架构文档已归档至 docs/v1.0/architecture.md
>
> 状态提示：本文描述 v2.0 当前架构契约；当前可运行能力见 docs/README.md 状态矩阵。

## 概述

本系统是一个使用 Go 实现的单用户 HTTP 服务。用户在 YAML 中声明节点来源、分组和路由，服务拉取节点后，把这些声明转换为 Clash Meta、Surge 或 Loon 配置文件。远程规则集只写入 URL，正文由客户端自行拉取。

程序分三步完成转换：`pipeline` 先构建节点与组之间的引用关系，`target` 再根据目标格式删除不支持的节点并整理引用，`render` 最后输出文本。Web 后台通过应用服务调用同一套转换逻辑。

v2.0 在 v1.0 核心管道基础上新增：

- **Web 管理后台**：React SPA，生产镜像将构建产物嵌入 Go 二进制，由同一服务托管静态资源和 API
- **配置热重载**：运行时重新加载配置，无需重启服务
- **Admin API**：配置读取、保存、导入导出、校验、预览和系统状态接口

系统目标：

- 将用户关注点拆分为来源、节点组、服务组、规则集和兜底策略
- 用统一中间表示支撑 Clash Meta、Surge 与 Loon 三种输出
- 保持单二进制、低依赖部署；Web 前端在生产镜像中作为嵌入式静态资源发布
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

- 浏览器可以加载 SPA 静态文件；前端 `ProtectedRoute` 将未登录用户转到 `/login?next=<原路径>`。受保护的 `/api/*` 接口由后端校验 `session_id` Cookie；`/api/auth/status`、login、setup 和 logout 按各自认证流程处理
- 后端首次启动且没有管理员凭据时进入 setup 流程；setup 必须提交 bootstrap setup token，创建单一管理员账号并写入独立 auth state 文件。若未显式配置 `SUBCONVERTER_SETUP_TOKEN`，服务启动时生成一次性 32-byte URL-safe token 并仅打印到日志。若 auth state 不可写，服务必须 fail closed，不允许自动放开管理接口
- 管理后台登录成功后写入 `session_id` Cookie（HttpOnly、SameSite=Lax、HTTPS 下 Secure）；未选择“记住我”时最长 24 小时，选择后最长 7 天
- auth state 只保存 PBKDF2 密码哈希和 session token 的 SHA-256 哈希，不保存明文密码或明文 session id；文件权限必须收紧到 `0600`
- `SUBCONVERTER_TOKEN` / `-access-token` 只表示 `/generate` 订阅访问 token，用于 Clash / Surge 等客户端自动更新订阅；它不作为 `/api/*` 或后台页面的权限凭据
- `POST /api/preview/*` 接受草稿配置中的任意订阅 URL 并实际拉取——这是设计意图（允许用户预览新增来源的效果），而非安全疏忽
- `/generate?token=...` 的 token 会出现在 URL 中（访问日志、浏览器历史记录等），使用者应知悉此泄漏路径；同理，若 `-config` 参数是含 token 的私有远程 URL（如 GitHub Raw `?token=...`），该 URL 同样可能出现在前置代理或进程日志中
- 本系统不提供多用户、RBAC、审计系统、IP 白名单或公网 WAF 能力——若需要这些能力，由前置反向代理（nginx、Cloudflare Tunnel 等）承担

---

## 运行模型

程序只有一个进程。`cmd/subconverter/main.go` 创建共享的资源缓存、`app.Service`、认证服务和 HTTP 服务。配置保存在 YAML 中，账号和会话保存在独立认证状态文件（auth state）中，远程资源缓存只保留在内存里。

配置有三种状态，编辑、保存和生效是三个不同动作：

| 状态 | 谁持有 | 何时改变 |
|------|--------|----------|
| 浏览器草稿 | 前端 `ConfigProvider` | 用户编辑或导入配置时 |
| 配置源中的内容 | 本地 YAML 文件或远程 HTTP(S) 地址 | 本地保存或外部系统修改时 |
| 当前生效配置 | `app.Service` 中的 `RuntimeConfig` | 启动成功或热重载成功时 |

例如，把地区组名称改好并点击“保存”，只会写回 YAML。点击“热重载”且校验成功后，后续生成请求才会使用新名称。草稿、保存和生效状态的字段及接口见 [app.Service 契约](design/app-service.md)。

### 启动

- `app.New` 读取并解析 YAML，再通过 `prepareAdminConfig` 调用 `config.Prepare`，校验字段、编译正则并展开 `@auto`
- 成功后保存 `RuntimeConfig`、源 YAML 字节和对应 revision；失败则退出启动流程
- 请求按只读约定使用 `RuntimeConfig`。其方法会返回含切片等引用的数据，Go 类型本身没有强制禁止修改；管道应另建结果，保持这份准备好的配置不变

### 热重载

- `POST /api/reload` 触发：在配置锁外重新读取、解析和校验 → 成功后短暂加写锁，一起替换配置指针、revision、生效 YAML 和重载状态
- 校验失败时保留旧配置不变，返回错误
- 已取得快照的请求继续使用旧配置完成；新请求在替换后读取新配置

### 请求期

- `/generate` 和 GET 预览短暂加读锁，取得当前配置指针后立即释放锁，再执行生成或预览所需阶段
- POST 草稿预览从请求体生成临时 `RuntimeConfig`，使用草稿运行所需阶段，保持文件和生效配置不变
- 每个请求各自构建节点图；请求间共享配置快照、资源缓存、认证状态和计数器。这些共享数据分别按只读约定或各自的锁、原子操作保护

```text
config.yaml + remote sources
        │
        ▼
   subconverter
        │
        ├─► /generate?format=clash|surge|loon    （生成配置）
        ├─► /api/config                     （配置管理）
        ├─► /api/preview/*                  （运行时预览）
        ├─► /api/reload                     （热重载）
        ├─► /api/status                     （系统状态）
        └─► /healthz                        （健康检查）
```

v2.0 正式 Web 部署时，浏览器直接访问 `subconverter` 服务端口。生产发布先通过 `pnpm web:embed` 将前端产物同步到 `internal/webui/dist`，再由 Go `webui` 构建嵌入二进制；运行时由同一个进程同时提供 SPA、Admin API、订阅生成和健康检查：

```text
browser
  │
  ▼
subconverter (Go)
  ├─► /                 embedded SPA + 前端路由 fallback
  ├─► /api/*            Admin API
  ├─► /generate         订阅生成
  └─► /healthz          健康检查
```

---

## 核心对象

系统围绕三类对象组织：

- 节点：订阅节点（TXT 三种输出、Clash YAML 仅 Clash、Surge CONF 仅 Surge、Loon 仅 Loon）、Snell 节点（仅 Surge 输出）、VLESS 节点（Clash / Loon 输出）、自定义节点、链式节点
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
读取并解析 YAML
  -> Prepare（产出 RuntimeConfig）
     ↑ POST /api/reload 可重新触发

运行时生成（/generate、GET /api/generate/preview）:
RLock()
  -> 取得 *RuntimeConfig 指针
RUnlock()
  -> Build(Source -> Filter -> Group -> Route -> ValidateGraph)
  -> Target
  -> Render
```

对应职责：

- 配置加载：读取并解析用户 YAML，产出 `Config`；当前应用入口由 `app.Service` 完成，代码对应关系见 [管道设计](design/pipeline.md#stage-1-loadconfig)
- `Prepare`：校验字段合法性、编译正则、解析自定义代理 URL、展开 `@auto`、检测命名冲突与路由环路，产出 `RuntimeConfig`
- `Build`：把来源、过滤和路由声明变成格式无关的节点图，即中间表示（IR）。图内用 `ProxyID` 引用代理，节点显示名可以重复
- `Target`：过滤目标格式不支持的节点，清理因此失效的链式节点、组和规则，再分配本格式内唯一的输出名
- `Render`：只接收已整理好的 `TargetPipeline`，将其与可选底版模板合并，输出 Clash Meta、Surge 或 Loon 文本

例如，两个来源都声明了 `HK` 节点，Build 会保留两个不同 ID。Clash 和 Surge 各自过滤后再分配名称；最终只剩一个时可保留 `HK`，两个都保留时则用后缀区分。组成员始终通过 ID 跟随对应节点，直到 Target 将引用转换为最终名称。

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
- `render`：Clash Meta、Surge 与 Loon 渲染器，只做序列化
- `generate`：单一"生成配置"服务，承接 `Build -> Target -> Render`。v2.0 起改为无状态设计：`Generate` 方法接收 `*RuntimeConfig` 参数，不再通过结构体字段持有配置指针；`app.Service` 在每次请求时取快照后传入
- `app`：应用服务入口，承接配置快照、保存、导入导出、热重载、预览、订阅链接和状态查询；将内部节点图转换为 API 所需数据，让 HTTP 层无需理解管道细节
- `auth`：v2.0 管理后台认证层，承接 bootstrap setup token、管理员 PBKDF2 密码哈希、auth state 文件、session 创建/校验/注销和登录失败锁定；不依赖配置生成管道
- `admin`：Admin API 处理器，只做 JSON 解析、同源校验、调用 `app.Service` 或 `auth`、错误映射；不直接编排管道或渲染逻辑
- `server`：HTTP 路由注册、`/generate`、`/healthz`、SPA 路由回退、响应缓存头和文本错误映射；生成响应设置 `no-store`，静态资源按路径设置缓存头，具体见 [API 契约](design/api.md#错误语义)

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

### 生产部署方式

- 前端构建产物输出到 `web/dist/`
- `pnpm web:embed` 将 `web/dist` 同步到 `internal/webui/dist`
- 根 `Dockerfile` 不运行 pnpm；它使用已同步产物，并在 Go 构建阶段用 `webui` build tag 将 Web 静态资源嵌入二进制
- Go HTTP server 对 `/` 执行 SPA fallback，未命中静态文件时返回 `index.html`
- `/api/*`、`/generate`、`/healthz` 仍由同一 Go 服务处理，不经过反向代理
- 生产模式下 SPA 与 API 对浏览器同源，不需要启用 CORS

### 开发模式

- 前端：Vite dev server（默认 `localhost:5173`）
- 后端：Go 服务（默认 `localhost:8080`）
- 推荐通过 Vite proxy 将 `/api/*`、`/generate` 和 `/healthz` 转发到后端；若不使用 proxy，可在后端启动时加 `-cors` 标志用于本地跨域调试

### 配置以 YAML 为持久来源

- 前端通过 `/api/config` 读取和保存业务配置；浏览器中的草稿在保存前只存在于内存，主题等界面偏好另行保存在浏览器中
- 已保存内容的版本由 `config_revision` 表示，生效内容的版本由 `runtime_config_revision` 表示；两者不同时，状态接口返回 `config_dirty=true`
- 保序字段（groups / routing / rulesets）在 JSON 中用 `[{key, value}]` 数组表示
- 本地文件及所在目录均可写时，后台允许保存；本地不可写或 HTTP(S) 配置源进入只读模式。状态字段与保存错误码见 [配置源能力](design/api.md#配置源能力)

前端状态的具体职责见 [前端架构契约](../web/docs/frontend-architecture.md#状态边界)。

---

## 热重载机制

```text
POST /api/reload
  │
  ├─ reloadMu.TryLock()        已有重载在执行时，立即返回 429
  ├─ 读取并解析 YAML            远程主配置先 Invalidate，再读取
  ├─ prepareAdminConfig       静态校验 + Prepare 预计算；均在配置写锁外
  │
  ├─ 校验失败？ ──► 返回错误，不替换
  │
  └─ 校验通过：
       WLock()
       替换配置指针、revision、生效 YAML 和重载状态
       WUnlock()
       返回成功 + 耗时
```

并发模型：

- `app.Service` 内部持有 `sync.RWMutex` 保护 `*RuntimeConfig` 指针
- `/generate` 和 GET 预览只在复制配置指针时短暂使用 `RLock`；POST 草稿预览另建临时配置
- `/api/reload` 使用 `WLock` 替换配置指针
- 慢速订阅拉取、模板加载或渲染不持有配置锁，因此不会阻塞热重载获取写锁
- 同时只执行一次重载，重复重载请求直接返回 `429 reload_in_progress`；生成请求按取得快照的时间点使用旧配置或新配置

热重载成功只说明主配置通过静态检查。订阅是否可拉取、地区组是否匹配到节点，以及某种格式的兜底组是否会被过滤清空，仍由后续预览或生成发现。资源刷新范围见 [缓存设计](design/caching.md#热重载与预览如何使用缓存)。

---

## 日志策略

当前使用 Go 标准库 `log`，不引入结构化日志框架。理由：单用户工具、部署规模小、不需要日志聚合或查询。日志输出到 stderr，由 Docker / systemd 等外层基础设施管理持久化和轮转。

---

## 优雅停止

进程收到 `SIGTERM` 或 `SIGINT` 后按以下顺序停止：

1. 停止接受新连接
2. 调用 `http.Server.Shutdown`，最多等待 10 秒，让已接受的请求完成
3. 正常完成后退出；等待超时时记录错误，随后主函数返回，进程退出。当前代码没有另行调用 `Server.Close`

实现位于 `cmd/subconverter/main.go`。部署时应让容器或进程管理器留出足够的停止等待时间，使程序有机会完成这 10 秒的退出流程。

---

本地主配置路径尚无文件时，进程先提供管理员后台和初始化接口；首次保存创建主配置，再显式热重载生成运行快照。初始化不绕过认证，完整状态及首次创建规则见[首次配置 API](design/api.md#首次配置)。

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

订阅节点（TXT / Clash YAML / Surge CONF / Loon）+ Snell 节点 + VLESS 节点 + 自定义节点
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
- 格式无关管道用 `ProxyID` 表达节点身份；候选节点名不承担图引用职责
- Clash / Surge / Loon 在完成各自格式过滤后独立分配唯一输出名
- `@all` 只展开原始节点，不包含链式节点
- 原始节点 = 订阅节点（TXT / Clash YAML / Surge CONF / Loon）+ Snell 节点 + VLESS 节点 + 不带 `relay_through` 的自定义节点
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
| 输出目标 | Clash Meta + Surge + Loon | 满足当前目标，避免过早扩展 |
| 规则集策略 | 仅透传 URL | 客户端运行时自行拉取 |
| 节点组策略 | 所有节点组必须显式声明策略 | 面板行为一致，避免隐式默认值 |
| 链式组建模 | 属于节点组，由 `relay_through` 派生 | 与用户心智一致 |
| `@all` 范围 | 仅原始节点，不含链式节点 | 控制节点膨胀 |
| `@auto` 语义 | 自动补充节点组 + 包含 `@all` 的服务组 + DIRECT | 消除 routing 冗余 |
| 缓存范围 | 缓存节点来源、模板和主配置的远程响应 | 缓存行为与刷新边界见 [缓存设计](design/caching.md) |
| 资源加载模型 | 配置文件和模板均支持本地路径或 HTTP(S) URL | 统一 `LoadResource` |
| 渲染器合并策略 | Clash 用 yaml.Node 替换；Surge / Loon 用 section 切分替换 | 保留底版用户设置 |
| 结构化订阅表示 | `KindSubscription` + 可选 `NativeProxy` | 复用统一过滤/分组语义，同时完整保留目标原生节点字段 |
| 节点身份 | Build 内唯一 `ProxyID`；节点名只是显示名候选 | 同名节点可安全共存，过滤/分组/链式关系不依赖可变名称 |
| 输出命名 | 每个目标在格式过滤后独立消歧 | Clash-only / Surge-only 同名节点都可保留原名，目标内仍满足唯一性 |
| **热重载并发模型** | `sync.RWMutex` 保护 `RuntimeConfig` | 单进程无需分布式锁，读多写少场景最优 |
| **前端部署方式** | `web/dist` 同步至 `internal/webui/dist`，再用 `webui` 标签嵌入二进制 | 单镜像、单进程，生产同源访问 |
| **开发模式** | Vite proxy 优先；`-cors` 仅作本地调试兜底 | 前后端分离开发 |
| **Admin API 前缀** | `/api/*` | 与 `/generate` 平行，不冲突 |
| **YAML 真相源** | UI 是 YAML 的可视化外壳 | 数据一致性，修改可追溯 |
| **配置源写入边界** | 本地文件及所在目录均可写时允许保存；HTTP(S) 配置只读 | 状态接口报告实际写入能力，前端据此控制保存入口 |
| **条件写回** | 已保存配置使用 `sha256:<hex>`，本地缺失使用 `missing` | 乐观并发令牌，防止旧页面或旧 revision 覆盖已观测到的新配置；不承诺外部多写者线性一致 |
| **应用服务边界** | `admin -> app -> pipeline/generate` | HTTP 层保持薄边界，管道编排不泄漏到 handler |

---

## 已知架构局限

### 1. ValidateGraph 不感知输出格式

> **触及时行动**：新增 format-specific 过滤时 → 重新评估是否引入 per-format validation hook。

- **现象**：Build 阶段"合法"的配置，在某一输出格式的 Target 阶段可能失败
- **报错路径**：`target.ForClash` / `target.ForSurge` 在 Target 阶段返回 `TargetError`
- **影响**：静态校验或分组预览通过后，某一种格式仍可能在 Target 阶段失败；错误类型是 `TargetError`，发生在 Render 之前

### 2. 中间表示对"格式限定字段"的宽松包容

> **触及时行动**：考虑严格模式时 → 优先扩充 `xxxKeyOrder` 而非引入白名单拒绝。

- **现象**：规范化节点的 `Proxy.Params` 是 `map[string]string`，容纳任意键；结构化订阅的 `NativeProxy.Payload` 则保留完整目标原生声明
- **权衡**：TXT/Snell/VLESS 的规范化解析器不随目标格式版本迭代，但未知 Params 键可能被固定列表渲染器丢弃；Clash YAML / Surge CONF / Loon 原生声明不做字段白名单校验，用户 typo 会被原样带到目标客户端

### 3. 协议或来源格式专属性导致 Target 阶段级联过滤

> **触及时行动**：新增格式专属协议或结构化来源 → 复用共享 cascade 策略；同步补跨格式过滤测试。

- **现象**：Snell / Surge CONF 节点仅支持 Surge，Clash YAML 节点仅支持 Clash，Loon 原生节点仅支持 Loon；规范化 VLESS 进入 Clash 及支持其传输的 Loon；不支持方在 Target 阶段做级联过滤

### 4. RuntimeConfig 快照一致性

> **触及时行动**：若未来需要保证 reload 返回后所有已在途请求都使用新配置 → 引入配置版本号或请求栅栏。

- **现象**：热重载成功前已经取得快照的请求仍会使用旧配置完成
- **权衡**：单用户服务更重视请求不中断和热重载低阻塞；严格线性一致性不是当前目标
- **锁边界**：`LoadConfig`、`Prepare`、订阅拉取、模板加载和渲染都不在写锁内，写锁仅保护指针替换

### 5. YAML 写回丢失注释与格式

> **触及时行动**：若用户反馈注释丢失影响过大 → 评估 `yaml.Node` 级 patch-merge 方案。

- **现象**：`PUT /api/config` 将 JSON 反序列化为 `Config` 后通过 `yaml.Marshal` 写回，`gopkg.in/yaml.v3` 的 `Marshal` 不保留原始文件中的注释节点和格式风格（缩进、引号选择）
- **影响**：每次通过 Web 后台保存配置后，YAML 文件中的注释永久丢失；原有缩进和引号风格可能改变
- **缓解**：Web UI 在首次覆写已有本地配置前弹出确认，明确提示注释、引号和格式风格可能丢失；用户确认后才发起 `PUT /api/config`
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
- `docs/design/rendering.md`：Clash Meta / Surge / Loon 渲染映射
- `docs/design/validation.md`：配置与图校验规则
- `docs/design/caching.md`：订阅拉取与缓存策略
- `docs/design/web-ui.md`：Web 管理后台设计
- `docs/implementation/project-structure.md`：代码目录与包边界
- `docs/implementation/implementation-plan.md`：v2.0 开发计划
- `docs/implementation/testing-strategy.md`：测试与验收策略

### Loon 目标投影与校验边界

新增 Loon 后继续保持 Build 格式无关、Target 负责格式过滤的边界；生成或预览指定格式时才报告该格式的级联清空错误。Loon 复用已有级联引擎，并在 Target 分配 `[Proxy Chain]` 名称与出口辅助节点名。各目标支持范围及客户端规则优先级以 [渲染设计](design/rendering.md#loon) 为准。
