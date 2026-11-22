# 设计系统契约

## 视觉方向

Web 后台是运维和开发者工具，优先信息密度、可读性和稳定布局。避免营销式页面、过度装饰和大面积渐变。

## 布局

- 最低支持 1280x800。
- 左侧导航固定列宽（视口 `>=1200px` 为 `240px`，更窄视口为 `200px`）。
- 顶栏展示页面标题、状态提示和主操作。
- 主内容按全宽工作区布局，页面区块使用轻量边框面板（`.content-panel` / `.setting-block`）承载，避免嵌套装饰性大卡片。

## 颜色

- 颜色用于状态和层级，不用于装饰。
- 主强调色用于当前导航、主按钮和链接。
- 成功、警告、错误、信息必须有稳定语义色。
- 浅色与深色主题均属于 v2.0 验收范围。
- 主题 token 覆盖：`surface`、`text`、`border`、`primary`、`success`、`warning`、`error`、`info`、`chain`、`focus`、`code background` 及对应 `*-soft` / `shadow` 变量。
- 深色主题不得只反转颜色；状态色语义、焦点环、代码预览和诊断列表在深色背景下必须保持可读。

## 字体与数字

- 正文优先 `Inter`，回退系统 sans 字体。
- URL、正则、端口、文件路径优先 `JetBrains Mono`，回退系统等宽字体。
- 数字统计使用 `tabular-nums` 避免跳列（注：`.big-stat strong` 与 `.summary-stat strong` 当前尚未设置 `tabular-nums`）。
- 文本不得依赖 viewport 宽度缩放。

## 组件

当前正式前端组件组织如下：

- 壳层与导航：`layout/AppShell`（内置侧边栏、顶栏与状态卡）
- 基础 UI（`components/ui.tsx`）：`Button`、`IconButton`、`Field`、`TextInput`、`TextArea`、`SelectInput`、`Modal`、`StatCard`、`SplitWorkbench`、`RailPanel`、`Chip`、`CategoryPill`、`EmptyState`、`ErrorState`、`LoadingState`（复选框使用原生 `<input type="checkbox">` + `.checkbox-row`，无独立 `Switch` 组件）
- 业务复合组件：`SortableList`、`SourceEditor`、`ResourceEditor`、`ResourceBody`、`DiagnosticList`（`components/Diagnostics.tsx`）
- 页面内联组件：`DiagnosticDrawer`（`ValidatePage.tsx`）、`CodePreview`（`DownloadPage.tsx`）、原生节点 `<table>`（`NodesPage.tsx`）、成员列表 `.member-tree`（`GroupPreviewPage.tsx`）
- 全局反馈状态（`state/`）：`ConfirmProvider`、`ToastProvider`、`ThemeProvider`、`ConfigProvider`

图标优先使用正式图标库。Emoji 可作为用户配置名的一部分展示，但不作为正式功能图标的唯一表达。

## 溢出与长内容

必须处理：

- 长订阅 URL。
- 长正则。
- 含 emoji、空格、点号的 key。
- 长节点名。
- 大量组成员。

长内容应截断、换行或提供横向滚动，不能覆盖相邻控件。
