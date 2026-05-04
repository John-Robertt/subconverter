# Web 管理后台设计

> 状态提示：本文描述 v2.0 Web 管理后台当前契约；`web/` 是正式 Vite SPA 工程，生产镜像将构建产物嵌入 Go 二进制。旧设计原型位于 `web/prototype/`，能力状态见 docs/README.md。

## 目标

本文件定义 Web 管理后台的前端架构、页面结构和后端集成方式。subconverter v2.0 在 v1.0 纯 API 基础上新增 Web 管理后台，让用户可通过浏览器可视化编辑配置、预览运行时数据、生成并下载目标格式配置。

---

## 设计原则

- YAML 是真相源：UI 是 YAML 配置的可视化外壳，不存在"前端独有"状态
- 任何 UI 元素都能映射回 YAML 某个 key
- 草稿与运行时分离：编辑页基于前端草稿调用 POST 预览；运行时预览页基于当前生效配置调用 GET 预览
- 本地可写配置源：保存操作 = JSON → YAML 写回文件 + 可选热重载
- 远程 HTTP(S) 配置源：后台以只读模式运行，可查看、校验、预览和热重载，但不提供保存写回

---

## 信息架构

三个功能区：

### A 区：配置编辑

对应 YAML 配置的各顶层段，拆分为独立页面：

| 页面 | 对应 YAML 段 | 核心功能 |
|------|-------------|---------|
| A1 订阅来源 | `sources` | 四类来源（subscriptions / snell / vless / custom_proxies）的卡片式管理，每类来源独立区块；自定义代理含 `relay_through` 子表单 |
| A2 过滤器 | `filters` | exclude 正则编辑 + 草稿匹配预览（调用 `POST /api/preview/nodes` 展示当前草稿会匹配/排除的节点）。注意：预览会实际拉取订阅，应显示"正在拉取订阅..."提示 |
| A3 节点分组 | `groups` | 分组卡片列表（拖拽排序，保序）；每组：名称 + match 正则 + strategy 选择器 + 草稿匹配预览（调用 `POST /api/preview/groups`）。注意：预览会实际拉取订阅，应显示"正在拉取订阅..."提示 |
| A4 路由策略 | `routing` | 服务组列表（拖拽排序，保序）；每组：名称 + 成员选择器（可引用节点组、服务组、DIRECT、REJECT、`@all`、`@auto`）；约束提示 |
| A5 规则集 | `rulesets` | 按服务组分组展示 URL 列表；增删改 URL |
| A6 内联规则 | `rules` | 规则表格（拖拽排序）+ 语法提示 |
| A7 其他配置 | `fallback` / `base_url` / `templates` | fallback 下拉（已定义服务组）；base_url 输入；templates 路径输入 |
| A8 静态配置校验 | （全配置） | 调用 `POST /api/config/validate`；展示 errors / warnings / infos 三级；通过 `locator.json_pointer` 跳转到对应 A1–A7 页面的具体字段 |

### B 区：运行时预览

展示实际拉取和处理后的运行时数据：

| 页面 | 数据来源 | 核心功能 |
|------|---------|---------|
| B1 节点预览 | `GET /api/preview/nodes` | 全来源节点表格（按来源分类：订阅 / Snell / VLESS / 自定义）；显示名称、类型、服务器、端口、Kind；标注格式限定（Snell = 仅 Surge / VLESS = 仅 Clash） |
| B2 分组预览 | `GET /api/preview/groups` | 当前运行时节点组、链式组和服务组结果；展示 `@all` / `@auto` 展开后的成员 |
| B3 生成下载 | `GET/POST /api/generate/preview` + `/generate` + `GET /api/generate/link` | 格式选择（Clash Meta / Surge）；预览当前运行时或草稿生成内容（语法高亮）；下载按钮；复制订阅链接（由服务端生成，可含订阅 token） |

### C 区：系统状态

| 内容 | 数据来源 | 说明 |
|------|---------|------|
| 服务健康 | `GET /healthz` | 健康/异常指示 |
| 版本信息 | `GET /api/status` | 版本号、commit、构建时间 |
| 配置信息 | `GET /api/status` | 配置源位置、可写性、上次加载时间 |
| 热重载历史 | `GET /api/status` | 上次热重载时间、成功/失败状态 |

---

## 交互模式

