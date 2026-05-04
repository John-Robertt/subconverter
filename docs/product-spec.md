# 产品规格：subconverter v2.0

> v1.0 产品规格已归档至 docs/v1.0/product-spec.md
>
> 状态提示：本文描述 v2.0 产品目标；当前可用能力与规划能力的边界见 docs/README.md 状态矩阵。

## 一、我们有什么（输入素材）

### 1.1 SS 订阅源

一个或多个订阅链接，返回节点列表：

```
ss://...@hk.example.com:8388#HK-01
ss://...@hk2.example.com:8388#HK-02
ss://...@sg.example.com:8388#SG-01
ss://...@us.example.com:8388#US-01
ss://...@jp.example.com:8388#JP-东京-01
```

节点名称是后续分组的依据。

### 1.1b Snell 来源（Surge 专属）

Snell 服务（如 jinqians/snell.sh 搭建）不提供标准订阅 URL，只产出 Surge 风格的单行节点配置。为支持这类来源，系统提供独立的 `sources.snell` 段，接受纯文本 URL：

```yaml
sources:
  snell:
    - url: "https://my-server.com/snell-nodes.txt"
```

URL 返回内容（纯文本，按行）：

```
HK-Snell = snell, 1.2.3.4, 57891, psk=xxx, version=4, reuse=true, tfo=true
JP-Snell = snell, 9.10.11.12, 443, psk=zzz, version=4, shadow-tls-password=sss, shadow-tls-sni=www.microsoft.com, shadow-tls-version=3
```

关键特性：

- **只进入 Surge 输出**：Clash Meta 主线不支持 Snell v4/v5（jinqians 默认版本），Clash 渲染会自动过滤 Snell 节点并级联清理空组
- **全字段支持**：包括 ShadowTLS（Surge 独有）、reuse、tfo、obfs、udp-relay 等
- 节点参与与 SS 订阅共享的去重池、`filters.exclude` 过滤、区域组 regex 匹配
- 可作为 `relay_through` 的链式上游
- **失败可定位**：单行解析失败会报整源错误，消息附带脱敏后的来源 URL 和 1-based 物理行号；原始解析根因保留在错误链中

### 1.1c VLESS 来源（Clash 专属）

VLESS 来源通过独立的 `sources.vless` 段接入，接受纯文本 URL；URL 返回内容为"一行一个标准 VLESS URI"：

```yaml
sources:
  vless:
    - url: "https://my-server.com/vless-nodes.txt"
```

URL 返回内容（纯文本，按行）：

```text
vless://11111111-2222-3333-4444-555555555555@hk.example.com:443?security=tls&sni=hk.example.com&type=tcp#HK-VL
vless://11111111-2222-3333-4444-555555555555@sg.example.com:443?security=reality&sni=www.cloudflare.com&pbk=KEY&sid=SHORT#SG-VL
```

关键特性：

- **只进入 Clash 输出**：Surge 不原生支持 VLESS，Surge 渲染会自动过滤 VLESS 节点并级联清理空组
- `type` 缺失或未知值回落到 `tcp`；`encryption` 非空时透传
- 节点参与与 SS / Snell 共享的去重池、`filters.exclude` 过滤、区域组 regex 匹配
- 可作为 `relay_through` 的链式上游
- **失败可定位**：单行解析失败会报整源错误，消息附带脱敏后的来源 URL 和 1-based 物理行号；原始解析根因保留在错误链中

### 1.2 自定义代理节点

不来自订阅、手动定义的节点（如机房专线、ISP 代理）：

```
名称: HK-ISP
类型: socks5/http
地址: 154.197.1.1:45002
认证: user/pass
```

### 1.3 代理链关系

自定义代理无法直连，需声明"通过哪些节点中转"。三种筛选中转节点的方式：

- **group** — 引用已定义的节点组（如 `🇭🇰 Hong Kong`）
- **select** — 用正则从拉取类节点（订阅 + Snell + VLESS）中直接筛选（如 `(港|HK)`）
- **all** — 使用全部拉取类节点

