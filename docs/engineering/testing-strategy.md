# v3.0 测试策略

> 状态：v3.0 目标测试策略。本文定义按架构边界组织的测试覆盖。

## 测试目标

v3.0 测试重点是稳定产品语义和演化入口：

- 核心不变量稳定。
- 状态语义稳定。
- 扩展入口集中。
- 诊断可定位、可分类。
- 预览与生成一致。
- API wire shape 稳定。

## Core 测试

- 节点名全局唯一。
- 节点组名和服务组名互斥。
- 链式上游只能是拉取类节点。
- `@all` 不含链式节点。
- `groups`、`routing`、`rulesets` 保序。
- CapabilityRegistry 中来源、协议、目标格式关系一致。

## Config I/O 测试

- decode / encode round-trip。
- revision 冲突不写配置。
- 只读源拒绝保存。
- 导入只返回草稿。
- 导出工作配置和生效配置来源不同。

## RuntimeSnapshot 测试

- 保存后 dirty 为 true。
- reload 成功替换快照并清除 dirty。
- reload 失败保留旧快照。
- 快照访问器返回 clone 或只读视图。
- 生效导出与 snapshot revision 对应。

## Engine 测试

Prepare：

- URL、正则、引用、命名空间校验。
- `fetch_order` 完整性。
- `@auto` 展开。

Build：

- 来源拉取顺序。
- 过滤、分组、路由、fallback。
- 图不变量。

Target Projection：

- Clash 过滤 Snell。
- Surge 过滤 VLESS。
- 链式节点因 dialer 被过滤而失效。
- fallback 清空包含 cause path。
- 原 Pipeline 不被修改。

Render：

- Clash / Surge golden。
- 模板合并。
- 输出顺序确定。

## Service 测试

- Workspace 保存不替换 RuntimeSnapshot。
- Validate 不执行 Build / Target / Render。
- Preview 草稿不写配置。
- Artifact 只从 RuntimeSnapshot 生成。
- Runtime reload 单飞互斥。

## API 测试

- `GET /api/workspace/config` wire shape。
- `PUT /api/workspace/config` revision 冲突和语义失败。
- `POST /api/runtime/reload` 成功和失败路径。
- `POST /api/preview/pipeline` 与 `POST /api/preview/target`。
- `GET /api/artifacts/{format}`。
- DiagnosticBundle wire shape。

## 依赖测试

- `core` 不导入 service、engine、adapter。
- HTTP adapter 不直接导入 engine 实现。
- render 测试不导入 build 或 project。
- service 测试通过 fake adapter 控制 I/O。

## 本地预检

```bash
go mod verify
test -z "$(gofmt -l .)"
go test ./...
go test -tags webui ./...
go vet ./...
test -s internal/webui/dist/index.html
```

涉及 Web 源码时额外执行：

```bash
pnpm web:test
pnpm web:typecheck
pnpm web:build
```
