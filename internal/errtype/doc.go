// Package errtype 定义 subconverter 流水线中通用的六类错误。
//
// 每类错误对应一个固定的 HTTP 状态码（由 server 层负责映射）：
//
//   - ConfigError  → 400（YAML 无效、缺少字段、正则错误）
//   - FetchError   → 502（拉取远程资源失败）
//   - ResourceError → 500（读取本地资源失败）
//   - BuildError   → 400（分组、路由或配置语义错误）
//   - TargetError  → 500（目标格式投影失败）
//   - RenderError  → 500（生成输出失败）
//
// 设计参考：docs/design/validation.md
package errtype
