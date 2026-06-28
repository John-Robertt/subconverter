# Resource Adapter 设计

> 状态：v3.0 目标契约。本文定义远程资源读取、缓存和脱敏规则。

## 目标

Resource Adapter 是远程网络和本地资源读取边界。它为 Build、Config I/O、ArtifactService、PreviewService 和导出能力提供资源内容，但不拥有配置语义、图规则或渲染规则。

## 资源类型

| 资源 | 示例 | 读取方 |
|------|------|--------|
| 订阅 / Snell / VLESS 来源 | `sources.*[].url` | Build |
| 远程配置输入 | `https://example.com/config.yaml` | Config I/O |
| Clash / Surge 模板 | `templates.clash`、`templates.surge` | ArtifactService / PreviewService / Export |
| 规则集 URL | `rulesets.*.urls[]` | 当前作为配置数据渲染；未来需要内联内容时由服务层读取 |

配置文件的保存、revision 和只读边界由 ConfigStore 负责；Resource Adapter 不直接写主配置。

## 缓存与失效

- 订阅、Snell、VLESS 来源可以使用缓存，但运行时生成和预览必须走同一 Resource Adapter。
- reload 读取远程配置输入时必须绕过陈旧缓存。
- 远程模板读取失败时，生成和导出失败，不返回不完整产物。
- Render Adapter 不管理缓存，也不直接读取模板；它只接收调用方传入的模板内容或模板读取结果。

## URL 脱敏

任何进入 Diagnostic、Error 响应、日志摘要或测试 golden 的用户可见 URL 都必须先脱敏：

- 订阅 URL、远程配置 URL、远程模板 URL 中的 query token 必须隐藏。
- 脱敏在 Resource Adapter 或其调用边界完成，不由前端修补。
- Diagnostic `message` 和 `metadata` 中不得出现未脱敏 URL。

## 导入配置包中的资源

- 配置包只读取固定条目，不按包内路径直接解压。
- 配置包内模板必须写入受控模板目录或作为草稿引用返回。
- 模板副本写入失败时，不得部分导入。
- 导入结果默认只进入草稿，不替换工作配置或 RuntimeSnapshot。

## 测试要求

- 订阅和远程配置 URL 在 Diagnostic message 与 metadata 中均已脱敏。
- 预览和生成使用同一资源读取实现。
- 远程模板失败不返回不完整产物。
- 配置包路径穿越被拒绝。
