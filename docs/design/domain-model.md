# 领域模型设计

> 状态提示：§前端状态模型为 v2.0 新增；请求期中间表示已采用 ID 图，并在 Target 阶段投影为渲染专用模型。

## 目标

本文件定义系统内部的数据视图，描述配置、节点、组和规则在系统中的抽象方式。实现目标是让解析、分组、渲染共享同一套稳定语义，而不是各自直接操作 YAML 文本。

---

## 五层模型

系统使用五层模型：

- 用户配置层（`Config`）：直接映射 YAML 结构
- 预计算层（`RuntimeConfig`）：启动期由 `Prepare` 产出，含编译后正则、解析后自定义代理、展开后路由成员
- 中间表示层（`Pipeline`）：使用 `ProxyID` 和类型化引用表达格式无关的节点、组和规则关系
- 目标表示层（`TargetPipeline`）：完成格式过滤和目标内唯一命名，成员转换为最终输出名
- 输出文本层：面向 Clash Meta 或 Surge 的最终序列化结果

依赖关系：

```text
用户配置层 -> 预计算层 -> ID 中间表示层 -> 目标表示层 -> 输出文本层
```

`Pipeline` 是格式无关 IR 的稳定边界；`TargetPipeline` 是 Target 与 Render 的接口边界；预计算层是启动期与请求期的边界。

补充边界：

- 用户配置层中的 `custom_proxies` 只保留原始 YAML 字段（`name` / `url` / `relay_through`）
- `url` 解析得到的 `type/server/port/params/plugin` 在 `Prepare` 阶段存入 `PreparedCustomProxy.Parsed`，不回写到 `Config`
- `ChainTemplate` 是 Group 阶段使用的运行期结构，不属于稳定领域模型
- `PreparedRouteMember` 携带 `RouteMemberOrigin` 枚举（`Literal` / `AutoExpanded` / `AllExpanded`），用于 ValidateGraph 区分用户声明 vs 机器展开的成员
- `ProxyID` 在节点创建阶段分配：Source 为原始节点分配，Group 为链式节点分配；ID 在本次 Build 内不复用且保持稳定，不承诺跨配置 revision 持久化
- 节点显示名不进入图引用命名空间；节点组、服务组、`DIRECT`、`REJECT` 仍共享策略命名空间

---

## 核心实体

### Proxy

表示一个可被客户端引用的节点。

节点类型分为（由 `Kind` 字段区分）：

| Kind | 来源 | 示例 `Type` | 参与 region regex 匹配 | 可作为链式上游 | 进入 `@all` |
|------|------|-----------|---------------------|----------------|-------------|
| `KindSubscription` | 订阅拉取（TXT / Clash YAML / Surge CONF） | TXT: `ss`, `anytls`；结构化格式可为任意原生类型 | ✓ | ✓ | ✓ |
| `KindSnell` | Snell 来源拉取（Surge 专属） | `snell` | ✓ | ✓ | ✓ |
| `KindVLess` | VLESS 来源拉取（Clash 专属） | `vless` | ✓ | ✓ | ✓ |
| `KindCustom` | 用户手工声明，且**未**设 `relay_through` | `ss`, `socks5`, `http` | ✗（名字已显式） | ✗ | ✓ |
| `KindChained` | `custom_proxy` 带 `relay_through` 派生 | 继承自 custom_proxy | ✗ | ✗ | ✗ |

> 带 `relay_through` 的 custom_proxy 本身不产生 `KindCustom` 节点——它仅作为链式模板被 Group 阶段消费，生成 `KindChained` 节点和一个同名链式组。用户若需"直连 + 中继"两种入口，声明两条不同 `name` 的 custom_proxies。

建议属性：

- `ID`
- `Name`
- `Type`
- `Server`
- `Port`
- `Params`
- `Plugin`
- `Kind`
- `DialerID`
- `ChainNameSuffix`
- `Native`（可选）

设计意图：

- 用 `Kind` 区分节点来源与行为，而不是只用一个布尔值标记是否链式
- `ID` 是节点在一次 Build 内的唯一身份；`Name` 只是用户可见的候选显示名
- `DialerID` 仅在链式节点上生效，用于按身份引用其上游；`ChainNameSuffix` 保存链式模板名
- `Params` 为 `map[string]string`，承载代理类型的核心参数（如 SS 的 `cipher/password`、AnyTLS 的 `password/sni`、socks5/http 的 `username/password`）
- `Plugin` 为可选结构，承载 SS plugin 的名称与参数，避免把 plugin 语义混入 `Params` 的字符串 key 约定中
- 渲染器按目标客户端能力解释 `Plugin`：Clash Meta 可通用透传，Surge 仅支持可映射的子集
- `NativeProxy` 只用于已采用目标原生语法的结构化订阅节点：`Format` 为 `clash` 或 `surge`，`Payload` 保存一个 Clash 节点映射或一个 Surge `[Proxy]` 声明右值
- 带 `NativeProxy` 的节点仍是 `KindSubscription`，因此共享过滤、地区匹配、ID 引用、链式上游和 `@all` 语义；其 `server` / `port` 只作可选预览信息，不作为模型必填项
- 目标格式投影只保留 `NativeProxy.Format` 与目标一致的节点；渲染时由目标内分配的 `TargetProxy.Name` 覆盖原生声明中的旧名称

