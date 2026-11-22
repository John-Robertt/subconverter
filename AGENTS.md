# subconverter 项目协作指南

**项目成功标准**：每次修改在保持管道不变量（代理 ID 唯一、目标输出名唯一、组名互斥、保序、链式上游约束、`@all` 语义）的前提下正确扩展或修复功能，且 Clash Meta / Surge / Loon 三种输出格式的 golden 测试通过。

**决策路由**：通用工程方法、调研与取舍遵循全局规则；本文件约束管道阶段行为、扩展步骤、项目约定与架构边界；详细设计见 [文档导航](docs/README.md)。文档与代码的偏离本身是 bug，修改前先读对应设计文档。

---

## 项目速读

**定位**：Go 单二进制 HTTP 服务（YAML 配置 → 拉取订阅 → 生成 Clash Meta / Surge / Loon 配置），v2.0 含 Web 管理后台（React SPA via `go:embed`）、配置热重载与 Admin API。

**管道阶段**：

```text
启动期 / 热重载期:
读取并解析 YAML -> Prepare（产出 RuntimeConfig）  ↑ POST /api/reload 可重新触发

请求期:
RLock → 取得当前 RuntimeConfig 指针 → RUnlock
  -> Build(Source -> Filter -> Group -> Route -> ValidateGraph)
  -> Target
  -> Render
```

