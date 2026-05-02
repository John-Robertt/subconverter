# 工作流契约

## 配置加载

1. 页面启动后调用 `GET /api/status` 读取能力与状态。
2. 调用 `GET /api/config` 读取已保存配置和 `config_revision`。
3. 将配置复制为前端草稿。

`GET /api/config` 不等同于当前运行时配置。当前运行时 revision 由 `GET /api/status` 返回。

## 保存与生效

保存工作流：

1. 用户编辑草稿。
2. A8 调用 `POST /api/config/validate` 做静态校验。
3. 本地可写配置源首次保存前展示 YAML 注释和格式可能丢失的确认框。
4. 调用 `PUT /api/config` 条件写回完整配置。
5. 调用 `POST /api/reload` 使配置生效。

`PUT /api/config` 成功不自动表示运行时已更新。reload 成功后，`runtime_config_revision` 才更新为新的 `config_revision`。

## reload 边界

reload 只执行 `LoadConfig + Prepare`：

- 不拉取订阅、Snell 或 VLESS 来源。
- 不执行 Source / Filter / Group / Target / Render。
- 不证明 Clash Meta 或 Surge 一定能生成。

生成可用性由 B1/B2/B3 预览或 `/generate` 验证。

## 中间态

`PUT /api/config` 成功但 `POST /api/reload` 失败时：

- 配置文件已更新。
- 旧 `RuntimeConfig` 仍继续服务请求。
- UI 保持 `config_dirty = true` 提示。
- 用户可重新触发 reload 或继续修改后再保存。

## 409 保存错误

当 `PUT /api/config` 返回 `409 config_revision_conflict`：

- 不覆盖用户草稿。
- 展示“配置文件已被外部修改”提示。
- 提供重新加载配置或继续手动合并的入口。

编辑页活跃期间，前端建议每 30 秒 poll `GET /api/status` 的 `config_revision`。若 revision 变化，提前提示用户。

当 `PUT /api/config` 返回 `409 config_source_readonly` 或 `409 config_file_not_writable`：

- 不覆盖用户草稿。
- 只读配置源进入只读查看模式。
- 文件不可写错误展示权限或部署挂载问题，并保留可查看详情入口。

## 只读配置源

当 `capabilities.config_write = false`：

- 编辑页进入只读查看模式。
- 禁用新增、删除、拖拽排序和保存。
- 允许 A8 校验、B 区预览、B3 生成预览和 reload。
