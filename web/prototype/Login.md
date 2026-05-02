# 登录页 · 设计与交互文档

居中卡片版式 · 密码登录 · Cookie session

## 1. 入口与跳转

| 触发 | 行为 |
|---|---|
| 直接访问 `/login` | 渲染登录页 |
| 未登录访问任何页面 | 重定向到 `/login?next=<原路径>` |
| 登录成功 | 跳转到 `next` 或默认 `/sources`（A1） |
| Session 失效（401） | 全局拦截，踢回 `/login`，提示"登录已过期" |
| 后端首次启动且无凭据 | 强制走 setup 流程（`/login` 自动渲染 setup 模式） |

## 2. 版式

居中卡片，宽 380 / 圆角 16 / 软阴影。卡片上方是品牌区（Logo 文字标 "subconverter"），右上角主题切换（浅/深），底部页脚版本号 + 文档/GitHub。

背景：单色 + 顶部一团 accent 色径向渐变光晕（柔和氛围）。

## 3. 字段

| 字段 | 说明 |
|---|---|
| 用户名 | 默认占位 admin |
| 密码 | 右侧 👁 切显，type=password |
| Setup Token | 仅 setup 模式展示，从 `SUBCONVERTER_SETUP_TOKEN` 或服务启动日志获得 |
| 记住我 | 选中后 cookie 7 天有效；不选 24 小时 |
| 忘记密码？ | 链接到文档（subconverter 自部署，不内置邮箱重置） |

## 4. 七种状态

| state | 触发 | UI 变化 |
|---|---|---|
| **idle** | 默认 | 表单可输入，提交按钮可点 |
| **validating** | 提交后 / 校验中 | 按钮显示 spinner + "正在验证…"，整表单 disabled |
| **wrong-pwd** | 401 返回 | 密码框红框 + 红色 box-shadow，下方提示"用户名或密码错误 · 还可尝试 N 次" |
| **locked** | 5 次失败 / 后端返回 423 | 顶部红色 🔒 banner 显示解锁时间，按钮变灰禁用 |
| **redirecting** | 200 返回后 | 按钮 spinner + "登录成功 · 正在跳转…"，~600ms 后跳页 |
| **network-err** | fetch failed / 超时 | 顶部固定红条 banner + 重试按钮，表单 disabled |
| **setup** | `GET /api/auth/status` 返回 `setup_required: true` | 顶部绿色提示，多一个"Setup Token"和"确认密码"字段，按钮文案变"创建管理员并登录"，密码区显示强度条 |

## 5. 交互细节

- **提交方式**：Enter 键、按钮点击、表单 submit 都触发
- **错误次数**：第 3 次失败后提示"还可尝试 N 次"；第 5 次后服务端锁定
- **锁定时长**：默认 15 分钟，由后端控制；UI 只读显示
- **失焦校验**：用户名 / 密码必填，离焦时检查；setup 模式下密码强度实时计算
- **Caps Lock**：密码框聚焦且 Caps Lock 开启时，下方显示"⇪ Caps Lock 已开启"（已留位）
- **主题切换**：右上角小开关，写入 `localStorage`，登录后跟随到主应用

## 6. 接口契约

```
GET  /api/auth/status        → { authed, setup_required, setup_token_required, locked_until? }
POST /api/auth/login         { username, password, remember }
                             → 200 { redirect } | 401 { remaining } | 423 { until }
POST /api/auth/setup         { username, password, setup_token }    （仅首次）
POST /api/auth/logout
```

Cookie：`session_id`（HttpOnly + SameSite=Lax；HTTPS 下 Secure）

## 7. 安全约束

- 密码至少 12 位（setup 时强制）
- 首次 setup 必须提交 bootstrap setup token；前端不能通过 HTTP 获取自动生成的 token，只提示用户查看服务日志
- 失败次数按 IP+用户名联合计数
- 锁定状态对所有路径生效（不仅 /login）
- Session 失效后所有受保护接口返回 401，前端全局拦截

## 8. 已实现状态（高保真稿）

`screen-login.jsx` 单文件，靠 `state` prop 驱动 7 个分支。已接入画布顶部「认证 · 登录页」section，右下 Tweaks → 登录页状态 切换。
