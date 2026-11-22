// Package generate 承载“生成配置”这一用例的应用服务。
//
// 它负责请求期间的整套编排：构建与格式无关的 Pipeline，投影到目标格式，
// 加载模板，最后渲染输出。HTTP 参数校验这类传输层事务留在 internal/server；
// 加载和校验 YAML 这类启动期事务留在 cmd/subconverter 与 internal/config。
package generate
