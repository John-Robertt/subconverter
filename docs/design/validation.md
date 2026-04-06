# 校验设计

## 目标

本文件定义配置校验与引用校验的边界。校验分为两类：字段级静态校验和图级语义校验。

---

## 静态校验

静态校验发生在配置加载后、管道执行前。

检查范围：

- 必填字段是否存在
- 枚举值是否合法
- 条件字段是否成对出现
- 正则表达式是否可编译
- URL 是否满足基本格式要求

重点规则：

- `subscriptions[].url` 必填
- `custom_proxies[].name/type/server/port` 必填
- `groups[*].match` 必填
- `groups[*].strategy` 必填且只能是 `select` 或 `url-test`
- `relay_through.type` 必填
- `relay_through.strategy` 必填且只能是 `select` 或 `url-test`
- `relay_through.type=group` 时必须提供 `name`
- `relay_through.type=select` 时必须提供 `match`
- `fallback` 必填

---

## 图级校验

图级校验发生在 Group 和 Route 阶段之后。

检查范围：

- 名称引用是否存在
- 规则集绑定是否存在目标服务组
- fallback 是否引用有效服务组
- 服务组之间是否存在循环引用
- 链式展开后结果是否为空
- `@all` 是否正确排除了链式节点

重点规则：

- `routing` 中引用的节点组、服务组、保留字都必须可解析
- `rulesets` 的 key 必须存在于 `routing`
- `relay_through.type=group` 引用的节点组必须存在
- 自动生成的链式组必须至少包含一个成员

---

## 错误分层

推荐按来源区分错误：

- 配置错误
- 远程拉取错误
- 内部构建错误
- 渲染错误

目标：

- 让错误能明确定位到配置、网络还是内部逻辑
- 保持 HTTP 层错误码映射简单稳定