工具自动将筛选到的每个节点展开为链式路径：

```
HK-01 → HK-ISP
HK-02 → HK-ISP
HK-03 → HK-ISP
```

### 1.4 远程规则集

每条远程规则集 URL 绑定到一个服务名：

| 服务       | 规则集（可多条合并）                                                                  |
| ---------- | ------------------------------------------------------------------------------------- |
| 广告拦截   | BanAD.list, BanProgramAD.list, adblockloon.list                                       |
| Netflix    | Netflix.list                                                                          |
| YouTube    | YouTube.list, YouTubeMusic.list                                                       |
| Telegram   | Telegram.list                                                                         |
| Google     | Google.list                                                                           |
| Github     | Github.list                                                                           |
| Apple      | Apple.list                                                                            |
| Microsoft  | Microsoft.list                                                                        |
| OneDrive   | OneDrive.list                                                                         |
| PayPal     | PayPal.list                                                                           |
| Stripe     | Stripe.list                                                                           |
| DisneyPlus | DisneyPlus.list                                                                       |
| ViuTV      | ViuTV.list (x2)                                                                       |
| DMM        | Dmm.list                                                                              |
| 中国直连   | ChinaDomain.list, ChinaMedia.list, SteamCN.list, Download.list, LAN.list, Direct.list |
| 全球代理   | ProxyMedia.list, ProxyGFWlist.list, Global.list                                       |

### 1.5 兜底

未匹配任何规则集的流量 → 用户选择的默认出口（fallback）

---

## 二、我们要什么（用户在客户端看到的）

### 2.1 面板结构

用户打开 Surge / Clash Meta 客户端后，看到的代理组面板：

#### 服务组 —— "这个服务走哪个出口"

用户为每个服务选择一个出口（通常是地区组），日常很少改动。

```
🚀 快速选择      ─▶  🇭🇰 HK │ 🇸🇬 SG │ 🇨🇳 TW │ 🇯🇵 JP │ 🇺🇲 US │ 🔗 HK-ISP │ 手动切换 │ DIRECT
🚀 手动切换      ─▶  HK-01 │ HK-02 │ SG-01 │ US-01 │ HK-ISP │ ...（全部原始节点，不含链式节点）
📲 Telegram      ─▶  🇭🇰 HK │ 快速选择 │ 手动切换 │ 🇸🇬 SG │ 🇨🇳 TW │ 🇯🇵 JP │ 🇺🇲 US │ DIRECT
📺 Netflix       ─▶  🇸🇬 SG │ 快速选择 │ 手动切换 │ 🇭🇰 HK │ 🇨🇳 TW │ 🇯🇵 JP │ 🇺🇲 US │ DIRECT
📺 DisneyPlus    ─▶  🇭🇰 HK │ 快速选择 │ 手动切换 │ 🇸🇬 SG │ 🇨🇳 TW │ 🇯🇵 JP │ 🇺🇲 US │ DIRECT
📺 ViuTV         ─▶  🇭🇰 HK │ 快速选择 │ 手动切换 │ ...
🎬 YouTube       ─▶  🇭🇰 HK │ 快速选择 │ 手动切换 │ ...
🍎 Apple         ─▶  DIRECT │ 快速选择 │ 手动切换 │ 🇭🇰 HK │ ...
🔍 Google        ─▶  🇸🇬 SG │ 快速选择 │ 手动切换 │ ...
💻 Github        ─▶  快速选择 │ 手动切换 │ ...
☁️ OneDrive      ─▶  快速选择 │ 手动切换 │ ...
Ⓜ️ Microsoft     ─▶  快速选择 │ 手动切换 │ ...
💳 PayPal        ─▶  🇺🇲 US │ 快速选择 │ 手动切换 │ ...
💳 Stripe        ─▶  🇺🇲 US │ 快速选择 │ 手动切换 │ ...
🌍 DMM           ─▶  🇯🇵 JP │ 快速选择 │ 手动切换 │ DIRECT
🎯 Global        ─▶  快速选择 │ 手动切换 │ ...
🎯 China         ─▶  DIRECT │ 快速选择 │ 手动切换
🛑 BanList       ─▶  REJECT │ DIRECT
🐟 FINAL         ─▶  快速选择 │ 手动切换 │ 🇭🇰 HK │ ... │ DIRECT
```

