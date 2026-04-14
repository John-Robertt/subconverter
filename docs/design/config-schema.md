# 配置结构设计

## 目标

本文件定义用户 YAML 配置的正式结构、字段约束和配置语义。它只回答“用户如何声明系统行为”，不涉及 Go 代码实现。

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
- `sources`：声明订阅源、Snell 来源和自定义代理
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
      type: socks5
      server: 154.197.1.1
      port: 45002
      username: user
      password: pass
      relay_through:
        type: group
        name: 🇭🇰 Hong Kong
        strategy: select
```

注：`subscriptions` / `snell` / `vless` 三类拉取来源按 YAML 中的书写顺序遍历（`Sources.FetchOrder`）。例如上例顺序 `subscriptions → snell → vless` 即节点在后续管道中的相对顺序。`custom_proxies` 不属于拉取类，排在最后，与 YAML 位置无关。

### subscriptions

- `url` 必填，必须为 HTTP(S) URL
- 支持多个订阅源
- 每个订阅源返回 SS 节点列表

### snell

- `url` 必填，必须为 HTTP(S) URL
- 支持多个 Snell 来源
- URL 返回纯文本内容，每行一条 Surge 风格的 Snell 节点声明：

  ```
  NAME = snell, SERVER, PORT, psk=..., version=..., [其他可选键]
  ```

- 可选字段：`version`、`obfs`、`obfs-host`、`obfs-uri`、`reuse`、`tfo`、`udp-relay`、`udp-port`、`shadow-tls-password`、`shadow-tls-sni`、`shadow-tls-version`
- 空行和以 `#` / `//` 开头的注释行会被跳过；单行解析失败时整源报错（与 SS 订阅的静默跳过不同——Snell 来源通常是小规模手工清单，严格报错更有利于发现拼写问题）
- 单行解析失败的错误消息会附带脱敏后的来源 URL 与 1-based 物理行号；原始解析根因保留在 `BuildError.Cause`
- 节点名参与与订阅节点共享的跨源去重池（重复名追加 ②③... 后缀）
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
- 节点名参与与订阅节点、Snell 节点共享的跨源去重池（重复名追加 ②③... 后缀）
- 当前 URI 模型仅承接本节列出的 query；像 `packet-encoding`、`support-x25519mlkem768` 等 Mihomo 代理字段本次不接入，避免在未定义稳定 query 契约前产生隐式支持承诺
- VLESS 节点**只进入 Clash Meta 输出**；Surge 输出会过滤掉这些节点及级联清理的空组、失效链式节点、空规则（与 Snell 在 Clash 侧的处理对称）。详见 `rendering.md`

### custom_proxies

- 用于声明不来自订阅的代理节点
- 当前支持 `socks5`、`http`
- 可带认证信息
- 可选声明 `relay_through`

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
- 系统自动生成一个链式节点组，组名为 `🔗 <custom_proxy.name>`
- 链式组属于节点组，策略取自 `relay_through.strategy`

---

## filters

```yaml
filters:
  exclude: "(过期|剩余流量|到期)"
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
- `@all` 展开为全部原始节点（订阅节点 + Snell 节点 + VLESS 节点 + 自定义代理），不包含链式节点
- 自定义代理即使声明了 `relay_through`，仍属于原始节点，必须被 `@all` 包含
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
- 生成的 managed URL 会继承当前请求里的 `filename`，以及服务端启用时的访问 `token`

约束：

- 可选字段
- 值为 scheme + host（如 `https://my-server.com` 或 `http://192.168.1.1:8080`），不含路径、query 或 fragment
- 为空时 Surge 输出不包含 `#!MANAGED-CONFIG` 头

说明：

- 访问 token 属于服务运行时参数，不属于 YAML 配置字段

---

## templates

```yaml
templates:
  clash: "configs/base_clash.yaml"
  surge: "https://example.com/base_surge.conf"
```

用途：

- 声明 Clash Meta 和 Surge 的底版配置模板
- 渲染时将生成的节点、分组和规则注入底版，保留底版中的通用设置（如 `port`、`dns`、`[General]` 等）
- 无底版时仅输出生成段（proxies / proxy-groups / rule-providers / rules）

约束：

- 两个字段均可选
- 值可为本地文件路径或 HTTP(S) URL
- 若为 HTTP(S) URL，必须满足基本 URL 格式要求
- 若为本地路径，不做格式校验（加载时由 OS 报错）
- 远程模板与订阅共享同一缓存机制（CachedFetcher + TTL）

注：配置文件本身（`-config` 参数）也支持 HTTP(S) URL，加载时同样经过 `LoadResource` 和 `CachedFetcher`。

---

## 顺序规则

以下段落的书写顺序直接影响输出：

- `groups`
- `routing`
- `rulesets`

系统必须保留此顺序，以保证客户端面板与配置书写顺序一致。

`base_url` 是标量值，不属于顺序敏感字段。
