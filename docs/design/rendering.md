# 渲染设计

## 目标

本文件定义统一中间表示如何映射到 Clash Meta 与 Surge。它关注语义对齐，不描述具体代码组织。

---

## 渲染原则

- 两种输出共享同一套中间表示
- 面板结构保持一致
- 路由行为保持一致
- 仅在目标语法上分叉

---

## Clash Meta

映射要求：

- 节点输出到 `proxies`
- 节点组和服务组输出到 `proxy-groups`
- 远程规则集输出到 `rule-providers`
- 内联规则和 ruleset 引用输出到 `rules`
- fallback 输出为 `MATCH,<fallback>`

链式节点要求：

- 对链式节点输出 `dialer-proxy`

`url-test` 组默认参数：

- `url=http://www.gstatic.com/generate_204`
- `interval=300`
- `tolerance=100`

说明：

- Clash Meta 的规则集引用采用 provider 名称，而不是直接在规则中内联完整 URL
- 规则集固定生成 `behavior: classical` 且显式输出 `format: text`
- provider 缓存路径固定为 `.txt`

rule-provider 命名规则（两阶段分配）：

1. 从 URL 路径中提取文件名并去掉扩展名，如 `https://example.com/Clash/Netflix.list` → `Netflix`
2. 跨所有 rulesets 统一去重（provider 命名空间全局共享）：
   - 唯一名称直接使用
   - 重复名称追加递增后缀（`Netflix-2`、`Netflix-3`）
   - 生成的后缀名若与其他 URL 的自然名称碰撞，则继续递增直到不冲突
3. provider 名称仅用于 Clash Meta 输出的内部引用，不影响语义

### VLESS 渲染

Clash Meta 是 VLESS 的主要目标格式。字段按固定顺序输出：

```
name / type:vless / server / port / uuid / network / udp:true
  → flow（非空才输出）
  → alpn（逗号分隔串展开为 YAML 列表）
  → encryption（非空才输出）
  → 若 security ∈ {tls, reality}：tls:true / servername / client-fingerprint
  → 若 security == reality：reality-opts:{ public-key / short-id }
  → dialer-proxy（链式节点）
```

URI query 到 Params 的命名映射由解析器完成（见 `config-schema.md` 的 `sources.vless`），Clash 渲染器直接按目标键读取：

| URI query | Params key | Clash YAML key | 备注 |
|-----------|-----------|----------------|------|
| UUID (userinfo) | `uuid` | `uuid` | 必填，标准 UUID |
| `security` | `security` | —（分支判定）| none/tls/reality |
| `encryption` | `encryption` | `encryption` | 非空时透传并输出 |
| `flow` | `flow` | `flow` | 空则不输出 |
| `type` | `network` | `network` | 已知值 `tcp`/`ws`/`http`/`h2`/`grpc`/`xhttp` 原样保留；缺失或未知值回落到 `tcp` |
| `sni` | `servername` | `servername` | 仅在 tls/reality 时输出 |
| `fp` | `client-fingerprint` | `client-fingerprint` | 仅在 tls/reality 时输出 |
| `alpn` | `alpn` | `alpn` | 展开为 YAML 列表 |
| `pbk` | `reality-public-key` | `reality-opts.public-key` | 仅在 reality 时 |
| `sid` | `reality-short-id` | `reality-opts.short-id` | 仅在 reality 时（可空） |
| `spx` | `reality-spider-x` | —（不输出） | 宽松存入，forward compat |

每种非 tcp 的 `network` 值都会触发渲染器输出对应的 `*-opts` 块。URI 里的 `path` / `host` / `serviceName` / `mode` 由 parser **按 network 分发**到 transport-specific Params 键，渲染器按键直接输出，无运行时分派：

| network | 相关 URI query | 对应 Params key | 对应 Clash `*-opts` 位置 |
|---------|---------------|----------------|-----------------------|
| `tcp`   | — | — | 不输出 `*-opts` |
| `ws`    | `path` / `host` | `ws-path` / `ws-host` | `ws-opts.path` / `ws-opts.headers.Host` |
| `http`  | `path` / `host` | `http-path` / `http-host` | `http-opts.path[0]` / `http-opts.headers.Host[0]` |
| `h2`    | `path` / `host` | `h2-path` / `h2-host` | `h2-opts.path` / `h2-opts.host[0]` |
| `grpc`  | `serviceName` | `grpc-service-name` | `grpc-opts.grpc-service-name` |
| `xhttp` | `mode` / `path` / `host` | `xhttp-mode` / `xhttp-path` / `xhttp-host` | `xhttp-opts.{mode,path,host}` |

若 network 已指定但所有对应 Params 键为空（如 `type=ws` 但 URI 未带 `path`/`host`），渲染器**省略整个 `*-opts` 块**——Clash 客户端对该值视为默认空 opts，语义等价。

