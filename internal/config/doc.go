// Package config 负责加载 YAML 配置、按声明顺序解析映射、
// 静态校验字段，以及启动时的预处理。
//
// 职责：
//   - 定义 Config 结构体并完成 YAML 反序列化
//   - 为 groups、routing、rulesets 提供 OrderedMap（保留声明顺序）
//   - YAML 加载器（本地文件，或经 fetch.LoadResource 读取 HTTP URL）
//   - 静态校验（必填字段、枚举值、正则编译）
//   - Prepare：校验配置完整性（名称唯一、routing 成员引用），
//     并产出启动时预处理好的 RuntimeConfig（已编译的正则、已解析的
//     自定义代理 URL、已展开的 @auto、静态命名空间、循环检测）。
//     请求阶段把 RuntimeConfig 当作只读输入使用。
//
// 设计参考：docs/design/config-schema.md
package config
