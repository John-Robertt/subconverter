# 领域模型设计

## 目标

本文件定义系统内部的数据视图，描述配置、节点、组和规则在系统中的抽象方式。实现目标是让解析、分组、渲染共享同一套稳定语义，而不是各自直接操作 YAML 文本。

---

## 三层模型

系统使用三层模型：

- 用户配置层：直接映射 YAML 结构
- 中间表示层：表达系统中的节点、组和规则关系
- 输出表示层：面向 Clash Meta 或 Surge 的最终文本结构

依赖关系：

```text
用户配置层 -> 中间表示层 -> 输出表示层
```

中间表示层是整个系统的稳定边界。

---

## 核心实体

### Proxy

表示一个可被客户端引用的节点。

节点类型分为（由 `Kind` 字段区分）：

| Kind | 来源 | 示例 `Type` | 参与 region regex 匹配 | 可作为链式上游 | 进入 `@all` |
|------|------|-----------|---------------------|----------------|-------------|
| `KindSubscription` | SS 订阅拉取 | `ss` | ✓ | ✓ | ✓ |
| `KindSnell` | Snell 来源拉取（Surge 专属） | `snell` | ✓ | ✓ | ✓ |
| `KindVLess` | VLESS 来源拉取（Clash 专属） | `vless` | ✓ | ✓ | ✓ |
| `KindCustom` | 用户手工声明 | `socks5`, `http` | ✗（名字已显式） | ✗ | ✓ |
| `KindChained` | `relay_through` 派生 | 继承自 custom_proxy | ✗ | ✗ | ✗ |

建议属性：

- `Name`
- `Type`
- `Server`
- `Port`
- `Params`
- `Plugin`
- `Kind`
- `Dialer`

设计意图：

- 用 `Kind` 区分节点来源与行为，而不是只用一个布尔值标记是否链式
- `Dialer` 仅在链式节点上生效，用于标识其上游节点
- `Params` 为 `map[string]string`，承载代理类型的核心参数（如 SS 的 `cipher/password`、socks5/http 的 `username/password`）
- `Plugin` 为可选结构，承载 SS plugin 的名称与参数，避免把 plugin 语义混入 `Params` 的字符串 key 约定中
- 渲染器按目标客户端能力解释 `Plugin`：Clash Meta 可通用透传，Surge 仅支持可映射的子集

### ProxyGroup

表示客户端面板上的一个组。

分为两类：

- 节点组：地区组、链式组
- 服务组：Telegram、Netflix、FINAL 等

建议属性：

- `Name`
- `Scope`：`node` 或 `route`
- `Strategy`
- `Members`：有序字符串列表

`Members` 的合法成员种类：

| 成员种类 | 示例 | 来源 |
|---------|------|------|
| 节点组名 | `🇭🇰 Hong Kong` | `groups` 段定义 |
| 服务组名 | `🚀 快速选择` | `routing` 段互引用 |
| 链式组名 | `🔗 HK-ISP` | `relay_through` 派生 |
| 原始节点名 | `HK-01` / `HK-Snell` / `HK-VL` | `@all` 展开后的具体原始节点（订阅 / Snell / VLESS / 自定义） |
| 保留策略 | `DIRECT`、`REJECT` | 内置 |

说明：

- `@auto` 在 Route 阶段展开为节点组名、包含 `@all` 的服务组名、DIRECT 后写入 Members
- `@all` 在 Route 阶段展开为具体节点名后写入 Members
- 中间表示中不出现 `@all` 或 `@auto` 字面值
- Members 中的字符串统一在同一命名空间中解析（节点名、组名、保留字共享一个查找空间）
- 引用合法性由 ValidateGraph 阶段校验，ProxyGroup 自身不承担校验职责

约束：

- 节点组策略允许 `select` 或 `url-test`
- 服务组策略固定为 `select`

### Ruleset

表示一个服务组绑定的一组远程规则集 URL。

建议属性：

- `Policy`
- `URLs`

### Rule

表示一个用户声明的内联规则条目。

建议属性：

- `Raw`：原始字符串（如 `"GEOIP,CN,🎯 China"`）
- `Policy`：从原始字符串中提取的最后一个逗号后的策略名

设计意图：

- 采用透传方案。系统不解析规则的类型和值语义，只提取 `Policy` 用于引用校验
- 渲染时直接输出 `Raw` 原始字符串
- 理由：内联规则的语法（GEOIP、DOMAIN、IP-CIDR 等）在 Clash Meta 和 Surge 中一致，无需格式转换
- RULE-SET 条目不经过 Rule 实体——它们由渲染器根据 Ruleset 实体自动生成，格式差异在渲染层处理

### Pipeline

表示一次生成流程的完整中间结果。

建议聚合：

- 全部节点
- 全部节点组
- 全部服务组
- 全部规则集
- 全部内联规则
- fallback
- `@all` 展开结果

---

## 保序模型

`groups`、`routing`、`rulesets` 都有顺序语义，不能用普通 map 表达。

因此需要一个保序映射抽象：

- 保留 key 的书写顺序
- 支持按顺序遍历
- 支持按 key 查询

它的职责只是保存顺序与映射关系，不承载业务逻辑。

---

## 关键不变量

中间表示层需要维持这些不变量：

- 每个节点名称唯一
- 每个组名称唯一
- 节点组名和服务组名共享同一命名空间，不允许重名
- 链式节点只能引用拉取类节点（订阅 / Snell / VLESS）作为上游
- `@all` 只包含原始节点（订阅 / Snell / VLESS / 自定义）
- 节点组和服务组的名称都可被服务组引用
- fallback 必须引用已存在服务组

### 节点名称唯一性保证策略

节点名称是引用体系的 key，必须保证全局唯一。不同冲突场景的处理策略：

| 冲突场景 | 策略 | 原因 |
|---------|------|------|
| 拉取类节点跨源重名（订阅、Snell、VLESS 来源共享同一去重池） | 两轮去重：第一轮按出现顺序追加递增后缀（如 `HK-01`、`HK-01②`、`HK-01③`），第二轮检测并解决生成名与原始名的碰撞 | 用户无法控制订阅商/Snell/VLESS 清单命名，需覆盖后缀与原始名碰撞的极端场景 |
| 自定义代理与拉取类节点（订阅 / Snell / VLESS）重名 | 报错，错误消息指明冲突源 kind | 用户可修改自定义代理名称 |
| 自定义代理之间重名 | 报错 | 用户可修改 |

### 链式节点命名格式

链式节点名称格式固定为 `{upstream.Name}→{customProxy.Name}`，如 `HK-01→HK-ISP`。

该格式天然与普通节点名不冲突（包含 `→` 字符），无需额外去重。

---

## 模型边界

领域模型不负责：

- HTTP 请求与响应细节
- YAML 解析细节
- Clash Meta / Surge 的具体文本格式

领域模型负责：

- 在格式无关的语义层表达系统行为
- 作为管道阶段之间的唯一共享边界
