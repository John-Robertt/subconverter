# 管道设计

## 目标

本文件定义系统从配置输入到配置输出的阶段划分、输入输出和阶段职责。它聚焦数据流和边界，不展开代码实现细节。

---

## 总体流程

```text
LoadConfig
  -> ValidateConfig
  -> Source
  -> Filter
  -> Group
  -> Route
  -> ValidateGraph
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

## Stage 2: ValidateConfig

职责：

- 校验字段是否存在
- 校验字段值是否合法
- 校验互斥和依赖关系

典型检查：

- `relay_through.type=group` 时必须有 `name`
- `relay_through.type=select` 时必须有 `match`
- `relay_through.strategy` 必填
- 每个地区节点组都必须声明 `strategy`

输出：

- 结构合法的配置对象

---

## Stage 3: Source

职责：

- 拉取所有订阅源
- 解析 SS 节点
- 将自定义代理转换为原始节点对象

输入：

- `sources`

输出：

- 原始节点集合

说明：

- 本阶段不生成链式节点
- 规则集 URL 不在本阶段拉取
- 拉取失败时的错误信息不得包含完整订阅 URL（URL 中可能含有用户 token），应对 query 参数做脱敏处理
- 跨订阅重名通过两轮去重处理：第一轮追加递增后缀（②③...），第二轮检测并解决生成的后缀名与原始节点名的碰撞
- SS URI 按 SIP002 解析：支持 `ss://userinfo@host:port[/][?query][#tag]`
- `userinfo` 支持 base64/base64url 形式，也支持明文 `method:password`（必要时带 percent-encoding）
- 未识别的 query 参数直接忽略；`plugin` query 会保留到中间表示，交由渲染阶段按目标格式解释
- SS URI 端口值必须在 1-65535 范围内，超出范围视为无效 URI

---

## Stage 4: Filter

职责：

- 对订阅节点执行名称过滤

输入：

- 原始节点集合
- `filters`

输出：

- 过滤后的节点集合

说明：

- 只过滤订阅节点
- 自定义代理不参与过滤

---

## Stage 5: Group

职责：

- 根据 `groups` 生成地区节点组
- 根据 `relay_through` 生成链式节点和链式组
- 计算 `@all` 展开列表

输入：

- 过滤后的节点集合
- 地区节点组定义
- 自定义代理定义

输出：

- 全部节点
- 节点组集合
- `@all` 展开列表

子步骤执行顺序：

1. **构建地区节点组**：根据 `groups` 定义，用正则匹配过滤后的订阅节点，生成地区节点组
2. **构建链式节点和链式组**：根据 `relay_through` 定义，利用已构建的地区组（`type=group` 时）或正则匹配（`type=select` 时）确定上游节点，展开链式节点并生成链式组
3. **计算 `@all`**：收集全部原始节点（订阅节点 + 自定义节点），不把链式节点写入结果

顺序理由：步骤 2 依赖步骤 1 的产出（`relay_through.type=group` 需要引用已构建的地区组）。步骤 3 只要来源限定为原始节点，就可以天然排除链式节点；实现上可在链式生成前后计算，但结果语义必须一致。

关键规则：

- 链式组属于节点组
- 链式组策略取自 `relay_through.strategy`
- `@all` 只包含原始节点，不包含链式节点
- 原始节点包括订阅节点和自定义节点；自定义节点即使声明了 `relay_through`，也仍属于 `@all`
- 链式展开的上游只来自订阅节点

---

## Stage 6: Route

职责：

- 根据 `routing` 生成服务组
- 根据 `rulesets` 生成服务组绑定关系
- 解析 `rules`
- 记录 `fallback`

输入：

- `routing`
- `rulesets`
- `rules`
- `fallback`
- `@all` 展开列表

输出：

- 服务组集合
- 规则集集合
- 内联规则集合
- fallback

说明：

- 服务组统一为 `select`
- `@all` 在本阶段展开

---

## Stage 7: ValidateGraph

职责：

- 检查引用关系是否闭合
- 检查服务组之间是否存在循环引用
- 检查链式组展开结果是否为空
- 检查 fallback 和 ruleset 绑定是否有效

输入：

- 全部节点
- 节点组
- 服务组
- ruleset
- fallback

输出：

- 可渲染的完整中间表示

---

## Stage 8: Render

职责：

- 将统一中间表示映射到目标客户端格式

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
