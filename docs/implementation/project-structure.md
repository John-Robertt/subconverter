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
│   ├── fetch/
│   ├── model/
│   ├── pipeline/
│   ├── render/
│   └── server/
├── configs/
│   └── example.yaml
└── docs/
```

---

## 包职责

`internal/config`

- 配置结构
- YAML 加载
- 保序映射
- 静态校验

`internal/model`

- 统一中间表示

`internal/fetch`

- 订阅拉取
- TTL 缓存

`internal/pipeline`

- 各阶段转换
- 图级校验
- 管道编排

`internal/render`

- Clash Meta 渲染器
- Surge 渲染器

`internal/server`

- HTTP handler
- 参数校验
- 错误映射

---

## 依赖方向

```text
cmd/subconverter
  -> server
  -> config
  -> fetch

server
  -> pipeline
  -> render

pipeline
  -> config
  -> fetch
  -> model

render
  -> model
```

约束：

- `model` 不依赖其他业务包
- `render` 不直接读取 YAML 配置
- `server` 不承担业务转换逻辑