**选项排列顺序 = 用户偏好优先级**（第一个是默认推荐出口）。

#### 节点组 —— "这个地区/链路用哪个具体节点"

```
🇭🇰 Hong Kong    ─▶  HK-01 │ HK-02 │ HK-03 │ ...
🇸🇬 Singapore    ─▶  SG-01 │ SG-02 │ ...
🇨🇳 Taiwan       ─▶  TW-01 │ ...
🇯🇵 Japan        ─▶  JP-东京-01 │ ...
🇺🇲 United States ─▶  US-01 │ ...
🔗 HK-ISP        ─▶  HK-01→HK-ISP │ HK-02→HK-ISP │ HK-03→HK-ISP │ ...
```

每个节点组支持两种策略：

- **select**：用户手动选节点
- **url-test**：自动选延迟最低的节点

### 2.2 路由行为（用户无感，自动生效）

```
用户访问 netflix.com
  → 匹配 Netflix 规则集
  → 走用户在「📺 Netflix」中选的出口（默认 🇸🇬 SG）
  → 走用户在「🇸🇬 Singapore」中选的具体节点（如 SG-01）
  → 连接目标

用户访问 baidu.com
  → 匹配中国直连规则集 / GEOIP CN
  → 走 DIRECT

用户访问未匹配的域名
  → 走「🐟 FINAL」中用户选的出口

用户访问广告域名
  → 匹配 BanList 规则集
  → REJECT
```

### 2.3 用户日常操作频率

| 操作                     | 频率                          |
| ------------------------ | ----------------------------- |
| 什么都不做，自动路由     | 99%                           |
| 切换某个服务的出口地区   | 偶尔（如 Netflix 换区看内容） |
| 切换某个地区组的具体节点 | 偶尔（节点挂了或变慢）        |
| 修改配置文件重新生成     | 很少（加服务/换订阅时）       |

---

## 三、输出格式

工具以 HTTP API 服务运行，用户请求时指定格式：

- Clash Meta 配置文件（.yaml）
- Surge 配置文件（.conf）

两者的面板结构、路由行为在目标格式都支持的协议范围内保持一致，只是语法不同。已知格式例外有两类：Snell 只进入 Surge 输出，Clash 会在渲染入口做级联过滤；VLESS 只进入 Clash 输出，Surge 会在渲染入口做级联过滤。输出格式不在配置文件中指定，由请求参数决定。

请求参数语义：

- `format=clash|surge`：必填，决定输出格式
- `token=<access-token>`：当服务端启用了订阅访问 token 时，`/generate` 可通过 query 参数携带；该 token 只用于 Clash / Surge 等客户端自动更新订阅，不作为 Web 管理后台登录凭据
- `filename=<custom-name>`：可选，自定义下载文件名；未传时默认使用 `clash.yaml` / `surge.conf`；仅允许 ASCII 字母、数字、`.`、`-`、`_`

补充约束：

- 订阅访问 token 属于服务运行时参数，不写入用户 YAML 配置
- Surge 的 `#!MANAGED-CONFIG` 需要回写服务端配置的订阅访问 token（若启用）和最终 `filename`；不得依赖当前请求是否通过 query token 或后台 session 鉴权，保证客户端后续自动更新仍能访问同一 URL

---

## 四、Web 管理后台（v2.0 新增）

### 定位变迁

