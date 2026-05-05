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
// ConfigSnapshot 读取配置源中的已保存配置，返回 Config JSON + revision。
// 用于 GET /api/config。
func (s *Service) ConfigSnapshot(ctx context.Context) (*ConfigSnapshotResult, error)

// SaveConfig 基于 config_revision 做条件写回。
// 仅本地可写配置源支持；HTTP(S) 配置源返回 ErrConfigSourceReadonly(409)。
// 本地文件或所在目录不可写返回 ErrConfigFileNotWritable(409)。
// revision 冲突返回 RevisionConflictError(409)，并携带当前 revision。
// 写回成功后不自动 reload。
// 用于 PUT /api/config。
func (s *Service) SaveConfig(ctx context.Context, input *SaveConfigInput) (*SaveConfigResult, error)

// ValidateDraft 对草稿配置执行 Prepare 阶段静态校验。
// 不写文件、不替换 RuntimeConfig、不拉取远程源。
// 用于 POST /api/config/validate。
func (s *Service) ValidateDraft(ctx context.Context, configJSON json.RawMessage) (*ValidateResult, error)
```

### 热重载

```go
// Reload 触发完整热重载流程：
// 1. 重新加载配置源（远程主配置 bypass 缓存）
// 2. Prepare 校验
// 3. 成功 → WLock 替换 RuntimeConfig 指针 + runtime_config_revision
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
    AllProxies    []string    `json:"all_proxies"`
}

type GroupItem struct {
    Name     string   `json:"name"`
    Match    string   `json:"match,omitempty"` // 地区组原始正则；链式组无此概念，nil 时省略
    Strategy string   `json:"strategy"`
    Members  []string `json:"members"`
}

type ServiceGroupItem struct {
    Name            string                `json:"name"`
    Strategy        string                `json:"strategy"`
    Members         []string              `json:"members"`
    ExpandedMembers []ExpandedMemberItem  `json:"expanded_members"`
}

type ExpandedMemberItem struct {
    Value  string `json:"value"`
    Origin string `json:"origin"` // "literal" | "auto_expanded" | "all_expanded"
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
- `Kind` 枚举在 DTO 层用字符串表示（`"subscription"` / `"snell"` / `"vless"` / `"custom"`），避免 `model.ProxyKind` 泄漏到 HTTP 契约
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

`app.Service` 内部持有 `sync.RWMutex`：

- 读路径（`PreviewNodes`、`PreviewGroups`、`Generate`）：只在复制 `*RuntimeConfig` 指针时短暂 `RLock`，随后释放锁
- `Status`：只在 `RLock` 内复制运行时 revision、加载时间、最近 reload 和远程配置最近观测 revision 等内存态；本地配置源的文件读取与 sha256 计算在释放锁后执行，`config_dirty` 由响应中的 `config_revision != runtime_config_revision` 计算
- 写路径（`Reload`）：`WLock` 只保护指针替换，不包含 `LoadConfig` / `Prepare` / I/O
- 慢速订阅拉取、模板加载和渲染不持有配置锁
- `SaveConfig` 的原子写回使用临时文件 + `os.Rename`，与配置锁独立
- Reload 互斥：`app.Service` 内部持有独立的 `reloadMu sync.Mutex`。`Reload` 入口处 `TryLock`，失败时返回 `ErrReloadInProgress`（`admin` 映射为 429）。此锁独立于 `RWMutex`，不影响读路径
- `SaveConfig` 使用 `config_revision` 作为乐观并发令牌，目标是防止旧页面、旧标签页或旧 revision 覆盖已经观测到的新配置；它不承诺外部多写者线性一致
- `SaveConfig` 的 revision 检查依赖文件系统原子性（`os.Rename`），不使用额外文件锁保护 check-then-write 序列。理论上存在 TOCTOU 窗口（外部进程在 revision 比对和 rename 之间修改文件），但单用户场景下该窗口极窄且可接受；若未来明确支持 GitOps 多写者并发写入，再引入文件锁、rename 前重检或备份策略

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
