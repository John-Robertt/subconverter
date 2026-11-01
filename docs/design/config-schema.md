# 配置结构设计

> 状态提示：结构化订阅、Snell / VLESS 来源段及 §JSON API 表示为 v2.0 新增契约。

## 目标

本文件定义用户 YAML 配置的正式结构、字段约束和配置语义。它只回答"用户如何声明系统行为"，不涉及 Go 代码实现。

---

## 顶层结构

```yaml
base_url:
sources:
filters:
groups:
routing:
rulesets:
rules:
fallback:
templates:
```

各段职责：

- `base_url`：声明服务的外部可访问地址（可选）
- `sources`：声明订阅源（TXT / Clash YAML / Surge CONF / Loon）、Snell 来源、VLESS 来源和自定义代理
- `filters`：定义拉取类节点过滤规则
- `groups`：声明地区节点组
- `routing`：声明服务组和出口优先级
- `rulesets`：将远程规则集 URL 绑定到服务组
- `rules`：声明内联规则
- `fallback`：定义最终兜底出口
- `templates`：声明输出格式的底版配置模板（可选）

---

## sources

```yaml
sources:
  subscriptions:
    - url: "https://sub.example.com/api/v1/client/subscribe?token=xxx"
  snell:
    - url: "https://my-server.com/snell-nodes.txt"
  vless:
    - url: "https://my-server.com/vless-nodes.txt"
  custom_proxies:
    - name: HK-ISP
      url: socks5://user:pass@154.197.1.1:45002
      relay_through:
        type: group
        name: 🇭🇰 Hong Kong
        strategy: select
```

注：`subscriptions` / `snell` / `vless` 三类拉取来源按 YAML 中的书写顺序遍历（`Sources.FetchOrder`）。例如上例顺序 `subscriptions → snell → vless` 即节点在后续管道中的相对顺序。`custom_proxies` 不属于拉取类，排在最后，与 YAML 位置无关。

### subscriptions

- `url`、`content`、`content_file` 三选一：`url` 为 HTTP(S) URL；`content` 为非空节点正文；`content_file` 为本地正文文件路径。Web 编辑请求使用 `content`，保存后主配置改为 `content_file` 引用
- `content` 使用下述同一套解析规则，不访问上游，也不自动更新。显式 `format: surge` / `loon` 时可只粘贴节点声明；含 section 的完整文档仍只提取 `[Proxy]`
- 支持多个订阅源
- `format` 可选，接受 `auto`、`text`、`clash`、`surge`、`loon`，省略等同 `auto`。显式格式优先于 URL 后缀和内容识别
- 自动识别时，`.yaml` / `.yml` 为 Clash，`.conf` 为 Surge，`.lcf` 为 Loon；忽略 query 和 fragment。显式 `format: loon` 不依赖 URL 后缀
- 无已知后缀时，依次检查 Clash 顶层 `proxies`、Loon 专属段（`[Remote Proxy]` / `[Remote Filter]` / `[Remote Rule]` / `[Proxy Chain]`）、Surge `[Proxy]`；均不匹配才进入 TXT/Base64
- `text` 显式按 SS / AnyTLS 明文或 Base64 列表解析，忽略 URL 后缀；Web 检查节点列表后保存此值，避免 `.conf` 等地址被旧版 `auto` 规则改判
- Web 检查与上述旧版 `auto` 相互独立：默认先检查内容，确认后把结果保存为明确的 `format`；已有 YAML 的 `auto` 优先级保持不变。复制已检查正文时保留确认的格式
- Clash YAML 复制顶层 `proxies` 序列中的每个节点映射：
  - 每项必须是映射，且 `name` / `type` 必须是非空标量
  - 节点的完整映射（包括未知协议和嵌套字段）保留到 Clash Render；Clash 目标内分配的最终名称会覆盖映射中的原 `name`
  - 节点只进入 Clash 输出；其他 Target 投影会连同依赖它的链式节点、空组和规则一起级联剔除
