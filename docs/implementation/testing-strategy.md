# 测试策略

## 目标

本文件定义系统在实现阶段需要覆盖的核心验证范围，确保管道、分组和渲染行为稳定。

---

## 单元测试

建议覆盖：

- 保序映射解析
- SS URI 解析
- Snell Surge 行解析（valid / invalid / skip / duplicate key / 边界）
- VLESS URI 解析（valid / invalid / transport fallback / encryption 透传 / transport query dispatch）
- SIP002 明文 `userinfo`
- SS plugin query 解析与转义处理
- 拉取类节点过滤（订阅 + Snell + VLESS）
- 地区节点组匹配（订阅 + Snell + VLESS）
- `relay_through` 三种模式展开
- Snell 节点可作为 `relay_through` 上游
- VLESS 节点可作为 `relay_through` 上游
- `@all` 不包含链式节点
- `@all` 包含全部原始节点（订阅 + Snell + VLESS + 不带 `relay_through` 的自定义）
- `@auto` 展开为节点组+包含 `@all` 的服务组+DIRECT，去重且排除自身
- `REJECT` 不在 `@auto` 中，需显式声明且位置保持不变
- 同一 entry 内重复 `@auto` 会被静态校验拒绝
- `@auto` 与 `@all` 在同一 entry 中互斥
- `Route(cfg, nil)` 按空 `GroupResult` 处理，不发生 panic
- `routing` 不允许显式引用原始代理名
- 代理名、节点组名、服务组名共享命名空间无冲突
- 服务组引用校验
- 循环引用校验
- Snell 单行失败错误携带脱敏 URL 与 1-based 物理行号
- VLESS 单行失败错误携带脱敏 URL 与 1-based 物理行号

---

## 渲染测试

建议覆盖：

- Clash Meta 输出快照
- Surge 输出快照
- 链式节点渲染字段
- Clash 的 Snell 级联过滤与 fallback 清空路径
- Clash 的 VLESS 节点渲染（含 encryption 透传、transport fallback、transport opts）
- Surge 的 VLESS 级联过滤与 fallback 清空路径
- 清空路径中的 `(snell)` / `(chained)` 标记，以及共享掉落子图不误报 `(cycle)`
- Clash Meta 的通用 SS plugin 透传
- Surge 对不支持 SS plugin 的错误路径
- ruleset 输出顺序
- fallback 输出位置
- Clash / Surge 的 `url-test` 默认参数一致

---

## 集成测试

建议覆盖：

- 从示例配置生成 Clash Meta
- 从示例配置生成 Surge
- 真实订阅样本的解析回归
- 订阅拉取失败场景
- 配置非法场景

---

## 验收重点

- 面板顺序与配置书写顺序一致
- 同一份配置在两种输出中语义一致
- 链式组出现在节点组层，而不是服务组层
- 所有节点组都显式指定策略

---

## Admin API 测试（v2.0）

建议覆盖：

