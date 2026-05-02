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
- `T-ADM-002`：PUT 非法配置返回 400 + 结构化错误，诊断项包含 `code`、`display_path` 和 `locator.json_pointer`
- `T-ADM-003`：Validate 返回 errors / warnings / infos 三级结构，且只执行 JSON 反序列化与 `Prepare` 静态校验
- `T-ADM-004`：Reload 成功路径（新 RuntimeConfig 生效）
- `T-ADM-005`：Reload 失败路径（旧 RuntimeConfig 不变）
- `T-ADM-006`：慢速生成/预览请求不持有配置读锁，不阻塞 reload 指针替换
- `T-ADM-007`：保序字段 JSON round-trip 顺序不变
- `T-ADM-008`：HTTP(S) 配置源下 `PUT /api/config` 返回 409，且不尝试写回远端
- `T-ADM-009`：`GET /api/config` 返回 `config_revision=sha256:<hex>`
- `T-ADM-010`：`PUT /api/config` 缺少 revision 返回 400，revision 冲突返回 `409 config_revision_conflict` 且不写入
- `T-ADM-011`：远程主配置 URL 在 TTL 未过期时执行 reload 仍读取最新内容
- `T-ADM-012`：`/api/*` 接受 `Authorization: Bearer ...`，query token 仅对 `/generate` 兼容路径生效
- `T-ADM-013`：`internal/admin` 不直接依赖 `internal/pipeline` 或 `internal/model`
- `T-ADM-014`：诊断定位对含空格、点号或 emoji 的 `groups` / `routing` / `rulesets` key 仍稳定，前端可通过 `locator.index` / `locator.json_pointer` 定位

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
- `T-PRV-010`：本地配置源的 `GET /api/status` 每次重算 sha256，能发现同大小且 mtime 未变化的外部改写
- `T-PRV-011`：`/generate` 与 `/api/generate/preview` 中，`CodeTargetClashFallbackEmpty` / `CodeTargetSurgeFallbackEmpty` 经 HTTP 层返回 400，projection invariant 类 TargetError 仍返回 500

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

---

## 前端测试（v2.0）

建议覆盖：

- `T-WEB-001`：组件渲染测试（React Testing Library）
- `T-WEB-002`：API 集成测试（mock backend）
- `T-WEB-003`：交互模式测试（Modal / Toast / Confirm / Spinner / Drawer）
- `T-WEB-004`：保序字段编辑后顺序不变
- `T-WEB-005`：校验结果展示与跳转定位使用 `locator.json_pointer`，`display_path` 只作为用户可读文案
- `T-WEB-006`：A2/A3 编辑态调用 POST 草稿预览，而 B1/B2 运行时页调用 GET 预览
- `T-WEB-007`：token 输入后 API client 使用 Authorization header；复制订阅链接时显式确认 query token
- `T-WEB-008`：本地可写配置首次保存前显示 YAML 注释、引号和格式风格可能丢失的确认；用户确认后才发起 `PUT /api/config`
- `T-WEB-009`：B2 分组预览页面展示节点组/链式组/服务组树形结构，ValidateGraph 失败时显示诊断且不展示部分成功结果
- `T-WEB-010`：B3 生成下载页面完成预览 → 下载 → 复制订阅链接全流程；未配置 `base_url` 时复制订阅链接功能给出提示
- `T-WEB-011`：A5 规则集页面 URL/Policy 绑定编辑，多条 URL 顺序不变（归属 M10）
- `T-WEB-012`：A6 内联规则页面自由文本 + Policy 选择器编辑（归属 M10）
- `T-WEB-013`：A7 其他配置页面 fallback / base_url / templates 字段编辑（归属 M10）
- `T-WEB-014`：A8 静态校验 Drawer 展示 errors/warnings/infos 三级，通过 `locator.json_pointer` 跳转到对应页面字段（归属 M10）
- `T-WEB-015`：B2 分组预览页面树形展示，ValidateGraph 失败时显示诊断且不展示部分成功结果（归属 M10）
- `T-WEB-016`：B3 生成下载页面预览 → 下载 → 复制订阅链接全流程，含草稿预览模式（归属 M10）
- `T-WEB-017`：[占位] 端到端测试：空配置 → 逐步编辑 → 保存 → 重载 → 预览 → 生成 → 下载（归属 M10）
- `T-WEB-018`：[占位] 端到端测试：错误路径覆盖（静态校验失败/分组预览图级错误/生成预览失败/重载失败）（归属 M10）
- `T-WEB-019`：[占位] 端到端测试：两种输出格式的内容校验（归属 M10）
- `T-WEB-020`：[占位] 端到端测试：HTTP(S) 配置源只读模式全流程（归属 M10）

---

## Admin API 补充测试（归属 M7）

建议覆盖：

- `T-ADM-011`：远程主配置 URL 在 TTL 未过期时执行 reload 仍 bypass 缓存，读取最新内容
- `T-ADM-012`：`/api/*` 仅接受 `Authorization: Bearer ...` header，query token 仅对 `/generate` 兼容路径生效
- `T-ADM-013`：`internal/admin` 包不直接依赖 `internal/pipeline` 或 `internal/model`
- `T-ADM-014`：诊断定位对含空格、点号或 emoji 的 `groups` / `routing` / `rulesets` key 仍稳定，前端可通过 `locator.index` 和 `locator.json_pointer` 定位
