# 页面级实现矩阵

本文件补充 `pages.md` 的路由总览，定义正式 SPA 每个页面的字段、动作、状态和测试映射。页面实现时以顶层 `docs/design/*` 为后端契约来源，本文件只约束前端消费和交互落点。

## 通用页面状态

所有页面都必须处理以下状态：

| 状态 | 展示要求 | 交互要求 |
|------|----------|----------|
| loading | 显示骨架或明确加载文案；涉及订阅拉取时必须说明正在拉取远程来源 | 禁用当前 mutation 的主按钮 |
| empty | 说明当前列表或字段为空，并提供新增、编辑或前往配置入口 | 只读模式下只展示说明，不显示可执行新增入口 |
| error | 展示 HTTP 状态、错误码、用户可读信息（需恢复操作的场景提供重试或重载按钮） | 错误 Toast 默认持续显示直到手动关闭或被同 `scope` 反馈替换；无通用“查看详情”弹窗 |
| readonly | 配置源不可写时禁用保存、新增、删除和排序 | 仍允许校验、预览、生成预览和 reload |
| dirty | `config_dirty=true` 时提示已保存配置尚未生效 | 提供 reload 入口或跳转到状态页 |

诊断定位统一使用 `locator.json_pointer`。`display_path` 只用于展示，不作为程序定位依据。

## 认证页

### 登录 / 首次 setup

| 项目 | 契约 |
|------|------|
| 路由 | `/login` |
| 所属里程碑 | M9 |
| 字段 | 用户名、密码、记住我；setup 模式额外包含 setup token、确认密码和密码强度提示 |
| 主要动作 | 登录、首次创建管理员、显示/隐藏密码、浅色/深色主题切换、退出后重新登录 |
| 调用 API | `GET /api/auth/status`、`POST /api/auth/login`、`POST /api/auth/setup` |
| 跳转行为 | 未登录访问受保护页面跳转 `/login?next=<原路径>`；登录成功后跳回 `next` 或 `/sources` |
| loading / empty / error | 提交校验中仅提交按钮进入 loading/disabled；登录提交返回 `401 invalid_credentials` 或 `423 auth_locked` 时以右下角 Toast 分别展示剩余次数与解锁时间（首次加载若 `authStatus.locked_until` 未过期仅禁用按钮，不额外展示解锁时间）；`/api/auth/status` 失败 Toast 带重试按钮，登录/setup 提交网络错误 Toast 不带重试按钮 |
| setup 行为 | `setup_required=true` 时渲染 setup 模式；提交前必须填写 bootstrap setup token；密码至少 12 位；setup 失败（含 `setup_token_*`、`auth_state_not_writable`）以右下角 Toast 展示状态码与错误原因 |
| 对应测试 | `T-WEB-AUTH-LOCK-001`–`004`、`T-INT-AUTH-*`、`T-E2E-015` |

## A 区配置编辑页

### A1 订阅来源

| 项目 | 契约 |
|------|------|
| 路由 | `/sources` |
| 所属里程碑 | M9 |
| 字段 / JSON pointer 根 | `/config/sources`、`/config/sources/subscriptions`、`/config/sources/snell`、`/config/sources/vless`、`/config/sources/custom_proxies`、`/config/sources/fetch_order` |
| 主要动作 | 新增、编辑、删除、拖拽排序三类拉取来源及组内条目；节点订阅（TXT SS/AnyTLS 及 Clash/Surge/Loon 原生）、Snell/VLESS 节点池支持名称、链接/本地正文、自动读取、来源检查、歧义确认与显式编辑副本；编辑 custom proxy 与 `relay_through` |
| 调用 API | 启动加载用 `GET /api/config`；读取正文用 `POST /api/resources/read`；来源检查用 `POST /api/subscriptions/check`；保存工作流用 `POST /api/config/validate`、`PUT /api/config`；热重载由全局按钮单独调用 `POST /api/reload` |
| 只读行为 | 禁用新增、删除、排序、修改与保存；三类拉取来源仍允许点击“查看来源”弹窗自动读取、检查、展开正文与重新读取（自定义代理禁用打开） |
| dirty 行为 | 顶栏提示 reload；离开页面前不自动覆盖草稿 |
| loading / empty / error | 全局加载期侧栏显示骨架、主区显示 `LoadingState`；四类来源均为空时显示新增入口；保存失败按 `error.code` 展示 |
| 诊断定位 | `sources.*`（`url`/`name`/`content_file`/`format`/`custom_proxies.*`）定位到对应来源并自动打开编辑弹窗聚焦；`sources.fetch_order` 定位到来源排序控件 |
| 对应测试 | `T-WEB-SOURCE-*`、`T-WEB-LOCAL-*`、`T-WEB-CHECK-*`、`T-WEB-LOON-*`、`T-INT-SRC-*` |

