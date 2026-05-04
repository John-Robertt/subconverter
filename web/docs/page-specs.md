# 页面级实现矩阵

本文件补充 `pages.md` 的路由总览，定义正式 SPA 每个页面的字段、动作、状态和测试映射。页面实现时以顶层 `docs/design/*` 为后端契约来源，本文件只约束前端消费和交互落点。

## 通用页面状态

所有页面都必须处理以下状态：

| 状态 | 展示要求 | 交互要求 |
|------|----------|----------|
| loading | 显示骨架或明确加载文案；涉及订阅拉取时必须说明正在拉取远程来源 | 禁用当前 mutation 的主按钮 |
| empty | 说明当前列表或字段为空，并提供新增、编辑或前往配置入口 | 只读模式下只展示说明，不显示可执行新增入口 |
| error | 展示 HTTP 状态、错误码、用户可读信息和可重试动作 | 失败 Toast 不自动消失，提供查看详情 |
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
| 主要动作 | 登录、首次创建管理员、显示/隐藏密码、退出后重新登录 |
| 调用 API | `GET /api/auth/status`、`POST /api/auth/login`、`POST /api/auth/setup` |
| 跳转行为 | 未登录访问受保护页面跳转 `/login?next=<原路径>`；登录成功后跳回 `next` 或 `/sources` |
| loading / empty / error | 校验中禁用表单；`401 invalid_credentials` 展示剩余次数；`423 auth_locked` 展示解锁时间；网络错误提供重试 |
| setup 行为 | `setup_required=true` 时渲染 setup 模式；提交前必须填写 bootstrap setup token；密码至少 12 位；auth state 不可写时展示部署配置错误 |
| 对应测试 | `T-WEB-001`、`T-WEB-007`、`T-WEB-021` |

## A 区配置编辑页

### A1 订阅来源

| 项目 | 契约 |
|------|------|
| 路由 | `/sources` |
| 所属里程碑 | M9 |
| 字段 / JSON pointer 根 | `/config/sources`、`/config/sources/subscriptions`、`/config/sources/snell`、`/config/sources/vless`、`/config/sources/custom_proxies`、`/config/sources/fetch_order` |
| 主要动作 | 新增、编辑、删除、排序拉取类来源；编辑 custom proxy 与 `relay_through` |
| 调用 API | 启动加载用 `GET /api/config`；保存工作流用 `POST /api/config/validate`、`PUT /api/config`；热重载由全局按钮单独调用 `POST /api/reload` |
| 只读行为 | 禁用新增、编辑、删除、排序和保存；URL 脱敏展示仍可查看 |
| dirty 行为 | 顶栏提示 reload；离开页面前不自动覆盖草稿 |
| loading / empty / error | config 加载显示骨架；四类来源均为空时显示新增入口；保存失败按 `error.code` 展示 |
| 诊断定位 | `sources.*.url` 定位到对应来源输入；`sources.fetch_order` 定位到来源排序控件 |
| 对应测试 | `T-WEB-004`、`T-WEB-008`、`T-WEB-017`、`T-WEB-020` |

### A2 过滤器

| 项目 | 契约 |
|------|------|
| 路由 | `/filters` |
| 所属里程碑 | M9 |
| 字段 / JSON pointer 根 | `/config/filters`、`/config/filters/exclude` |
| 主要动作 | 编辑 exclude 正则；显式触发草稿节点预览 |
| 调用 API | `POST /api/preview/nodes` 用于草稿预览；保存工作流同 A1 |
| 只读行为 | 禁用编辑和保存；允许运行草稿预览以查看当前配置效果 |
| dirty 行为 | 显示 reload 提示；草稿预览不改变 dirty 状态 |
| loading / empty / error | 预览时显示“正在拉取订阅”；exclude 为空时说明不过滤节点；502 按上游拉取失败展示 |
| 诊断定位 | `/config/filters/exclude` 定位到正则输入 |
| 对应测试 | `T-WEB-006`、`T-WEB-017`、`T-WEB-018` |

### A3 节点分组

