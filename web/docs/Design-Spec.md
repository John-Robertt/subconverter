# subconverter-go Admin · 设计规范

> Design Tokens + 组件用法 · 版本 v1.0 · 配套实现：`direction-b.jsx`

---

## 1. 设计原则

1. **YAML 是真相，UI 是镜子。** 任何 UI 元素都必须能映射回 YAML 的某个 key，反之亦然。不存在"前端独有"的状态。
2. **静态信息密度优先。** 这是给运维 / 开发者用的工具，不是消费产品。卡片留白克制，正则、URL、端口能塞进一行就不换行。
3. **错误必须可点。** 任何错误提示都带明确去往的页面 / 字段。「保存失败」绝不能不告诉用户去哪修。
4. **节制使用色彩。** 颜色用于状态分级（成功 / 警告 / 错误），不用于装饰。强调色仅用于主按钮、当前导航、链接。
5. **一种交互一种范式。** Modal / Toast / Confirm / Spinner / Drawer 五种交互模式各司其职，不混用（详见 PRD 第 5 节）。

## 2. 设计 Tokens

token 定义在 `direction-b.jsx` 顶部 `B = { ... }` 对象中。

### 2.1 颜色

| Token | Light | Dark | 用途 |
|---|---|---|---|
| `B.bg` | `#f8fafc` | `#0f172a` | 应用底色 |
| `B.panel` | `#ffffff` | `#1e293b` | 卡片 / 面板背景 |
| `B.panelAlt` | `#f1f5f9` | `#334155` | 次级面板（表头、徽标背景） |
| `B.border` | `#e2e8f0` | `#334155` | 卡片 / 输入边框 |
| `B.text` | `#0f172a` | `#f1f5f9` | 主文本 |
| `B.textMuted` | `#475569` | `#94a3b8` | 次文本（说明、副标题） |
| `B.textDim` | `#94a3b8` | `#64748b` | 弱文本（占位、时间戳） |
| `B.accent` | tweak 控制 | tweak 控制 | 主按钮 / 链接 / 当前页 |

### 2.2 强调色预设

| 名称 | Hex |
|---|---|
| Indigo（默认） | `#6366f1` |
| Sky | `#0ea5e9` |
| Emerald | `#10b981` |
| Amber | `#f59e0b` |
| Red | `#ef4444` |
| Purple | `#a855f7` |

### 2.3 状态色

| 状态 | 主色 | 浅底 | 边框 |
|---|---|---|---|
| Success | `#16a34a` | `#dcfce7` | `#bbf7d0` |
| Warning | `#d97706` | `#fef3c7` | `#fde68a` |
| Danger | `#dc2626` | `#fee2e2` | `#fecaca` |
| Info | `#0284c7` | `#e0f2fe` | `#bae6fd` |

### 2.4 字体

```css
font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif;
font-mono:   ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
```

| 用途 | size | weight | letter-spacing |
|---|---|---|---|
| H1 页面标题 | 20px | 600 | -0.015em |
| H3 区块标题 | 15px | 600 | 默认 |
| 正文 | 13–14px | 400 | 默认 |
| 次文本 | 12–13px | 400 | 默认 |
| 辅助 / 时间戳 | 11–12px | 400/500 | uppercase 时 0.05em |
| 数字 / Stat | 22–28px | 700 | -0.02em |
| 代码 / URL / 端口 | 11–12px | 400 | 等宽字体 |

数字必须开 `font-variant-numeric: tabular-nums;` 防止跳列。

### 2.5 间距 / 圆角 / 阴影

| Token | 值 | 用途 |
|---|---|---|
| 卡片内边距 | `16–22px` | 卡片纵向 16，横向 18–22 |
| 区块间距 | `22–28px` | section 间 |
| Topbar padding | `20px 32px` | 全局 |
| 主内容 padding | `24–28px 32px` | 全局 |
| 圆角 / 卡片 | `12px` | 卡片、模态、Drawer |
| 圆角 / 按钮 | `8px` | 按钮、输入 |
| 圆角 / 徽标 | `4–6px` 或 `999` | 标签按形状选 |
| 阴影 / 卡片 | `0 1px 2px rgba(0,0,0,0.03)` | 静态卡片 |
| 阴影 / 浮层 | `0 24px 48px rgba(15,23,42,0.25)` | Modal |
| 阴影 / Toast | `0 12px 32px rgba(15,23,42,0.15)` | Toast / 弹出 |

### 2.6 视觉栅格

设计画布 **1280 × 820**。Sidebar 宽 `220px`，Topbar 高约 `73px`，主内容区横向 padding `32px`。

## 3. 组件库

实现位于 `direction-b.jsx`，外部组件按引用关系列出。

### 3.1 ShellB

整个后台的外壳。负责 Sidebar + Topbar + 内容区三段布局。