- **v1.0**：纯 HTTP API 服务，配置编辑依赖用户手动修改 YAML 文件
- **v2.0**：在 API 服务之上叠加 Web 管理后台，提供可视化编辑、静态配置校验、运行时预览和热重载能力
- **不变的原则**：YAML 配置文件始终是唯一真相源（source of truth）；Web 后台是 YAML 的可视化外壳，不持有前端独有状态。任何通过后台做的修改最终都写回 YAML 文件，可被 git 追踪、diff 审查

### 4.1 目标用户与核心场景

**目标用户**：自部署的单用户（与 v1.0 一致）。

单用户不等于无后台认证。v2.0 Web 管理后台面向公网部署时，必须通过独立的管理员登录态保护：首次启动且无管理员凭据时进入 setup 流程，并通过 bootstrap setup token 防止公网抢先初始化；setup 创建单一管理员账号；后续访问后台页面和 `/api/*` 管理接口都依赖 `session_id` HttpOnly Cookie。订阅访问 token 继续只保护 `/generate` 自动更新链接，二者互不替代。

**核心场景**：可视化配置编辑。对应 2.3 节操作频率表中"修改配置文件重新生成"一行——频率很低（加服务/换订阅时），但每次操作涉及多个配置段的协调修改，出错成本高。Web 后台将这类低频高风险操作从"手写 YAML + 人工校验"升级为"表单编辑 + 静态校验 + 预览确认 + 一键热重载"。

**辅助场景**：运行时数据预览——查看当前节点列表、组匹配结果、生成的配置文件内容。这些信息在 v1.0 中只能通过请求 `/generate` 端点间接获得，后台提供更直观的查看方式。

### 4.2 信息架构

后台包含一个登录入口 `/login`，以及登录后可访问的三个功能区，共 12 个受保护路由页面；另有一个校验 Drawer 组件用于诊断修复引导：

| 区域 | 定位 | 页面数 | 典型页面 |
| ---- | ---- | ------ | -------- |
| A 配置编辑 | 对应 YAML 各段落的表单化编辑与即时校验 | 8 | 来源管理(A1)、节点组(A3)、路由(A4) |
| B 运行时预览 | 展示管道各阶段的运行时数据 | 3 | 节点列表(B1)、组匹配结果(B2)、生成下载(B3) |
| C 系统状态 | 服务运行状态与版本信息 | 1 | 系统状态(C) |

完整页面定义与技术交互细节见 `docs/design/web-ui.md`。

### 4.3 交互模式

以下五种交互模式在后台中统一使用，不引入其他交互范式：

| 模式 | 触发场景 | 行为 |
| ---- | -------- | ---- |
| Modal 对话框 | 新增/编辑条目（来源、节点组、服务组、规则集等） | 弹出表单，填写完成后确认提交 |
| Toast 提示 | 操作成功或失败的即时反馈 | 右上角短暂显示结果消息，自动消失 |
| Confirm 确认框 | 破坏性操作（删除来源、删除节点组等） | 要求用户二次确认后执行 |
| Spinner 加载态 | 网络请求进行中（保存配置、拉取预览数据等） | 按钮/区域显示加载指示器，阻止重复提交 |
| Drawer 抽屉 | 校验结果修复引导（A8 页面的错误项点击展开详情） | 右侧滑出详情面板，展示错误上下文与修复建议 |

### 4.4 非功能需求

- **最小分辨率**：1280x800（适配 13 寸笔记本）
- **主题**：支持 Light / Dark 两套主题，跟随系统偏好或手动切换
- **离线能力**：配置编辑（A 区表单填写）不依赖网络；保存配置、运行时预览（B 区）、系统状态（C 区）需要与后端通信

A8 静态配置校验只覆盖字段、引用、命名和环路等 `Prepare` 阶段问题；远程源拉取、过滤后空组、目标格式级联过滤和渲染错误需通过 B1/B2/B3 运行时或生成预览确认。

---

## 五、用户配置文件草案

用户编写一个 YAML 配置文件，工具读取后生成 Clash Meta / Surge 配置。

