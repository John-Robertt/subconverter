// Package server 提供 HTTP 接口，包括公开的生成接口、管理 API、健康检查，
// 以及可选的内嵌 Web UI。
//
// 职责：
//   - 注册 HTTP 处理器（server.go）
//   - 校验请求参数并写出响应（handler.go）
//   - 把错误映射为 HTTP 状态码（errors.go）
//   - 内嵌 SPA 的 fallback 与静态资源缓存头（webui.go）
//
// 业务生成交给通过 [New] 注入的 Generator。管理 API 处理、会话校验和可选的
// Web 文件系统同样通过 Options 接入。运行时参数（-config、-listen、-cache-ttl、
// -timeout、-access-token）在 cmd/subconverter/main.go 中解析，并在组装时注入。
//
// 设计参考：docs/design/api.md
package server
