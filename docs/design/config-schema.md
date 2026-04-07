# 配置结构设计

## 目标

本文件定义用户 YAML 配置的正式结构、字段约束和配置语义。它只回答“用户如何声明系统行为”，不涉及 Go 代码实现。

---

## 顶层结构

```yaml
sources:
filters:
groups:
routing:
rulesets:
rules:
fallback:
```

各段职责：

- `sources`：声明订阅源和自定义代理
- `filters`：定义订阅节点过滤规则
- `groups`：声明地区节点组
- `routing`：声明服务组和出口优先级
- `rulesets`：将远程规则集 URL 绑定到服务组
- `rules`：声明内联规则
- `fallback`：定义最终兜底出口

---

## sources

```yaml
sources:
  subscriptions:
    - url: "https://sub.example.com/api/v1/client/subscribe?token=xxx"
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

### subscriptions

- `url` 必填
- 支持多个订阅源
- 每个订阅源返回 SS 节点列表

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
- `select`：用正则从订阅节点中选择上游
- `all`：使用全部订阅节点作为上游

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
- 仅作用于订阅节点
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
- 链式组不写在 `groups` 中，由 `relay_through` 派生
- 所有节点组都必须显式声明策略，不允许隐式默认值

---

## routing

```yaml
routing:
  🚀 快速选择: [🇭🇰 Hong Kong, 🇸🇬 Singapore, 🔗 HK-ISP, 🚀 手动切换, DIRECT]
  🚀 手动切换: ["@all"]
  📲 Telegram: [🇭🇰 Hong Kong, 🚀 快速选择, DIRECT]
  🐟 FINAL: [🚀 快速选择, DIRECT]
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

约束：

- key 为服务组名
- value 为有序列表
- 服务组策略固定为 `select`
- `@all` 展开为全部原始节点，不包含链式节点

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

## 顺序规则

以下段落的书写顺序直接影响输出：

- `groups`
- `routing`
- `rulesets`

系统必须保留此顺序，以保证客户端面板与配置书写顺序一致。
