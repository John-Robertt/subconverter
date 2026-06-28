# 管道设计

> 状态：v3.0 目标契约。本文定义从配置到产物的阶段边界。

## 总览

```text
Config
  -> Prepare
  -> RuntimeSnapshot
  -> Build
  -> Target Projection
  -> assemble RenderInput
  -> Render
```

草稿预览从 `Config` 开始，运行时预览和生成从 `RuntimeSnapshot` 开始。

## Prepare

输入：`Config`

输出：`PreparedConfig + DiagnosticBundle`

职责：

- 校验字段结构和必填项。
- 校验 URL、端口、正则、引用关系。
- 编译正则。
- 解析自定义代理。
- 展开 `@auto`。
- 建立静态命名空间。
- 记录模板引用。

Prepare 不拉取订阅，不构建目标格式，不渲染。

## RuntimeSnapshot 构造

输入：`PreparedConfig + config_revision + RuntimeExportSource`

输出：`RuntimeSnapshot`

规则：

- 构造成功后通过原子替换成为当前快照。
- 构造失败时旧快照保持不变。
- 快照创建后请求期只读。

## Build

输入：`PreparedConfig`

输出：`Pipeline + DiagnosticBundle`

职责：

- 按 `fetch_order` 拉取来源。
- 解析来源节点。
- 执行过滤。
- 构建节点组。
- 构建链式节点和链式组。
- 构建服务组、rulesets、rules、fallback。
- 校验格式无关图不变量。

Build 不判断目标格式是否支持某个协议。

## Target Projection

输入：`Pipeline + TargetFormat`

输出：`TargetView + DiagnosticBundle`

职责：

- 从 CapabilityRegistry 读取目标格式能力。
- 过滤目标格式不支持的协议。
- 过滤以上游不可用节点为 dialer 的链式节点。
- 级联移除空组、失效 ruleset、失效 rule。
- 校验目标格式图不变量。
- 生成 cause path。

预览和实际生成必须使用同一 Target Projection 实现。

## Render

输入：`RenderInput`

输出：`Artifact bytes + DiagnosticBundle`

职责：

- 序列化目标格式。
- 合并目标格式模板。
- 注入 managed section。
- 保持输出确定性。

`RenderInput` 由 ArtifactService 或 PreviewService 组装，包含 TargetView、当前格式模板内容和 managed URL。Render 不做协议过滤，不修正 TargetView，不读取 ConfigStore 或 Resource Adapter。

## 调用矩阵

| 用例 | 起点 | 阶段 |
|------|------|------|
| 保存校验 | Config | Prepare |
| reload | ConfigStore | Prepare -> RuntimeSnapshot |
| 草稿图预览 | Config | Prepare -> Build |
| 草稿目标预览 | Config | Prepare -> Build -> Target Projection |
| 运行时图预览 | RuntimeSnapshot | Build |
| 运行时目标预览 | RuntimeSnapshot | Build -> Target Projection |
| 生成产物 | RuntimeSnapshot | Build -> Target Projection -> assemble RenderInput -> Render |

## 失败语义

- Prepare 失败：配置不能保存或不能 reload。
- Build 失败：图不可用，目标格式无关。
- Target Projection 失败：特定目标格式不可生成。
- Render 失败：目标格式序列化或模板合并失败。

所有产品层失败必须转换为 DiagnosticBundle。
