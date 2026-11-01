# app.Service 接口契约

> 状态提示：本文定义 v2.0 `internal/app` 包的当前接口契约。所属能力已随 M6-M7 验收落地，状态见 docs/README.md 与 docs/implementation/progress.md。

## 目标

本文件定义 `app.Service` 的外部可见方法签名、DTO 结构和依赖关系，使 `admin` 包实现者无需阅读 `app` 包源码即可理解契约。本文是 `api.md`（HTTP 契约）和 `project-structure.md`（包职责）之间的**接口契约桥接**——`api.md` 定义 HTTP 请求/响应格式，本文定义 Go 层 DTO 和方法签名。

---

## 依赖边界

```text
internal/admin ──► internal/app ──► internal/config
                        │          ├─► internal/fetch
                        │          ├─► internal/generate
                        │          ├─► internal/pipeline
                        │          ├─► internal/model
                        │          └─► internal/errtype
```

约束：
- `admin` 只调用 `app.Service` 的公开方法，不直接导入 `model`、`pipeline` 或 `generate`
- `app.Service` 在方法内部完成 `model.Proxy` → `NodePreview` 等 DTO 转换
- `admin` 只接收/返回 JSON 友好的 DTO 和标准错误

---

## 方法签名

### 配置管理

```go
// ConfigSnapshot 读取已保存配置，还原可读正文；无法读取的文件保留引用供修复。
// revision 仍取主配置文件。
// 用于 GET /api/config。
func (s *Service) ConfigSnapshot(ctx context.Context) (*ConfigSnapshotResult, error)

// SaveConfig 基于 config_revision 做条件写回。
// 仅本地可写配置源支持；HTTP(S) 配置源返回 ErrConfigSourceReadonly(409)。
// 本地文件或所在目录不可写返回 ErrConfigFileNotWritable(409)。
// revision 冲突返回 RevisionConflictError(409)，并携带当前 revision。
// 正文写入带时间戳的版本文件后，主配置原子写回引用；失败时回收本次新增文件。
// 正文目录错误使用 ResourceError(CodeResourceContentWriteFailed)，HTTP 映射为 409。
// 写回成功后不自动 reload。
// 用于 PUT /api/config。
func (s *Service) SaveConfig(ctx context.Context, input *SaveConfigInput) (*SaveConfigResult, error)

// EffectiveConfigYAML 返回当前 RuntimeConfig 对应的源 YAML 字节。
// 启动成功与 Reload 成功时更新；SaveConfig 不会更新该快照。
// 用于 GET /api/config/effective.yaml。
func (s *Service) EffectiveConfigYAML(ctx context.Context) ([]byte, error)

// EffectiveConfigArchive 返回当前 RuntimeConfig 对应的源 YAML 与模板 ZIP 包（含已配置的 Clash / Surge / Loon 底版）。
// 启动成功与 Reload 成功时更新配置快照；模板按当前生效 templates 位置读取。
// 用于 GET /api/config/effective.zip。
func (s *Service) EffectiveConfigArchive(ctx context.Context) (*ConfigArchiveResult, error)

// SavedConfigArchive 读取当前配置源，打包已保存的配置与正文，无需先 Reload。
// 两种导出共用包结构：订阅正文内嵌、模板引用包内相对路径，读取失败即终止。
// 用于 GET /api/config/saved.zip。
func (s *Service) SavedConfigArchive(ctx context.Context) (*ConfigArchiveResult, error)

// ImportConfigYAML 将上传 YAML 解析为 Config JSON，并读取已有本地正文引用。
// 不写文件、不替换 RuntimeConfig、不执行 Prepare。
// 用于 POST /api/config/import。
func (s *Service) ImportConfigYAML(ctx context.Context, raw []byte) (*ImportConfigYAMLResult, error)

// ReadResource 读取节点订阅或模板的文本副本，不修改资源和配置。
func (s *Service) ReadResource(ctx context.Context, location string) (*ResourceContentResult, error)

// CheckSubscription 读取并检查节点，不保存配置或替换运行快照。
func (s *Service) CheckSubscription(ctx context.Context, input CheckSubscriptionInput) (*CheckSubscriptionResult, error)

// ImportConfigArchive 将上传 ZIP 包解析为 Config JSON，包内模板还原为草稿正文。
// 仅本地可写配置源支持；不创建文件、不替换 RuntimeConfig、不执行 Prepare。
// 用于 POST /api/config/import application/zip。
func (s *Service) ImportConfigArchive(ctx context.Context, raw []byte) (*ImportConfigYAMLResult, error)

// ValidateDraft 对草稿配置执行 Prepare 阶段静态校验。
// 不写文件、不替换 RuntimeConfig、不拉取远程源。
// 用于 POST /api/config/validate。
func (s *Service) ValidateDraft(ctx context.Context, configJSON json.RawMessage) (*ValidateResult, error)
```