Parser 会校验 `security` 只接受 `none`/`tls`/`reality`；`encryption` 在非空时不做值校验、直接透传，以匹配 Mihomo 当前支持的扩展模式。传输层高级字段（如 `packet-encoding`、`ws-opts.max-early-data`、`grpc-opts.ping-interval`）以及 TLS 扩展字段（如 `support-x25519mlkem768`）当前 iteration 不消费——`sources.vless` 仍是“一行一个 URI”的输入模型，本次未为这些字段定义稳定 query 契约。

**关于 Params 键命名**：transport-specific 键（`ws-path`、`ws-host`、`h2-path`、`h2-host`、`http-path`、`http-host`、`grpc-service-name`、`xhttp-path`、`xhttp-host`、`xhttp-mode`）是 **network-qualified** 的前缀键，**不是 URI 查询的原始键名**。原因：VLESS URI 里的 `path` / `host` 在不同 `type` 下对应 Clash 中不同的 `*-opts` 嵌套位置（例如 `ws-opts.path` vs `h2-opts.path`）；parser 在 query 解析时按 network 分发到具体键，让渲染器按键直接读取，避免"同名不同位"的运行时分派。tcp 没有传输层 opts 块，不产生前缀键。该命名规则是 CLAUDE.md "Params 的 key 名称保持目标格式原样" 原则的一个**网络受限例外**——由 Clash 的配置结构决定，而非 parser/renderer 的风格选择。

### SS 渲染

SS（Shadowsocks）节点同时支持 Clash Meta 和 Surge 输出。字段按固定顺序输出：

**Clash Meta**：

```
name / type:ss / server / port / cipher / password
  → plugin（若有）
  → plugin-opts（若有）
  → dialer-proxy（链式节点）
```

Plugin 透传规则：
- plugin 名称经过归一化：`simple-obfs` / `obfs-local` / `obfs` 统一输出为 `obfs`；其他名称（如 `v2ray-plugin`）原样透传
- plugin-opts 按 key 排序输出，key 名称经过映射：obfs 类插件的 `obfs` → `mode`、`obfs-host` → `host`；其他插件的 key 原样透传
- 无 plugin 时不输出 `plugin` / `plugin-opts` 字段

**Surge**：

```
<name> = ss, <server>, <port>, encrypt-method=<cipher>, password=<password>[, obfs=..., obfs-host=..., obfs-uri=...][, underlying-proxy=...]
```

Plugin 限制：
- 仅支持 obfs 类插件（`simple-obfs` / `obfs-local` / `obfs`）
- 仅接受 `obfs`（必须为 `http` 或 `tls`）、`obfs-host`、`obfs-uri` 三个选项
- 不支持的插件名称或包含未知选项的 obfs 插件返回 `RenderError`（而非静默降级）
- 无 plugin 时不输出 obfs 相关字段

### Snell 过滤

Clash Meta 主线不支持 Snell v4/v5（jinqians/snell.sh 默认版本）。`Target` 阶段会先做级联过滤：

1. 剔除 `Type=="snell"` 的节点
2. 剔除链式节点中 `Dialer` 属于已剔除集合的节点（失效上游）；诊断路径中这类叶子会标记为 `<name>(chained) ← [<upstream>]`
3. 对每个节点组 / 服务组，剔除 Members 中属于已剔除集合的名字；若 Members 清空则该组自身被剔除。迭代到不动点
4. 规则集、内联规则中 Policy 属于已剔除组的条目被剔除
5. 若 `fallback` 所指服务组在级联中被清空，返回 `RenderError`（`CodeRenderClashFallbackEmpty`），错误消息附带清空路径（如 `FINAL ← [GRP_CHAIN ← [HK-Snell→MY-PROXY(chained) ← [HK-Snell(snell)]]]`），便于定位根因

清空路径使用显式原因图：

- 原始 Snell 根节点标记为 `NAME(snell)`
- 受已删除上游牵连的链式节点标记为 `NAME(chained) ← [<upstream>]`
- 共享掉落子图会正常展开；`(cycle)` 只表示真实递归保护命中，而不是普通共享引用

过滤算法作用在 `*model.Pipeline` 的**副本**上，原 Pipeline 不变；现在它位于 `internal/target/filter_cascade.go`，由 `target.ForClash` 包装调用。渲染器只接收已经完成 Clash 投影的视图。

---

## Surge

映射要求：

- 若配置了 `base_url`，输出首行为 `#!MANAGED-CONFIG <managed-url> interval=86400 strict=false`
- 节点输出到代理定义段
- 节点组和服务组输出到组定义段
- 远程规则集在规则中直接引用 URL
- fallback 输出为 `FINAL,<fallback>`

链式节点要求：

- 对链式节点输出 `underlying-proxy`

`url-test` 组默认参数：

- `url=http://www.gstatic.com/generate_204`
- `interval=300`
- `tolerance=100`

### Snell 渲染

Snell 节点专属 Surge 输出（Clash 视图过滤掉 Snell 节点，见上文）。字段按固定顺序输出，以保证确定性（golden 文件可比对）：

```
<name> = snell, <server>, <port>, psk=..., version=..., obfs=..., obfs-host=..., obfs-uri=..., reuse=..., tfo=..., udp-relay=..., udp-port=..., shadow-tls-password=..., shadow-tls-sni=..., shadow-tls-version=...
```

