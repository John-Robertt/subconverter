# subconverter-go Admin · 开发文档

> 工程结构 + 运行 / 调试 / 扩展指南 · 版本 v1.0

---

## 1. 技术栈

- **React 18.3.1**（UMD 版本，无打包工具）
- **Babel Standalone 7.29**（浏览器内编译 JSX）
- **纯 HTML/JSX**：单一入口 `subconverter Admin.html`，所有逻辑分散在多个 `.jsx` 文件，通过 `<script type="text/babel" src>` 串行加载
- **零构建**：双击 HTML 即可在浏览器打开。也可丢到任意静态服务器。

> 这是**设计稿 / 高保真原型**，不是生产前端工程。要接到真实 subconverter-go 后端，按"接入真实后端"章节改造。

## 2. 文件结构

```
subconverter Admin.html       ← 入口 HTML，按顺序加载所有脚本
├ design-canvas.jsx           ← 设计画布外壳（来自 starter）
├ tweaks-panel.jsx            ← Tweaks 面板（来自 starter）
├ mock-data.jsx               ← 节点 / 分组 / 路由等 mock 数据
├ mock-data-extra.jsx         ← 校验错误、热重载历史等扩展 mock
├ direction-b.jsx             ← Shell + Sidebar + Topbar + Sources + 通用组件
├ screens-config.jsx          ← A2–A7 编辑类页面（过滤器/分组/路由/规则集/规则/其他）
├ screens-runtime.jsx         ← A8 校验、B1 节点预览、B2 分组预览、C 系统状态
├ demos-modal.jsx             ← Modal/Drawer/Confirm 演示组件
├ demos-feedback.jsx          ← Toast/Banner/Spinner/Drawer 演示组件 + Spinner 实现
├ interaction-layer.jsx       ← StatefulArtboard：state-driven 交互覆盖层
└ app.jsx                     ← 主入口：组装所有 artboard，连接 Tweaks
docs/
├ PRD.md                      ← 产品需求文档
├ Design-Spec.md              ← 设计规范
└ Dev-Guide.md                ← 本文件
```

### 2.1 加载顺序

`subconverter Admin.html` 里 script 顺序很重要：

```html
1. React + ReactDOM + Babel Standalone（CDN，钉版本）
2. design-canvas.jsx       ← DCSection / DCArtboard
3. tweaks-panel.jsx        ← TweaksPanel + useTweaks + Tweak* 控件
4. mock-data.jsx           ← NODES / GROUPS / ROUTING / SOURCES …
5. mock-data-extra.jsx     ← VALIDATION_ERRORS / RELOAD_HISTORY
6. direction-b.jsx         ← B 命名空间所有基础组件
7. screens-config.jsx      ← A2–A7
8. screens-runtime.jsx     ← A8 / B / C
9. demos-modal.jsx         ← Demo* Modal 系列
10. demos-feedback.jsx     ← Demo* Toast/Spinner/Drawer 系列
11. interaction-layer.jsx  ← StatefulArtboard
12. app.jsx                ← 入口 <App />
```

每个 `.jsx` 文件末尾会 `Object.assign(window, { ... })` 把组件挂到全局，因为 Babel 每个 script 是独立 scope。

### 2.2 命名约定

- **B 后缀** = direction-B 的视觉系统下的组件（`SourcesB`, `TopbarB`, `btnB(...)`, `iconBtnB()`, ...）
- **DC 前缀** = Design-Canvas 提供的（`DCArtboard`, `DCSection`）
- **Tweak 前缀** = TweaksPanel 提供的（`TweakSection`, `TweakRadio`, `TweakSelect`...）
- **Demo 前缀** = 早期交互模式探索的演示组件，最终保留供 `interaction-layer.jsx` 复用（如 `DemoJumpDrawer`）

## 3. 运行

直接：

```bash
# 任选一种
open subconverter\ Admin.html
python3 -m http.server 8000   # 然后 http://localhost:8000
npx serve .
```

不需要 `npm install`。

## 4. 调试

### 4.1 切换演示状态

打开右上角 **Tweaks** 面板（toolbar 切换）：

| Tweak | 说明 |
|---|---|
| 演示状态 | 在 A1 / A8 上叠加交互覆盖层（modal/toast/spinner/confirm/drawer） |
| 主题 | light / dark |
| 强调色 | 6 个预设 |

### 4.2 设计画布操作

- 滚轮缩放、空格拖动平移
- 单击 artboard 标题进入 fullscreen
- ←/→ 在 fullscreen 之间切换
- ESC 退出 fullscreen

### 4.3 直接修改 artboard 顺序

`app.jsx` 里每个 `<DCArtboard id="..." label="...">` 即一张设计稿，调整 JSX 顺序即可。

## 5. 扩展

### 5.1 加一个新页面（比如 A9）

1. 在 `screens-config.jsx` 末尾写：
   ```jsx
   function MyNewPageB(props) {
     return (
       <ShellB {...props} page="mynew">
         <div style={{ padding: '28px 32px' }}>
           {/* 内容 */}
         </div>
       </ShellB>
     );
   }
   Object.assign(window, { MyNewPageB });
   ```
