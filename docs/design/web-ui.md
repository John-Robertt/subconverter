# Web 管理后台设计

> 状态提示：本文描述 v2.0 Web 管理后台当前契约；`web/` 是正式 Vite SPA 工程，生产镜像将构建产物嵌入 Go 二进制。能力状态见 docs/README.md。

## 目标

本文件定义 Web 管理后台的前端架构、页面结构和后端集成方式。subconverter v2.0 在 v1.0 纯 API 基础上新增 Web 管理后台，让用户可通过浏览器可视化编辑配置、预览运行时数据、生成并下载目标格式配置。

---

## 设计原则

- YAML 是真相源：UI 是 YAML 配置的可视化外壳，前端草稿承载尚未保存的编辑，保存后写回 YAML 和正文文件
- 任何 UI 元素都能映射回 YAML 某个 key
- 草稿与运行时分离：编辑页基于前端草稿调用 POST 预览；运行时预览页基于当前生效配置调用 GET 预览
- 本地可写配置源：保存操作 = JSON → YAML 写回文件；热重载为独立显式操作
- 远程 HTTP(S) 配置源：后台以只读模式运行，可查看、校验、预览和热重载，但不提供保存写回

---

## 信息架构

三个功能区：

### A 区：配置编辑

对应 YAML 配置的各顶层段，拆分为独立页面：

| 页面 | 对应 YAML 段 | 核心功能 |
|------|-------------|---------|
| A1 订阅来源 | `sources` | 四类来源（subscriptions / snell / vless / custom_proxies）的卡片式管理；三类拉取来源均支持 URL 或本地正文编辑，每类来源独立区块；自定义代理含 `relay_through` 子表单 |
| A2 过滤器 | `filters` | exclude 正则编辑 + 草稿匹配预览（调用 `POST /api/preview/nodes` 展示当前草稿会匹配/排除的节点）。注意：预览会实际拉取订阅，应显示"正在拉取订阅..."提示 |
| A3 节点分组 | `groups` | 分组卡片列表（拖拽排序，保序）；每组：名称 + match 正则 + strategy 选择器 + 草稿匹配预览（调用 `POST /api/preview/groups`）。注意：预览会实际拉取订阅，应显示"正在拉取订阅..."提示 |
| A4 路由策略 | `routing` | 服务组列表（拖拽排序，保序）；每组：名称 + 成员选择器（可引用节点组、服务组、DIRECT、REJECT、`@all`、`@auto`）；约束提示 |
| A5 规则集 | `rulesets` | 按服务组分组展示 URL 列表；增删改及拖拽调整绑定和 URL 顺序 |
| A6 内联规则 | `rules` | 规则表格（拖拽排序）+ 语法提示 |
| A7 其他配置 | `fallback` / `base_url` / `templates` | fallback 下拉（已定义服务组）；base_url 输入；templates 路径输入 |
| A8 静态配置校验 | （全配置） | 调用 `POST /api/config/validate`；展示 errors / warnings / infos 三级；通过 `locator.json_pointer` 跳转到对应 A1–A7 页面的具体字段 |

### B 区：运行时预览

展示实际拉取和处理后的运行时数据：