### A2 过滤器

| 项目 | 契约 |
|------|------|
| 路由 | `/filters` |
| 所属里程碑 | M9 |
| 字段 / JSON pointer 根 | `/config/filters`、`/config/filters/exclude` |
| 主要动作 | 编辑 exclude 正则（防抖后端正则校验 + 常用排除模板一键拼接）；显式触发草稿节点预览（草稿变更后旧预览标注过期提示） |
| 调用 API | 防抖调用 `POST /api/config/validate` 做后端 Go 正则校验；`POST /api/preview/nodes` 用于草稿预览；保存工作流同 A1 |
| 只读行为 | 禁用编辑和保存；允许运行草稿预览以查看当前配置效果 |
| dirty 行为 | 显示 reload 提示；草稿预览不改变 dirty 状态 |
| loading / empty / error | 预览时显示“正在拉取订阅”；502 按上游拉取失败展示 |
| 诊断定位 | `/config/filters/exclude` 定位到正则输入 |
| 对应测试 | `T-WEB-UX-004`、`T-INT-FLT-*` |

### A3 节点分组

| 项目 | 契约 |
|------|------|
| 路由 | `/groups` |
| 所属里程碑 | M9 |
| 字段 / JSON pointer 根 | `/config/groups`、`/config/groups/{index}/key`、`/config/groups/{index}/value/match`、`/config/groups/{index}/value/strategy` |
| 主要动作 | 新增、编辑、删除、拖拽排序节点组；防抖后端正则校验；重命名影响提示与一键同步修改 `routing` / `relay_through` 引用；显式触发草稿分组预览（草稿变更后旧预览标注过期） |
| 调用 API | 防抖调用 `POST /api/config/validate`；`POST /api/preview/groups` 用于草稿预览；保存工作流同 A1 |
| 只读行为 | 禁用新增、编辑、删除、排序和保存；允许预览当前草稿副本 |
| dirty 行为 | 显示 reload 提示；排序只改变草稿，不改变运行时 |
| loading / empty / error | 预览时显示“正在拉取订阅”；groups 为空时显示新增入口；ValidateGraph 错误显示诊断，不展示部分成功结果 |
| 诊断定位 | 通过 `locator.index` 和 `locator.json_pointer` 定位到组名、match 或 strategy |
| 对应测试 | `T-WEB-UX-*`、`T-INT-GRP-*` |

### A4 路由策略

| 项目 | 契约 |
|------|------|
| 路由 | `/routing` |
| 所属里程碑 | M9 |
| 字段 / JSON pointer 根 | `/config/routing`、`/config/routing/{index}/key`、`/config/routing/{index}/value` |
| 主要动作 | 新增、编辑、删除、拖拽排序服务组；编辑及拖拽排序组内成员列表 |
| 调用 API | 保存工作流同 A1；可引用成员来自当前草稿的节点组、链式分组（带 `relay_through` 的 `custom_proxies`）、服务组，加上 `DIRECT`、`REJECT`、`@all`、`@auto` |
| 只读行为 | 禁用新增、编辑、删除、排序和保存 |
| dirty 行为 | 显示 reload 提示；成员变化只写入草稿 |
| loading / empty / error | 全局加载期显示 `LoadingState`；routing 为空时显示新增入口；非法引用由 validate 诊断展示 |
| 诊断定位 | 通过 `locator.json_pointer` 选中并高亮对应服务组卡片（当前未对名称输入框与单个成员 chip 细粒度绑定 pointer） |
| 对应测试 | `T-WEB-UX-*`、`T-INT-RTE-*` |

### A5 规则集

| 项目 | 契约 |
|------|------|
| 路由 | `/rulesets` |
| 所属里程碑 | M10 |
| 字段 / JSON pointer 根 | `/config/rulesets`、`/config/rulesets/{index}/key`、`/config/rulesets/{index}/value` |
| 主要动作 | 新增、编辑、删除、拖拽排序规则集绑定及各组内 URL 列表 |
| 调用 API | 保存工作流同 A1 |
| 只读行为 | 禁用新增、编辑、删除、排序和保存 |
| dirty 行为 | 显示 reload 提示 |
| loading / empty / error | rulesets 为空时显示新增入口；URL 格式或未知服务组由 validate 诊断展示 |
| 诊断定位 | 通过 `locator.index` 和 `locator.json_pointer` 定位到规则集 key 或 URL |
| 对应测试 | `T-WEB-011`、`T-INT-RST-*`、`T-E2E-010` |

