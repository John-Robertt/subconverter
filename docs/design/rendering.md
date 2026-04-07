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

rule-provider 命名规则：

- 从 URL 路径中提取文件名并去掉扩展名，如 `https://example.com/Clash/Netflix.list` → `Netflix`
- 同名时追加递增序号，如 `Netflix`、`Netflix-2`
- provider 名称仅用于 Clash Meta 输出的内部引用，不影响语义

---

## Surge

映射要求：

- 若配置了 `base_url`，输出首行为 `#!MANAGED-CONFIG <base_url>/generate?format=surge interval=86400 strict=false`
- 节点输出到代理定义段
- 节点组和服务组输出到组定义段
- 远程规则集在规则中直接引用 URL
- fallback 输出为 `FINAL,<fallback>`

链式节点要求：

- 对链式节点输出 `underlying-proxy`

---

## 规则排列顺序

最终输出的规则按以下顺序排列：

1. `rulesets`（按 `rulesets` 段书写顺序，同一服务组的多条 URL 按声明顺序排列）
2. `rules`（按 `rules` 段书写顺序）
3. `MATCH` / `FINAL` 兜底规则（引用 `fallback` 指定的服务组）

理由：

- 规则集通常包含精确域名/IP 匹配，应优先命中
- 用户内联规则通常是宽泛规则（如 `GEOIP`），放在规则集之后
- 兜底规则天然是最后一条

---

## Surge Managed Profile

当用户配置了 `base_url` 时，Surge 渲染器在输出首行写入：

```
#!MANAGED-CONFIG <base_url>/generate?format=surge interval=86400 strict=false
```

此行告知 Surge 客户端配置的更新源地址和检查间隔。

参数说明：

- URL：由 `base_url` + `/generate?format=surge` 拼接
- `interval`：更新检查最小间隔，默认 86400 秒（24 小时）
- `strict`：是否强制过期更新，默认 false

当 `base_url` 为空时不输出此行。Clash Meta 输出不受影响。

---

## 一致性要求

无论输出 Clash Meta 还是 Surge，都必须保证：

- 同样的节点组顺序
- 同样的服务组顺序
- 同样的 ruleset 顺序
- 同样的规则排列顺序
- 同样的 fallback 语义
- 同样的 `@all` 展开结果

---

## 非目标

本层不负责：

- 验证规则集 URL 是否在线
- 修正客户端特有的运行时问题
- 为不同客户端增加行为差异补丁
