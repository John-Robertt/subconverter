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

说明：

- Clash Meta 的规则集引用采用 provider 名称，而不是直接在规则中内联完整 URL

---

## Surge

映射要求：

- 节点输出到代理定义段
- 节点组和服务组输出到组定义段
- 远程规则集在规则中直接引用 URL
- fallback 输出为 `FINAL,<fallback>`

链式节点要求：

- 对链式节点输出 `underlying-proxy`

---

## 一致性要求

无论输出 Clash Meta 还是 Surge，都必须保证：

- 同样的节点组顺序
- 同样的服务组顺序
- 同样的 ruleset 顺序
- 同样的 fallback 语义
- 同样的 `@all` 展开结果

---

## 非目标

本层不负责：

- 验证规则集 URL 是否在线
- 修正客户端特有的运行时问题
- 为不同客户端增加行为差异补丁