### 热重载

```go
// Reload 触发完整热重载流程：
// 1. reloadMu.TryLock；已有重载在执行时返回 ErrReloadInProgress
// 2. 在配置锁外读取并解析 YAML，再执行 prepareAdminConfig / Prepare
//    远程主配置读取前先 Invalidate 对应缓存项
// 3. 成功 → 短暂加写锁，替换配置指针、revision、生效 YAML 和重载状态
// 4. 失败 → 旧配置不变，返回错误
// 用于 POST /api/reload。
func (s *Service) Reload(ctx context.Context) (*ReloadResult, error)
```

### 运行时预览（GET）

```go
// PreviewNodes 基于当前 RuntimeConfig 快照执行 Source + Filter 阶段。
// 只持有 RLock 复制指针快照后立即释放锁。
// 用于 GET /api/preview/nodes。
func (s *Service) PreviewNodes(ctx context.Context) (*NodePreviewResult, error)

// PreviewGroups 基于当前 RuntimeConfig 快照执行 Source→Filter→Group→Route→ValidateGraph。
// ValidateGraph 发现图级错误时返回结构化诊断，不返回部分成功的分组结果。
// 用于 GET /api/preview/groups。
func (s *Service) PreviewGroups(ctx context.Context) (*GroupPreviewResult, error)
```

### 草稿预览（POST）

```go
// PreviewNodesFromDraft 接收草稿 Config JSON，Prepare 后执行 Source + Filter。
// 不写文件、不替换 RuntimeConfig、不改变 config_dirty。
// 用于 POST /api/preview/nodes。
func (s *Service) PreviewNodesFromDraft(ctx context.Context, configJSON json.RawMessage) (*NodePreviewResult, error)

// PreviewGroupsFromDraft 接收草稿 Config JSON，Prepare 后执行到 ValidateGraph。
// 不写文件、不替换 RuntimeConfig。
// 用于 POST /api/preview/groups。
func (s *Service) PreviewGroupsFromDraft(ctx context.Context, configJSON json.RawMessage) (*GroupPreviewResult, error)
```

### 生成与生成预览

```go
// Generate 基于当前 RuntimeConfig 快照执行 Build→Target→Render，返回完整配置文本。
// 用于 /generate 和 GET /api/generate/preview。
func (s *Service) Generate(ctx context.Context, req GenerateInput) (*GenerateResult, error)

// GenerateFromDraft 接收草稿 Config JSON，Prepare 后执行 Build→Target→Render。
// 不写文件、不替换 RuntimeConfig。
// 用于 POST /api/generate/preview。
func (s *Service) GenerateFromDraft(ctx context.Context, req GenerateInput, configJSON json.RawMessage) (*GenerateResult, error)

// GenerateLink 基于当前 RuntimeConfig 的 base_url、目标格式、filename 和服务端配置的订阅访问 token
// 生成客户端订阅链接。用于 GET /api/generate/link。
// includeToken=false 时不写入 token query。
func (s *Service) GenerateLink(ctx context.Context, input *GenerateLinkInput) (*GenerateLinkResult, error)
```

### 系统状态

```go
// Status 返回系统运行状态。
// 只在 RLock 内复制 runtime_config_revision、config_loaded_at、
// last_reload 和远程配置最近观测 revision 等内存态。
// 本地配置源的 sha256 在释放锁后重新读取文件计算；
// HTTP(S) 配置源不主动拉取远程。
// 用于 GET /api/status。
func (s *Service) Status(ctx context.Context) (*StatusResult, error)
```

---

## DTO 定义

### ConfigSnapshotResult

```go
type ConfigSnapshotResult struct {
    ConfigRevision string          `json:"config_revision"` // sha256:<hex>
    Config         json.RawMessage `json:"config"`
}
```

### SaveConfigInput