```yaml
base_url: "https://my-server.com"

sources:
  subscriptions:
    - url: "https://sub.example.com/api/v1/client/subscribe?token=xxx"
  snell:
    - url: "https://my-server.com/snell-nodes.txt"
  vless:
    - url: "https://my-server.com/vless-nodes.txt"
  custom_proxies:
    - name: 🔗 HK-ISP # 链式组名 = name 原样值；如需视觉前缀（🔗/⚡/CHAIN-）自行写入 name
      url: socks5://tXJ695acaa15:4jtE3Mq7d0zcoO@154.197.1.1:45002 # 支持 ss:// / socks5:// / http://
      relay_through: # 声明后仅作链式模板，不作独立 KindCustom 节点
        type: group # group — 引用节点组 | select — 正则筛选拉取类节点 | all — 全部拉取类节点
        name: 🇭🇰 Hong Kong # group 时填组名，select 时改为 match: "(港|HK)"
        strategy: select # 链式组策略，必须显式指定 select/url-test

filters: # 可选
  exclude: "(过期|剩余流量|到期)" # 排除匹配的节点

groups:
  🇭🇰 Hong Kong: { match: "(港|HK|Hong Kong)", strategy: select }
  🇸🇬 Singapore: { match: "(新加坡|坡|狮城|SG|Singapore)", strategy: select }
  🇨🇳 Taiwan: { match: "(台|新北|彰化|TW|Taiwan)", strategy: select }
  🇯🇵 Japan:
    {
      match: "(日本|川日|东京|大阪|泉日|埼玉|沪日|深日|[^-尼]日|JP|Japan)",
      strategy: url-test,
    }
  🇺🇲 United States:
    {
      match: "(美国|波特兰|达拉斯|俄勒冈|凤凰城|费利蒙|硅谷|拉斯维加斯|洛杉矶|圣何塞|圣克拉拉|西雅图|芝加哥|US|USA|United States)",
      strategy: select,
    }
  # 链式组（上例 "🔗 HK-ISP"）由 relay_through 自动生成，组名 = custom_proxies.name 原样，无需在此定义

# 服务组：key = 组名，value = 有序出口列表（第一个是默认推荐）
# 可引用：节点组名、其他服务组名、链式组名（custom_proxies.name 原样，如 "🔗 HK-ISP"）、DIRECT、REJECT、@all（全部原始节点 = 订阅节点 + Snell 节点 + VLESS 节点 + 无 relay_through 的自定义代理；不含链式节点）、@auto（自动补充剩余成员）
# @auto 展开为：全部节点组（声明序）→ 包含 @all 的服务组（声明序）→ DIRECT（去重、排除自身）
# REJECT 不在 @auto 中；如需使用，必须显式写在成员列表里
# 同一 entry 中 @auto 最多出现一次；@auto 与 @all 不能在同一 entry 中同时使用
# 书写顺序 = 面板显示顺序
routing:
  🚀 快速选择: ["@auto"]
  🚀 手动切换: ["@all"]
  📲 Telegram:
    [
      🇭🇰 Hong Kong,
      🚀 快速选择,
      "@auto",
      REJECT,
    ]
  📺 Netflix: [🇸🇬 Singapore, 🚀 快速选择, "@auto"]
  📺 DisneyPlus: [🇭🇰 Hong Kong, 🚀 快速选择, "@auto"]
  # ... 其余服务组结构相同，仅首选出口不同 ...
  🍎 Apple: [DIRECT, 🚀 快速选择, "@auto"]
  💳 PayPal: [🇺🇲 United States, 🚀 快速选择, "@auto"]
  🌍 DMM: [🇯🇵 Japan, 🚀 快速选择, "@auto"]
  🎯 Global: [🚀 快速选择, "@auto"]
  🎯 China: [DIRECT, 🚀 快速选择, "@auto"]
  🛑 BanList: [REJECT, DIRECT]
  🐟 FINAL: ["@auto"]

# 规则集：key = routing 中的服务组名，value = URL 列表（多条合并匹配）
# 远端内容必须是纯文本规则列表，不支持 Clash payload YAML
rulesets:
  🛑 BanList:
    - "https://gcore.jsdelivr.net/gh/217heidai/adblockfilters@main/rules/adblockloon.list"
    - "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/BanAD.list"
    - "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/BanProgramAD.list"
  📺 Netflix:
    - "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/Netflix.list"
  📲 Telegram:
    - "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Telegram.list"
  # ... 其余服务组各绑定对应的规则集 URL ...
  🎯 China:
    - "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ChinaDomain.list"
    - "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ChinaMedia.list"
    - "https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/LocalAreaNetwork.list"
    # ... 等 ...

rules:
  - "GEOIP,CN,🎯 China" # 中国 IP  → 直连

fallback: 🐟 FINAL # 未匹配任何规则的流量走这里（Clash 生成 MATCH，Surge 生成 FINAL）
```