| 场景 | 模式 | 说明 |
|------|------|------|
| 添加/编辑条目 | 居中 Modal | 表单编辑，确认后保存 |
| 操作成功反馈 | 右下绿色 Toast | 4s 自动消失 |
| 操作失败反馈 | 右下红色 Toast | 不自动消失，含错误详情，可点击跳转 |
| 进行中状态 | 顶栏按钮 Spinner + 禁用 | 防止重复提交 |
| 不可撤销操作 | 居中红色确认弹窗 | 删除、重置等需二次确认 |
| 首次保存本地 YAML | 居中确认弹窗 | 提示注释、引号和格式风格可能丢失，用户确认后才发起 `PUT /api/config` |
| 校验修复 | 右侧 Drawer | 展示静态诊断列表，点击后通过 `locator.json_pointer` 跳转到对应页面/字段 |

---

## 前端路由

SPA 使用以下路由结构（React Router），与 Go 服务的 SPA fallback 配合。除 `/login` 外，所有页面都要求有效管理员 session；未登录访问时跳转到 `/login?next=<原路径>`。

| 路由 | 页面 | 区域 |
|------|------|------|
| `/login` | 登录 / 首次 setup | 认证 |
| `/sources` | A1 订阅来源 | A 配置编辑 |
| `/filters` | A2 过滤器 | A 配置编辑 |
| `/groups` | A3 节点分组 | A 配置编辑 |
| `/routing` | A4 路由策略 | A 配置编辑 |
| `/rulesets` | A5 规则集 | A 配置编辑 |
| `/rules` | A6 内联规则 | A 配置编辑 |
| `/settings` | A7 其他配置 | A 配置编辑 |
| `/validate` | A8 静态配置校验 | A 配置编辑 |
| `/nodes` | B1 节点预览 | B 运行时预览 |
| `/preview/groups` | B2 分组预览 | B 运行时预览 |
| `/download` | B3 生成下载 | B 运行时预览 |
| `/status` | C 系统状态 | C 系统状态 |

所有路由在未匹配静态文件时由 Go 服务 fallback 到 `index.html`，无需服务端渲染支持。
`/generate` 保留为后端生成接口和订阅链接路径，不作为 SPA 页面路由。

---

## 前端技术栈

- 框架：React SPA（TypeScript）
- 构建：Vite
- 状态管理：React Query（服务端状态）+ React 本地状态
- 产物发布：构建输出到 `web/dist/`，生产镜像将其嵌入 Go 二进制

---

## 前端-后端集成模型

### 生产模式（v2.0 目标）

- Docker Compose 启动单个 `subconverter` 服务
- Go 服务运行后端 API，并托管嵌入式 SPA 静态资源
- SPA fallback 由 Go 服务负责：静态文件未命中时返回 `index.html`
- 浏览器只访问 `subconverter` 服务端口，Web 页面与 API 保持同源，生产环境不需要 CORS

### 开发模式

- 前端：Vite dev server（默认 `localhost:5173`）
- 后端：Go 服务（默认 `localhost:8080`）
- 推荐 Vite 配置 proxy 将 `/api/*`、`/generate`、`/healthz` 代理到 Go 后端
- 若不使用 Vite proxy，可由 Go 开启 CORS middleware 允许前端 dev server 跨域

### API 调用约定

- 所有管理 API 前缀 `/api/*`
- 生成接口保持原有 `/generate` 路径，且不与 SPA 页面路由复用
- `/api/*` 请求使用同源 `session_id` Cookie；API client 需设置 `credentials: "include"` 或等价行为
- `/generate` 订阅链接继续使用 query token 兼容客户端；后台内下载可凭 session Cookie 调用 `/generate`
- 复制订阅链接时，前端调用 `GET /api/generate/link` 由服务端生成 URL；复制含 token 的链接前需显式确认
- 请求/响应均为 JSON（除 `/generate` 和 `/api/generate/preview` 返回配置文本）
- 前端通过 `GET /api/status` 读取 `capabilities.config_write`；当配置不可写时，编辑页进入只读查看模式，保存按钮禁用
- API client 将错误归一化为 `status` / `code` / `message` / `details`；`409` 必须按 `error.code` 区分 revision 冲突、只读配置源和本地文件不可写

### 登录态流程

- 页面启动时先调用 `GET /api/auth/status` 判断 `authed`、`setup_required`、`setup_token_required` 和 `locked_until`
- `setup_required=true` 时，`/login` 渲染 setup 模式，要求输入 bootstrap setup token，提交 `POST /api/auth/setup`
- 未登录访问受保护页面时跳转 `/login?next=<原路径>`；登录成功后跳回 `next` 或默认 `/sources`
- 受保护 API 返回 `401 auth_required` 或 `401 session_expired` 时，全局拦截并跳转 `/login`，提示“登录已过期”
- 登录失败 `401 invalid_credentials` 时在密码框下展示剩余次数；`423 auth_locked` 时展示锁定截止时间并禁用提交
- setup 失败 `401 setup_token_required` 或 `401 setup_token_invalid` 时在 setup token 字段下展示错误，不创建管理员凭据
- 右上角或用户菜单提供退出登录，调用 `POST /api/auth/logout` 后跳转 `/login`
- “记住我”只影响服务端 session 过期时间：未选最长 24 小时，选中最长 7 天；前端不持久保存密码、session id 或订阅 token