- `T-ADM-001`：Config CRUD round-trip（`GET → PUT → GET` 幂等）
- `T-ADM-002`：PUT 配置语义校验失败返回 400 + `ValidateResult`（`valid=false`），诊断项包含 `code`、`display_path` 和 `locator.json_pointer`
- `T-ADM-003`：Validate 请求体合法时始终返回 200；配置无效返回 `valid=false` + errors / warnings / infos，JSON 无法解析、缺少 `config` 或 `config` 非对象返回 400
- `T-ADM-004`：Reload 成功路径（新 RuntimeConfig 生效）
- `T-ADM-005`：Reload 失败路径（旧 RuntimeConfig 不变）
- `T-ADM-006`：慢速 `/generate` 请求不持有配置读锁，不阻塞 reload 指针替换
- `T-ADM-007`：保序字段 JSON round-trip 顺序不变，覆盖 `groups` / `routing` / `rulesets`、`sources.fetch_order` 和 `rules`；`sources.fetch_order` 同时覆盖缺失/空值默认、完整排列、重复值、未知值和缺项
- `T-ADM-008`：HTTP(S) 配置源下 `PUT /api/config` 返回 `409 config_source_readonly`，且不尝试写回远端
- `T-ADM-009`：`GET /api/config` 返回 `config_revision=sha256:<hex>`
- `T-ADM-010`：`PUT /api/config` 缺少 revision 返回 400，revision 冲突返回 `409 config_revision_conflict` 且不写入
- `T-ADM-011`：远程主配置 URL 在 TTL 未过期时执行 reload 仍读取最新内容
- `T-ADM-012`：受保护 `/api/*` 仅接受有效 `session_id` Cookie；缺少 session 返回 `401 auth_required`，过期 / 注销 session 返回 `401 session_expired`；`Authorization: Bearer <SUBCONVERTER_TOKEN>` 不授予后台权限；query token 仅对 `/generate` 兼容路径生效
- `T-ADM-013`：`internal/admin` 不直接依赖 `internal/pipeline` 或 `internal/model`
- `T-ADM-014`：诊断定位对含空格、点号或 emoji 的 `groups` / `routing` / `rulesets` key 仍稳定，前端可通过 `locator.index` / `locator.json_pointer` 定位
- `T-ADM-015`：`groups` 为空时 `POST /api/config/validate` 返回 `200 valid=false` + 结构化配置诊断；`PUT /api/config` / `POST /api/reload` 返回 400，不允许保存或重载为有效配置
- `T-ADM-016`：本地配置文件或目录不可写时 `PUT /api/config` 返回 `409 config_file_not_writable`，且不覆盖原配置
- `T-ADM-017`：首次无管理员凭据时 `GET /api/auth/status` 返回 `setup_required=true` 与 `setup_token_required=true`；`POST /api/auth/setup` 缺少 setup token 返回 `401 setup_token_required`，错误 token 返回 `401 setup_token_invalid`，正确 token 才创建管理员并登录
- `T-ADM-018`：管理员密码以 `PBKDF2-HMAC-SHA256`、`600000` iterations、32-byte salt、32-byte derived key 写入；auth state 不保存明文密码；密码比较使用 constant-time；参数落后时登录成功路径会重哈希
- `T-ADM-019`：`POST /api/auth/login` 成功设置 HttpOnly `session_id` Cookie；未选择 remember 的 session 最长 24 小时，选择 remember 的 session 最长 7 天；auth state 只保存 session token 的 SHA-256 哈希；`POST /api/auth/logout` 清除 session
- `T-ADM-020`：登录失败按 IP + 用户名计数；错误凭据返回 `401 invalid_credentials` 和剩余次数；第 5 次失败返回 `423 auth_locked` 和解锁时间
- `T-ADM-021`：auth state 自动创建目录权限为 `0700`、文件权限为 `0600`；写入使用同目录临时文件、fsync、rename；auth state 不可写时 setup 返回 `409 auth_state_not_writable`
- `T-ADM-022`：Cookie session 下非安全管理请求校验同源 `Origin` 或 `Referer`

---

## 热重载测试（v2.0）

建议覆盖：

- `T-RLD-001`：并发 reload 互斥——第二个 `POST /api/reload` 在前一个执行中时立即返回 429，不排队等待
- `T-RLD-002`：reload 期间生成请求不阻塞——慢速 `/generate` 请求不持有配置读锁，不阻塞 reload 获取写锁
- `T-RLD-003`：reload 失败后旧 RuntimeConfig 不变——`Prepare` 校验失败返回错误，后续 `/generate` 仍使用旧配置生成
- `T-RLD-004`：reload 成功后新请求使用新配置——`POST /api/reload` 成功返回后，后续 `/generate` 使用新 RuntimeConfig
- `T-RLD-005`：reload 成功前已取得快照的请求继续使用旧配置完成——不发生中途切换
- `T-RLD-006`：reload 成功后 `GET /api/status` 的 `runtime_config_revision` 更新，`config_dirty` 变为 `false`
- `T-RLD-007`：远程主配置源读取失败时 reload 返回 502，旧 RuntimeConfig 不变，`config_dirty` 不被错误清除
- `T-RLD-008`：reload 429 后锁状态可恢复——当前 reload 完成后，后续重试可以成功进入 reload 流程
- `T-RLD-009`：reload 不拉取订阅/Snell/VLESS 来源；订阅源不可用不影响 reload 成功，但会由预览或生成路径返回对应错误

---

## 缓存契约测试（v2.0）

建议覆盖：

- `T-CCH-001`：远程主配置 reload 会 invalidate / bypass 该配置 URL 的未过期缓存
- `T-CCH-002`：reload 不主动 invalidate 订阅和模板缓存，未过期条目继续命中
- `T-CCH-003`：订阅 URL 变更后使用新 URL 作为缓存键；旧 URL 过期后不再命中，但测试不依赖后台主动清理

---

## 预览与状态 API 测试（v2.0）

建议覆盖：