| 页面 | 数据来源 | 核心功能 |
|------|---------|---------|
| B1 节点预览 | `GET /api/preview/nodes` | 全来源节点表格（按来源分类：订阅 / Snell / VLESS / 自定义）；显示名称、类型、服务器、端口、Kind；标注可由类型判断的格式限定（Snell = 仅 Surge / VLESS = Clash / Loon（按传输过滤）） |
| B2 分组预览 | `GET /api/preview/groups` | 当前运行时节点组、链式组和服务组结果；展示 `@all` / `@auto` 展开后的成员 |
| B3 生成下载 | `GET /api/generate/preview` + `/generate` + `GET /api/generate/link` + `GET /api/config/saved.zip` + `GET /api/config/effective.zip` + `POST /api/config/import` | 自动展示 Clash Meta / Surge / Loon 三种格式运行时生成内容（语法高亮）；下载目标格式配置；复制订阅链接（由服务端生成，可含订阅 token）；分别导出已保存配置与当前生效配置的可迁移 ZIP；导入 ZIP 或 YAML 到前端草稿 |

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
| 操作成功反馈 | 右下绿色 Toast | 约 4s 自动消失，并替换同一操作的旧反馈 |
| 操作失败反馈 | 右下反馈 | 普通提示 8s 后消失；需恢复操作的错误持续显示，同一操作合并；反馈正文不拦截下方控件，取消恢复操作后保留按钮 |
| 进行中状态 | 顶栏按钮 Spinner + 禁用 | 防止重复提交 |
| 不可撤销操作 | 居中红色确认弹窗 | 删除、重置等需二次确认 |
| 首次保存本地 YAML | 居中确认弹窗 | 提示注释、引号和格式风格可能丢失，用户确认后才发起 `PUT /api/config` |
| 导入配置包 | 文件选择 + 确认弹窗 | B3 中选择 `.zip/.yaml/.yml` 后调用 `POST /api/config/import` 替换前端草稿；两种导入都只还原草稿与正文，不创建文件；替换未保存草稿前确认；保存时统一生成正文文件并写回 YAML，再显式热重载 |
| 导出已保存配置包 | 下载链接 | 调用 `GET /api/config/saved.zip`，读取保存后的配置和正文；未保存修改需先保存，导出新保存内容无需热重载 |
| 导出生效配置包 | 下载链接 | B3 中调用 `GET /api/config/effective.zip` 下载当前 RuntimeConfig 对应的源 YAML 与模板；保存但未 reload 时仍导出旧生效配置包 |
| 校验修复 | 直接定位 + 详情 Drawer | “跳转”直接进入字段，来源诊断自动打开编辑弹窗并聚焦；“详情”单独展示定位信息 |

---

## 编辑与反馈约定

- 保存记录提交时的草稿快照。响应返回时只推进已保存基线与 revision；请求期间的新编辑保留在草稿中，保存按钮继续可用。
- 刷新或关闭页面时，有未保存草稿则触发浏览器离开提示；注销前确认丢弃草稿，站内切换页面保留草稿。
- 过滤和分组正则通过防抖后的静态校验使用后端 Go 正则规则，不拉取订阅；草稿变化后，旧预览标记为需重新预览，错误与“尚未运行”不同时展示。
- 节点组重排保留选中的编辑对象；重命名后展示仍使用旧名称的服务组与链式代理，并允许同步修改引用。
- 路由成员提供节点组、服务组、链式组和内置策略；普通自定义节点通过 `@all` 参与。
- 自定义代理名称或 URL 未填写时不能应用到草稿；内联规则搜索展示匹配数量和空结果，隐藏不在结果中的编辑器。搜索后的排序只调整匹配规则所在的位置，其他规则保持顺序。
- 运行时服务组展示 `@all` / `@auto` 展开后的策略成员，成员可能是节点、分组或内置策略；服务运行时长使用 `runtime_environment.uptime_seconds`。

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
- 产物发布：`pnpm web:embed` 构建 `web/dist/` 并同步到 `internal/webui/dist/`，生产镜像使用已同步产物嵌入 Go 二进制

---

## 前端-后端集成模型