### NodeGroup

表示只包含代理节点的地区组或链式组。

建议属性：

- `Name`
- `Strategy`
- `Members []ProxyID`

`Members` 只保存代理 ID，因此多个同名节点仍可同时进入同一节点组，过滤和级联删除也不会误伤其他同名节点。

### RouteGroup

表示 Telegram、Netflix、FINAL 等服务级策略组。

建议属性：

- `Name`
- `Strategy`
- `Members []PolicyRef`

`PolicyRef` 是带类型的引用：

| 成员种类 | 示例 | 来源 |
|---------|------|------|
| `PolicyRefNodeGroup` | `🇭🇰 Hong Kong` / `HK-ISP` | `groups` 或 `relay_through` 派生组名 |
| `PolicyRefRouteGroup` | `🚀 快速选择` | `routing` 段互引用 |
| `PolicyRefProxy` | `ProxyID(12)` | `@all` 展开后的具体原始节点身份 |
| `PolicyRefReserved` | `DIRECT`、`REJECT` | 内置策略 |

说明：

- `@auto` 在启动期 Prepare 阶段展开（存入 `PreparedRouteGroup.ExpandedMembers`）；Route 阶段仅展开 `@all` 为具体代理 ID
- `@all` 在 Route 阶段展开为 `PolicyRefProxy`
- 中间表示中不出现 `@all` 或 `@auto` 字面值
- 用户配置中的字符串只解析为组名、保留策略或 token；不允许直接引用节点显示名
- 引用合法性由 ValidateGraph 阶段校验，`RouteGroup` 自身不承担校验职责

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
- 全部 `NodeGroup`
- 全部 `RouteGroup`
- 全部规则集
- 全部内联规则
- fallback
- `@all` 展开得到的 `[]ProxyID`

### TargetPipeline

表示单个目标格式完成能力过滤与命名投影后的渲染输入：

- `TargetProxy.Name` 是最终输出名，链式节点的 `Dialer` 也是最终输出名
- `TargetGroup.Members` 已从 ID / 类型化引用转换为最终字符串
- 渲染器不再观察内部 ID，也不负责名称消歧

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

- 每个节点 ID 非零且唯一
- 每个组名称唯一
- 节点组名和服务组名共享同一命名空间，不允许重名
- 节点显示名允许重复，不参与组引用命名空间
- 链式节点只能引用拉取类节点（订阅 / Snell / VLESS）作为上游
- `@all` 只包含原始节点（订阅 / Snell / VLESS / 不带 `relay_through` 的自定义）
- 节点组和服务组的名称都可被服务组引用
- fallback 必须引用已存在服务组

### 目标输出名称唯一性保证策略

节点名不是引用体系的 key。Source 保留原始节点候选名并分配独立 ID，Group 为派生链式节点分配独立 ID；Clash 和 Surge 分别在格式专属节点过滤完成后分配唯一输出名。

| 冲突场景 | 策略 | 原因 |
|---------|------|------|
| 同一目标内多个节点候选名相同 | 按目标内保序顺序使用原名、`②`…`⑩`、`(11)`… 后缀 | 满足客户端名称唯一约束，同时保持确定性 |
| 同名节点分别只支持 Clash / Surge | 两个目标都保留原始名 | 名称分配发生在格式过滤之后，互不占用名额 |
| 节点候选名与节点组、服务组、`DIRECT`、`REJECT` 冲突 | 保留策略/组名，节点追加后缀 | 策略引用仍以名称工作，节点身份不需要抢占该命名空间 |
| 候选名本身含 `②` 等自然后缀 | 统一检查已占用最终名并继续递增 | 保证最终输出仍唯一 |

### 链式节点命名格式

链式节点先通过 `DialerID` 找到上游在当前目标中的最终输出名，再生成 `{dialerOutputName}→{ChainNameSuffix}`，如 `HK-01②→HK-ISP`。链式节点随后进入同一个目标名称分配器，仍会处理与其他节点或组名的冲突。

---

## 模型边界

领域模型不负责：

- HTTP 请求与响应细节
- YAML 解析细节
- Clash Meta / Surge 的具体文本格式

领域模型负责：

- 在格式无关的语义层表达系统行为
- 作为管道阶段之间的唯一共享边界

---

## 前端状态模型（v2.0）

Web 管理后台不维护独立的领域模型：

- 前端直接消费 `GET /api/config` 返回的 JSON（与 `Config` 结构体对应）
- `GET /api/config` 返回 `{config_revision, config}`；前端编辑 `config`，保存时携带原 `config_revision` 做条件写回
- 前端直接消费 `/api/preview/*` 返回的 JSON；节点和代理成员包含本次 Build 内的 `proxy-N` 调试 ID，显示名可重复
- 前端不接触 `Pipeline`、`RuntimeConfig` 等内部中间表示
- 前端编辑操作 = 修改 Config JSON → POST 草稿预览 / 校验 → `PUT /api/config`（仅本地可写配置源可用；远程配置源为只读）
- 保序字段在前端以数组形式操作，拖拽排序直接改变数组索引
