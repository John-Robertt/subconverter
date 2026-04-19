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
│   ├── model/
│   ├── pipeline/
│   ├── render/
│   └── server/
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
- 静态校验

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
- 管道编排

`internal/render`

- Clash Meta 渲染器（yaml.Node API + 底版模板合并 + Snell 级联过滤 + VLESS 渲染）
- Surge 渲染器（INI section 切分替换 + 底版模板合并 + VLESS 级联过滤）

`internal/server`

- HTTP handler
- 参数校验
- 错误映射

`internal/ssparse`

- Shadowsocks URI 解析（SIP002 body 解析、plugin query 解析）
- 被 `config`（自定义代理 URL 解析）和 `pipeline`（SS 订阅 URI 解析）共享

---

## 依赖方向

```text
cmd/subconverter
  -> server
  -> config
  -> fetch

server
  -> config
  -> fetch
  -> pipeline
  -> render
  -> errtype

pipeline
  -> config
  -> fetch
  -> model
  -> errtype
  -> ssparse

render
  -> model
  -> errtype

config
  -> errtype
  -> fetch
  -> model
  -> ssparse

fetch
  -> errtype

ssparse
  -> model
```

约束：

- `model` 不依赖其他业务包
- `errtype` 不依赖其他业务包
- `render` 不直接读取 YAML 配置
- `server` 不承担业务转换逻辑