| 项目 | 契约 |
|------|------|
| 路由 | `/groups` |
| 所属里程碑 | M9 |
| 字段 / JSON pointer 根 | `/config/groups`、`/config/groups/{index}/key`、`/config/groups/{index}/value/match`、`/config/groups/{index}/value/strategy` |
| 主要动作 | 新增、编辑、删除、拖拽排序节点组；显式触发草稿分组预览 |
| 调用 API | `POST /api/preview/groups` 用于草稿预览；保存工作流同 A1 |
| 只读行为 | 禁用新增、编辑、删除、排序和保存；允许预览当前草稿副本 |
| dirty 行为 | 显示 reload 提示；排序只改变草稿，不改变运行时 |
| loading / empty / error | 预览时显示“正在拉取订阅”；groups 为空时显示新增入口；ValidateGraph 错误显示诊断，不展示部分成功结果 |
| 诊断定位 | 通过 `locator.index` 和 `locator.json_pointer` 定位到组名、match 或 strategy |
| 对应测试 | `T-WEB-004`、`T-WEB-005`、`T-WEB-006`、`T-WEB-018` |

### A4 路由策略

| 项目 | 契约 |
|------|------|
| 路由 | `/routing` |
| 所属里程碑 | M9 |
| 字段 / JSON pointer 根 | `/config/routing`、`/config/routing/{index}/key`、`/config/routing/{index}/value` |
| 主要动作 | 新增、编辑、删除、拖拽排序服务组；编辑成员列表 |
| 调用 API | 保存工作流同 A1；可引用成员来自当前草稿的 groups/routing 加上 `DIRECT`、`REJECT`、`@all`、`@auto` |
| 只读行为 | 禁用新增、编辑、删除、排序和保存 |
| dirty 行为 | 显示 reload 提示；成员变化只写入草稿 |
| loading / empty / error | config 加载显示骨架；routing 为空时显示新增入口；非法引用由 validate 诊断展示 |
| 诊断定位 | 通过 `locator.json_pointer` 定位到服务组名或成员数组项 |
| 对应测试 | `T-WEB-004`、`T-WEB-005`、`T-WEB-017`、`T-WEB-018` |

### A5 规则集

| 项目 | 契约 |
|------|------|
| 路由 | `/rulesets` |
| 所属里程碑 | M10 |
| 字段 / JSON pointer 根 | `/config/rulesets`、`/config/rulesets/{index}/key`、`/config/rulesets/{index}/value` |
| 主要动作 | 新增、编辑、删除、排序规则集绑定；维护 URL 列表 |
| 调用 API | 保存工作流同 A1 |
| 只读行为 | 禁用新增、编辑、删除、排序和保存 |
| dirty 行为 | 显示 reload 提示 |
| loading / empty / error | rulesets 为空时显示新增入口；URL 格式或未知服务组由 validate 诊断展示 |
| 诊断定位 | 通过 `locator.index` 和 `locator.json_pointer` 定位到规则集 key 或 URL |
| 对应测试 | `T-WEB-011`、`T-WEB-017` |

### A6 内联规则

| 项目 | 契约 |
|------|------|
| 路由 | `/rules` |
| 所属里程碑 | M10 |
| 字段 / JSON pointer 根 | `/config/rules`、`/config/rules/{index}` |
| 主要动作 | 新增、编辑、删除、拖拽排序内联规则 |
| 调用 API | 保存工作流同 A1 |
| 只读行为 | 禁用新增、编辑、删除、排序和保存 |
| dirty 行为 | 显示 reload 提示 |
| loading / empty / error | rules 为空时说明无内联规则；规则语义错误由 validate 或生成预览展示 |
| 诊断定位 | 定位到对应规则行 |
| 对应测试 | `T-WEB-004`、`T-WEB-012`、`T-WEB-017` |

### A7 其他配置

| 项目 | 契约 |
|------|------|
| 路由 | `/settings` |
| 所属里程碑 | M10 |
| 字段 / JSON pointer 根 | `/config/fallback`、`/config/base_url`、`/config/templates`、`/config/templates/clash`、`/config/templates/surge` |
| 主要动作 | 编辑 fallback、base_url、模板路径 |
| 调用 API | 保存工作流同 A1 |
| 只读行为 | 禁用编辑和保存 |
| dirty 行为 | 显示 reload 提示 |
| loading / empty / error | fallback 为空或不可引用时提示；模板读取错误由生成预览或 `/generate` 展示 |
| 诊断定位 | 定位到 fallback、base_url 或模板路径字段 |
| 对应测试 | `T-WEB-013`、`T-WEB-017`、`T-WEB-019` |

### A8 静态配置校验

