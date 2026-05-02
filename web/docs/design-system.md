# 设计系统契约

## 视觉方向

Web 后台是运维和开发者工具，优先信息密度、可读性和稳定布局。避免营销式页面、过度装饰和大面积渐变。

## 布局

- 最低支持 1280x800。
- 左侧导航固定宽度。
- 顶栏展示页面标题、状态提示和主操作。
- 主内容按全宽工作区布局，不把页面 section 包在装饰性大卡片中。

## 颜色

- 颜色用于状态和层级，不用于装饰。
- 主强调色用于当前导航、主按钮和链接。
- 成功、警告、错误、信息必须有稳定语义色。
- 浅色与深色主题均属于 v2.0 验收范围。
- 主题 token 至少覆盖：surface、text、border、primary、success、warning、error、info、focus、code background。
- 深色主题不得只反转颜色；状态色语义、焦点环、代码预览和诊断列表在深色背景下必须保持可读。

## 字体与数字

- 正文使用系统 sans 字体。
- URL、正则、端口、文件路径使用等宽字体。
- 数字统计使用 tabular nums，避免跳列。
- 文本不得依赖 viewport 宽度缩放。

## 组件

正式组件至少覆盖：

- Shell / Sidebar / Topbar
- Button / IconButton
- Input / Select / Checkbox / Switch
- Modal / Confirm / Toast / Drawer
- Table / Tree / CodePreview
- StatusBadge / DiagnosticList

图标优先使用正式图标库。Emoji 可作为用户配置名的一部分展示，但不作为正式功能图标的唯一表达。

## 溢出与长内容

必须处理：

- 长订阅 URL。
- 长正则。
- 含 emoji、空格、点号的 key。
- 长节点名。
- 大量组成员。

长内容应截断、换行或提供横向滚动，不能覆盖相邻控件。