约束：

- `psk` 必填；其他字段可选，值为空时整条键值对不输出
- `Params` 中的未知键**不输出**（渲染器只遍历固定列表）；这使解析器可以保持宽松地接纳新 Surge 选项，但渲染层的扩充需要显式更新 `surgeSnellKeyOrder`
- 全字段支持 ShadowTLS（Surge 独有），Clash 不消费

### VLESS 过滤

Surge 不原生支持 VLESS。`Target` 阶段会先做级联过滤（对称于 Clash 侧过滤 Snell）：

1. 剔除 `Type=="vless"` 的节点
2. 剔除链式节点中 `Dialer` 属于已剔除集合的节点
3. 对每个节点组 / 服务组，剔除 Members 中属于已剔除集合的名字；若 Members 清空则该组自身被剔除。迭代到不动点
4. 规则集、内联规则中 Policy 属于已剔除组的条目被剔除
5. 若 `fallback` 所指服务组在级联中被清空，返回 `RenderError`（`CodeRenderSurgeFallbackEmpty`），错误消息附带清空路径（VLESS 根节点标记为 `NAME(vless)`，链式节点标记为 `NAME(chained) ← [<upstream>]`）

算法由共享 cascade 引擎 `filterByDroppedTypes` 提供，与 Clash 侧过滤 Snell 是同一份代码，参数化注入标签（`formatName`、`rootLabel`、`emptyCode`、`emptyReasonClause`）。过滤作用在 `*model.Pipeline` 的**副本**上，原 Pipeline 不变；Render 阶段只消费已投影后的 Pipeline。

---

## 规则排列顺序

最终输出的规则按以下顺序排列：

1. `rulesets`（按 `rulesets` 段书写顺序，同一服务组的多条 URL 按声明顺序排列）
2. `rules`（按 `rules` 段书写顺序）
3. `MATCH` / `FINAL` 兜底规则（引用 `fallback` 指定的服务组）

理由：

- 规则集通常包含精确域名/IP 匹配，应优先命中
- 用户内联规则通常是宽泛规则（如 `GEOIP`），放在规则集之后
- 兜底规则天然是最后一条

---

## Surge Managed Profile

当用户配置了 `base_url` 时，Surge 渲染器在输出首行写入：

```
#!MANAGED-CONFIG <managed-url> interval=86400 strict=false
```

此行告知 Surge 客户端配置的更新源地址和检查间隔。

参数说明：

- URL：由 `base_url` + `/generate` 拼接，并继承当前请求的 `format=surge`、访问 `token`（若启用）和最终 `filename`；`filename` 已由 HTTP 层收紧为安全 ASCII 文件名
- `interval`：更新检查最小间隔，默认 86400 秒（24 小时）
- `strict`：是否强制过期更新，默认 false

未显式传入 `filename` 时，最终文件名默认使用 `surge.conf`；若请求带了自定义 `filename`，managed URL 中也使用该值。当 `base_url` 为空时不输出此行。Clash Meta 输出不受影响。

---

## 一致性要求

无论输出 Clash Meta 还是 Surge，都必须保证：

- 同样的节点组顺序
- 同样的服务组顺序
- 同样的 ruleset 顺序
- 同样的规则排列顺序
- 同样的 fallback 语义
- 同样的 `@auto` 和 `@all` 展开结果
- 对目标格式不支持的协议，按格式能力做显式处理：Snell 为 Surge-only（Clash 目标投影侧级联过滤），VLESS 为 Clash-only（Surge 目标投影侧级联过滤）——两种方向由同一个共享 cascade 引擎（`internal/target/filter_cascade.go`）实现

---

## 底版模板合并

渲染器支持可选的底版模板（base template），用于保留用户自定义的通用设置。

模板来源：由用户在 `templates.clash` / `templates.surge` 中声明，值可为本地路径或 HTTP(S) URL。

### Clash Meta 合并策略

- 使用 yaml.v3 Node API 解析底版为 AST
- 在根 MappingNode 中定位并替换 `proxies`、`proxy-groups`、`rule-providers`、`rules` 四个 key
- 底版中的其他 key（如 `mixed-port`、`dns`、`tun`）原样保留
- 底版为空或非 YAML 映射文档时报错

### Surge 合并策略

- 按 `[Section]` header 正则切分底版为段落列表
- 替换 `[Proxy]`、`[Proxy Group]`、`[Rule]` 三个段落的内容
- 底版中的其他段落（如 `[General]`、`[Host]`）原样保留
- 未在底版中找到的生成段落追加到末尾
- 底版 preamble 中的 `#!MANAGED-CONFIG` 行会被剥离，避免与新生成的 header 重复

### 无底版时的行为

- Clash Meta：仅输出 `proxies` / `proxy-groups` / `rule-providers` / `rules` 四个段
- Surge：仅输出 `[Proxy]` / `[Proxy Group]` / `[Rule]` 三个段（加可选的 managed header）

---

## 非目标

本层不负责：

- 验证规则集 URL 是否在线
- 修正客户端特有的运行时问题
- 为不同客户端增加行为差异补丁
