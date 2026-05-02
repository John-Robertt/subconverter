# 校验设计

> 状态提示：本文描述 v2.0 校验与错误映射目标契约；当前可用能力与规划能力的边界见 docs/README.md 状态矩阵。

## 目标

本文件定义配置校验与引用校验的边界。校验分为三类：字段级静态校验、构建期校验和图级语义校验。

---

## 静态校验（Prepare 阶段）

静态校验在 `config.Prepare()` 中完成，发生在配置加载后、请求期管道执行前。Prepare 同时执行正则编译、URL 解析和 `@auto` 展开，产出启动期准备好的 `RuntimeConfig`；请求期阶段按只读契约消费它，并派生新的动态结果。

检查范围：

- 必填字段是否存在
- 枚举值是否合法
- 条件字段是否成对出现
- 正则表达式是否可编译
- URL 是否满足基本格式要求

说明：

- 访问 token 不属于用户 YAML 配置，因此不在本层做字段校验；其存在性和匹配性由 HTTP 层在请求进入时校验

重点规则：

- `subscriptions[].url` 必填
- `snell[].url` 必填，必须为 HTTP(S) URL
- `vless[].url` 必填，必须为 HTTP(S) URL
- `custom_proxies[].name` 必填，且互不重复
- `custom_proxies[].url` 必填；scheme 必须是 `ss://`、`socks5://` 或 `http://`；URL 解析失败（host/port 缺失、port 越界、SS userinfo 解码失败、SS plugin 选项格式错误等）记为字段错误
- `custom_proxies[].url` 为 SS 时还需含 `cipher` + `password`（base64/明文 userinfo 解码后必须形如 `method:password`）
- `groups[*].match` 必填
- `groups[*].strategy` 必填且只能是 `select` 或 `url-test`
- `relay_through.type` 必填
- `relay_through.strategy` 必填且只能是 `select` 或 `url-test`
- `relay_through.type=group` 时必须提供 `name`
- `relay_through.type=select` 时必须提供 `match`
- `fallback` 必填
- `base_url` 可选；若非空，必须以 `http://` 或 `https://` 开头
- `routing` 中同一 entry 内 `@auto` 最多出现一次
- `routing` 中同一 entry 内 `@all` 与 `@auto` 不能同时出现

---

## 构建期校验

构建期校验发生在 `Source`、`Filter`、`Group`、`Route` 等管道阶段内部，用于尽早拦截依赖运行期输入的数据问题。

检查范围：

- 订阅拉取后是否得到至少一个有效节点
- 运行期名称冲突是否出现
- `relay_through.type=group` 的局部引用是否存在
- SS URI 与 plugin query 是否能按 SIP002 基本结构解析

重点规则：

- 订阅拉取结果不得为空（0 个有效节点视为上游订阅内容错误）
- Snell 来源拉取结果不得为空（0 个有效节点报同类错误）
- VLESS 来源拉取结果不得为空（0 个有效节点报同类错误）
- Snell 来源中单行解析失败整源报错（与 SS 订阅的静默跳过不同，详见 `pipeline.md`）；错误消息附带脱敏后的来源 URL 和 1-based 物理行号
- VLESS 来源中单行解析失败整源报错（与 Snell 一致，详见 `pipeline.md`）；错误消息附带脱敏后的来源 URL 和 1-based 物理行号
- 自定义代理名称不得与订阅、Snell 或 VLESS 节点名称冲突（错误消息会指明冲突源的 kind）
- `relay_through.type=group` 引用的节点组必须存在
- SS URI 中端口值必须在 1-65535 范围内；Snell / VLESS 节点的 port 字段同理
- 非法 SS URI、非法 fragment 编码、非法 query 编码、损坏的 plugin 参数都属于 Source 阶段构建期错误
- Snell 行格式错误（缺 `=`、type 非 snell、缺必填 psk 等）属于 Source 阶段构建期错误
- VLESS URI 中非法 UUID、非法 `security`、非法端口等属于 Source 阶段构建期错误；未知 `type` 不报错，而是按 Mihomo 兼容语义回落到 `tcp`

---

## 图级校验

图级校验发生在 Group 和 Route 阶段之后。

检查范围：

- 共享命名空间是否无冲突、重复声明
- `@all` 展开是否正确排除了链式节点
- 空节点组（地区组和链式组）
- 路由成员引用合法性（区分原始声明 vs 展开后的成员溯源）
- 服务组之间是否存在循环引用

说明：ruleset/rule 策略存在性和 fallback 存在性由启动期 Prepare 保证，ValidateGraph 不再重复检查。

重点规则：

- 跨订阅重名节点自动追加递增后缀
- 代理名、节点组名、服务组名在引用体系中共享命名空间，不允许重名或重复声明
- 节点组名与服务组名不得重复（共享同一命名空间）
- `routing` 中原始声明的成员只允许引用节点组、服务组、`DIRECT`、`REJECT`、`@all`、`@auto`
- `routing` 中 `@all` 展开后的具体代理名可出现在中间表示里，但用户配置不能直接声明这些代理名
- `routing` 中引用的节点组、服务组、保留字都必须可解析
- 自动生成的链式组必须至少包含一个成员
- 地区节点组匹配结果不得为空

---

## 错误分层

按来源区分错误：