2. 在 `direction-b.jsx` 的 `SidebarB` 三段菜单里加一项 `{ id: 'mynew', label: 'A9 我的新页面' }`。
3. 在 `TopbarB` 的 `titles / subs` 字典里加 `mynew`。
4. 在 `app.jsx` 里加：
   ```jsx
   <DCArtboard id="a9-mynew" label="A9 我的新页面" width={W} height={H}>
     <MyNewPageB {...p} />
   </DCArtboard>
   ```

### 5.2 加一种新的交互模式

1. 在 `interaction-layer.jsx` 的 `InteractionLayer` 里加一段 `{state === 'my-new-state' && (...)}`。
2. 在 `app.jsx` 的 `<TweakSelect>` 里加该选项。
3. 必要时在 `direction-b.jsx` 的 `TopbarB` 里读 `loadingState` 加新分支。

### 5.3 改默认强调色

编辑 `app.jsx`：

```jsx
const TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{
  "theme": "light",
  "accent": "#0ea5e9",   // 改这里
  "demoState": "idle"
}/*EDITMODE-END*/;
```

## 6. 接入真实后端

当前所有数据都来自 `mock-data.jsx`。要接 subconverter-go 真实后端：

### 6.1 数据层

新建 `api.js`：

```js
const API_BASE = 'http://localhost:25500/admin'; // 或环境变量

export async function fetchSources()  { return (await fetch(`${API_BASE}/sources`)).json(); }
export async function saveSources(s)  { return fetch(`${API_BASE}/sources`, {method:'PUT', body:JSON.stringify(s)}); }
export async function validate()      { return (await fetch(`${API_BASE}/validate`)).json(); }
export async function reload()        { return fetch(`${API_BASE}/reload`, {method:'POST'}); }
export async function fetchNodes()    { return (await fetch(`${API_BASE}/nodes`)).json(); }
// ...
```

### 6.2 状态管理

把每个页面组件改成读 `useState + useEffect`：

```jsx
function SourcesB(props) {
  const [sources, setSources] = React.useState(null);
  React.useEffect(() => { fetchSources().then(setSources); }, []);
  if (!sources) return <Loading/>;
  // 用 sources 替代 mock SOURCES
}
```

如果项目变大，建议引入 Zustand 或 React Query。

### 6.3 后端要求

subconverter-go 需补上以下 HTTP 端点（建议挂在 `/admin/*`，与现有 `/generate` 平行）：

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | /admin/config | 读完整 config.yaml（结构化） |
| PUT | /admin/sources | 写订阅区块 |
| PUT | /admin/groups | 写分组区块 |
| PUT | /admin/routing | 写路由区块 |
| ... | ... | 其他 A2–A7 区块 |
| POST | /admin/validate | 校验当前 config，返回 errors/warnings/infos |
| POST | /admin/reload | 触发热重载 |
| GET | /admin/nodes | 拉取所有真实节点（B1） |
| GET | /admin/groups/expanded | 分组展开后的真实成员（B2） |
| GET | /admin/health | 进程状态、版本、热重载历史（C） |

校验响应 schema：

```ts
{
  errors:   { id, severity:'error',   page, field, message, detail }[],
  warnings: { id, severity:'warning', page, field, message, detail }[],
  infos:    { id, severity:'info',    page, field, message, detail }[],
}
```

### 6.4 热重载语义

- `POST /admin/reload` 必须先做一次内部校验，失败则返回 4xx + errors 数组，**不替换正在跑的配置**。
- 成功返回耗时 + 摘要，给 Toast 显示。
- 后端应保留最近 N 份配置，提供 `POST /admin/rollback?version=...`。

### 6.5 鉴权

至少加 token：

```
Authorization: Bearer <token>
```

token 通过环境变量或启动参数注入 subconverter-go。前端从 `localStorage.adminToken` 读取，登录页/弹窗输入。

## 7. 已知限制

- 仅设计稿，没有真实持久化、网络请求、错误处理
- 设计宽度固定 1280×820，未做响应式
- 深色主题做了基础适配，但未对所有徽标 / 状态色做完整审视
- 没接 i18n，文案硬编码中文

## 8. 依赖版本（钉死）

```html
react@18.3.1
react-dom@18.3.1
@babel/standalone@7.29.0
```

升级时同步更新 SRI hash（HTML 里 `integrity` 字段）。

## 9. 调试技巧

- 控制台报「XXX is not defined」基本都是 `Object.assign(window, ...)` 漏掉了组件名
- 如果改了 `TWEAK_DEFAULTS` 的 JSON 块发现没生效，注意 `EDITMODE-BEGIN` / `EDITMODE-END` 标记必须保留，且块内必须是合法 JSON（双引号）
- 改样式无效时，搜一下 `<style id="__om-edit-overrides">`，可能是直接编辑产生的 `!important` 覆盖
