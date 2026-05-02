# 鉴权与安全契约

## 后台登录

Web 管理后台使用独立管理员登录态，不使用 `SUBCONVERTER_TOKEN` 作为后台权限。

- 直接访问 `/login` 渲染登录页。
- 未登录访问受保护页面时跳转 `/login?next=<原路径>`。
- 登录成功后跳转 `next` 或默认 `/sources`。
- Session 失效后，受保护接口返回 `401 auth_required` 或 `401 session_expired`，SPA 全局拦截并跳回 `/login`。
- 后端首次启动且无管理员凭据时，`/login` 渲染 setup 模式，并要求输入 bootstrap setup token。

## 认证接口

正式接口：

- `GET /api/auth/status`：返回 `authed`、`setup_required`、`setup_token_required`、`locked_until`。
- `POST /api/auth/login`：提交 `username`、`password`、`remember`。
- `POST /api/auth/setup`：首次创建管理员账号，提交 `username`、`password`、`setup_token`。
- `POST /api/auth/logout`：注销当前 session。

Cookie：`session_id`，HttpOnly、SameSite=Lax、Path=/；HTTPS 下必须 Secure。

## 登录状态

| 状态 | 前端行为 |
|------|----------|
| `idle` | 表单可输入，提交按钮可点 |
| `validating` | 按钮显示加载态，表单禁用 |
| `invalid_credentials` | 密码框展示错误，提示剩余尝试次数 |
| `auth_locked` | 展示锁定截止时间，禁用提交 |
| `redirecting` | 展示登录成功并跳转 |
| `network_error` | 展示连接错误和重试动作 |
| `setup` | 增加 setup token、确认密码和密码强度提示，提交创建管理员；`401 setup_token_required` / `setup_token_invalid` 展示 token 错误 |

失败次数由后端按 IP + 用户名联合计数。第 5 次失败后后端返回 `423 auth_locked`，默认锁定 15 分钟，UI 只读显示后端返回的解锁时间。

## Session 持久性

- 未选择“记住我”：session 最长 24 小时。
- 选择“记住我”：session 最长 7 天。
- 前端不持久保存密码、session id 或订阅访问 token。
- 主题选择可以写入 `localStorage`，但不属于认证材料。

## Setup Bootstrap

- 首次 setup 必须提交 bootstrap setup token，防止公网首次启动时被抢先初始化。
- 生产部署推荐通过 `SUBCONVERTER_SETUP_TOKEN` 显式配置 token；完成 setup 后可移除该环境变量并重启。
- 未显式配置时，服务启动会生成一次性 32-byte URL-safe token 并只打印到服务日志；前端只能提示用户去部署日志查看，不能通过 HTTP 获取 token。
- setup token 只用于首次创建管理员。管理员凭据存在后，再次调用 setup 返回 `409 setup_not_allowed`。

## Auth State 存储

- 管理员密码使用 `PBKDF2-HMAC-SHA256`，当前参数为 `600000` iterations、32-byte random salt、32-byte derived key。
- auth state 只保存密码哈希和 session token 的 SHA-256 哈希，不保存明文密码或明文 session id。
- auth state 目录权限为 `0700`，文件权限为 `0600`，写入必须使用同目录临时文件、fsync 和 rename。

## 订阅链接确认

`SUBCONVERTER_TOKEN` 只用于 `/generate` 客户端订阅更新，不用于后台登录。

复制订阅链接时，前端调用 `GET /api/generate/link`，由后端根据 `base_url`、格式、文件名和订阅访问 token 返回完整 URL。前端不得自行持有或拼接 `SUBCONVERTER_TOKEN`。

复制 `token_included=true` 的链接前，UI 必须提示：

- token 会进入 URL。
- URL 可能出现在客户端配置、浏览器历史或代理日志中。
- 用户可以选择不附带 token，自行手动处理访问控制。

浏览器内下载按钮可凭当前 session Cookie 调用 `/generate`，不需要把订阅 token 写入前端状态。

## CSRF 边界

管理接口使用 Cookie session，因此所有会修改状态的请求必须保持同源。正式部署通过 nginx 同源反向代理实现；本地开发优先使用 Vite proxy。

后端应对非安全方法校验同源 `Origin` 或 `Referer`。前端不依赖跨域 Cookie 调试作为正式工作流。

## 草稿预览安全边界

`POST /api/preview/*` 会按草稿配置实际拉取来源 URL。这是单用户信任模型下的设计行为，不作为 SSRF 防护边界。

前端需要明确展示“正在拉取订阅”或类似文案，避免用户误以为只是本地静态校验。

## 错误展示

涉及 URL 的错误展示必须使用后端返回的脱敏 URL 或摘要。前端不得自行把含 token 的原始 URL 拼入错误日志或 Toast。
