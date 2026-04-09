# 产品草案：SS 订阅 → Surge / Clash Meta 配置生成工具

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
- **select** — 用正则从订阅节点中直接筛选（如 `(港|HK)`）
- **all** — 使用全部订阅节点

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

两者的面板结构、路由行为完全一致，只是语法不同。输出格式不在配置文件中指定，由请求参数决定。

---

## 四、用户配置文件草案

用户编写一个 YAML 配置文件，工具读取后生成 Clash Meta / Surge 配置。

```yaml
base_url: "https://my-server.com"

sources:
  subscriptions:
    - url: "https://sub.example.com/api/v1/client/subscribe?token=xxx"
  custom_proxies:
    - name: HK-ISP
      type: socks5
      server: 154.197.1.1
      port: 45002
      username: tXJ695acaa15
      password: 4jtE3Mq7d0zcoO
      relay_through: # 经由中转，自动生成 🔗 HK-ISP 链式节点组
        type: group # group — 引用节点组 | select — 正则筛选 | all — 全部节点
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
  # 🔗 HK-ISP 由 relay_through 自动生成，无需定义

# 服务组：key = 组名，value = 有序出口列表（第一个是默认推荐）
# 可引用：节点组名、其他服务组名、🔗 链式组名、DIRECT、REJECT、@all（全部原始节点 = 订阅节点 + 自定义代理，不含链式节点）、@auto（自动补充剩余成员）
# @auto 展开为：全部节点组（声明序）→ 包含 @all 的服务组（声明序）→ DIRECT → REJECT（去重、排除自身）
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

fallback: 🐟 FINAL # 未匹配任何规则集的流量走这里（生成 MATCH 规则）
```

### 配置与面板的对应关系

```
配置段落                         →    客户端面板
────────                             ────────
sources                         →    节点池（订阅节点 + 自定义节点 + 链式节点）
groups                          →    节点组层：🇭🇰 Hong Kong, 🔗 HK-ISP, ...
routing                         →    服务组层：📺 Netflix, 📲 Telegram, ...
rulesets + rules + fallback     →    自动路由（用户无感）
```

---

## 五、设计决策记录

| 决策               | 结论                                      | 原因                             |
| ------------------ | ----------------------------------------- | -------------------------------- |
| 配置风格（用户侧） | 声明式分层 YAML                           | 关注点分离，可读性强             |
| 代码架构（开发侧） | 管道模型 Source→Filter→Group→Route→Render | 灵活、可调试                     |
| 内置规则库         | 不做                                      | 统一用 ruleset URL，不维护规则库 |
| Overlay 多设备支持 | 不做                                      | 当前不需要                       |
| 代理链             | 支持，作为 source 上的可选声明            | 不常用但重要                     |
| 链式节点组         | 不计入 @all                               | 防止节点膨胀                     |
| 链式节点组使用方式 | 与地区组完全一致，可被服务组引用          | 无特殊限制                       |
| 路由自动补充       | `@auto` 展开为节点组+@all 服务组+DIRECT+REJECT | 消除 routing 冗余，链式组自动可用 |
| 节点组策略         | 所有节点组都需显式指定 select/url-test    | 手动 + 自动，避免隐式默认值      |
| 输出目标           | Clash Meta + Surge                        | Shadowrocket/QuantumultX 暂不做  |
| Surge 订阅更新     | 在配置中声明 `base_url`，渲染时生成 `#!MANAGED-CONFIG` | 用户显式控制，无需依赖反向代理头 |
