# 管道设计

## 目标

本文件定义系统从配置输入到配置输出的阶段划分、输入输出和阶段职责。它聚焦数据流和边界，不展开代码实现细节。

---

## 总体流程

```text
启动期:
LoadConfig
  -> Prepare (produces RuntimeConfig)

请求期:
Build(Source -> Filter -> Group -> Route -> ValidateGraph)
  -> Target
  -> Render
```

设计原则：

- 每个阶段尽量只负责一类转换
- 阶段之间通过稳定的中间表示传递数据
- I/O 与纯转换逻辑尽量分离

---

## Stage 1: LoadConfig

职责：

- 读取 YAML 文件
- 反序列化为用户配置对象
- 保留 `groups`、`routing`、`rulesets` 的书写顺序

输出：

- 配置对象

---

## Stage 2: Prepare

职责：

- 校验字段是否存在、值是否合法、互斥和依赖关系
- 编译正则表达式（`groups[*].match`、`filters.exclude`、`relay_through.match`）
- 解析自定义代理 URL（`custom_proxies[].url`），校验 SS 参数
- 展开 `@auto`（存入 `ExpandedMembers`），`@all` 保留到请求期 Route 阶段展开
- 构建静态命名空间（`StaticNamespace`）：注册 DIRECT/REJECT、节点组名、服务组名、自定义代理名、链式组名，检测跨类别命名冲突
- 检测路由环路
- 校验 ruleset/rule 策略引用合法性、fallback 存在性

典型检查：

- `relay_through.type=group` 时必须有 `name`
- `relay_through.type=select` 时必须有 `match`
- `relay_through.strategy` 必填
- 每个地区节点组都必须声明 `strategy`
- `routing` 成员必须引用节点组、服务组、DIRECT、REJECT、`@all` 或 `@auto`
- `rulesets` 的 key 必须存在于 `routing`
- `fallback` 必须引用已定义服务组

输出：

- 启动期准备好的 `RuntimeConfig`（含编译后的正则、解析后的自定义代理、展开后的路由成员、静态命名空间）；请求期按只读契约消费，不强制 accessor 深拷贝

---

## Stage 3: Source

职责：

- 拉取所有远程来源
- 解析 SS 节点、Snell 节点、VLESS 节点
- 将自定义代理转换为原始节点对象或链式模板

输入：

- `sources`

输出：

- `SourceResult`
- 其中包含原始节点集合与供 Group 阶段消费的链式模板

说明：

- 本阶段不生成链式节点
- 规则集 URL 不在本阶段拉取
- 拉取失败时的错误信息不得包含完整来源 URL（URL 中可能含有用户 token），应对 query 参数做脱敏处理
- 跨订阅重名通过两轮去重处理：第一轮追加递增后缀（②③...），第二轮检测并解决生成的后缀名与原始节点名的碰撞
- SS URI 按 SIP002 解析：支持 `ss://userinfo@host:port[/][?query][#tag]`
- `userinfo` 支持 base64/base64url 形式，也支持明文 `method:password`（必要时带 percent-encoding）
- 未识别的 query 参数直接忽略；`plugin` query 会保留到中间表示，交由渲染阶段按目标格式解释
- SS URI 端口值必须在 1-65535 范围内，超出范围视为无效 URI

Snell 来源解析：

- URL 返回纯文本（非 base64），按行解析为 Surge 风格的 Snell 节点声明
- 格式：`NAME = snell, SERVER, PORT, KEY=VALUE[, KEY=VALUE ...]`；KEY 两侧允许 `\s*=\s*` 空白
- 节点 `Type="snell"`、`Kind=KindSnell`；所有 KEY=VALUE 保存至 `Params`
- 空行与 `#` / `//` 注释行跳过；**单行解析失败整源报错**（与 SS 订阅的静默跳过不同——Snell 来源规模小，严格报错更利于发现拼写问题）
- 报错消息包含脱敏后的来源 URL 与 1-based 物理行号；外层 `BuildError` 负责提供来源上下文，内层解析错误通过 `Cause` 保留根因链
- Snell 节点与订阅节点共享同一跨源去重池（同名节点按出现顺序追加 ②③... 后缀）
- 与订阅一起在 Filter 阶段参与 `filters.exclude` 过滤，在 Group 阶段参与区域组 regex 匹配，可作为 `relay_through` 的链式上游

VLESS 来源解析：

- URL 返回纯文本（非 base64），每行一条标准 VLESS URI：`vless://UUID@SERVER:PORT[?query][#NAME]`
- 节点 `Type="vless"`、`Kind=KindVLess`；URI query 按 Clash 目标命名写入 `Params`（如 `sni→servername`、`fp→client-fingerprint`、`pbk→reality-public-key`），渲染器无需命名映射
- `type`（传输层）按 Mihomo 兼容语义归一化：已知值 `tcp`/`ws`/`http`/`h2`/`grpc`/`xhttp` 原样保留，缺失或未知值回落到 `tcp`
- `security` 只接受 `none`/`tls`/`reality`；`encryption` 非空时透传并保留到 `Params`
- 空行与 `#` / `//` 注释行跳过；单行解析失败整源报错（策略与 Snell 对齐）
- 报错消息与 Snell 等价：脱敏后的来源 URL + 1-based 物理行号，外层 `BuildError` 通过 `Cause` 保留内层根因
- VLESS 节点与订阅节点、Snell 节点共享同一跨源去重池
- 在 Filter / Group / ValidateGraph 中的行为与订阅节点一致；可作为 `relay_through` 链式上游
- 当前 URI 契约不扩展到 `packet-encoding`、`support-x25519mlkem768` 等未显式定义 query 映射的 Mihomo 字段