- 配置错误（`ConfigError`）
- 远程拉取错误（`FetchError`）
- 本地资源读取错误（`ResourceError`）
- 构建错误（`BuildError`）
- 目标投影错误（`TargetError`）
- 渲染错误（`RenderError`）

补充说明：

- `BuildError` 的 `Error()` 文本格式保持稳定；需要补充来源上下文时，可通过 `Cause` / `Unwrap()` 保留内层根因链
- 当前 Snell 单行失败路径采用“外层 `BuildError` 提供来源 URL + 物理行号，内层解析错误保留具体语法原因”的模式
- 当前 VLESS 单行失败路径采用与 Snell 相同的“外层 `BuildError` 提供来源 URL + 物理行号，内层解析错误保留具体语法原因”的模式

HTTP 层映射：

- `400`：请求参数错误、静态配置错误、图级语义错误、可归因于用户配置的构建错误
- `400`：`TargetError` 中的 `CodeTargetClashFallbackEmpty` / `CodeTargetSurgeFallbackEmpty`。这类错误表示用户配置在目标格式级联过滤后不可生成，用户可通过调整 fallback、分组或格式专属节点配置修复
- `401`：服务端启用访问 token 时，请求缺少 token 或 token 不匹配
- `502`：远程资源拉取失败，或远程订阅内容不可用（如 0 个有效节点）
- `500`：本地资源读取失败、目标投影内部不变量错误、渲染错误或未分类内部错误

目标：

- 让错误能明确定位到配置、网络还是内部逻辑
- 保持 HTTP 层错误码映射简单稳定

---

## 目标格式投影校验

`ValidateGraph` 仍然保持格式无关，只负责统一 IR 上的引用与图结构正确性。目标格式差异则前移到 `Target` 阶段：

- `target.ForClash`：剔除 Snell 节点及其级联影响
- `target.ForSurge`：剔除 VLESS 节点及其级联影响
- 若 `fallback` 在目标格式视图中被清空，立即返回带清空路径的错误
- 级联过滤的诊断路径格式（`(snell)`、`(chained)`、`(cycle)` 等标记）详见 `rendering.md` §级联过滤

这样分层后：

- `Build` 只产出格式无关 IR
- `Target` 承接 format-specific 过滤与校验
- `Render` 只负责文本序列化

当前 Target 阶段统一使用 `TargetError`，以保持阶段语义清晰；HTTP 层按错误码分流：fallback 被清空使用 `CodeTargetClashFallbackEmpty` / `CodeTargetSurgeFallbackEmpty`，映射为 400；内部不变量异常使用独立 projection 错误码，映射为 500。

---

## Web 管理后台校验集成（v2.0）

`POST /api/config/validate` 触发 `Prepare` 阶段静态校验流程，返回结构化 JSON。它不执行订阅拉取、分组构建、目标格式投影或渲染。

```json
{
  "valid": false,
  "errors": [...],
  "warnings": [...],
  "infos": [...]
}
```

每条诊断项包含：

| 字段 | 说明 |
|------|------|
| `severity` | `error`（阻塞保存/重载）、`warning`（建议修改）、`info`（提示） |
| `code` | 稳定错误码，用于前端分类展示和测试断言 |
| `message` | 中文错误描述 |
| `display_path` | 面向用户展示的 YAML 风格路径，不作为程序定位依据 |
| `locator` | 结构化定位信息，包含 `section` / `key` / `index` / `value_path` / `json_pointer`；前端根据 `section` 自行映射到对应页面 |

示例：

```json
{
  "severity": "error",
  "code": "invalid_regex",
  "message": "正则表达式无效：...",
  "display_path": "groups.🇭🇰 Hong Kong.match",
  "locator": {
    "section": "groups",
    "key": "🇭🇰 Hong Kong",
    "index": 0,
    "value_path": "match",
    "json_pointer": "/config/groups/0/value/match"    ← 指向请求体 `config` 键下的路径
  }
}
```

定位规则：

- 前端跳转和字段高亮必须优先使用 `locator.json_pointer`
- `display_path` 只用于展示，不能用于解析定位
- `groups` / `routing` / `rulesets` 在 API JSON 中是 `[{key,value}]` 数组；即使 key 含空格、点号或 emoji，也通过 `index` 和 `json_pointer` 定位

校验与热重载的关系：

- `POST /api/reload` 内部先执行与 `validate` 相同的校验流程
- 校验失败则拒绝重载，返回与 `validate` 相同格式的错误
- 前端可在保存前调用 `validate` 预检，避免保存后因静态配置错误导致重载失败

静态校验与生成可用性的关系：

- `validate` 不拉取订阅、不执行 Source / Filter / Group / Route / Target / Render，因此不能证明生成一定成功
- 远程源不可用、远程源为空、过滤后节点组为空、目标格式级联过滤后 fallback 清空等问题，只能由预览或生成路径发现；其中 `GET/POST /api/preview/groups` 执行到 ValidateGraph，能提前发现格式无关图级错误
- `POST /api/config/validate` 的 API 语义是”静态配置校验”——仅覆盖 Prepare 阶段，不涉及运行时数据
- `GET/POST /api/preview/*` 和 `GET/POST /api/generate/preview?format=...` 才是生成可用性检查入口

前端实时校验：

- 前端在用户编辑时做格式级校验（正则语法、URL 格式、必填检查）
- 深层校验（跨段引用、命名冲突、环路检测）始终走后端 `Prepare`
