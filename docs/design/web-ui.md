# v3.0 Web UI 设计

> 状态：v3.0 目标契约。本文描述 Web 工作台如何使用 v3 API 与诊断模型。

## 顶层体验

顶栏持续展示：

- 草稿是否有未保存修改。
- 工作配置 revision。
- 当前 RuntimeSnapshot revision。
- dirty 状态。
- reload 是否可执行或正在执行。
- 最近一次阻塞诊断入口。

## 配置编辑

- 编辑页按来源、分组、路由、规则、模板和全局选项组织。
- 页面编辑对象是草稿配置。
- 保存发送完整 Config DTO 与 `expected_config_revision`。
- 保存成功后提示是否 reload。
- 保存失败时保留草稿。
- revision 冲突时提示刷新或手动合并。

## 诊断中心

诊断中心聚合：

- Config I/O 诊断。
- Prepare 静态诊断。
- 来源拉取诊断。
- 图构建诊断。
- 目标格式投影诊断。
- 渲染诊断。

用户点击诊断后，应能跳回具体配置字段、运行时对象或目标格式视图。字段跳转使用 `locator.json_pointer`，不能解析错误文案。

## 预览

预览分两类：

- 草稿预览：基于浏览器当前 Config DTO，不保存、不生效。
- 运行时预览：基于当前 RuntimeSnapshot。

目标格式预览必须调用后端 Target Projection，前端不复制协议过滤逻辑。

## 目标格式视图

Clash 和 Surge 视图应展示：

- 将进入该格式的节点。
- 因协议能力被过滤的节点。
- 因级联被移除的组、ruleset、rule。
- fallback 是否可用。
- 当前格式是否可生成。

## 导入导出

- 导入配置只替换草稿，不自动保存或生效。
- 导出工作配置来自 WorkspaceService。
- 导出生效配置来自 RuntimeSnapshot。
- 复制含 token 的订阅链接需要明确展示风险。

## 非目标

- 前端不作为持久状态源。
- 前端不自行拼接敏感 token。
- 前端不根据错误文本反查字段位置。
- 前端不复刻后端生成、投影或渲染规则。
