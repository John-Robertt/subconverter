# 配置 Schema 与 DTO 契约

> 状态：v3.0 目标契约。本文定义 `Config` 的稳定 JSON 形状。

## 总体原则

- `Config` 是 API、Web 草稿、保存请求和 Prepare 的共同语义模型。
- 字段名使用稳定 snake_case。
- 保序集合统一表达为数组。
- DTO 不暴露文件格式、存储路径、AST 或 Go 内部结构体名。
- 新增来源、协议或目标格式时，schema 必须与 CapabilityRegistry 同步。
- 本文 DTO 字段必须与 `core-model.md` 中的 `Config` 逐字段对应。

## 顶层形状

```json
{
  "sources": {
    "fetch_order": ["subscriptions", "snell", "vless"],
    "subscriptions": [
      { "id": "main", "url": "https://example.com/sub?token=redacted" }
    ],
    "snell": [],
    "vless": []
  },
  "filters": {
    "exclude": ["剩余流量|套餐到期"]
  },
  "groups": [
    {
      "name": "HK",
      "value": {
        "match": "HK|Hong Kong",
        "strategy": "url-test"
      }
    }
  ],
  "custom_proxies": [
    {
      "name": "Local SOCKS",
      "url": "socks5://127.0.0.1:1080"
    }
  ],
  "routing": [
    {
      "name": "FINAL",
      "value": {
        "members": ["HK", "DIRECT"],
        "fallback": "HK"
      }
    }
  ],
  "rulesets": [
    {
      "name": "streaming",
      "value": {
        "policy": "FINAL",
        "urls": ["https://example.com/streaming.list"]
      }
    }
  ],
  "rules": [
    "DOMAIN-SUFFIX,example.com,FINAL"
  ],
  "fallback": "FINAL",
  "templates": {
    "clash": "templates/clash.yaml",
    "surge": "templates/surge.conf"
  },
  "base_url": "https://sub.example.com"
}
```

固定形状：

- 已知顶层字段必须显式出现。
- 空数组使用 `[]`。
- 空对象使用 `{}`。
- 可选字符串为空时使用 `""`。
- `sources` 必须包含每个已注册 fetch-kind source key。

## 保序集合

以下字段必须使用数组：

| 字段 | 元素形状 | 顺序语义 |
|------|----------|----------|
| `groups` | `{ "name": string, "value": Group }` | 节点组构建顺序 |
| `custom_proxies` | `CustomProxy` | 自定义节点和链式模板声明顺序 |
| `routing` | `{ "name": string, "value": RouteGroup }` | 服务组解析顺序 |
| `rulesets` | `{ "name": string, "value": Ruleset }` | 规则集渲染顺序 |
| `rules` | `string` | inline rule 渲染顺序 |

规则：

- `name` 是用户定义名。
- `value` 是该条目的配置对象。
- JSON Pointer 使用数组索引定位。对 API 请求和响应中的 Config，统一以 `/config` 为根，例如 `/config/groups/0/value/match`。
- name 含点号、空格或 emoji 时不得被拆成路径段。

## sources

```json
{
  "fetch_order": ["subscriptions", "snell", "vless"],
  "subscriptions": [],
  "snell": [],
  "vless": []
}
```

规则：

- `fetch_order` 必须且只能包含 CapabilityRegistry 中 `fetch_kind=true` 的 source key。
- 每个 fetch-kind source key 必须恰好出现一次。
- `sources` 必须显式包含每个 fetch-kind source key，未配置时为空数组。
- `custom_proxies` 不进入 `fetch_order`。

## custom_proxies

`custom_proxies` 使用数组保持声明顺序。

规则：

- 不带 `relay_through` 的条目产生 `custom` 原始节点。
- 带 `relay_through` 的条目只作为链式模板，不产生普通 custom 节点。
- `relay_through` 可以选择单个上游或 all 上游。
- `name` 参与全局命名空间检查。

## templates 与 base_url

- `templates.clash` 和 `templates.surge` 是模板引用。
- `base_url` 用于生成订阅链接和目标格式需要的 managed header。
- RuntimeSnapshot 记录进入生效态的模板引用。

## rulesets 与 rules

- `rulesets[].value.policy` 必须引用节点组或服务组。
- `rulesets[].value.urls` 按声明顺序渲染为目标格式的规则集资源。
- `rules[]` 保存用户声明的原始 inline rule 字符串；Prepare / Build 负责解析其中的 policy 并生成核心 `Rule`。
- Target Projection 可因 policy 被过滤而级联移除 ruleset 或 rule。

## Locator

诊断定位以 Config DTO 为准：

```text
/config/groups/0/value/match
/config/routing/2/value/members/1
/config/rulesets/0/value/urls/0
/config/sources/subscriptions/0/url
/config/sources/fetch_order/1
```

`display_path` 只用于用户阅读，不作为程序定位依据。

## 测试要求

- Config JSON round-trip 后保序集合顺序不变。
- `sources.fetch_order` 的未知项、重复项、遗漏项均产生诊断。
- 特殊字符 name 通过数组索引定位。
- API 示例必须与本文 DTO 形状一致。