### 生产模式

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
- 登录失败 `401 invalid_credentials` 时展示剩余次数；`423 auth_locked` 时展示锁定截止时间并禁用提交。页面按截止时间注册定时器，到期恢复提交并清除锁定提示；已过期的缓存时间不禁用登录。失败计数的过期规则见 [鉴权](api.md#鉴权)，实际登录请求仍由后端校验
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
| 后端草稿生成预览 API | `POST /api/generate/preview` | 前端草稿 `{ config }` | 否 |
| B3 当前生成预览 | `GET /api/generate/preview` | 当前 `RuntimeConfig` | 否 |

当前 Web UI 将保存与生效拆成两个显式动作：编辑草稿 → 校验 → 本地可写配置首次保存确认 → `PUT /api/config` 条件写回；随后用户通过全局“热重载”按钮调用 `POST /api/reload` 使配置生效。

中间态处理：`PUT /api/config` 成功但 `POST /api/reload` 失败时（如远程主配置源读取失败，或新配置未通过 `Prepare` 静态校验），配置文件已更新但 RuntimeConfig 仍为旧版。此时 UI 应保留 `config_dirty = true` 提示，并允许用户重新触发 reload。若用户在保存后、reload 前离开页面（浏览器崩溃或关闭标签页），下次打开时 `GET /api/status` 会返回 `config_dirty = true`，UI 应提示用户执行 reload 使配置生效。

其中 A8 和 reload 都只覆盖 `Prepare` 阶段的静态配置校验；生成可用性需要通过 B1/B2/B3 预览确认，尤其是订阅（TXT / Clash YAML / Surge CONF / Loon）/Snell/VLESS 来源拉取、过滤后空组、目标格式级联过滤和渲染错误。

B2 分组预览执行到 ValidateGraph：若后端返回图级错误，页面显示结构化诊断并保留草稿，不展示部分成功的分组结果。

前端运行时预览缓存以 `runtime_config_revision` 为边界。reload 成功或 status poll 发现 `runtime_config_revision` 变化后，B1/B2/B3 当前运行时预览必须重新拉取；编辑页草稿预览仍由用户显式触发，不随字段输入自动请求远程来源。后端草稿生成预览 API 保留，但当前 B3 页面不暴露独立草稿生成入口。

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


### 节点与模板正文编辑

`SourceEditor` 共用于节点订阅、Snell 和 VLESS 节点池，提供来源名称、链接或本地正文切换、读取、检查及正文展开。节点池使用“节点池名称”，按所属类型固定解析器，隐藏订阅格式选择；三类来源都在检查成功后才能应用到草稿。

名称随草稿、保存与导入导出保留；列表优先以名称作为主标题，正文行数或脱敏地址作为辅助信息。未命名时沿用来源摘要。只修改名称不重新检查正文，名称语义见[配置规约](config-schema.md#拉取来源的输入与展示)。

订阅与模板编辑器在进入编辑界面时自动读取已有引用；新增或修改地址后停止输入 400ms 再读取，避免逐键请求。订阅列表不批量拉取来源，模板按页面中已配置的引用读取。读取成功后保留引用，查看内容不会修改草稿。重新读取沿用服务端缓存策略，不表示强制绕过 TTL。

正文默认收起。“展开正文 / 收起正文”只控制正文文本框的可见性，不发起读取或检查，也不清空输入。来源地址、读取状态、订阅识别结果、格式选择、编辑入口和保存按钮始终可见。既有本地正文也默认收起，选择“本地正文”或“编辑为本地副本”时展开以便编辑。

三类来源先通过 `POST /api/resources/read` 读取原文，再通过 `POST /api/subscriptions/check` 携带 `kind` 检查这份原文，并携带 `source_location` 保留来源位置的识别提示。读取与检查独立反馈：格式待确认或解析失败时仍可展开原文、编辑副本；只有当前内容检查成功后才允许“应用到草稿”。更正格式只重新检查原文，不重新下载。修改输入使旧检查结果失效，旧异步响应不能覆盖当前内容。模板格式由 Clash / Surge / Loon 所属栏目确定，读取后即可查看，不执行节点检查。

引用正文以只读方式展示。“编辑为本地副本”保留已读取内容，切换到本地正文；保存后改由用户维护，不再跟随原地址更新，原资源不被覆盖。首次输入或读取失败时可选择粘贴正文修复。切换来源方式保留本次未保存的正文；更换地址后不复用旧来源的编辑副本。

订阅列表仅展示配置中的格式名称，例如“Surge 配置”；未指定格式或使用 `auto` 时显示“按内容识别”。这些文字使用中性色，不附加状态圆点或“解析方式”前缀。进入或重新进入列表不读取订阅，也不显示“尚未检查”等检查状态；检查仅在新建、编辑或只读查看某条来源时执行，结果留在对应编辑器中，不缓存到列表或写入 YAML。

待应用内容与草稿一致时，弹窗显示“配置未修改，无需保存”和“完成”，关闭弹窗不更新草稿；只读查看也使用“完成”。新增或有实际配置差异时才显示“应用到草稿”，订阅须检查成功才能应用，应用后顶部保存按钮按配置差异启用。保存与热重载沿用全局流程，“应用到草稿”不写回配置。正文文件的生成与复用遵循[正文存储规则](config-schema.md#templates)；写入失败保留草稿。

配置只读时允许自动读取、检查、展开和重新读取，禁止修改来源、编辑副本和保存。正文文件缺失时保留原引用，可在可写配置中重填正文或删除来源。订阅仍只提供节点，分组和规则由对应编辑页管理。

解析、保存及更新规则见 [配置规约](config-schema.md#web-订阅检查)，接口见 [订阅检查 API](api.md#post-apisubscriptionscheck)。