- Surge CONF 复制 `[Proxy]` 段中每条非注释 `NAME = VALUE` 声明：
  - `[Proxy]` 之后遇到下一个 section 即停止读取
  - 精确名称 `DIRECT` / `REJECT` 是本项目内置策略，不作为代理节点导入
  - 完整 `VALUE` 保留到 Surge Render；Surge 目标内分配的最终名称会覆盖原 `NAME`
  - 节点只进入 Surge 输出；Clash / Loon Target 投影会执行对称的级联剔除
- Loon 与 Surge 一样只复制 `[Proxy]` 中的 `NAME = VALUE`，记录为 Loon 原生节点，仅进入 Loon 输出。密码位置参数、`tls-name` 等原始字段保留；来源的策略组、规则、Host 和通用设置由本项目 YAML 与底版负责
- 结构化文档语法错误、缺少目标字段/section 或节点声明无效时整源报错；错误 URL 会移除 query 与 fragment
- 旧版 TXT 响应体可以是 Base64 编码后的 SS / AnyTLS URI 列表，也可以是明文列表
- SS 支持三种行格式：SIP002 `ss://...#节点名`、Quantumult X `shadowsocks = host:port, method=..., password=..., tag=...`、Surge `<节点名> = ss, host, port, encrypt-method=..., password=...`
- AnyTLS 支持三种行格式：

  ```
  anytls://PASSWORD@SERVER:PORT/?sni=...&insecure=1#NODE_NAME
  NAME = anytls, SERVER, PORT, password=..., tls=true, sni=..., skip-cert-verify=...
  anytls=SERVER:PORT, password=..., tls-host=..., tls-verification=false, tag=NODE_NAME
  ```

- AnyTLS URI 的 `PORT` 缺失时默认使用 `443`；`insecure=1` 映射为 `skip-cert-verify=true`；`group` 等元信息 query 会被忽略
- AnyTLS 的 Surge / Quantumult X 输入按兼容方言解析；`tls=true` / `over-tls=true` 只用于识别输入形态，不进入通用渲染契约
- TXT 中的空行和以 `#` / `//` 开头的注释行会被跳过；单行解析失败会静默跳过，最终 0 个有效节点时报错

### Web 订阅检查

订阅编辑先提供链接或节点正文，再检查解析结果；确认后才允许“保存来源”进入草稿。检查不保存配置、不改变运行快照，也不将链接自动转成副本。

自动检查按以下顺序判断：

1. Clash 顶层 `proxies` 使用 Clash 解析器；声明错误直接报错。
2. 无 INI 段落且能解析成 SS / AnyTLS 列表时，采用 `text`。
3. INI 订阅结合段落和节点声明判断：Loon 专属段、位置参数凭证、`Shadowsocks` 类型是 Loon 线索；`ss` / `snell` 类型和键值形式的凭证是 Surge 线索。
4. 有一致线索时使用对应解析器；没有线索时可使用 `.lcf` 提示。共享写法或线索冲突返回 Surge / Loon 候选，由用户选择原客户端后重新检查。`.conf` 本身不能在 Web 检查中消除歧义。
5. 用户更正格式后使用该解析器；成功检查必须提取至少一个节点。原生声明与 TXT 的校验、容错范围沿用既有 Source 规则。

