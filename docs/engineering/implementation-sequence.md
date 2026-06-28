# v3.0 实施顺序

> 状态：v3.0 目标实施顺序。本文描述推荐落地节奏。

## 阶段一：Core 类型和不变量

交付：

- `Config`、`PreparedConfig`、`RuntimeSnapshot`。
- `Pipeline`、`TargetView`。
- `DiagnosticBundle`。
- `CapabilityRegistry`。
- 命名、引用、fallback、`@all` 等不变量测试。

验收：

- core 不依赖 engine、service、adapter。
- 核心不变量有单测覆盖。

## 阶段二：Prepare 最小闭环

交付：

- `engine/prepare` 的静态校验和预计算。
- Config DTO 到 PreparedConfig 的转换。
- URL、正则、静态引用、命名空间和 `@auto` 展开。

验收：

- Prepare 不拉取远程来源。
- Prepare 失败输出 DiagnosticBundle。
- `WorkspaceService.Validate` 和 `RuntimeService.Reload` 可复用同一 Prepare 实现。

## 阶段三：ConfigStore 与 WorkspaceService

交付：

- `port.ConfigStore`、`port.ConfigCodec` 与 adapter 实现。
- `WorkspaceService.GetConfig / SaveConfig / Validate / Import / Export`。
- revision 冲突、只读源、保存不生效语义。

验收：

- 保存成功只改变 config revision。
- 保存失败不写配置。
- `Validate` 只执行 Prepare，不执行 Build / Target / Render。
- 导入只返回草稿。

## 阶段四：RuntimeService 与 RuntimeSnapshot

交付：

- 当前快照持有与原子替换。
- `RuntimeSnapshot.ExportSource`。
- reload 单飞互斥。
- status、dirty、last reload diagnostics。

验收：

- reload 成功替换快照。
- reload 失败保留旧快照。
- 保存后未 reload 时生效导出仍对应旧快照。
- 请求期访问器不暴露可变内部结构。

## 阶段五：Build / Project / Render

交付：

- Build 格式无关图。
- Target Projection 产出 TargetView 和 cause path。
- RenderInput 组装边界。
- Render 只序列化 RenderInput。

验收：

- Build 不判断目标格式协议差异。
- Render 不执行协议过滤。
- Render 不读取 ConfigStore 或 Resource Adapter。
- Clash / Surge 产物 golden 稳定。

## 阶段六：PreviewService 与 ArtifactService

交付：

- 草稿图预览。
- 草稿目标格式预览。
- 运行时图预览。
- 运行时目标格式预览。
- 目标格式产物和订阅链接。

验收：

- 草稿预览不保存、不生效。
- 预览和实际生成共用同一 Project 实现。
- artifact 只从当前快照生成。
- ArtifactService 负责模板读取、filename 规范化和 managed URL 构造。

## 阶段七：HTTP API 与 Web UI

交付：

- v3 API handler。
- DiagnosticBundle wire contract。
- 工作台状态、诊断中心、目标格式视图。
- API contract 测试。

验收：

- API 只暴露 v3 wire shape。
- UI 不复刻后端投影和渲染规则。
- capabilities 驱动来源、协议、格式提示。

## 阶段八：部署与发布检查

交付：

- 单二进制部署。
- Web UI embed。
- 配置源读写能力检查。
- 端到端验收脚本。

验收：

- 本地预检和 CI 检查一致。
- Docker 构建可用。
- 目标格式生成端到端通过。
