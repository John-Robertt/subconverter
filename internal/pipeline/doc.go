// Package pipeline 编排与输出格式无关的构建阶段：
// Source -> Filter -> Group -> Route -> ValidateGraph。
//
// 各阶段：
//   - Source：拉取订阅/Snell/VLESS 来源，解析 SS/AnyTLS/Snell/VLESS，
//     分配原始节点 ID，保留候选名称，转换自定义节点
//   - Filter：用 exclude 正则过滤拉取类节点
//   - Group：区域分组、分配链式节点 ID、链式分组、计算 @all
//   - Route：服务组、规则集、规则、fallback
//   - ValidateGraph：图级别的引用与命名空间校验
//   - Build：编排完整流水线（入口）
//
// 工具：
//   - URI 辅助函数：拆分 host:port、base64 解码
//   - 节点校验：检查生成节点的不变量
//
// 设计参考：docs/design/pipeline.md
package pipeline