---

## 草稿与运行时预览

编辑页和运行时页使用不同 API，避免用户误把旧 RuntimeConfig 当作草稿结果：

| 场景 | API | 配置来源 | 是否影响运行时 |
|------|-----|----------|----------------|
| A2/A3 编辑态预览 | `POST /api/preview/nodes`、`POST /api/preview/groups` | 前端草稿 `{ config }` | 否 |
| A8 静态校验 | `POST /api/config/validate` | 前端草稿 Config JSON | 否 |
| B1/B2 运行时预览 | `GET /api/preview/nodes`、`GET /api/preview/groups` | 当前 `RuntimeConfig` | 否 |
| B3 草稿生成预览 | `POST /api/generate/preview` | 前端草稿 `{ config }` | 否 |
| B3 当前生成预览 | `GET /api/generate/preview` | 当前 `RuntimeConfig` | 否 |

保存工作流仍为：编辑草稿 → 校验 → 本地可写配置首次保存确认 → `PUT /api/config` 条件写回 → `POST /api/reload` 生效。

中间态处理：`PUT /api/config` 成功但 `POST /api/reload` 失败时（如远程主配置源读取失败，或新配置未通过 `Prepare` 静态校验），配置文件已更新但 RuntimeConfig 仍为旧版。此时 UI 应保留 `config_dirty = true` 提示，并允许用户重新触发 reload。若用户在保存后、reload 前离开页面（浏览器崩溃或关闭标签页），下次打开时 `GET /api/status` 会返回 `config_dirty = true`，UI 应提示用户执行 reload 使配置生效。

其中 A8 和 reload 都只覆盖 `Prepare` 阶段的静态配置校验；生成可用性需要通过 B1/B2/B3 预览确认，尤其是订阅/Snell/VLESS 来源拉取、过滤后空组、目标格式级联过滤和渲染错误。

B2 分组预览执行到 ValidateGraph：若后端返回图级错误，页面显示结构化诊断并保留草稿，不展示部分成功的分组结果。

前端运行时预览缓存以 `runtime_config_revision` 为边界。reload 成功或 status poll 发现 `runtime_config_revision` 变化后，B1/B2/B3 当前运行时预览必须重新拉取；草稿预览仍由用户显式触发，不随字段输入自动请求远程来源。

### 编辑期 revision 监控

用户长时间编辑配置期间，外部进程（GitOps、其他标签页、手动编辑）可能已修改配置文件。若直到保存时才发现 revision 冲突，已编辑内容需要手动合并，体验差。

缓解策略（纯前端行为，无需后端改动）：

- 前端在编辑页面活跃期间，定期（建议 30s）poll `GET /api/status` 的 `config_revision`
- 当 revision 与编辑起始时的 revision 不一致时，顶栏显示 warning："配置文件已被外部修改，保存前请对比最新版本"
- 不自动覆盖用户正在编辑的草稿，仅提示用户自行决定是否重新加载

## 保序字段的 JSON 表示

YAML 配置中 `groups` / `routing` / `rulesets` 使用 OrderedMap 保序。JSON 序列化规范（含完整示例和 round-trip 不变量）定义在 `config-schema.md` §JSON API 表示。以下为前端消费示例：

```json
{
  "groups": [
    { "key": "🇭🇰 Hong Kong", "value": { "match": "(港|HK)", "strategy": "select" } },
    { "key": "🇸🇬 Singapore", "value": { "match": "(新加坡|SG)", "strategy": "select" } }
  ]
}
```

前端编辑保序字段时，拖拽排序直接操作数组索引，保证写回后顺序不变。`sources.fetch_order` 用于保存 `subscriptions` / `snell` / `vless` 的拉取顺序；A6 `rules` 是普通数组，拖拽排序也必须直接保持数组顺序。

---

## 非功能需求

- 最低支持分辨率：1280x800
- 支持浅色/深色主题
- 静态校验不依赖外部网络（前端格式级校验 + 后端 Prepare 校验）；运行时预览和生成预览可能访问远程订阅与模板
- 页面加载后可离线操作配置编辑（保存/预览需网络）
- 当后端 API 不可用时（网络错误或 `api` 容器宕机），SPA 应显示明确的连接错误提示，而非空白页面或静默失败