检查结果包含节点数和能完整保留这些节点的输出格式。输出兼容性复用 Target 和 Render，不应用全局过滤、分组或模板，也不测试节点连通性。接口见 [订阅检查](api.md#post-apisubscriptionscheck)。

### snell

- `url` 必填，必须为 HTTP(S) URL
- 支持多个 Snell 来源
- URL 返回纯文本内容，每行一条 Surge 风格的 Snell 节点声明：

  ```
  NAME = snell, SERVER, PORT, psk=..., version=..., [其他可选键]
  ```

- 可选字段：`version`、`obfs`、`obfs-host`、`obfs-uri`、`reuse`、`tfo`、`udp-relay`、`udp-port`、`shadow-tls-password`、`shadow-tls-sni`、`shadow-tls-version`
- 空行和以 `#` / `//` 开头的注释行会被跳过；单行解析失败时整源报错（与 `subscriptions` 的静默跳过不同——Snell 来源通常是小规模手工清单，严格报错更有利于发现拼写问题）
- 单行解析失败的错误消息会附带脱敏后的来源 URL 与 1-based 物理行号；原始解析根因保留在 `BuildError.Cause`
- 节点名是显示名候选；同名节点保留独立 ID，最终输出名在各目标格式投影内独立消歧
- Snell 节点**只进入 Surge 输出**；Clash 输出会过滤掉这些节点及级联清理的空组、失效链式节点、空规则。详见 `rendering.md`

### vless

- `url` 必填，必须为 HTTP(S) URL
- 支持多个 VLESS 来源
- URL 返回纯文本内容，每行一条标准 VLESS URI：

  ```
  vless://UUID@SERVER:PORT?security=...&sni=...&type=...#NODE_NAME
  ```

- 支持的 query 参数：`security`（`none`/`tls`/`reality`）、`encryption`（非空透传）、`flow`、`type`（已知值保留，缺失或未知值回落到 `tcp`）、`sni`、`fp`、`alpn`、`pbk`、`sid`、`spx`
- 空行和以 `#` / `//` 开头的注释行会被跳过；单行解析失败时整源报错（与 Snell 一致——VLESS 来源通常也是小规模手工清单，严格报错更利于发现拼写问题）
- 单行解析失败的错误消息会附带脱敏后的来源 URL 与 1-based 物理行号；原始解析根因保留在 `BuildError.Cause`
- 节点名是显示名候选；同名节点保留独立 ID，最终输出名在各目标格式投影内独立消歧
- 当前 URI 模型仅承接本节列出的 query；像 `packet-encoding`、`support-x25519mlkem768` 等 Mihomo 代理字段本次不接入，避免在未定义稳定 query 契约前产生隐式支持承诺
- VLESS 节点进入 Clash；Loon 支持 `tcp` / `ws` / `http` 传输，其余传输在 Loon Target 级联过滤；Surge 过滤全部 VLESS。详见 `rendering.md`

### custom_proxies

每条 `custom_proxy` 由 `name` + `url` 两个必填字段定义，可选 `relay_through`。

```yaml
custom_proxies:
  - name: 🔗 HK-ISP
    url: socks5://user:pass@154.197.1.1:45002
  - name: MY-SS
    url: "ss://YWVzLTI1Ni1nY206bXlwYXNz@1.2.3.4:8388?plugin=obfs-local%3Bobfs%3Dhttp"
  - name: US-HTTP
    url: http://user:pass@10.0.0.1:8080
```

字段约束：

- `name` 必填。无 `relay_through` 时是代理显示名候选，允许重名；带 `relay_through` 时是链式组名（见下），必须满足组命名空间唯一性
- `url` 必填。支持三种 scheme：
  - `ss://userinfo@server:port[?plugin=...]`：SIP002 Shadowsocks。`userinfo` 可以是 base64 编码的 `method:password`，也可以是 percent-encoded 明文。`?plugin=` query 由"plugin 名称 + 分号分隔的 KEY=VALUE 选项"组成（参考 SS 客户端通用约定）。URI 末尾的 `#fragment` 节点名**会被静默忽略**，统一以外层 `name` 字段为准
  - `socks5://[user:pass@]server:port`：SOCKS5
  - `http://[user:pass@]server:port`：HTTP
- `relay_through` 可选

校验规则：

- 带 `relay_through` 的 `name` 与其他组或保留策略重名时报错；无 `relay_through` 的重复节点名由 Target 在各输出格式内独立消歧
- `url` 缺失或不能解析（含 scheme 不识别、host/port 缺失、SS URI 缺少 cipher/password 等）报错
- `url` 解析在 `Prepare` 阶段完成，错误聚合输出，便于一次性发现多条配置问题

### relay_through

`relay_through` 用于生成链式节点和链式组。

字段约束：

- `type` 必填，可选值：`group`、`select`、`all`
- `strategy` 必填，可选值：`select`、`url-test`
- `name` 仅在 `type=group` 时必填
- `match` 仅在 `type=select` 时必填

语义：

- `group`：使用某个已定义节点组中的全部成员作为上游
- `select`：用正则从拉取类节点（订阅 + Snell + VLESS）中选择上游
- `all`：使用全部拉取类节点作为上游

结果：

- 系统为每个上游节点生成一个链式节点
- 系统自动生成一个链式节点组，组名 = `custom_proxy.name` 原样值（无任何前缀，若希望组名带视觉标识如 `🔗 `，由用户自行写入 `name`）
- 链式组属于节点组，策略取自 `relay_through.strategy`
- 带 `relay_through` 的 `custom_proxy` 仅作为链式模板，不再作为独立 `KindCustom` 代理出现在 `proxies` 列表和 `@all` 中；如需同时保留"直连 + 链式"两种入口，应声明两条 `custom_proxies` 条目（名字必须不同）

---

## filters

```yaml
filters:
  exclude: "(过期|剩余流量|到期|Traffic|Expire|Reset|Date|\\|.*G)"
```

约束：

- `exclude` 可选，值为正则表达式
- 作用于拉取类节点（订阅 + Snell + VLESS）
- 不作用于自定义代理和链式节点

---

## groups

```yaml
groups:
  🇭🇰 Hong Kong: { match: "(港|HK|Hong Kong)", strategy: select }
  🇯🇵 Japan: { match: "(日本|JP|Japan)", strategy: url-test }
```

用途：

- 定义地区节点组
- 决定节点组层的面板内容和顺序

字段约束：

- `groups` 不得为空（至少声明一个地区节点组）
- key 为节点组名
- `match` 必填，值为正则表达式
- `strategy` 必填，可选值：`select`、`url-test`

说明：

- `groups` 只声明地区节点组
- 地区组的 `match` 作用于过滤后的拉取类节点（订阅 + Snell + VLESS）
- 链式组不写在 `groups` 中，由 `relay_through` 派生
- 所有节点组都必须显式声明策略，不允许隐式默认值

---

## routing

```yaml
routing:
  🚀 快速选择: ["@auto"]
  🚀 手动切换: ["@all"]
  📲 Telegram: [🇭🇰 Hong Kong, 🚀 快速选择, "@auto", REJECT]
  💻 Github: [🚀 快速选择, "@auto"]
  🍎 Apple: [DIRECT, 🚀 快速选择, "@auto"]
  🛑 BanList: [REJECT, DIRECT]
  🐟 FINAL: ["@auto"]
```

用途：

- 定义服务组层的面板内容和显示顺序
- 列表中的顺序即推荐顺序

可引用对象：

- 节点组名
- 其他服务组名
- `DIRECT`
- `REJECT`
- `@all`
- `@auto`

约束：

- key 为服务组名
- value 为有序列表
- 服务组策略固定为 `select`
- `@all` 展开为全部原始节点（订阅节点 + Snell 节点 + VLESS 节点 + 不带 `relay_through` 的自定义代理），不包含链式节点
- 带 `relay_through` 的自定义代理仅作链式模板，不以独立代理形式出现在 `proxies` 里，因而也不进入 `@all`
- 用户配置中不允许直接写原始代理名；若需要“全部原始节点”，必须通过 `@all` 展开
- `@auto` 展开为自动补充池，替换其所在位置。池内容按顺序：全部节点组名（地区组 + 链式组，按声明序）→ 包含 `@all` 的服务组名（按声明序）→ `DIRECT`
- `@auto` 自动去重：已在同一 entry 中出现的项不会重复；组不包含自身
- `REJECT` 不在 `@auto` 补充池中；如需使用，必须由用户显式声明，且位置保持不变
- 同一 entry 中 `@auto` 最多出现一次
- `@all` 与 `@auto` 不能在同一 entry 中同时使用

---

## rulesets

```yaml
rulesets:
  🛑 BanList:
    - "https://example.com/BanAD.list"
  📺 Netflix:
    - "https://example.com/Netflix.list"
```

约束：

- key 必须对应一个已定义服务组
- value 为一个或多个 URL
- 多条 URL 合并匹配到同一服务组
- 远端规则集内容必须是纯文本规则列表

---

## rules

```yaml
rules:
  - "GEOIP,CN,🎯 China"
```

约束：

- 保持声明顺序
- 语义由目标客户端规则语法决定
- 本系统只提取最后一个逗号后的策略名做引用校验，不解析规则类型和值
- 渲染时直接透传原始字符串

---

## fallback

```yaml
fallback: 🐟 FINAL
```

约束：

- 必填
- 必须引用一个已定义服务组
- 用于生成最终兜底规则

---

## base_url

```yaml
base_url: "https://my-server.com"
```

用途：

- 声明服务的外部可访问地址
- 用于 Surge 输出时生成 `#!MANAGED-CONFIG` 头，使 Surge 客户端能自动更新配置
- 生成的 managed URL 会使用最终 `filename`，以及服务端启用时的订阅访问 `token`；token 来源不依赖当前请求是 query token 还是后台 session

约束：

- 可选字段
- 值为 scheme + host（如 `https://my-server.com` 或 `http://192.168.1.1:8080`），不含路径、query 或 fragment
- 为空时 Surge 输出不包含 `#!MANAGED-CONFIG` 头

说明：

- 订阅访问 token 属于服务运行时参数，不属于 YAML 配置字段；它只保护 `/generate` 客户端订阅更新，不作为 Web 管理后台登录凭据

---

## templates

```yaml
templates:
  clash: "configs/base_clash.yaml"
  surge: "https://example.com/base_surge.conf"
  loon: "configs/base_loon.lcf"
```

用途：

- 声明 Clash Meta、Surge 和 Loon 的底版配置模板
- 渲染时将生成的节点、分组和规则注入底版，保留底版中的通用设置（如 `port`、`dns`、`[General]` 等）
- 无底版时仅输出目标格式的生成段，见 [渲染设计](rendering.md#底版模板合并)

约束：

- `clash`、`surge`、`loon` 均可选，值可为本地文件路径或 HTTP(S) URL
- `inline` 可选，为格式名到非空正文的映射，仅接受 `clash`、`surge`、`loon`。同一格式的路径与 `inline` 正文只能选一个
- Web 保存时将 `inline` 正文写为版本文件，把对应 `clash` / `surge` / `loon` 改为该文件路径；生成阶段对正文和外部模板使用相同的合并规则
- 若为 HTTP(S) URL，必须满足基本 URL 格式要求
- 若为本地路径，不做格式校验（加载时由 OS 报错）
- 远程模板与订阅共享同一缓存机制（CachedFetcher + TTL）

正文存储规则：

- 默认直接写入 `/config`，不增加子目录。可通过 `-data-dir` 或 `SUBCONVERTER_DATA_DIR` 指定本地开发或其他部署的目录，参数优先于环境变量
- Loon 节点正文与底版模板均以 `.lcf` 保存；其他节点正文使用 `.txt`，Clash 模板使用 `.yaml`，Surge 模板使用 `.conf`
- 保存时逐字比较当前已保存配置引用的服务正文文件：相同类型、扩展名和正文复用原引用；订阅重排不生成新文件。只有新增或内容变化时才生成版本文件，例如 `subscription-20261101T123456.123456789Z-123456789.txt`、`template-loon-20261101T123456.123456789Z-987654321.lcf`；时间戳使用 UTC，唯一后缀避免同时保存重名
- 只复用当前正文目录内、符合服务版本文件命名的已保存引用，不扫描历史文件。普通文件或远程模板切换为本地正文时仍生成独立副本；旧版本缺失或不可读时，允许用提交的正文重新保存
- 原订阅文件与模板不覆盖，旧版本保留。服务读取主配置中的版本引用，热重载前的运行快照继续使用旧版本；主配置提交失败时移除本次新增文件
- 新建目录权限为 `0700`，正文文件为 `0600`。保存时实际探测目录写入权限，失败返回 `409 resource_content_write_failed`，消息包含目录位置和处理建议
- `GET /api/config` 会把可读正文文件还原为编辑正文，revision 仍取主配置文件。无法读取的文件保留原引用，用户仍可在 Web 修改路径、重新输入正文或删除来源；读取错误显示在对应编辑器的读取状态中
- 模板只有位于当前写入目录且完整匹配服务版本文件名时才自动还原正文。普通模板文件保持路径引用；更改写入目录不会移动已有版本，需要时可读取旧路径再保存副本
- 配置包中的订阅正文与模板存放方式见 [导出配置 API](api.md#get-apiconfigeffectivezip)。正文读取失败则整个导出失败；单独导出生效 YAML 保留实际文件引用
- 旧版内联正文仍可加载，下次 Web 保存时转为上述版本文件。历史版本由部署者保留或清理；清理时保留主配置引用文件，并在停止服务后处理，避免影响运行快照

Web 正文编辑对应的 YAML 示例（JSON 使用相同字段）：

```yaml
sources:
  subscriptions:
    - format: loon
      content: |
        HK = anytls, hk.example.com, 443, "example-password"
templates:
  inline:
    loon: |
      [General]
      dns-server = system
```

注：配置文件本身（`-config` 参数）也支持 HTTP(S) URL，加载时同样经过 `LoadResource` 和 `CachedFetcher`。

---

## 顺序规则

以下段落的书写顺序直接影响输出：

- `groups`
- `routing`
- `rulesets`

系统必须保留此顺序，以保证客户端面板与配置书写顺序一致。

`base_url` 是标量值，不属于顺序敏感字段。

---

## JSON API 表示（v2.0）

Web 管理后台通过 `GET /api/config` 和 `PUT /api/config` 以 JSON 格式读写配置。本节是保序字段 JSON 序列化的**权威定义**——其他文档（`api.md`、`web-ui.md`、`domain-model.md`）中的相关描述均引用此处。

### 总体规则

- JSON 结构与 YAML 结构一一映射（Go struct 的 `json` tag）
- `GET /api/config` 外层返回 `{config_revision, config}`；`config_revision` 为配置源原始字节的 `sha256:<hex>`
- `PUT /api/config` 请求体同样使用 `{config_revision, config}`；服务端仅在 revision 与当前文件一致时写回
- 写回 YAML 时使用稳定缩进，但不保证保留原始引号、行内样式或字段注释
- YAML 写回可能丢失原始文件中的注释（`gopkg.in/yaml.v3` 限制）
- 当 `-config` 是 HTTP(S) URL 时，配置源为只读：`GET /api/config` 可用，`PUT /api/config` 返回 `409`

### 保序字段序列化规范

标准 JSON 对象不保证键序，因此 `groups`、`routing`、`rulesets` 三个保序字段统一使用 `[{key, value}]` 数组表示。每个数组元素是一个对象，固定包含 `key`（字符串）和 `value`（对应字段的值类型）两个属性。

**groups 示例**：

```json
{
  "groups": [
    { "key": "🇭🇰 Hong Kong", "value": { "match": "(港|HK|Hong Kong)", "strategy": "select" } },
    { "key": "🇯🇵 Japan", "value": { "match": "(日本|JP|Japan)", "strategy": "url-test" } }
  ]
}
```

**routing 示例**：

```json
{
  "routing": [
    { "key": "🚀 快速选择", "value": ["@auto"] },
    { "key": "📲 Telegram", "value": ["🇭🇰 Hong Kong", "🚀 快速选择", "@auto", "REJECT"] }
  ]
}
```

**rulesets 示例**：

```json
{
  "rulesets": [
    { "key": "🛑 BanList", "value": ["https://example.com/BanAD.list"] },
    { "key": "📺 Netflix", "value": ["https://example.com/Netflix.list"] }
  ]
}
```

**空集合**：空保序字段序列化为空数组 `[]`，不使用 `null`。

**round-trip 不变量**：`GET /api/config` → 不修改 → `PUT /api/config` 必须保持数组元素顺序和 value 内容完全一致（即 JSON round-trip 幂等）。前端拖拽排序通过交换数组索引实现顺序变更。

### sources 的 JSON 表示

`sources` 是异构结构体（各拉取类型的 value 类型不同），不适用 `[{key,value}]` 数组格式。JSON 表示保持对象结构，各字段与 YAML 一一映射，并新增 `fetch_order` 字段显式记录拉取顺序：

```json
{
  "sources": {
    "subscriptions": [{ "url": "https://..." }],
    "snell": [{ "url": "https://..." }],
    "vless": [{ "url": "https://..." }],
    "custom_proxies": [
      { "name": "🔗 HK-ISP", "url": "socks5://...", "relay_through": { "type": "group", "name": "🇭🇰 Hong Kong", "strategy": "select" } }
    ],
    "fetch_order": ["subscriptions", "snell", "vless"]
  }
}
```

字段说明：

- `fetch_order`：字符串数组，记录 `subscriptions` / `snell` / `vless` 三类拉取来源在 YAML 中的声明顺序。此顺序决定管道中代理节点的相对排列
- `fetch_order` 只包含拉取类键名，不包含 `custom_proxies`
- `GET /api/config` 返回时根据 YAML 声明顺序填充；`PUT /api/config` 写回 YAML 时按 `fetch_order` 排列 `sources` 子键顺序
- `GET /api/config` 始终返回完整三项 `fetch_order`，即使某类来源列表为空；空来源是否存在由对应数组内容表达，不由 `fetch_order` 表达
- `PUT /api/config` 中 `fetch_order` 缺失或为空数组时，服务端使用默认顺序 `["subscriptions", "snell", "vless"]`
- `PUT /api/config` 中 `fetch_order` 非空时必须完整包含 `subscriptions`、`snell`、`vless` 三项，且每项恰好出现一次
- API JSON 输入中 `fetch_order` 出现未知值、重复值或缺项时，app 层返回 `invalid_fetch_order` 诊断；`POST /api/config/validate` 返回 `200 valid=false`，`PUT /api/config` 返回 `400 ValidateResult`；`POST /api/reload` 加载 YAML 源，不消费该 JSON 字段
- 前端拖拽调整来源顺序时，修改 `fetch_order` 数组即可
- YAML 写回时，拉取类来源按 `fetch_order` 顺序输出；`custom_proxies` 不进入 `fetch_order`，始终排在拉取类来源之后

round-trip 不变量：`GET → PUT` 后 `fetch_order` 和各拉取类型列表内容均不变。

### `json_pointer` 定位规则

保序字段在 JSON 中是数组，因此诊断定位使用数组索引。例如 YAML 中 `groups` 的第一个组 `🇭🇰 Hong Kong` 的 `match` 字段，其 `json_pointer` 为 `/config/groups/0/value/match`。即使 key 含空格、点号或 emoji，也通过 `index` 和 `json_pointer` 定位，不依赖 key 值解析。