```go
type SaveConfigInput struct {
    ConfigRevision string          `json:"config_revision"`
    Config         json.RawMessage `json:"config"`
}
```

### SaveConfigResult

```go
type SaveConfigResult struct {
    ConfigRevision string `json:"config_revision"` // 写回后新的 sha256:<hex>
}
```

### ImportConfigYAMLResult

```go
type ImportConfigYAMLResult struct {
    Config json.RawMessage `json:"config"`
}
```

### CheckSubscriptionInput / CheckSubscriptionResult

```go
type CheckSubscriptionInput struct {
    Location       string `json:"location,omitempty"`
    Content        string `json:"content,omitempty"`
    SourceLocation string `json:"source_location,omitempty"`
    Format         string `json:"format,omitempty"`
}

type CheckSubscriptionResult struct {
    Status        string   `json:"status"`
    Format        string   `json:"format,omitempty"`
    NodeCount     int      `json:"node_count"`
    OutputFormats []string `json:"output_formats"`
    Candidates    []string `json:"candidates,omitempty"`
    Content       string   `json:"content"`
}
```

`SourceLocation` 仅为已读取的 `Content` 提供来源位置提示，不触发资源读取。`subscription_check.go` 负责读取与 DTO 编排；`pipeline.InspectSubscription` 负责识别和提取，`generate.Service.CheckProxyFormats` 复用实际 Target/Render 检查输出兼容性。字段和错误契约见 [订阅检查 API](api.md#post-apisubscriptionscheck)。

### ResourceContentResult

```go
type ResourceContentResult struct {
    Content string `json:"content"`
    Format  string `json:"format,omitempty"`
}
```

`Options.DataDir` 指定正文文件目录，省略时为 `/config`。`content_store.go` 负责版本文件发布、失败回收和正文还原；SaveConfig 串行执行 revision 检查、正文写入和主配置提交，避免并发保存互相覆盖。

`Format` 是复用 Source 识别规则得到的订阅格式提示；复制正文时仅为自动识别来源固定格式，用户显式选择优先。模板编辑忽略此提示。配置字段约束见 [配置规约](config-schema.md#subscriptions)。

### ConfigArchiveResult

```go
type ConfigArchiveResult struct {
    Filename    string
    ContentType string
    Body        []byte
}
```

### ValidateResult

```go
type ValidateResult struct {
    Valid    bool              `json:"valid"`
    Errors   []DiagnosticItem  `json:"errors"`
    Warnings []DiagnosticItem  `json:"warnings"`
    Infos    []DiagnosticItem  `json:"infos"`
}

type DiagnosticItem struct {
    Severity    string          `json:"severity"`  // "error" | "warning" | "info"
    Code        string          `json:"code"`      // 稳定错误码
    Message     string          `json:"message"`   // 中文错误描述
    DisplayPath string          `json:"display_path"` // 用户可读路径，不作为定位依据
    Locator     DiagnosticLocator `json:"locator"`
}
// 后端不感知前端页面结构；前端根据 Locator.Section 自行决定展示位置。

type DiagnosticLocator struct {
    Section    string `json:"section"`
    Key        string `json:"key,omitempty"`
    Index      int    `json:"index,omitempty"`
    ValuePath  string `json:"value_path,omitempty"`
    JSONPointer string `json:"json_pointer"` // 唯一稳定定位依据
}
```

### ReloadResult

```go
type ReloadResult struct {
    Success    bool  `json:"success"`
    DurationMs int64 `json:"duration_ms"`
}
```

### NodePreviewResult

```go
type NodePreviewResult struct {
    Nodes         []NodePreviewItem `json:"nodes"`
    Total         int               `json:"total"`
    ActiveCount   int               `json:"active_count"`
    FilteredCount int               `json:"filtered_count"`
}

type NodePreviewItem struct {
    ID       string `json:"id"`       // 本次 Build 内的 proxy-N 调试标识
    Name     string `json:"name"`
    Type     string `json:"type"`
    Kind     string `json:"kind"`     // "subscription" | "snell" | "vless" | "custom"
    Server   string `json:"server"`
    Port     int    `json:"port"`
    Filtered bool   `json:"filtered"` // 是否被 filters.exclude 排除
}
```

### GroupPreviewResult

```go
type GroupPreviewResult struct {
    NodeGroups    []GroupItem `json:"node_groups"`
    ChainedGroups []GroupItem `json:"chained_groups"`
    ServiceGroups []ServiceGroupItem `json:"service_groups"`
    AllProxies    []string `json:"all_proxies"` // 兼容既有调用方
    AllProxyDetails []PreviewMemberItem `json:"all_proxy_details"`
}

type GroupItem struct {
    Name          string              `json:"name"`
    Match         string              `json:"match,omitempty"` // 地区组原始正则；链式组无此概念
    Strategy      string              `json:"strategy"`
    Members       []string            `json:"members"` // 兼容既有调用方
    MemberDetails []PreviewMemberItem `json:"member_details"`
}

type ServiceGroupItem struct {
    Name                  string               `json:"name"`
    Strategy              string               `json:"strategy"`
    Members               []string             `json:"members"` // 兼容既有调用方
    MemberDetails         []PreviewMemberItem  `json:"member_details"`
    ExpandedMembers       []ExpandedMemberItem `json:"expanded_members"` // 兼容既有调用方
    ExpandedMemberDetails []PreviewMemberItem  `json:"expanded_member_details"`
}

type ExpandedMemberItem struct {
    Value  string `json:"value"`
    Origin string `json:"origin"`
}

type PreviewMemberItem struct {
    Kind   string `json:"kind"`             // proxy | node_group | route_group | reserved
    ID     string `json:"id,omitempty"`     // 仅 proxy；本次 Build 内的 proxy-N
    Name   string `json:"name"`             // 显示名，允许重复
    Origin string `json:"origin,omitempty"` // literal | auto_expanded | all_expanded
}
```

### GenerateInput / GenerateResult

```go
// 当前实现中 GenerateInput 是 generate.Request 的类型别名。
// 字段：Format string；Filename string（已由 HTTP 层校验并规范化后的最终文件名）。
type GenerateInput = generate.Request

type GenerateResult struct {
    Filename    string // 最终下载文件名
    ContentType string // "text/yaml; charset=utf-8" 或 "text/plain; charset=utf-8"
    Body        []byte // 完整配置文本
}
```

### GenerateLinkInput / GenerateLinkResult

```go
type GenerateLinkInput struct {
    Format       string
    Filename     string
    IncludeToken bool
}

type GenerateLinkResult struct {
    URL           string `json:"url"`
    TokenIncluded bool   `json:"token_included"`
}
```

### StatusResult

```go
type StatusResult struct {
    Version                string             `json:"version"`
    Commit                 string             `json:"commit"`
    BuildDate              string             `json:"build_date"`
    ConfigSource           ConfigSource       `json:"config_source"`
    ConfigRevision         string             `json:"config_revision"`          // 配置源已保存内容的 revision
    RuntimeConfigRevision  string             `json:"runtime_config_revision"`  // 当前 RuntimeConfig 的 revision
    ConfigLoadedAt         string             `json:"config_loaded_at"`         // ISO 8601
    ConfigDirty            bool               `json:"config_dirty"`             // config_revision != runtime_config_revision
    Capabilities           Capabilities       `json:"capabilities"`
    LastReload             *LastReload        `json:"last_reload,omitempty"` // 仅在曾发生过 reload 时填充，否则 nil 并在 JSON 中省略
    RuntimeEnvironment     RuntimeEnvironment `json:"runtime_environment"`
}

type ConfigSource struct {
    Location string `json:"location"`
    Type     string `json:"type"`     // "local" | "remote"
    Writable bool   `json:"writable"` // supports save and current local permissions allow writing
}

type Capabilities struct {
    ConfigWrite bool `json:"config_write"`
    Reload      bool `json:"reload"`
}

type LastReload struct {
    Time       string `json:"time"`       // ISO 8601，仅在 LastReload 非 nil 时存在
    Success    bool   `json:"success"`
    DurationMs int64  `json:"duration_ms"`
    Error      string `json:"error,omitempty"` // 仅在 Success=false 时填充，记录 reload 失败的错误消息
}

type RuntimeEnvironment struct {
    ListenAddr      string `json:"listen_addr"`
    WorkingDir      string `json:"working_dir"`
    GoRuntime       string `json:"go_runtime"`
    MemoryAllocMB   string `json:"memory_alloc_mb"`
    RequestCount24h uint64 `json:"request_count_24h"` // 当前实现为进程内请求计数，启动未满 24h 时即自启动以来
    UptimeSeconds   int64  `json:"uptime_seconds"`
}
// 注意：LastReload 用指针 + omitempty，让"从未 reload"（nil → JSON 省略字段）与
// "上次 reload 失败"（非 nil + Success=false）在 wire format 上明确区分；前端可通过
// 字段是否存在判断"运行中（未重载）"，而不必再依赖 Time 是否为空字符串作为语义副信道。
```

---

## DTO 转换边界

`app.Service` 方法在内部完成以下转换，`admin` 不接触 `model` 包：

| app 方法 | 内部消费的 model 类型 | 返回的 DTO |
|----------|---------------------|-----------|
| `PreviewNodes` | `model.Proxy` (FilterResult.Included + Excluded) | `NodePreviewResult` |
| `PreviewGroups` | `model.Pipeline` (GroupResult + RouteResult) | `GroupPreviewResult` |
| `Generate` | `model.Pipeline` (经 Target 投影) → render 输出 | `GenerateResult` |
| `GenerateLink` | `*config.RuntimeConfig` 的 `base_url` + 服务端配置的订阅访问 token | `GenerateLinkResult` |
| `Status` | `*config.RuntimeConfig` 元信息 | `StatusResult` |

设计原则：
- DTO 是 `model` 的**子集视图**：只暴露前端需要的字段，不暴露内部实现细节
- `Kind` 枚举在 DTO 层用字符串表示（`"subscription"` / `"snell"` / `"vless"` / `"custom"`），避免 `model.ProxyKind` 泄漏到 HTTP 契约；AnyTLS 节点使用 `kind="subscription"`、`type="anytls"`
- `GroupPreviewResult` 中的 `ExpandedMembers` 携带 `origin` 字段，区分用户显式声明 / `@auto` 展开 / `@all` 展开

### ConfigError → DiagnosticItem 翻译

`config.Prepare` 产出的 `ConfigError` 携带结构化路径（`Section`、`Key`、`Index`、`ValuePath`），这些是配置域概念，不涉及 JSON 序列化格式。`app.Service` 在返回 `ValidateResult` 和 `PUT /api/config` 校验失败响应时，在 app 层完成翻译：

```text
ConfigError{Section:"groups", Key:"🇭🇰 Hong Kong", Index:0, ValuePath:"match"}
  ↓ app 层翻译（知道 OrderedMap JSON 是 [{key,value}] 数组）
DiagnosticItem{Locator:{Section:"groups", Key:"🇭🇰 Hong Kong", Index:0, ValuePath:"match", JSONPointer:"/config/groups/0/value/match"}}
```

职责分界：
- `config` 包：负责产出结构化路径字段（配置域知识），不感知 JSON 序列化格式
- `app` 包：负责计算 `json_pointer`（知道 OrderedMap 在 JSON 中是数组，需要 index），组装完整 `DiagnosticItem`
- `admin` 包：透传 `DiagnosticItem` 到 JSON 响应，不做任何路径计算

---

## 错误处理

`app.Service` 方法返回标准 Go `error`。`admin` 层将错误映射为 HTTP 状态码和 JSON 响应体。

错误类型约定：

| 错误类型 | 定义位置 | 判断方式 | HTTP 状态 / code |
|----------|---------|----------|------------------|
| `*errtype.ConfigError` | `internal/errtype` | `errors.As` | 400 |
| `*errtype.RevisionConflictError` | `internal/errtype` | `errors.As` | `409 config_revision_conflict` |
| `errtype.ErrConfigSourceReadonly` | `internal/errtype`（sentinel） | `errors.Is` | `409 config_source_readonly` |
| `errtype.ErrConfigFileNotWritable` | `internal/errtype`（sentinel） | `errors.Is` | `409 config_file_not_writable` |
| `errtype.ErrReloadInProgress` | `internal/errtype`（sentinel） | `errors.Is` | 429 |
| `*errtype.FetchError` | `internal/errtype` | `errors.As` | 502 |
| `*errtype.BuildError` / `*errtype.TargetError` / `*errtype.RenderError` | `internal/errtype` | `errors.As` | 400/500（按 `validation.md` 映射） |

revision 冲突需要携带当前配置源 revision，供 `PUT /api/config` 的 `409 config_revision_conflict` 响应返回 `current_config_revision`：

```go
type RevisionConflictError struct {
    CurrentConfigRevision string
}
```

sentinel error 定义形式：

```go
var (
    ErrConfigSourceReadonly  = errors.New("config source is read-only")
    ErrConfigFileNotWritable = errors.New("config file is not writable")
    ErrReloadInProgress      = errors.New("reload already in progress")
)
```

只读配置源、文件不可写和 reload 互斥不需要携带额外字段信息，使用 sentinel error；revision 冲突需要携带当前 revision，使用结构体错误。

`admin` 对 sentinel error 使用 `errors.Is`，对结构化错误使用 `errors.As`，不解析错误消息字符串。

---

## 并发安全

这里需要分清三件事：正在生成的请求使用哪份配置、是否已有重载在执行，以及保存时文件有没有变化。它们由不同机制处理。

`sync.RWMutex` 保护当前生效配置及相关内存状态：

- 读路径（`PreviewNodes`、`PreviewGroups`、`Generate`）：只在复制 `*RuntimeConfig` 指针时短暂 `RLock`，随后释放锁
- `Status`：只在 `RLock` 内复制运行时 revision、加载时间、最近 reload 和远程配置最近观测 revision 等内存态；本地配置源的文件读取与 sha256 计算在释放锁后执行，`config_dirty` 由响应中的 `config_revision != runtime_config_revision` 计算
- `EffectiveConfigYAML`：只在 `RLock` 内复制当前生效 YAML 字节；该快照只在启动成功和 `Reload` 成功时更新
- `EffectiveConfigArchive`：复用 `EffectiveConfigYAML` 取得生效 YAML 快照，再解析模板位置；正文读取与 ZIP 编码在释放锁后执行
- 写路径（`Reload`）：先在锁外读取、解析和准备配置；校验成功后，短暂加写锁一起更新配置指针、revision、生效 YAML、加载时间和重载状态
- 慢速订阅拉取、模板加载和渲染不持有配置锁

例如，请求 A 取得旧配置后开始下载订阅，重载 B 此时可以准备并发布新配置。A 仍用旧配置完成，之后取得快照的请求才使用新配置。`RuntimeConfig` 按只读约定使用，管道各自构建结果；其 Go 类型并未强制深层数据不可修改。

`reloadMu` 只负责重载互斥：入口调用 `TryLock`，已有重载在执行时立即返回 `ErrReloadInProgress`，由 `admin` 映射为 429。它不会让普通生成请求排队。

`SaveConfig` 用 `saveMu` 串行执行同一服务实例内的保存，保持生效快照不变：

- 先读取文件并计算 `config_revision`，与调用方传入的版本比较；不匹配时返回冲突，保留文件原样
- 校验通过后发布正文版本文件，再把引用写入完整 YAML 的同目录临时文件，用 `os.Rename` 替换原配置；主配置提交失败时回收本次新增正文，旧版本不覆盖
- 成功时根据写出的字节计算并返回新 revision。下一次本地 `Status` 会重新读文件并计算 revision，生效 revision 仍要等成功重载才更新

`saveMu` 覆盖本实例的 revision 检查、正文写入和主配置提交；相同 revision 的并发保存只有一次成功，其余返回版本冲突。外部进程仍不受该锁约束，若它在版本比较与替换之间写入，仍可能被覆盖。

---

## 与 admin 的协作模式

```text
HTTP Request
  │
  └─► admin/handler.go             解析 JSON/query，调用 app/auth 服务并映射错误
```

当前实现将 Admin API 路由集中在 `admin/handler.go`，各 handler 方法的共同模式：
1. 解析 HTTP 输入为 Go 值
2. 调用对应的 `app.Service` 方法
3. 将 DTO 序列化为 JSON 响应
4. 将 `error` 按类型映射为 HTTP 状态码 + JSON 错误体

## Loon 配置与生成

`GenerateInput.Format`、`GenerateLinkInput.Format` 接受 `loon`，默认文件名为 `loon.lcf`；现有运行时预览与草稿预览方法签名保持不变。配置 DTO 中的 `Config` JSON 保留 `sources.subscriptions[].format` 与 `templates.loon`，字段定义见 [配置规约](config-schema.md#subscriptions)。

配置包增加可选条目 `templates/loon.lcf`。导入后把模板正文放入返回草稿的 `templates.inline.loon`；保存、重载和导出沿用现有流程。