```jsx
<ShellB page="sources" dark={false} accent="#6366f1">
  {/* 内容 */}
</ShellB>
```

可选 `topbar={<TopbarB ...自定义.../>}` 替换默认顶栏。

### 3.2 SidebarB

固定宽 220px。三段：A 配置管理 / B 运行时预览 / C 系统。当前页有左侧 4px accent 色条。

### 3.3 TopbarB

```jsx
<TopbarB
  page="sources"          // 自动查标题 + 副标题
  title="可选覆盖"
  subtitle="可选覆盖"
  loadingState={null}     // 'validating' | 'saving' | null
  actions={<>...</>}      // 替代默认右侧按钮
/>
```

`loadingState` 控制顶栏「校验」「保存并热重载」按钮的 Spinner + 禁用。

### 3.4 卡片组件

| 组件 | 用法 |
|---|---|
| `StatCardB` | 数字大数字 + 标签 + 副标题，4 张并排 |
| `SectionB` | 区块容器，icon + 标题 + 计数徽标 + tag + 副标题 |
| `SourceCardB` | 订阅卡：拖拽柄 + URL + 状态点 + 操作按钮 |
| `CustomCardB` | 自定义代理卡：含中转链显示 |
| `AddButtonB` | 虚线 dashed `+ 添加 XX` |

### 3.5 按钮

```jsx
btnB('primary')    // accent 背景，白字，主操作
btnB('secondary')  // 白底，灰边，次操作
iconBtnB()         // 28×28 透明，仅图标
```

主按钮自带 indigo 阴影 `0 4px 12px rgba(99,102,241,0.25)`。

### 3.6 表单

```jsx
<FieldB label="名称">
  <input style={inputB()} />
</FieldB>
```

输入框统一 `36px` 高，`8px` 圆角，聚焦时 accent 色边框 + `0 0 0 3px` accent20% 光晕。

### 3.7 状态徽标

短文本 + 浅底色 + 同色文字。`fontSize: 11px, padding: 2px 8px, radius: 4–6px`。

```jsx
<span style={{
  fontSize: 11, padding: '2px 8px', borderRadius: 4,
  background: '#dcfce7', color: '#166534', fontFamily: B.mono, fontWeight: 600,
}}>success</span>
```

## 4. 交互模式规范

### 4.1 居中 Modal（pattern 1b）

- 遮罩 `rgba(15,23,42,0.45)` + `backdrop-filter: blur(2px)`
- 容器 `width: 520px`，最大高度 `85%`，溢出滚动
- 三段：标题区 / 表单区 / 操作区
- 操作区右对齐：`btnB('secondary')` 取消 + `btnB('primary')` 保存
- ESC 关闭，点遮罩关闭

### 4.2 Toast（pattern 2a/2b）

- 位置：`right: 24px; bottom: 24px;`
- 容器：`B.panel` 背景，`borderRadius: 10px`，`box-shadow: 0 12px 32px rgba(15,23,42,0.15)`
- 左侧 4px 色条标识类型（绿成功 / 红失败）
- 圆形 28px 图标 + 标题 + 副标题
- **成功** 4s 自动消失；**失败** 不自动消失，必带「查看详情 →」链接 + 「忽略」灰色链接

### 4.3 顶栏 Spinner（pattern 3a）

- 仅替换被点击的按钮：`Spinner` + 「校验中…」/「保存中…」+ `opacity: 0.7` + `cursor: not-allowed` + `disabled`
- 其它按钮不禁用；用户可继续点别的页面

### 4.4 居中 Confirm（pattern 4a）

- 同 Modal 容器，但宽 420px
- 左上 36px 圆形红底感叹号
- 标题「删除 XX？」+ 副标题点名要删的对象
- 右下两按钮：`btnB('secondary')` 取消 + 红色背景的「确认删除」（`background: #dc2626; borderColor: #dc2626`）

### 4.5 校验 Drawer（pattern 5b）

- 在校验页内右侧滑出 480px Drawer
- 头部显示「修复：< 错误信息 >」+ 关闭按钮
- 中部直接是该字段的编辑表单
- 底部「取消 / 保存并继续」
- 保存后自动重校 + 更新错误列表

## 5. Tweaks 可调项

后台所有页面通过 `Tweaks` 面板暴露：

- **明暗主题**：light / dark
- **强调色**：6 个预设色板
- **演示状态**（仅 A1/A8）：idle / add-modal / toast-success / toast-error / validating / saving / confirm-delete / a8-drawer

## 6. 配色避免清单

- ❌ 大面积渐变背景
- ❌ 多种强调色同时出现
- ❌ 阴影过重导致卡片"漂浮"
- ❌ 圆角 + 左侧彩色 border 的"AI 提示框"样式
- ❌ Emoji 作为正式图标（仅在 SectionB 装饰位允许，且整页统一）
