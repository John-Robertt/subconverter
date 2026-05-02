# 鉴权与安全契约

## token 输入

当 `/api/*` 返回 `401 token_required` 或 `401 token_invalid`：

- SPA 展示 token 输入对话框。
- 用户提交后重试原请求。
- token 默认只保存在内存。
- 用户选择“本次浏览器会话记住”时，可写入 `sessionStorage`。

不得默认持久保存 token。

当 `/api/*` 返回 `401 admin_auth_required`：

- SPA 不重复展示 token 输入对话框。
- 页面展示部署配置错误：Admin API 默认要求服务端配置 `-access-token` / `SUBCONVERTER_TOKEN`。
- 若用户确认只在本机或受信网络使用，可提示改用 `-allow-unauthenticated-admin` 或 `SUBCONVERTER_ALLOW_UNAUTHENTICATED_ADMIN=true` 显式开启无鉴权 Admin。

## 请求传递

- `/api/*` 只使用 `Authorization: Bearer <token>` header。
- 浏览器内调用 `/generate` 可使用 Authorization header。
- 复制给 Clash Meta 或 Surge 客户端的订阅链接可使用 query token，但必须由用户显式确认。

## 订阅链接确认

复制 `/generate?format=...&token=...` 前，UI 必须提示：

- token 会进入 URL。
- URL 可能出现在客户端配置、浏览器历史或代理日志中。
- 用户可以选择不附带 token，自行手动处理访问控制。

## 草稿预览安全边界

`POST /api/preview/*` 会按草稿配置实际拉取来源 URL。这是单用户信任模型下的设计行为，不作为 SSRF 防护边界。

前端需要明确展示“正在拉取订阅”或类似文案，避免用户误以为只是本地静态校验。

## 错误展示

涉及 URL 的错误展示必须使用后端返回的脱敏 URL 或摘要。前端不得自行把含 token 的原始 URL 拼入错误日志或 Toast。