| 项目 | 契约 |
|------|------|
| 路由 | `/validate` |
| 所属里程碑 | M10 |
| 字段 / JSON pointer 根 | 全配置 `/config` |
| 主要动作 | 运行静态校验；展示 errors/warnings/infos；点击诊断跳转到 A1-A7 字段 |
| 调用 API | `POST /api/config/validate` |
| 只读行为 | 允许校验；不提供保存修改入口 |
| dirty 行为 | dirty 不阻止静态校验，但提示校验对象是当前草稿 |
| loading / empty / error | 校验中显示明确状态；无诊断时显示通过；请求体错误显示 400；配置语义错误显示 `200 valid=false` |
| 诊断定位 | 必须使用 `locator.json_pointer`，定位失败时跳到父级 section |
| 对应测试 | `T-WEB-005`、`T-WEB-014`、`T-WEB-018` |

## B 区运行时预览页

### B1 节点预览

| 项目 | 契约 |
|------|------|
| 路由 | `/nodes` |
| 所属里程碑 | M9 |
| 数据来源 | 当前 `RuntimeConfig` 的节点预览 |
| 主要动作 | 查看节点、筛选 Kind/Type/名称、手动刷新预览 |
| 调用 API | `GET /api/preview/nodes`；刷新时调用 React Query `refetch` |
| 只读行为 | 不受配置源可写性影响 |
| dirty 行为 | dirty 时提示当前预览仍基于运行时配置，不等于已保存草稿 |
| loading / empty / error | 首次进入自动请求；空节点显示来源为空或全部过滤提示；502 显示上游拉取失败 |
| GET / POST 区分 | 本页只用 GET 运行时预览，不使用 POST 草稿预览 |
| 对应测试 | `T-WEB-006`、`T-WEB-009`、`T-WEB-017` |

### B2 分组预览

| 项目 | 契约 |
|------|------|
| 路由 | `/preview/groups` |
| 所属里程碑 | M10 |
| 数据来源 | 当前 `RuntimeConfig` 的分组、链式组、服务组和宏展开结果 |
| 主要动作 | 查看树形分组、查看 `@all` / `@auto` 展开、手动刷新预览 |
| 调用 API | `GET /api/preview/groups` |
| 只读行为 | 不受配置源可写性影响 |
| dirty 行为 | dirty 时提示当前预览仍基于运行时配置 |
| loading / empty / error | ValidateGraph 错误显示结构化诊断，不展示部分成功结果 |
| GET / POST 区分 | 本页只用 GET 运行时预览；编辑页 A3 使用 POST 草稿预览 |
| 对应测试 | `T-WEB-009`、`T-WEB-015`、`T-WEB-018` |

### B3 生成下载

| 项目 | 契约 |
|------|------|
| 路由 | `/download` |
| 所属里程碑 | M10 |
| 数据来源 | 当前运行时生成预览 |
| 主要动作 | 自动展示 Clash/Surge 双格式运行时预览；下载；复制订阅链接 |
| 调用 API | `GET /api/generate/preview?format=...`、`/generate?format=...`、`GET /api/generate/link?format=...` |
| 只读行为 | 允许预览、下载和复制链接；不提供保存入口 |
| dirty 行为 | dirty 时提示当前预览基于运行时配置，已保存草稿尚未 reload |
| loading / empty / error | 生成中显示格式与来源；TargetError 400 和 RenderError/内部错误按 API client 归一化展示 |
| GET / POST 区分 | 当前页面只用 GET 读取当前运行时；后端 POST 草稿生成预览 API 保留但不在页面暴露 |
| 对应测试 | `T-WEB-016`、`T-WEB-017`、`T-WEB-019`、`T-WEB-020` |

## C 区系统状态页

### C 系统状态

| 项目 | 契约 |
|------|------|
| 路由 | `/status` |
| 所属里程碑 | M9 |
| 数据来源 | `GET /api/status` 与 `GET /healthz` |
| 主要动作 | 查看健康、版本、配置源、可写性、revision、dirty、最近 reload；通过全局“热重载”按钮触发 reload |
| 调用 API | `GET /api/status`、`GET /healthz`、`POST /api/reload` |
| 只读行为 | 只读配置源仍允许 reload；展示 `capabilities.config_write=false` |
| dirty 行为 | dirty 时提供 reload 入口；reload 成功后刷新 status |
| loading / empty / error | status 加载显示骨架；healthz 失败显示连接错误；429 reload 展示短间隔退避或可重试提示 |
| 远程配置边界 | 不把 `GET /api/status` 解释为远程配置源实时探测；HTTP(S) 配置源 revision 基于最近观测结果 |
| 对应测试 | `T-WEB-007`、`T-WEB-009`、`T-WEB-010`、`T-WEB-018`、`T-WEB-020` |
