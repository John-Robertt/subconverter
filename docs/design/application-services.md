# 应用服务设计

> 状态：v3.0 目标契约。本文定义后台用例层接口。

## 目标

Application Services 把用户动作组织成稳定用例。HTTP handler 只调用服务接口，Product Core 不关心请求、响应、文件或认证。

服务层只编排，不拥有业务规则。

## WorkspaceService

```go
type WorkspaceService interface {
    GetConfig(ctx context.Context) (ConfigDocument, error)
    SaveConfig(ctx context.Context, input SaveConfigInput) (SaveConfigResult, error)
    Validate(ctx context.Context, cfg Config) DiagnosticBundle
    Import(ctx context.Context, input ImportConfigInput) (ImportConfigResult, error)
    Export(ctx context.Context, input ExportConfigInput) (ExportConfigResult, error)
}

type SaveConfigInput struct {
    ExpectedRevision string
    Config           Config
}

type SaveConfigResult struct {
    Document    ConfigDocument
    Diagnostics DiagnosticBundle
}
```

规则：

- `SaveConfig` 成功只更新工作配置，不替换 RuntimeSnapshot。
- `Validate` 只执行配置结构、静态语义和 Prepare dry-run，不执行 Build / Target / Render。
- `Import` 只返回草稿配置和诊断，不保存、不生效。
- `Export` 导出工作配置，不读取当前运行时快照。

## RuntimeService

```go
type RuntimeService interface {
    Status(ctx context.Context) RuntimeStatus
    Current(ctx context.Context) RuntimeSnapshot
    Reload(ctx context.Context) (ReloadResult, error)
}

type RuntimeStatus struct {
    ConfigRevision   string
    SnapshotRevision string
    Dirty            bool
    LastReload       ReloadRecord
}

type ReloadResult struct {
    Snapshot    *RuntimeSnapshot
    Diagnostics DiagnosticBundle
}
```

规则：

- `Reload` 从 ConfigStore 读取工作配置，Prepare 成功后构造新 RuntimeSnapshot。
- `Reload` 失败时旧快照保持不变。
- `Status` 不触发拉取、构建或渲染。
- 同一时刻只允许一个 reload。

## PreviewService

```go
type PreviewService interface {
    PreviewPipeline(ctx context.Context, input PreviewInput) (PipelinePreview, DiagnosticBundle, error)
    PreviewTarget(ctx context.Context, input TargetPreviewInput) (TargetPreview, DiagnosticBundle, error)
}

type PreviewInput struct {
    Source PreviewSource
    Config *Config
}

type TargetPreviewInput struct {
    Source PreviewSource
    Config *Config
    Format TargetFormat
}
```

规则：

- 预览输入可以是当前 RuntimeSnapshot，也可以是一次性草稿 Config。
- 草稿预览不写配置、不替换快照。
- 目标格式预览必须走 Target Projection，不能由前端模拟过滤。
- 预览返回 DTO 和 DiagnosticBundle，不返回内部 Pipeline 指针。

## ArtifactService

```go
type ArtifactService interface {
    Generate(ctx context.Context, format TargetFormat) (ArtifactResult, error)
    Link(ctx context.Context, format TargetFormat) (ArtifactLink, error)
    ExportRuntime(ctx context.Context, input RuntimeExportInput) (ExportConfigResult, error)
}

type ArtifactResult struct {
    Format      TargetFormat
    ContentType string
    Bytes       []byte
    Diagnostics DiagnosticBundle
}
```

规则：

- 目标格式产物只基于当前 RuntimeSnapshot。
- 生成路径固定为 Build -> Target Projection -> Render。
- `ExportRuntime` 导出当前快照对应的生效配置或配置包。

## 错误边界

- 请求层失败返回普通 error：鉴权失败、JSON 无法解析、revision 冲突、配置源只读。
- 产品层失败返回 DiagnosticBundle：配置无效、来源失败、图错误、目标格式不可生成、渲染失败。
- 服务层不拼接 renderer 或 parser 的原始错误文本，必须转换为稳定 code。

## 测试要求

- HTTP handler 不直接导入 engine 或 adapter 实现。
- 保存成功不改变 RuntimeSnapshot。
- reload 失败保留旧快照。
- 草稿预览不写配置。
- 生成产物只从当前快照出发。
