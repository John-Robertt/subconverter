// Package model 定义各流水线阶段共用的、与输出格式无关的 ID 关系图，
// 以及渲染器使用的目标投影结果。
//
// 核心实体：
//   - Proxy：          一个带编号的代理节点
//   - NodeGroup：      由代理编号组成的有序分组
//   - RouteGroup：     由带类型的策略引用组成的有序分组
//   - Pipeline：       一次生成过程中与格式无关的 ID 关系图
//   - TargetPipeline： 渲染器的输入，带有目标客户端内的最终名字
//
// 设计参考：docs/design/domain-model.md
package model
