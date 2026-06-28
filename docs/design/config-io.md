# 配置与资源 I/O 设计

> 状态：v3.0 目标契约。本文定义配置存储、配置编解码、导入导出和外部资源读取边界。

## 目标

Config I/O 的职责是把外部输入转换成 Product Core 可以理解的 `Config`，并把 `Config` 持久化或导出。它不定义配置语义、不执行生成管道、不拥有业务不变量。

## 边界接口

```go
type ConfigStore interface {
    Read(ctx context.Context) (ConfigDocument, error)
    Write(ctx context.Context, expectedRevision string, cfg Config) (ConfigDocument, error)
    Capabilities(ctx context.Context) ConfigStoreCapabilities
}

type ConfigDocument struct {
    Config   Config
    Revision string
    Source   ConfigSource
}

type ConfigSource struct {
    Type     string
    Location string
    Writable bool
}

type ConfigStoreCapabilities struct {
    Readable bool
    Writable bool
    Import   bool
    Export   bool
}
```

`ConfigStore` 负责 revision、只读边界和原子写入。调用方不能绕过它直接写配置文件。

## ConfigCodec

```go
type ConfigCodec interface {
    Decode(input []byte) (Config, DiagnosticBundle, error)
    Encode(cfg Config) ([]byte, DiagnosticBundle, error)
    MediaTypes() []string
}
```

初始实现至少包含 YAML codec。未来支持 JSON 或 TOML 时，只新增 codec，不改变 Product Core。

## 保存语义

```text
Config DTO + expected config_revision
  -> WorkspaceService
  -> Validate / Prepare dry-run
  -> ConfigStore.Write
  -> new ConfigDocument
```

规则：

- 保存成功只更新工作配置，不替换 RuntimeSnapshot。
- revision 不一致时拒绝写入。
- 配置语义无效时拒绝写入并返回 DiagnosticBundle。
- 存储不可写时返回请求层错误。
- codec 可以选择全量编码或字段级写回；这是实现能力，不进入 Product Core 契约。

## 导入

导入把外部内容转换成草稿配置：

```text
bytes / archive / remote content
  -> choose ConfigCodec
  -> Config DTO + DiagnosticBundle
```

导入不保存配置，不替换 RuntimeSnapshot。用户需要显式保存并 reload 才能生效。

## 导出

导出分为两类：

| 类型 | 来源 | 用途 |
|------|------|------|
| 工作配置导出 | ConfigStore 当前 Config | 备份、迁移、分享当前编辑结果 |
| 生效配置导出 | RuntimeSnapshot | 复现服务正在使用的状态 |

导出可以使用默认 codec 生成配置文件，也可以生成包含模板的配置包。

## 外部资源读取

资源读取包括订阅、远程配置、远程模板和规则集 URL。资源读取由 Resource Adapter 负责：

- 统一 URL 脱敏。
- 统一缓存策略。
- 统一超时和错误转换。
- 不解释配置语义。

## 错误边界

| code | 场景 | 归属 |
|------|------|------|
| `config_revision_conflict` | expected revision 与当前 revision 不一致 | 请求层 error |
| `config_source_readonly` | 配置源不可写 | 请求层 error |
| `config_decode_failed` | codec 无法解析输入 | Diagnostic |
| `config_encode_failed` | codec 无法编码配置 | Diagnostic |
| `config_store_write_failed` | 原子写入失败 | 请求层 error |
| `resource_fetch_failed` | 外部资源不可用 | Diagnostic 或 502 |

## 测试要求

- revision 冲突不写配置。
- 只读配置源拒绝保存。
- codec decode 错误可转换为 Diagnostic。
- 保存后不替换 RuntimeSnapshot。
- 导入只返回草稿配置。
- 生效配置导出与 snapshot revision 对应。