- 本地主配置缺失时先启动受保护的管理后台，首次保存并热重载后才获得运行快照；契约见[首次配置 API](docs/design/api.md#首次配置)。
- GET 预览只运行所需阶段；POST 草稿预览先准备临时配置；详见[管道设计](docs/design/pipeline.md)。

**包边界**（依赖单向，完整图与职责见 [项目结构](docs/implementation/project-structure.md#依赖方向)）：

```text
cmd/subconverter → internal/{admin,app,auth,fetch,generate,server,webui}
internal/server → internal/{generate,errtype}
internal/admin → internal/{app,auth,errtype}
internal/app → internal/{config,fetch,generate,pipeline,model,errtype}
internal/generate → internal/{config,fetch,model,pipeline,target,render}
internal/pipeline → internal/{config,fetch,model,errtype,proxyparse,ssparse}
internal/config → internal/{errtype,fetch,proxyparse}
internal/fetch → internal/{errtype}
internal/proxyparse → internal/{ssparse}
internal/render → internal/{model,errtype}
internal/target → internal/{model,errtype}
```

`model`、`errtype`、`auth`、`ssparse`、`webui` 为叶子包（不依赖其他业务包）。`render` 不反向依赖 `config`。`admin` 通过 `app.Service` 间接访问 `RuntimeConfig`（`RWMutex` 保护），不直接编排管道或渲染。

**核心术语**：

- **拉取类节点** = `KindSubscription` / `KindSnell` / `KindVLess`（Source 阶段从外部拉取或解析得到）
- **原始节点** = 拉取类节点 + 不带 `relay_through` 的 `custom_proxy`
- **链式节点** = `KindChained`（由带 `relay_through` 的 `custom_proxy` 作为模板派生）

**关键不变量**（修改时必须维持）：

- 每次 Build 内代理 ID 唯一；节点名只是显示名候选，不作为图引用或身份
- Clash / Surge / Loon 在各自 Target 投影内、完成格式过滤后独立分配唯一输出名
- 节点组名和服务组名共享命名空间，互斥
- 链式节点上游只能是拉取类节点
- `@all` 仅含原始节点，不含链式节点；带 `relay_through` 的 `custom_proxy` 仅派生 `KindChained` + 同名链式组，不产出 `KindCustom`，不进入 `@all`
- `groups` / `routing` / `rulesets` 三段**保序**
- **Loon 输入与输出**：Loon 订阅仍是 `KindSubscription`，仅提取 `[Proxy]`，不导入供应商组、规则、Host 或通用设置；`.lcf` 默认按 Loon 识别，`.conf` 默认仍按 Surge 识别，亦可显式指定 `sources.subscriptions[].format: loon`（优先级见 `docs/design/config-schema.md`）。`templates.loon` 提供底版，分组按现有 `groups.match` 筛选，链路与出口辅助节点名由 Target 分配、渲染器只序列化，规则优先级见 `docs/design/rendering.md#loon`。

---

## 扩展操作模板

### 新增 `ProxyKind` 枚举值

`ProxyKind` 是全系统的分派键，新增值必须同步审视所有 Kind 敏感点：

1. `internal/model/model.go` 加常量
2. `rg -n 'p\.Kind|model\.Kind' internal -g '*.go'` 列出全部分派点
3. **判定原则**：判断新 Kind 在每个分派点的行为是否与既有 Kind 相同，不同则必须处理。已知关键分派点（非完备清单）：

   | 分派点 | 关注问题 |
   | --- | --- |
   | `internal/pipeline/filter.go` 的 `isFetchedKind` | 是否参与 `filters.exclude` 过滤？ |
   | `internal/pipeline/group.go` 的 `fetchedProxies` | 是否参与区域组 regex 匹配？ |
   | `internal/pipeline/group.go` 的 `computeAllProxies` | 是否进入 `@all`？ |
   | `internal/pipeline/group.go` 的 `buildChainedNodesAndGroups` | 是否可作为链式上游？ |
   | `internal/pipeline/source.go` 的 ID 分配 | 是否获得非零、Build 内唯一的 `ProxyID`？ |
   | `internal/pipeline/validate.go` 的 `buildChainedProxyIDSet` + `buildNamespaceIndex` | 是否纳入 ID 引用检查？链式集合判定？ |
   | `internal/model/proxy_invariant.go` 的 `allowedProxyTypesByKind` | 允许哪些 Type？需新增映射条目 |
   | `internal/model/proxy_invariant.go` 的 `ValidateProxyInvariant` KindChained 分支 | 是否有类似 DialerID 的结构约束？ |
   | `internal/target/name_projection.go` 的 `allocateTargetProxyNames` | 是否按非链式/链式分轮分配输出名？ |

   > **注意**：`filterUnsupportedProxies` 同时按 `px.Type`（协议）和 `px.Native.Format`（结构化订阅原生格式）判断，`buildClashProxy` / `renderSurgeProxy` / `renderLoonProxy` switch 分派的是 `px.Type`。新增 Kind 不需因 Kind 本身修改协议分支，但须按行为语义评估目标格式投影。

4. 更新相关函数/变量的 doc comment（勿残留 `"只含 KindSubscription"` 等过时注释）
5. 写 group-level 回归测试（如 `TestGroup_<NewKind>ParticipatesInRegionMatch`），防止 helper 逻辑退化

### 新增 `Proxy.Type`（代理协议）

1. 在 `internal/render/clash.go`（`buildClashProxy`）、`surge.go`（`renderSurgeProxy`）、`loon.go`（`renderLoonProxy`）的 switch 中分别加 case
2. 若协议**只支持某一输出格式**，二选一：
   - **A 报错**：不支持方返回 `RenderError`（参考 SS v2ray-plugin 在 Surge / Loon 的处理）
   - **B 过滤**：不支持方做视图过滤（参考 `internal/target/filter_cascade.go` 的级联剔除）；级联清空 fallback 或投影不变量异常抛 `TargetError`（非 `RenderError`），边界见 §新增错误码
3. Surge 字段顺序用 `xxxKeyOrder` slice 固定，保证 golden 确定性输出
4. Params 的 key 名称**保持目标格式原样**（如 Snell 的 `shadow-tls-password`），避免在解析/渲染两处做命名映射
5. 更新 `docs/design/rendering.md` 映射表

### 新增来源类型（类比 `sources.snell`）

**最小测试集**（缺一不可）：

- [ ] 解析器单测：valid / invalid / skip（注释/空行）/ 重复键 / 边界（端口越界）
- [ ] Source 阶段：与其他来源共享 ID 序列、同名候选保留独立 ID；单行失败与整源为空路径（URL 必须脱敏）
- [ ] Filter / Group 阶段：若参与 `filters.exclude` 补过滤用例；补混合 Kind 的 region regex 匹配用例
- [ ] 渲染与端到端：Surge 含 base template + managed header 共存；Clash / Loon 若做过滤测级联清空 + 规则/ruleset 清理；Loon 覆盖五段底版合并；补 `TestExecute_<Source>EndToEnd` 与含新 Kind 的 `ValidateGraph` happy path
- [ ] 配置校验：`sources.<new>[].url` 空或非 HTTP(S) 报错

**文档与示例同步**（与测试同等重要）：

- [ ] `configs/base_config.yaml`：补充新 `sources.<kind>` 注释示例块，并核对既有注释中 `"订阅节点"` 等窄化措辞（`filters`、`groups.match`、`custom_proxies.name` 唯一性、`relay_through` 的描述均应涵盖所有拉取类节点）
- [ ] 设计文档同步：`docs/design/config-schema.md`（顶层段与校验）、`pipeline.md`（Source 子步骤/解析/错误/`FetchOrder`）、`domain-model.md`（`Proxy.Kind`/冲突策略）、`validation.md`（校验条目）、`rendering.md`（格式过滤小节）、`architecture.md`（若影响 `@all`/`@auto` 或原始节点定义，同步关键决策表）
- [ ] **依赖图同步**：若新增或变更包间 import，同步更新 `docs/implementation/project-structure.md` §依赖方向 与本文“包边界”图（若影响核心模块职责，同步 `docs/architecture.md` §模块边界）

### 新增错误码

1. 定义在 `internal/errtype/errors.go`，命名为 `Code<Domain><Scene>`，Domain ∈ `{Config, Fetch, Resource, Build, Target, Render}`，使用对应 `*Error` 结构体
2. **Target vs Render 边界判准**：渲染之前的格式投影失败（fallback 级联清空、协议过滤内部不变量异常）抛 `TargetError`；输出序列化、底版模板合并或 managed header 注入失败抛 `RenderError`。HTTP 层中用户可修复的 fallback 清空类 `TargetError`（`CodeTargetClashFallbackEmpty` / `CodeTargetSurgeFallbackEmpty` / `CodeTargetLoonFallbackEmpty`）映射为 `400`，投影不变量类 `TargetError` 与 `RenderError` 映射为 `500`
3. 在 `errors_test.go` 为每个新 Code 增加 `Error()` 输出格式断言

---

## 代码约定

### 错误消息

- 用户可见文案用**中文**；代码注释、Error Code 常量名用英文
- **URL 必须脱敏**：写入 `FetchError.URL` 前调用 `fetch.SanitizeURL(rawURL)`
- 错误消息需含**定位信息**（field path、upstream id、proxy name）；**级联失败**需附带**根因路径**（如 `"FINAL 为空 ← SVC_X 为空 ← GRP_SG 被过滤"`），而非仅给终态

### 测试

- 命名：`Test<Package>_<Scenario>`，顶部注释加 `T-<DOMAIN>-<NN>` id（规则见 `docs/implementation/testing-strategy.md`）；新特性测试文件命名为 `<feature>_test.go`
- **表驱动优先**（`cases` slice + `t.Run`），失败断言带 got/want 与相关 state
- **YAML 断言避坑**：astral-plane 字符（如国旗 🇭🇰 U+1F1ED+U+1F1F0、🔗 U+1F517）会被 go-yaml 转成 `\UXXXX`，YAML 路径测试中组名用 ASCII 前缀（`GRP_`/`SVC_`）更稳定；纯 Go fixture 可安全使用任意 emoji
- **测试不跨架构边界导入**：测试文件只导入被测包的直接依赖（如 `model`、`errtype`），不导入同层或上层业务包（如 `render` 测试不导入 `pipeline`）；复杂输入直接用 `model.Proxy` 字面量构造

### 配置可写性契约

触碰配置保存、状态接口或只读部署文档时，必须同步保持 `/api/status`、`PUT /api/config` 与前端只读模式一致：

- HTTP(S) 远程配置源：`config_source.type="remote"`，`config_source.writable=false`，`capabilities.config_write=false`；直接保存返回 `409 config_source_readonly`
- 本地主配置缺失：`config_source.exists=false`，接口返回空草稿与 `missing` revision；父目录存在且可写时允许导入、编辑和首次保存（未保存前禁用热重载，首次创建不覆盖并发创建的文件，见[首次配置 API](docs/design/api.md#首次配置)）
- 本地可写配置：`config_source.type="local"`，`config_source.writable=true`，`capabilities.config_write=true`；允许 `PUT /api/config`
- 本地文件或所在目录不可写：status 仍返回配置与 revision，但 `writable=false` 且 `capabilities.config_write=false`；直接保存返回 `409 config_file_not_writable`（文档中勿与远程 `config_source_readonly` 混淆）
- 前端只读模式同时依赖 `capabilities.config_write` 与 `config_source.writable`

### CI 流水线检查

Release workflow（`.github/workflows/release.yml`）按 `test → binaries → docker` 执行，具体边界见 [构建与部署](docs/deployment.md#release)。关键注意：

- `golangci-lint`（默认含 `unused`，加上 `.golangci.yml` 的 `errcheck` / `govet` / `staticcheck` / `gosec`）会拒绝 **`_test.go` 中的未使用函数/变量**；`gofmt -l .` 严格检查末尾多余空行
- Release workflow **不运行前端测试、类型检查或重新构建**，仅检查 `internal/webui/dist/index.html` 非空；修改 `web/` 源码后必须在本地完成检查并提交 `pnpm web:embed` 产物

**本地预检命令**：

- 后端/发布基线：`go mod verify && test -z "$(gofmt -l .)" && go test ./... && go test -tags webui ./... && go vet ./... && test -s internal/webui/dist/index.html`
- 涉及 Web 源码：额外跑 `pnpm web:test && pnpm web:typecheck && pnpm web:build`；需更新嵌入产物时跑 `pnpm web:embed`
- 发布镜像改动：Docker daemon 可用时跑 `docker build -f Dockerfile .`

### 文档术语与命名锚点

- **术语全文同步**：修改核心术语（如 `"原始节点"`、`"拉取类节点"`、`"@all"`）定义时，用 `rg -n "原始节点|拉取类节点|@all" docs AGENTS.md configs/base_config.yaml` 检查所有引用点；特别检查概要层文档（`docs/architecture.md`、决策表），避免只改细节层导致漂移
- **语义变更优先 rename**：函数/变量的行为语义扩展或变化时（如 `isFetchedKind` 从“仅订阅”扩展为“订阅 / Snell / VLESS”），优先改名或补充贴合新语义的 helper，避免调用者凭旧名字产生误判

---

## 已知架构局限（触及边界索引）

详细原因、影响与缓解权衡以 [架构局限](docs/architecture.md#已知架构局限) 为唯一权威来源。修改代码触及以下边界时保持其契约不变；若需调整边界，先与用户确认并同步更新该文档：

| 局限 | 关键路径与边界 | 触及时行动 |
| --- | --- | --- |
| 1. `ValidateGraph` 不感知输出格式 | Build 合法的配置可能在 `target.ForClash`/`ForSurge`/`ForLoon` 阶段抛 `CodeTarget*FallbackEmpty`（`TargetError`） | 新增 format-specific 过滤时 → 评估引入带 `formatHint` 的 per-format validation hook |
| 2. 中间表示宽松包容格式限定字段 | `Proxy.Params`（`map[string]string`）由 `xxxKeyOrder` 定序输出、未知键静默丢弃；`NativeProxy.Payload` 原样复制 | 考虑严格模式时 → 优先扩充 `xxxKeyOrder` 而非引入白名单拒绝（需同步改解析/渲染/错误码） |
| 3. 格式专属协议/来源引发 Target 级联过滤 | Snell/Surge CONF 仅 Surge，Clash YAML 仅 Clash，Loon 原生仅 Loon，规范化 VLESS 仅 Clash/Loon；走 `internal/target/filter_cascade.go` `filterUnsupportedProxies` | 新增格式专属协议或来源时 → 复用 `filterUnsupportedProxies` 并补跨格式级联过滤测试 |
| 4. 热重载后在途请求继续使用旧快照 | `app.Service` 用 `RWMutex` 保护 `*RuntimeConfig` 指针；锁外 `Prepare`，写锁仅保护指针替换（见 [并发安全](docs/design/app-service.md#并发安全)） | 若需求改为重载后在途请求立即切新配置 → 先确认是否修改快照一致性边界 |
| 5. YAML 写回丢失注释与格式 | `PUT /api/config` 通过 `yaml.Marshal` 写回；Web UI 首次覆写前弹窗确认，后端不生成 `.bak` | 若需保留注释与格式 → 评估 `yaml.Node` 级 patch-merge 方案 |
| 6. 用户正则表达式无执行复杂度限制 | `Prepare` 仅校验 `regexp`（RE2）可编译，不限节点规模 | 若反馈匹配慢请求 → 先 profiling 定位，再考虑规模限制或请求超时/取消 |

---

## 文档导航

全仓文档状态矩阵与完整清单以 [docs/README.md](docs/README.md) 为入口，前端契约以 [web/docs/README.md](web/docs/README.md) 为入口；高频设计参考：`docs/architecture.md`、`docs/design/{config-schema,domain-model,pipeline,rendering,validation,api,app-service,caching,web-ui}.md`、`docs/deployment.md`、`docs/implementation/{project-structure,testing-strategy,implementation-plan,progress}.md`。

---

## 提交前自检（本项目最易漏的检查）

- [ ] **ProxyKind 枚举变更** → 按 §新增 ProxyKind 评估所有 Kind 敏感分派点（pipeline 层 6 处 + model 层 2 处 + target 层 1 处）
- [ ] **用户可见错误消息含 URL**：已用 `fetch.SanitizeURL` 脱敏
- [ ] **新增来源类型** → `configs/base_config.yaml` 已加示例且窄化措辞已更新，docs 已同步
- [ ] **函数行为语义变化** → 已优先 rename 函数/变量名而非仅改注释
- [ ] **格式专属协议/来源改动** → 已测跨格式级联效应（fallback / 规则 / ruleset 清理）
- [ ] **包间依赖变更** → `docs/implementation/project-structure.md` §依赖方向 与本文“包边界”图已同步
- [ ] **测试跨边界导入** → 测试未导入同层或上层业务包
- [ ] **删除代码后的清理** → 无未使用的 test helper、无多余 import、无尾部空行（`gofmt -l .` 通过）
- [ ] **核心术语定义变更** → 全文 `rg` 搜索确认所有引用点（含 `docs/architecture.md` 概要层）均已同步
- [ ] **新增 API 端点或响应字段变更**：已同步 `docs/design/api.md` 与 `docs/design/app-service.md`（Method/DTO/JSON 示例）
- [ ] **管道阶段或渲染变更**：已同步 `docs/design/pipeline.md`、`domain-model.md`、`rendering.md` 对应表
- [ ] **Web 源码改动**：已完成 `pnpm web:test`、`pnpm web:typecheck` 与 `pnpm web:embed`，嵌入产物同步至 `internal/webui/dist`
