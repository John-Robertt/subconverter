// Package render 把 TargetPipeline 序列化为各客户端专用的配置文本。
//
// 已实现的渲染器：
//   - Clash Meta YAML（render.Clash）
//   - Surge conf（render.Surge）
//   - Loon conf（render.Loon）
//
// 按目标客户端过滤协议、裁剪关系图的工作在 internal/target 中完成，
// TargetPipeline 到达本包之前已经处理好。三个渲染器都接受可选的底版模板：
// 提供模板时，生成的段落（节点、分组、规则）注入模板，用户自定义的设置全部保留；
// 不提供时，只输出生成的段落。
//
// 设计参考：docs/design/rendering.md
package render
