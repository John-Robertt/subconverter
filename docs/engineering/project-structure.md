# 目标项目结构

> 状态：v3.0 目标结构。本文定义新架构下的 Go 包边界。

## 目录

```text
internal/core/          产品核心模型、诊断、能力矩阵、不变量
internal/service/       Workspace / Runtime / Preview / Artifact 用例
internal/engine/
  prepare/              Config -> PreparedConfig
  build/                PreparedConfig -> Pipeline
  project/              Pipeline -> TargetView
  render/               TargetView -> Artifact
internal/adapter/
  http/                 API handler、认证中间件、响应编码
  configstore/          ConfigStore、ConfigCodec、revision、原子写入
  resource/             远程资源读取、缓存、URL 脱敏
  auth/                 管理后台会话和凭据
  webui/                SPA 静态资源
cmd/subconverter/       进程入口、依赖装配、启动参数
```

## 依赖方向

```text
cmd/subconverter
  -> adapter/http
  -> service
  -> engine
  -> core

adapter/* -> service/core
service   -> engine/core/adapter interfaces
engine    -> core/adapter interfaces
core      -> 标准库
```

禁止方向：

- `core` 不导入 `service`、`engine`、`adapter`。
- `engine/render` 不读取 ConfigStore。
- HTTP handler 不直接调用 Build、Project 或 Render。
- UI DTO 不引用 engine 内部结构。

## core

`internal/core` 包含：

- `Config`、`PreparedConfig`。
- `RuntimeSnapshot`。
- `Pipeline`、`Proxy`、`ProxyGroup`、`Ruleset`、`Rule`。
- `TargetView`。
- `Diagnostic`、`DiagnosticBundle`。
- `CapabilityRegistry`。
- 命名、引用、fallback、`@all` 等不变量函数。

`core` 只表达产品语义，不处理 I/O。

## service

`internal/service` 包含：

- `WorkspaceService`。
- `RuntimeService`。
- `PreviewService`。
- `ArtifactService`。

服务层承接用户动作，组合 engine 和 adapter interface。服务层不定义协议支持矩阵，不写渲染规则。

## engine

`internal/engine` 包含纯转换：

- `prepare`：静态校验和预计算。
- `build`：来源拉取、过滤、分组、路由、格式无关图校验。
- `project`：目标格式能力过滤和 TargetView 构造。
- `render`：目标格式序列化和模板合并。

engine 可以依赖 core 和必要的 adapter interface；不能依赖 HTTP。

## adapter

`internal/adapter` 包含边界实现：

- `http`：路由、认证、请求/响应、下载。
- `configstore`：配置读写、codec、revision、只读能力。
- `resource`：远程 URL、缓存、脱敏。
- `auth`：账号、session、密码哈希。
- `webui`：嵌入前端产物。

## 测试边界

- core 测试不导入 engine 或 adapter。
- engine 测试通过 core fixture 构造输入。
- service 测试使用 adapter fake。
- API 测试只断言 wire contract。
- 依赖方向用 import 测试锁定。
