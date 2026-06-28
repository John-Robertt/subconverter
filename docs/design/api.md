# v3.0 API 设计

> 状态：v3.0 目标契约。本文定义后台 API wire shape。

## 总览

```text
/api/session/*        登录、退出、会话状态
/api/workspace/*      工作配置：读取、保存、校验、导入、导出
/api/runtime/*        运行时：状态、reload、当前快照、生效导出
/api/preview/*        草稿或快照预览
/api/artifacts/*      目标格式产物
/api/capabilities     来源、协议、目标格式能力
```

API 只暴露产品语义 DTO，不暴露内部 engine、adapter 或存储实现。

## 通用响应

产品层问题统一使用 DiagnosticBundle：

```json
{
  "valid": false,
  "diagnostics": [
    {
      "severity": "error",
      "phase": "prepare",
      "code": "group_name_conflict",
      "message": "节点组和服务组不能同名",
      "locator": {
        "json_pointer": "/config/groups/0/name",
        "display_path": "groups.HK"
      }
    }
  ]
}
```

请求层错误使用普通错误响应：

```json
{
  "error": {
    "code": "config_revision_conflict",
    "message": "配置已被更新，请刷新后重试"
  }
}
```

HTTP 状态码表达请求是否被接受；DiagnosticBundle 表达产品语义是否有效。

## Workspace API

### `GET /api/workspace/config`

读取当前工作配置。

响应：

```json
{
  "config_revision": "sha256:7d9c...",
  "config": {},
  "source": {
    "type": "local",
    "location": "/etc/subconverter/config.yaml",
    "writable": true
  },
  "capabilities": {
    "write": true,
    "import": true,
    "export": true
  }
}
```

### `PUT /api/workspace/config`

保存完整工作配置。

请求：

```json
{
  "expected_config_revision": "sha256:7d9c...",
  "config": {}
}
```

成功响应：

```json
{
  "config_revision": "sha256:9a21...",
  "diagnostics": {
    "valid": true,
    "diagnostics": []
  }
}
```

规则：

- revision 不一致返回 `409 config_revision_conflict`。
- 配置源不可写返回 `409 config_source_readonly`。
- 配置语义无效返回 `400` + DiagnosticBundle，且不写配置。
- 保存成功不替换 RuntimeSnapshot。

### `POST /api/workspace/validate`

校验草稿配置。

请求：

```json
{
  "config": {}
}
```

响应始终为 `200` + DiagnosticBundle，除非请求 JSON 本身非法。

### `POST /api/workspace/import`

导入外部配置为草稿。

输入可以是 YAML、JSON、TOML 或配置包，具体支持格式由 `/api/capabilities` 声明。

响应：

```json
{
  "config": {},
  "diagnostics": {
    "valid": true,
    "diagnostics": []
  }
}
```

导入不保存配置，不替换 RuntimeSnapshot。

### `GET /api/workspace/export`

导出当前工作配置。

查询参数：

| 参数 | 说明 |
|------|------|
| `format` | `yaml`、`json` 或其他已声明 codec |
| `bundle` | 是否导出包含模板的配置包 |

## Runtime API

### `GET /api/runtime/status`

响应：

```json
{
  "config_revision": "sha256:9a21...",
  "snapshot_revision": "sha256:7d9c...",
  "dirty": true,
  "loaded_at": "2026-06-28T14:00:00Z",
  "last_reload": {
    "success": false,
    "time": "2026-06-28T14:05:00Z",
    "diagnostics": {
      "valid": false,
      "diagnostics": []
    }
  }
}
```

### `POST /api/runtime/reload`

从工作配置创建新的 RuntimeSnapshot。

成功响应：

```json
{
  "snapshot_revision": "sha256:9a21...",
  "dirty": false,
  "diagnostics": {
    "valid": true,
    "diagnostics": []
  }
}
```

失败响应返回 DiagnosticBundle，旧快照保持不变。

### `GET /api/runtime/export`

导出当前 RuntimeSnapshot 对应的生效配置。

查询参数同 `/api/workspace/export`。

## Preview API

### `POST /api/preview/pipeline`

预览格式无关图。

请求：

```json
{
  "source": "draft",
  "config": {}
}
```

`source` 可为 `draft` 或 `runtime`。`runtime` 不需要传 `config`。

响应：

```json
{
  "preview": {
    "proxies": [],
    "node_groups": [],
    "route_groups": [],
    "rulesets": [],
    "rules": [],
    "fallback": "FINAL"
  },
  "diagnostics": {
    "valid": true,
    "diagnostics": []
  }
}
```

### `POST /api/preview/target`

预览目标格式投影。

请求：

```json
{
  "source": "draft",
  "format": "clash",
  "config": {}
}
```

响应：

```json
{
  "format": "clash",
  "generatable": true,
  "target_view": {},
  "dropped": {
    "proxies": [],
    "groups": [],
    "rulesets": [],
    "rules": []
  },
  "diagnostics": {
    "valid": true,
    "diagnostics": []
  }
}
```

## Artifact API

### `GET /api/artifacts/{format}`

从当前 RuntimeSnapshot 生成目标格式配置。

规则：

- `format` 必须存在于 CapabilityRegistry。
- 执行 Build -> Target Projection -> Render。
- 失败返回 DiagnosticBundle。

### `GET /api/artifacts/{format}/link`

返回目标格式订阅链接。

响应：

```json
{
  "format": "clash",
  "url": "https://example.com/api/artifacts/clash"
}
```

## Capabilities API

### `GET /api/capabilities`

响应：

```json
{
  "sources": [],
  "protocols": [],
  "targets": [],
  "config_codecs": ["yaml", "json"]
}
```

前端所有来源、协议、目标格式、格式专属提示都来自此接口或其静态等价物。

## API 测试要求

- API contract 测试只断言 v3 wire shape。
- 保存配置不改变 runtime status 中的 snapshot revision。
- reload 成功清除 dirty。
- reload 失败保留旧 snapshot revision。
- preview draft 不写配置。
- artifact 只从 RuntimeSnapshot 生成。