- `T-PRV-001`：Preview nodes 返回正确的节点列表（含 Kind 和 filtered 标记，并覆盖 Included / Excluded）
- `T-PRV-002`：Preview groups 成功时返回节点组、链式组、服务组和 `@all` / `@auto` 展开结果
- `T-PRV-003`：Generate preview 与 generate 输出一致（仅 Content-Disposition 不同）
- `T-PRV-004`：Status 返回进程信息（版本、配置源位置与可写性、热重载状态），HTTP(S) 配置源下标记 `capabilities.config_write=false`
- `T-PRV-005`：POST 草稿 nodes 预览使用请求体配置，GET nodes 预览仍使用当前 RuntimeConfig
- `T-PRV-006`：POST 草稿 groups 预览返回草稿服务组展开结果，不改变运行时状态
- `T-PRV-007`：POST 草稿 generate preview 输出草稿配置结果，不改变 `config_dirty`、`last_reload` 或 RuntimeConfig
- `T-PRV-008`：POST 草稿 generate preview 能发现 `config/validate` 不覆盖的生成期问题，例如远程源为空、过滤后组为空、fallback 级联清空或模板渲染错误
- `T-PRV-009`：Preview groups 执行到 ValidateGraph；空节点组、空链式组、非法引用或循环引用返回 400 结构化诊断，不返回部分成功分组结果
- `T-PRV-010`：本地配置源的 `GET /api/status` 释放运行时配置锁后每次重算 sha256，能发现同大小且 mtime 未变化的外部改写，且不阻塞 reload 指针替换
- `T-PRV-011`：`/generate` 与 `/api/generate/preview` 中，`CodeTargetClashFallbackEmpty` / `CodeTargetSurgeFallbackEmpty` 经 HTTP 层返回 400，projection invariant 类 TargetError 仍返回 500
- `T-PRV-012`：reload 期间预览请求不阻塞——慢速 `/api/preview/*` 请求不持有配置读锁，不阻塞 reload 获取写锁
- `T-PRV-013`：`GET /api/generate/link` 要求管理员 session；`base_url` 缺失返回 `400 base_url_required`；服务端配置订阅 token 时可返回含 token 链接，未配置时返回 `token_included=false`；链接 token 来源必须是服务端订阅访问 token，而不是当前请求鉴权方式
- `T-PRV-014`：后台 session 调用 `/generate?format=surge` 时，即使请求未带 query token，Surge `#!MANAGED-CONFIG` 仍包含服务端订阅访问 token（若启用）和最终 filename

---

## Web 容器与 Compose 测试（v2.0）

建议覆盖：

- `T-SPA-001`：`web/Dockerfile` 可成功构建 nginx 静态发布镜像
- `T-SPA-002`：访问 `/` 返回 SPA `index.html`
- `T-SPA-003`：刷新任意前端路由时由 nginx fallback 到 `index.html`
- `T-SPA-004`：`/generate?format=clash` 与 `/generate?format=surge` 经 `web` 容器反向代理到 `api` 成功
- `T-SPA-005`：`/healthz` 经 `web` 容器反向代理到 `api` 成功
- `T-SPA-006`：生产 Compose 路径不依赖 CORS；浏览器访问的 Web 页面与 API 为同源
- `T-SPA-007`：M7 完成后补充验证 `/api/status` 经 `web` 容器反向代理到 `api` 成功
- `T-SPA-008`：刷新 B3 前端路由 `/download` 时由 nginx fallback 到 `index.html`，且不会命中后端 `/generate`

---

## 前端测试（v2.0）

建议覆盖：