### 配置与面板的对应关系

```
配置段落                         →    客户端面板
────────                             ────────
sources                         →    节点池（订阅节点 + Snell 节点 + VLESS 节点 + 自定义节点 + 链式节点）
groups                          →    节点组层：🇭🇰 Hong Kong, 🔗 HK-ISP, ...
routing                         →    服务组层：📺 Netflix, 📲 Telegram, ...
rulesets + rules + fallback     →    自动路由（用户无感）
```

---

## 六、设计决策记录

| 决策               | 结论                                      | 原因                             |
| ------------------ | ----------------------------------------- | -------------------------------- |
| 配置风格（用户侧） | 声明式分层 YAML                           | 关注点分离，可读性强             |
| 代码架构（开发侧） | 管道模型 Source→Filter→Group→Route→Render | 灵活、可调试                     |
| 内置规则库         | 不做                                      | 统一用 ruleset URL，不维护规则库 |
| Overlay 多设备支持 | 不做                                      | 当前不需要                       |
| 代理链             | 支持，作为 source 上的可选声明            | 不常用但重要                     |
| 链式节点组         | 不计入 @all                               | 防止节点膨胀                     |
| 链式节点组使用方式 | 与地区组完全一致，可被服务组引用          | 无特殊限制                       |
| 路由自动补充       | `@auto` 展开为节点组+包含 `@all` 的服务组+DIRECT | 消除 routing 冗余，链式组自动可用 |
| 节点组策略         | 所有节点组都需显式指定 select/url-test    | 手动 + 自动，避免隐式默认值      |
| 输出目标           | Clash Meta + Surge                        | Shadowrocket/QuantumultX 暂不做  |
| Surge 订阅更新     | 在配置中声明 `base_url`，渲染时生成 `#!MANAGED-CONFIG`，使用服务端订阅访问 token（若启用）和最终 `filename` | 用户显式控制，无需依赖反向代理头；token 来源不依赖当前请求鉴权方式，见 `T-PRV-014` |
| 访问控制边界       | `/api/*` 使用管理员登录态和 `session_id` Cookie；`/generate` 保留 query token 兼容订阅链接；订阅 token 不进入 YAML，也不作为后台登录凭据 | 将公网后台权限与客户端订阅更新密钥解耦 |
| 默认文件名         | Clash 默认 `clash.yaml`；Surge 默认 `surge.conf` | 客户端订阅与浏览器下载都需要稳定文件名 |
| Web 管理后台       | React SPA 由生产镜像嵌入 Go 二进制，单个 `subconverter` 服务同源托管 SPA、`/api/*`、`/generate` 和 `/healthz` | 单镜像单服务，部署路径更简单 |
| 配置热重载         | RWMutex 保护 RuntimeConfig，写回 YAML + re-Prepare | 编辑后无需重启服务               |
| YAML 真相源        | UI 是 YAML 的可视化外壳，无前端独有状态   | 数据一致性，任何修改可追溯到 YAML |
| Admin API 前缀     | `/api/*` 与 `/generate` 平行              | 不影响现有生成接口               |