### A6 内联规则

| 项目 | 契约 |
|------|------|
| 路由 | `/rules` |
| 所属里程碑 | M10 |
| 字段 / JSON pointer 根 | `/config/rules`、`/config/rules/{index}` |
| 主要动作 | 新增、编辑、删除、关键词搜索过滤、拖拽排序内联规则，以及通过底部 Policy 选择器快捷替换规则末段策略 |
| 调用 API | 保存工作流同 A1 |
| 只读行为 | 禁用新增、编辑、删除、排序和保存 |
| dirty 行为 | 显示 reload 提示 |
| loading / empty / error | rules 为空或搜索无命中时显示空态；规则语义错误由 validate 或生成预览展示 |
| 诊断定位 | 定位到对应规则行 |
| 对应测试 | `T-WEB-012`、`T-WEB-UX-*`、`T-INT-RUL-*` |

### A7 其他配置

| 项目 | 契约 |
|------|------|
| 路由 | `/settings` |
| 所属里程碑 | M10 |
| 字段 / JSON pointer 根 | `/config/fallback`、`/config/base_url`、`/config/templates`、`/config/templates/clash`、`/config/templates/surge`、`/config/templates/loon`、`/config/templates/inline/*` |
| 主要动作 | 编辑 fallback、base_url（含前端格式校验与链接预览），以及 Clash / Surge / Loon 模板路径或本地正文（自动读取、展开查看、编辑为本地副本） |
| 调用 API | 模板读取用 `POST /api/resources/read`；保存工作流同 A1 |
| 只读行为 | 禁用编辑、编辑副本和保存；仍允许模板自动读取、展开查看与重新读取 |
| dirty 行为 | 显示 reload 提示 |
| loading / empty / error | `base_url` 格式非法时前端实时提示（`fallback` 合法性由后端 validate 校验）；模板读取错误展示在对应编辑器读取状态中 |
| 诊断定位 | 定位到 fallback、base_url 或对应模板外层区块（当前 `ResourceEditor` 内输入框未细粒度绑定 pointer） |
| 对应测试 | `T-WEB-013`、`T-WEB-INLINE-*`、`T-WEB-LOON-*`、`T-INT-SET-*` |

### A8 静态校验

| 项目 | 契约 |
|------|------|
| 路由 | `/validate` |
| 所属里程碑 | M10 |
| 字段 / JSON pointer 根 | 全配置 `/config` |
| 主要动作 | 运行静态校验（或自动承接保存失败带入的校验结果）；展示 errors/warnings/infos；点击“跳转”直接导航到 A1-A7 字段；点击“详情”打开右侧只读 `DiagnosticDrawer` |
| 调用 API | `POST /api/config/validate` |
| 只读行为 | 允许校验；不提供保存修改入口 |
| dirty 行为 | dirty 不阻止静态校验，但提示校验对象是当前草稿 |
| loading / empty / error | 校验中显示明确状态；无诊断时显示通过；请求体错误显示 400；配置语义错误显示 `200 valid=false` |
| 诊断定位 | 使用 `locator.json_pointer` 映射目标路由；无法匹配任何已知段时默认回退至 `/sources` |
| 对应测试 | `T-WEB-014`、`T-WEB-UX-*`、`T-INT-VAL-*`、`T-E2E-010` |

## B 区运行时预览页

### B1 节点预览

| 项目 | 契约 |
|------|------|
| 路由 | `/nodes` |
| 所属里程碑 | M9 |
| 数据来源 | 当前 `RuntimeConfig` 的节点预览 |
| 主要动作 | 查看节点（顶部分类胶囊标 `Surge only` / `Clash / Loon`，表格标签按 `kind` 标 `Surge` / `Clash / Loon` 及 `AnyTLS` / `active` / `filtered`）、筛选 Kind/Type/名称、点击“重新拉取”手动刷新 |
| 调用 API | `GET /api/preview/nodes`；刷新时调用 React Query `refetch` |
| 只读行为 | 不受配置源可写性影响 |
| dirty 行为 | dirty 时提示当前预览仍基于运行时配置，不等于已保存草稿 |
| loading / empty / error | 首次进入自动请求；空节点显示来源为空或全部过滤提示；502 显示上游拉取失败（无运行版本时不发请求，当前尚未单独显示空运行态引导） |
| GET / POST 区分 | 本页只用 GET 运行时预览，不使用 POST 草稿预览 |
| 对应测试 | `T-INT-RTM-*` |