- `T-WEB-001`：组件渲染与主题测试（React Testing Library；覆盖浅色/深色、系统偏好、手动切换、刷新后保持和关键状态可读性）
- `T-WEB-002`：API 集成测试（mock backend）
- `T-WEB-003`：交互模式测试（Modal / Toast / Confirm / Spinner / Drawer）
- `T-WEB-004`：保序字段编辑后顺序不变，覆盖 `groups` / `routing` / `rulesets`、`sources.fetch_order` 和 `rules`
- `T-WEB-005`：校验结果展示与跳转定位使用 `locator.json_pointer`，`display_path` 只作为用户可读文案
- `T-WEB-006`：A2/A3 编辑态调用 POST 草稿预览，而 B1/B2 运行时页调用 GET 预览
- `T-WEB-007`：登录页覆盖 idle、validating、invalid credentials、locked、redirecting、network error 和 setup；未登录访问受保护路由跳转 `/login?next=...`，session 失效后提示并跳回登录页
- `T-WEB-008`：本地可写配置首次保存前显示 YAML 注释、引号和格式风格可能丢失的确认；用户确认后才发起 `PUT /api/config`
- `T-WEB-009`：reload 成功或 status poll 发现 `runtime_config_revision` 变化后，当前已实现的运行时预览页使用新 query key 重新加载；B2/B3 在 M10 复用同一规则
- `T-WEB-010`：409 按 `error.code` 分流，分别覆盖 `config_revision_conflict`、`config_source_readonly`、`config_file_not_writable` 和未知 409 code
- `T-WEB-011`：A5 规则集页面 URL/Policy 绑定编辑，多条 URL 顺序不变（归属 M10）
- `T-WEB-012`：A6 内联规则页面自由文本 + Policy 选择器编辑（归属 M10）
- `T-WEB-013`：A7 其他配置页面 fallback / base_url / templates 字段编辑（归属 M10）
- `T-WEB-014`：A8 静态校验 Drawer 展示 errors/warnings/infos 三级，通过 `locator.json_pointer` 跳转到对应页面字段（归属 M10）
- `T-WEB-015`：B2 分组预览页面树形展示，ValidateGraph 失败时显示诊断且不展示部分成功结果（归属 M10）
- `T-WEB-016`：B3 生成下载页面预览 → 下载 → 复制订阅链接全流程，含草稿预览模式（归属 M10）
- `T-WEB-017`：端到端测试：本地可写配置全流程（归属 M10）
  - 测试入口：正式前端 E2E runner；后端使用临时本地配置文件、fake 订阅源和 fake 模板资源
  - fixture：最小可启动 YAML、至少一个 SS 订阅节点、一个规则集 URL、Clash / Surge 模板
  - 操作步骤：打开 SPA → 加载 `GET /api/config` 草稿 → 在 A1/A3/A4/A7 补齐来源、节点组、路由和 fallback → A8 validate → 首次保存确认 → `PUT /api/config` → `POST /api/reload` → B1/B2/B3 预览 → 下载生成结果
  - 预期结果：保存后返回新 `config_revision`；reload 后 `runtime_config_revision` 更新且 `config_dirty=false`；B1 有 active 节点；B2 展示节点组和服务组；B3 生成预览与下载内容一致
- `T-WEB-018`：端到端测试：错误路径覆盖（归属 M10）
  - 测试入口：正式前端 E2E runner；后端或 mock backend 提供可控错误响应
  - fixture：非法正则配置、会导致 ValidateGraph 失败的空组配置、会导致目标格式 fallback 清空的配置、reload 校验失败配置
  - 操作步骤：分别触发 A8 静态校验、B2 分组预览、B3 生成预览、保存后 reload
  - 预期结果：静态校验失败显示 `200 valid=false` 诊断；B2 图级错误不展示部分成功结果；B3 生成期错误保留草稿；reload 失败后旧 RuntimeConfig 不变并保持 dirty 提示
- `T-WEB-019`：端到端测试：Clash / Surge 双格式生成与下载路径（归属 M10）
  - 测试入口：正式前端 E2E runner；后端使用同一份已 reload 的本地配置
  - fixture：同时包含 SS、Snell、VLESS、rulesets、fallback 和 base_url 的配置，覆盖格式专属过滤
  - 操作步骤：在 B3 分别选择 Clash Meta 与 Surge → 调用当前运行时生成预览 → 触发下载 → 复制订阅链接确认
  - 预期结果：Clash 响应为 YAML 且不含 Snell；Surge 响应为 text/plain 且不含 VLESS；预览无 `Content-Disposition`，下载有附件响应；复制含 token 链接前出现确认
- `T-WEB-020`：端到端测试：HTTP(S) 配置源只读模式（归属 M10）
  - 测试入口：正式前端 E2E runner；后端以 HTTP(S) 主配置源启动
  - fixture：远程主配置 URL、fake 订阅源、有效订阅访问 token、可写 auth state
  - 操作步骤：打开 SPA → 读取 status/config → 进入 A1-A8 → 尝试编辑保存 → 运行 validate、B 区预览、B3 生成预览、reload
  - 预期结果：`capabilities.config_write=false`；编辑页禁用保存、新增、删除和排序；`PUT /api/config` 路径不可被静默触发；validate、preview、generate preview 和 reload 仍可用；409 `config_source_readonly` 进入只读模式并保留草稿
- `T-WEB-021`：端到端测试：登录、setup、logout 与订阅链接生成（归属 M9/M10）
  - 测试入口：正式前端 E2E runner；后端使用临时 auth state 和 fake 订阅访问 token
  - 操作步骤：首次打开 `/login` setup → 输入 setup token 并创建管理员 → 进入 `/sources` → logout → 登录 → 进入 B3 → 调用复制订阅链接
  - 预期结果：缺少或错误 setup token 时无法创建管理员；setup 成功后写入 session Cookie；logout 后受保护页面跳回登录；复制链接经 `GET /api/generate/link` 返回 URL，含 token 时先确认
