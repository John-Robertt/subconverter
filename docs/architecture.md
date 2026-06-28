# v3.0 架构规格

> 状态：v3.0 目标架构。本文定义产品内核、服务边界、引擎边界和适配器边界。

## 架构目标

v3.0 的核心矛盾是：产品细节会持续演化，架构必须让这些演化低成本、低风险、可验证地发生。

优先级：

1. 演化效率：新增来源、协议、目标格式、诊断和页面工作流时，修改点集中。
2. 结构清晰：状态、规则、转换和 I/O 分层明确。
3. 验证效率：每个边界都有稳定测试入口。
4. 运行清晰：保存、reload、快照、预览和生成的状态关系一眼可解释。

## 分层

```text
HTTP API / Web UI
  -> Application Services
  -> Product Core
  -> Engines
  -> Adapters
```

### Product Core

Product Core 定义长期稳定的产品语言：

- `Config`：用户配置语义模型。
- `RuntimeSnapshot`：一次成功生效的不可变运行态。
- `Pipeline`：格式无关生成图。
- `TargetView`：某个目标格式的可渲染视图。
- `DiagnosticBundle`：配置、构建、投影、渲染共用的诊断语言。
- `CapabilityRegistry`：来源、协议、目标格式能力矩阵。
- `Naming / Policy Rules`：节点、组、服务组、fallback、`@all`、链式节点等不变量。

Product Core 不依赖 HTTP、文件系统、远程网络、具体配置格式、parser、projector 或 renderer。

### Application Services

Application Services 承接用户动作，只编排，不持有业务规则：

- `WorkspaceService`：读取配置、保存配置、校验草稿、导入导出工作配置。
- `RuntimeService`：持有当前快照、reload、状态、运行时诊断。
- `PreviewService`：基于草稿或快照生成图预览和目标格式预览。
- `ArtifactService`：基于快照生成目标格式产物和订阅链接。

服务层可以调用 Product Core、Engines 和 port interfaces；具体 Adapters 由进程入口装配注入。服务层不能定义协议能力、命名空间规则、图校验规则或渲染规则。

### Engines

Engines 执行纯转换：

- `Prepare`：`Config -> PreparedConfig`，完成静态校验、正则编译、URL 解析和预计算。
- `Build`：`PreparedConfig -> Pipeline`，生成格式无关图。
- `Project`：`Pipeline + TargetFormat -> TargetView`，处理目标格式能力差异和级联诊断。
- `Render`：`RenderInput -> Artifact`，完成目标格式序列化、模板合并和 managed section 注入。

Engines 不读取 HTTP 请求，不保存配置，不管理当前快照。

### Adapters

Adapters 处理边界：

- HTTP 请求解析、认证、响应编码和下载头。
- 配置存储、配置格式编解码、revision 和原子写入。
- 远程资源拉取、缓存和 URL 脱敏。
- 模板读取、静态 Web 资源、部署环境输入。

Adapters 不拥有产品不变量。

## 主数据流

### 保存与校验

```text
Config DTO
  -> WorkspaceService
  -> Validate / Prepare dry-run
  -> ConfigStore write
  -> new config_revision
```

保存配置只改变工作配置 revision，不替换运行时快照。

### Reload

```text
ConfigStore read
  -> Prepare
  -> RuntimeSnapshot
  -> atomically replace current snapshot
```

reload 成功才替换当前快照。reload 失败时旧快照保持不变，并记录诊断。

### 预览

```text
Config DTO or RuntimeSnapshot
  -> Prepare if draft
  -> Build
  -> optional Project(format)
  -> Preview DTO + DiagnosticBundle
```

草稿预览不写配置、不替换快照。

### 生成

```text
RuntimeSnapshot
  -> Build
  -> Project(format)
  -> assemble RenderInput(format)
  -> Render
  -> Artifact bytes
```

生成产物只来自当前快照，不读取草稿。

## 关键决策

| 决策 | 方案 | 原因 |
|------|------|------|
| 架构中心 | Product Core + Engines | 让产品语义和转换链路稳定 |
| 配置状态 | 保存配置与运行时快照分离 | 用户能明确区分“已保存”和“已生效” |
| 目标格式差异 | Target Projection 统一处理 | Build 保持格式无关，Render 保持纯序列化 |
| 扩展入口 | Capability Registry + Binding Registry | 降低新增来源、协议、格式时的漏改风险 |
| 错误表达 | DiagnosticBundle | API、UI、测试共享同一种失败语言 |
| 配置格式 | Adapter 能力 | 格式读写不进入 Product Core |

## 不变量

- 节点名全局唯一。
- 节点组名和服务组名共享命名空间，互斥。
- 链式节点上游只能是拉取类节点。
- `@all` 只包含原始节点，不包含链式节点。
- 带 `relay_through` 的 custom proxy 只派生链式节点和同名链式组。
- `groups`、`routing`、`rulesets` 保序。
- TargetView 不得反向修改 Pipeline 或 RuntimeSnapshot。
- RuntimeSnapshot 创建后请求期只读。
