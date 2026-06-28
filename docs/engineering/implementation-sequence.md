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

## 阶段二：ConfigStore 与 WorkspaceService

交付：

- `ConfigStore`、`ConfigCodec`。
- `WorkspaceService.GetConfig / SaveConfig / Validate / Import / Export`。
- revision 冲突、只读源、保存不生效语义。

验收：

- 保存成功只改变 config revision。
- 保存失败不写配置。
- 导入只返回草稿。

## 阶段三：RuntimeService 与 RuntimeSnapshot

交付：

- 当前快照持有与原子替换。
- reload 单飞互斥。
- status、dirty、last reload diagnostics。

验收：

- reload 成功替换快照。
- reload 失败保留旧快照。
- 请求期访问器不暴露可变内部结构。

## 阶段四：Prepare / Build / Project / Render

交付：

- Prepare 静态校验和预计算。
- Build 格式无关图。
- Target Projection 产出 TargetView 和 cause path。
- Render 只序列化 TargetView。

验收：

- Build 不判断目标格式协议差异。
- Render 不执行协议过滤。
- Clash / Surge 产物 golden 稳定。

## 阶段五：PreviewService 与 ArtifactService

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

## 阶段六：HTTP API 与 Web UI

交付：

- v3 API handler。
- DiagnosticBundle wire contract。
- 工作台状态、诊断中心、目标格式视图。
- API contract 测试。

验收：

- API 只暴露 v3 wire shape。
- UI 不复刻后端投影和渲染规则。
- capabilities 驱动来源、协议、格式提示。

## 阶段七：部署与发布检查

交付：

- 单二进制部署。
- Web UI embed。
- 配置源读写能力检查。
- 端到端验收脚本。

验收：

- 本地预检和 CI 检查一致。
- Docker 构建可用。
- 目标格式生成端到端通过。
