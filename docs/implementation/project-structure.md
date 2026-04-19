# 项目结构建议

## 目标

本文件定义代码目录和包职责，作为实现阶段的结构约束。

---

## 推荐目录

```text
subconverter/
├── cmd/subconverter/
│   └── main.go
├── internal/
│   ├── config/
│   ├── errtype/
│   ├── fetch/
│   ├── generate/
│   ├── model/
│   ├── pipeline/
│   ├── proxyparse/
│   ├── render/
│   ├── ssparse/
│   ├── server/
│   └── target/
├── configs/
│   ├── base_config.yaml
│   ├── base_clash.yaml
│   └── base_surge.conf
├── testdata/
└── docs/
```

---

## 包职责

`internal/config`

- 配置结构（含 `Templates` 底版模板声明）
- YAML 加载（支持本地路径和远程 URL）
- 保序映射
- 静态校验与启动期预计算（`Prepare` 产出不可变 `RuntimeConfig`，含编译后正则、解析后自定义代理、展开后路由成员、静态命名空间）
- 原始配置层（`Config`）不持有运行期派生的代理字段；预计算层（`RuntimeConfig`）存储启动期产物

`internal/errtype`

- 五类错误定义（ConfigError、FetchError、ResourceError、BuildError、RenderError）
- 横跨所有业务包的共享错误类型；`BuildError` / `RenderError` 等可通过 `Cause` 保留内层根因链

`internal/model`

- 统一中间表示

`internal/fetch`

- 远程来源拉取
- TTL 缓存
- 统一资源加载（`LoadResource`：按 URL 前缀分发本地/远程）

`internal/pipeline`

- SS / Snell / VLESS 来源解析、各阶段转换
- 图级校验
- `Build` 编排（格式无关 IR）

`internal/proxyparse`

- 解析 `custom_proxies[].url`
- 返回运行期中立结构（type/server/port/params/plugin）
- 隔离 `config` 与 `model`

`internal/render`

- Clash Meta / Surge 序列化
- 底版模板合并
- 不承担协议级裁剪与图改写

`internal/target`

- Clash / Surge 目标格式投影
- 协议支持过滤（Snell / VLESS）
- fallback 清空等格式相关级联校验

`internal/generate`

- 单一“生成配置”应用服务
- 统一承接 `Build -> Target -> Render`
- 装配模板加载与 Surge managed URL

`internal/server`

- HTTP handler
- 参数校验
- 错误映射

`internal/ssparse`

- Shadowsocks URI 解析（SIP002 body 解析、plugin query 解析）
- 被 `proxyparse`（自定义代理 URL 解析）和 `pipeline`（SS 订阅 URI 解析）共享

---

## 依赖方向

```text
cmd/subconverter
  -> config
  -> fetch
  -> generate
  -> server

server
  -> generate
  -> errtype

generate
  -> config
  -> fetch
  -> pipeline
  -> target
  -> render

pipeline
  -> config
  -> fetch
  -> model
  -> errtype
  -> proxyparse
  -> ssparse

render
  -> model
  -> errtype

config
  -> errtype
  -> fetch
  -> proxyparse

target
  -> model
  -> errtype

proxyparse
  -> ssparse

fetch
  -> errtype

ssparse
  -> (leaf)
```

约束：

- `model` 不依赖其他业务包
- `errtype` 不依赖其他业务包
- `render` 不直接读取 YAML 配置
- `server` 不承担业务转换逻辑
- `config` 不直接依赖 `model`