### B2 分组预览

| 项目 | 契约 |
|------|------|
| 路由 | `/preview/groups` |
| 所属里程碑 | M10 |
| 数据来源 | 当前 `RuntimeConfig` 的分组、链式组、服务组和宏展开结果 |
| 主要动作 | 以节点组卡片、服务组展开列表（前 6 项 + `+{n}` 折叠）和链式组列表展示分组及 `@all` / `@auto` 展开（无常驻手动刷新按钮；非图级错误时提供“重试预览”） |
| 调用 API | `GET /api/preview/groups` |
| 只读行为 | 不受配置源可写性影响 |
| dirty 行为 | dirty 时提示当前预览仍基于运行时配置 |
| loading / empty / error | ValidateGraph 错误显示结构化诊断，不展示部分成功结果（无运行版本时不发请求） |
| GET / POST 区分 | 本页只用 GET 运行时预览；编辑页 A3 使用 POST 草稿预览 |
| 对应测试 | `T-WEB-015`、`T-INT-RTM-*`、`T-E2E-010` |

### B3 生成下载

| 项目 | 契约 |
|------|------|
| 路由 | `/download` |
| 所属里程碑 | M10 |
| 数据来源 | 当前运行时生成预览、运行时节点统计与无 token 订阅链接预览 |
| 主要动作 | 展示节点/分组/路由/规则统计；自动展示 Clash/Surge/Loon 三种格式运行时预览；下载配置；复制订阅链接（含 token 确认）；导入 YAML/ZIP 到草稿；导出已保存配置包与当前生效配置包 |
| 调用 API | `GET /api/generate/preview?format=...`、`GET /api/preview/nodes`、`/generate?format=...`、`GET /api/generate/link?format=...`、`POST /api/config/import`、`GET /api/config/saved.zip`、`GET /api/config/effective.zip` |
| 只读行为 | 允许预览、下载、复制链接与导出配置包；禁用导入配置；不提供保存入口 |
| dirty 行为 | dirty 时提示当前预览基于运行时配置，已保存草稿尚未 reload；存在未保存草稿时提示导出已保存配置前需先保存 |
| loading / empty / error | 无运行版本时隐藏下载卡片并禁用“导出生效配置”，显示初始化引导；加载时显示“正在生成三种格式预览”；预览失败提供“重试生成”按钮 |
| GET / POST 区分 | 当前页面只用 GET 读取当前运行时；后端 POST 草稿生成预览 API 保留但不在页面暴露 |
| 对应测试 | `T-WEB-016`、`T-WEB-022`–`024`、`T-WEB-BOOT-*`、`T-INT-RTM-*`、`T-E2E-010`、`T-E2E-014`、`T-E2E-015` |

## C 区系统状态页

### C 系统状态

| 项目 | 契约 |
|------|------|
| 路由 | `/status` |
| 所属里程碑 | M9 |
| 数据来源 | `GET /api/status` 与 `GET /healthz` |
| 主要动作 | 查看服务健康与运行时长、版本/commit/构建日期、配置状态（未创建/待重载/已加载）与条目统计、最近 reload 历史、运行环境 6 项指标与 `/healthz` 探针行（热重载操作由全局顶栏“热重载”按钮触发；页面正文当前未展开可写性与 revision 字段） |
| 调用 API | `GET /api/status`、`GET /healthz`（全局顶栏调用 `POST /api/reload`） |
| 只读行为 | 远程配置源或本地不可写配置仍允许通过顶栏触发 reload |
| dirty 行为 | 配置卡片显示“待重载”，顶栏提示并允许触发 reload，成功后刷新 status |
| loading / empty / error | status 加载显示 `LoadingState`；status/healthz 失败显示 `ErrorState`；429 reload 由全局工作流执行 1.5s 退避并重试一次 |
| 远程配置边界 | 不把 `GET /api/status` 解释为远程配置源实时探测；HTTP(S) 配置源 revision 基于最近观测结果 |
| 对应测试 | `T-WEB-UX-*`、`T-INT-STA-*` |


节点订阅和模板的正文编辑交互以 [Web UI 设计](../../docs/design/web-ui.md#节点与模板正文编辑) 为准。现有引用可读取为正文副本，随后使用全局保存、热重载流程。
