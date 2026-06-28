# 配置工作台语义

> 状态：v3.0 目标语义。本文定义用户在工作台中看到的状态和动作。

## 三种配置状态

### 草稿配置

草稿配置来自浏览器编辑、导入结果或一次性预览请求。草稿不自动保存，不自动生效，不替换当前快照。

### 工作配置

工作配置是系统当前保存的配置语义模型。保存成功只表示工作配置更新，并返回新的 `config_revision`。

### 运行时快照

运行时快照是最近一次启动或 reload 成功后进入服务的不可变状态。生成、订阅链接、运行时预览和生效配置导出都从它出发。

## Dirty 状态

`dirty = config_revision != snapshot_revision`。

dirty 表示工作配置已经变化，但运行时尚未使用新配置。reload 成功后 dirty 清除；reload 失败时旧快照保持不变，dirty 继续存在。

## 用户动作表

| 动作 | 输入 | 输出 | 是否保存配置 | 是否替换快照 |
|------|------|------|--------------|--------------|
| 读取工作配置 | 当前工作配置 | Config DTO + revision | 否 | 否 |
| 编辑草稿 | 用户输入 | Draft Config DTO | 否 | 否 |
| 校验草稿 | Draft Config DTO | DiagnosticBundle | 否 | 否 |
| 保存配置 | Config DTO + expected revision | 新 config_revision | 是 | 否 |
| reload | 工作配置 | RuntimeSnapshot 或 DiagnosticBundle | 否 | 成功时是 |
| 草稿图预览 | Draft Config DTO | PipelinePreview + diagnostics | 否 | 否 |
| 草稿目标预览 | Draft Config DTO + format | TargetPreview + diagnostics | 否 | 否 |
| 运行时预览 | RuntimeSnapshot | Preview DTO + diagnostics | 否 | 否 |
| 生成产物 | RuntimeSnapshot + format | Artifact bytes | 否 | 否 |
| 导入配置 | 外部文件或文本 | Draft Config DTO + diagnostics | 否 | 否 |
| 导出配置 | 工作配置或 RuntimeSnapshot | 配置文件或配置包 | 否 | 否 |

## 页面提示规则

- 保存成功但未 reload 时，页面必须显示 dirty。
- reload 失败时，页面必须显示诊断并保留旧运行时状态。
- 运行时预览必须标明来自当前 RuntimeSnapshot。
- 草稿预览必须标明不会保存、不会生效。
- 导出生效配置必须标明来自当前快照。

## 工作台边界

- 工作台不直接展示内部 Pipeline 指针或 engine 结构。
- 工作台不复刻后端目标格式过滤逻辑。
- 工作台能力提示来自 `/api/capabilities`。
- 工作台错误展示来自 `DiagnosticBundle`。