来源遍历顺序：

- `sources` 下的 `subscriptions` / `snell` / `vless` 三类 key 按 YAML 书写顺序记录到 `Sources.FetchOrder`，Source 阶段按此顺序调度拉取
- 例如 YAML 中声明顺序为 `snell → vless → subscriptions`，则最终 proxy slice 的相对顺序也是 snell 节点 → vless 节点 → 订阅节点
- 未在 YAML 中声明的 key 不进入 FetchOrder；当 FetchOrder 为空（in-memory 构造的测试 Config）时回退到默认顺序 `subscriptions → snell → vless`

---

## Stage 4: Filter

职责：

- 对拉取类节点执行名称过滤

输入：

- 原始节点集合
- `filters`

输出：

- 过滤后的节点集合

说明：

- 过滤对象是拉取类节点（`KindSubscription` + `KindSnell` + `KindVLess`）
- 自定义代理和链式节点不参与过滤

---

## Stage 5: Group

职责：

- 根据 `groups` 生成地区节点组
- 根据 `relay_through` 生成链式节点和链式组
- 计算 `@all` 展开列表

输入：

- `SourceResult`（其中 `Proxies` 已经过 Filter 阶段处理）
- 地区节点组定义

输出：

- 全部节点
- 节点组集合
- `@all` 展开列表

子步骤执行顺序：

1. **构建地区节点组**：根据 `groups` 定义，用正则匹配过滤后的拉取类节点（订阅 + Snell + VLESS），生成地区节点组
2. **构建链式节点和链式组**：根据 `relay_through` 定义，利用已构建的地区组（`type=group` 时）或正则匹配（`type=select` 时）确定上游节点，展开链式节点并生成链式组
3. **计算 `@all`**：收集全部原始节点（订阅节点 + Snell 节点 + VLESS 节点 + **不带 `relay_through` 的自定义节点**），不把链式节点写入结果

顺序理由：步骤 2 依赖步骤 1 的产出（`relay_through.type=group` 需要引用已构建的地区组）。步骤 3 只要来源限定为原始节点，就可以天然排除链式节点；实现上可在链式生成前后计算，但结果语义必须一致。

关键规则：

- 链式组属于节点组
- 链式组策略取自 `relay_through.strategy`
- `@all` 只包含原始节点，不包含链式节点
- 原始节点包括订阅节点、Snell 节点、VLESS 节点和**不带 `relay_through`** 的自定义节点；带 `relay_through` 的自定义代理仅作链式模板（不产生 `KindCustom`），因而也不进入 `@all`
- 链式展开的上游只来自拉取类节点（订阅 + Snell + VLESS）

---

## Stage 6: Route

职责：

- 根据 `routing` 生成服务组
- 展开 `@auto` 和 `@all` token
- 根据 `rulesets` 生成服务组绑定关系
- 解析 `rules`
- 记录 `fallback`

输入：

- `routing`
- `rulesets`
- `rules`
- `fallback`
- `GroupResult`（含 `@all` 展开列表和节点组列表）

输出：

- 服务组集合
- 规则集集合
- 内联规则集合
- fallback

说明：

- 服务组统一为 `select`
- `@auto` 在启动期 Prepare 阶段已展开（存入 `PreparedRouteGroup.ExpandedMembers`）；Route 阶段仅展开 `@all` 为具体原始节点名
- 两种 token 互斥，不可在同一 entry 中同时使用

---

## Stage 7: ValidateGraph

职责：

- 检查共享命名空间冲突和重复声明
- 检查 `@all` 展开排除链式节点
- 检查空节点组（地区组和链式组）
- 检查路由成员引用合法性（区分原始声明 vs 展开后的成员溯源）
- 检查服务组之间是否存在循环引用

说明：ruleset/rule 策略存在性和 fallback 存在性由启动期 Prepare 保证，ValidateGraph 不再重复检查。

输入：

- 全部节点
- 节点组
- 服务组

输出：

- 可渲染的完整中间表示

---

## Stage 8: Target

职责：

- 将格式无关 IR 投影为目标格式视图
- 做目标格式协议能力过滤
- 做目标格式特有的级联校验

输入：

- `Pipeline`
- 目标格式（Clash / Surge）

输出：

- 已投影的目标格式视图

说明：

- Clash 视图会剔除 Snell 节点及其级联影响
- Surge 视图会剔除 VLESS 节点及其级联影响
- 若 `fallback` 在目标格式视图中被清空，应在本阶段返回错误，而不是等到 Render

---

## Stage 9: Render

职责：

- 将已投影的目标格式视图序列化为目标客户端格式

输出目标：

- Clash Meta YAML
- Surge conf

要求：

- 两种输出的面板结构和路由语义保持一致
- 差异仅体现在目标语法
- Clash Meta 对 SS plugin 采用通用透传：输出 `plugin` 与 `plugin-opts`
- Surge 仅支持可映射的 SS obfs 类 plugin；不支持的 plugin 或 option 在本阶段返回渲染错误，而不是静默降级

---

## 阶段划分理由

这样拆分的原因：

- 便于把“获取数据”“重塑结构”“检查引用”“输出文本”四类问题隔离
- 便于单测直接覆盖单个阶段
- 便于后续扩展新的输出格式，而不需要回改前面阶段的核心逻辑
