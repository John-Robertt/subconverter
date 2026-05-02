# 验收契约

## 页面验收

正式 SPA 需要覆盖：

- A1-A8、B1-B3、C 全部路由可访问。
- `/login` 可访问，未登录访问受保护路由会跳转 `/login?next=<原路径>`。
- B3 页面路由为 `/download`，刷新 `/download` 由 SPA fallback 返回页面。
- `/generate?format=clash|surge` 继续作为后端生成接口经 nginx 反向代理，不被 SPA 路由接管。
- 1280x800 下无文本重叠、按钮溢出或关键内容不可见。
- 所有页面具备 loading、empty、error、readonly 状态。
- 长 URL、长正则、长节点名和 emoji key 显示稳定。
- 浅色与深色主题均可用，1280x800 下都无文本重叠、按钮溢出或关键内容不可见。

## API 行为验收

必须覆盖：

- 未登录访问 `/api/config` 等受保护接口返回 `401 auth_required`，前端跳转登录页并保留 `next`。
- Session 过期返回 `401 session_expired`，前端提示“登录已过期”并跳转登录页。
- 登录密码错误返回 `401 invalid_credentials`，登录页展示剩余尝试次数。
- 登录失败锁定返回 `423 auth_locked`，登录页展示解锁时间且禁用提交。
- 首次无管理员凭据时 `/login` 进入 setup 模式；缺少或错误 setup token 时不能创建管理员；auth state 不可写时展示部署配置错误。
- `409 config_revision_conflict` 不覆盖草稿，并提供重新加载配置或手动合并入口。
- `409 config_source_readonly` 进入只读模式，并禁用保存、新增、删除和排序。
- `409 config_file_not_writable` 展示文件权限或部署挂载问题，并保留草稿。
- 429 reload in progress 展示退避重试或可重试提示。
- 502 上游拉取失败按接口上下文展示。
- API 不可用时显示连接错误，不出现空白页面。

## 工作流验收

必须覆盖：

- `GET /api/config` 加载草稿。
- `POST /api/config/validate` 只做静态校验；配置无效时返回 `200 valid=false`，请求体格式错误才返回 400。
- `PUT /api/config` 成功后仍需 reload 才生效。
- reload 失败后保持 dirty 提示。
- reload 成功后刷新 `status`，B1/B2/B3 当前运行时预览使用新的 `runtime_config_revision` 重新加载。
- B1/B2 GET 预览使用当前运行时配置。
- B1/B2/B3 当前运行时预览提供手动“刷新预览”入口；刷新时重新请求后端，不依赖 `runtime_config_revision` 变化。
- 当前运行时预览不做后台轮询；远程订阅或模板在 TTL 过期后的变化由用户手动刷新或重新进入页面触发观测。
- A2/A3 POST 预览使用草稿配置。
- B3 当前预览与草稿预览能明确区分。
- A1 保存并回读后 `sources.fetch_order` 不变，A6 拖拽后 `rules` 顺序不变。

## 安全验收

必须覆盖：

- 管理后台不接受 `SUBCONVERTER_TOKEN` 作为 `/api/*` 权限凭据。
- 首次 setup 必须校验 bootstrap setup token，前端不能通过 HTTP 获取自动生成的 token。
- 登录成功后使用 HttpOnly `session_id` Cookie；前端不保存密码、session id 或订阅访问 token。
- “记住我”只影响 session 有效期：未选最长 24 小时，选中最长 7 天。
- `/api/*` 请求不把订阅访问 token 放入 query 或 Authorization header。
- 复制含 token 的 `/generate` 链接前出现确认。
- 订阅链接通过 `GET /api/generate/link` 由服务端生成，前端不自行拼接 `SUBCONVERTER_TOKEN`。
- 错误展示不泄漏原始含 token URL。
- 未登录时，`/api/*` 无法被前端静默绕过。

## 主题验收

必须覆盖：

- 跟随系统偏好时，浅色/深色主题随 `prefers-color-scheme` 生效。
- 用户手动切换主题后，刷新页面保持手动选择。
- 浅色和深色主题下，状态色语义一致，focus ring、代码预览、诊断列表和错误详情均可读。
- 深色主题下所有页面的 loading、empty、error、readonly、dirty 状态均可识别。

## 文档验收

正式文档不得重新引入旧原型契约，包括旧管理路径、持久化 token 键名、旧配置恢复接口、固定数量的版本保存承诺和按页面区块写回方案。
