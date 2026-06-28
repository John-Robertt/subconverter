# RuntimeSnapshot 设计

> 状态：v3.0 目标契约。本文定义请求期共享的不可变运行时状态。

## 目标

RuntimeSnapshot 表达“一次成功 reload 后，服务正在使用的配置状态”。生成、运行时预览、订阅链接和生效配置导出都必须从 RuntimeSnapshot 出发。

## 结构

```go
type RuntimeSnapshot struct {
    ID               string
    ConfigRevision   string
    SnapshotRevision string
    LoadedAt         time.Time
    Prepared         PreparedConfig
    Capabilities     CapabilitySet
    ExportSource     RuntimeExportSource
}

type RuntimeExportSource struct {
    Config         Config
    TemplateRefs   Templates
    ConfigRevision string
}
```

字段语义：

- `ID`：快照实例 ID，用于日志和状态展示。
- `ConfigRevision`：创建快照时读取到的工作配置 revision。
- `SnapshotRevision`：快照内容 revision，通常与 `ConfigRevision` 相同；若快照构造引入额外规范化，可单独计算。
- `LoadedAt`：快照创建时间。
- `Prepared`：请求期只读预计算配置。
- `Capabilities`：来源、协议、目标格式能力集合或版本。
- `ExportSource`：导出生效配置所需的 Config 副本、模板引用和创建快照时的配置 revision。

## 不可变性要求

- RuntimeSnapshot 创建后不得被请求期代码修改。
- 对外访问 slice、map、byte slice 时必须返回 clone 或只读视图。
- Build、Target Projection 和 Render 只能基于快照派生新对象。
- reload 通过原子替换快照引用生效，不在旧快照上局部 mutation。
- 已持有旧快照的请求继续完成，新请求读取新快照。

## 生命周期

```text
ConfigStore.Read
  -> Prepare
  -> capture RuntimeExportSource
  -> construct RuntimeSnapshot
  -> atomically replace current snapshot
```

失败路径：

- 读取配置失败：旧快照保持不变。
- 配置 decode 失败：旧快照保持不变。
- Prepare 失败：旧快照保持不变。
- Snapshot 构造失败：旧快照保持不变。

失败必须记录 last reload diagnostics，供 `GET /api/runtime/status` 展示。

## 与工作配置的关系

- `dirty = config_revision != snapshot_revision`。
- 保存配置只改变 `config_revision`。
- reload 成功才改变 `snapshot_revision`。
- reload 失败时 dirty 保持原状态或继续为 true。
- 生效配置导出使用 RuntimeSnapshot 的 `ExportSource`，不读取草稿，也不重新读取当前工作配置。
- 保存后未 reload 时，工作配置导出返回新配置，生效配置导出仍返回旧快照对应配置。

## 并发模型

- RuntimeService 持有当前快照的原子引用或短锁保护指针。
- 获取当前快照的锁范围只覆盖指针读取。
- 订阅拉取、Build、Target Projection 和 Render 不持有快照替换锁。
- 同一时刻只允许一次 reload。

## 测试要求

- 修改快照访问器返回的 slice/map/bytes 不影响后续请求。
- reload 成功后新请求读取新 snapshot revision。
- reload 失败后当前 snapshot revision 不变。
- 保存未 reload 时 dirty 为 true。
- 生效配置导出与 snapshot revision 对应。
