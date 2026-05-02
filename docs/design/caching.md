# 缓存设计

> 状态提示：§热重载时的缓存行为为 v2.0 新增契约。

## 目标

本文件定义远程资源拉取时的缓存范围和约束。

---

## 缓存对象

系统缓存以下远程资源的拉取结果：

- 订阅 URL（SS 节点列表）
- 模板 URL（底版配置文件，当 `templates.clash` / `templates.surge` 为 HTTP(S) URL 时）
- 配置文件 URL（主配置文件，当 `-config` 为 HTTP(S) URL 时）

三者共享同一个 `CachedFetcher` 实例和 TTL 参数，但主配置文件在热重载时有额外刷新规则，见下文。

不缓存：

- 规则集 URL 内容
- 本地文件（通过 `os.ReadFile` 直接读取）

原因：

- 规则集内容最终由 Clash Meta 或 Surge 客户端在运行时拉取和消费
- 服务端不解析规则集正文，因此缓存它没有收益，反而会扩大复杂度

---

## 缓存模型

- 键：远程资源 URL
- 值：响应体、拉取时间
- 类型：进程内 TTL 缓存

设计目标：

- 降低重复请求订阅源、远程模板和远程配置源的开销
- 避免引入外部存储依赖
- 保持单用户部署简单性

---

## 约束

- TTL 过期后重新拉取
- 缓存失效不影响功能正确性，只影响性能和远端请求次数
- 缓存是实现优化，不改变生成语义

---

## 热重载时的缓存行为（v2.0）

`POST /api/reload` 触发 re-LoadConfig + re-Prepare。为保证远程配置源的热重载语义，主配置文件 URL 必须强制刷新：

- 当 `-config` 是 HTTP(S) URL 时，reload 读取主配置必须 bypass 当前缓存，或先 invalidate 该配置 URL 的缓存项再拉取
- 这样即使 `-cache-ttl` 未过期，reload 也能看到远端最新配置
- reload 成功后，新拉取到的主配置内容可以重新写入缓存，供后续非 reload 读取复用

reload 不主动清除订阅和模板缓存：

- 订阅 URL 未变化时，TTL 内的缓存仍有效，避免重复拉取
- 若用户更换了订阅 URL，新 URL 本身就是新的缓存键，不存在脏数据；旧 URL 的缓存条目 TTL 过期后不再命中，但当前契约不要求后台主动清理
- 若需强制刷新订阅内容，等待 TTL 过期后下一次 `/generate` 或 `/api/preview/nodes` 请求会自动重新拉取
- 模板 URL 同理：reload 不主动刷新模板；模板在生成或预览渲染时按 TTL 读取

预览请求（`/api/preview/nodes`、`/api/preview/groups`）与 `/generate` 共享同一 `CachedFetcher`，不引入独立的缓存实例。

---

## CachedFetcher API 扩展（v2.0 前置）

当前 `CachedFetcher` 仅暴露 `Fetch(ctx, rawURL)` 方法。热重载要求 bypass/invalidate 特定 URL 的缓存，需在 M6 实现前新增以下能力：

```go
// Invalidate 移除指定 URL 的缓存项。
// 下一次 Fetch 将重新拉取远程资源。
func (c *CachedFetcher) Invalidate(rawURL string)
```

使用场景：`POST /api/reload` 在调用 `LoadConfig` 前，先 `Invalidate` 主配置 URL，确保 reload 读到远端最新内容。

设计约束：

- `Invalidate` 只移除单条缓存项，不清空全部缓存
- 不新增 `BypassCache` / `ForceRefresh` 等复杂模式——invalidate-then-fetch 已满足 reload 需求，且语义更简单
- 订阅和模板缓存不受 reload 影响（见上文"reload 不主动清除订阅和模板缓存"）
