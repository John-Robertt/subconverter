# v3.0 文档入口

> 状态：v3.0 目标契约。不要求延续既有后台 API、包结构或实现边界。

v3.0 的设计目标是支撑产品细节长期打磨。架构优先保护演化效率：新增来源、协议、目标格式、诊断、预览和 UI 工作流时，改动应集中、风险应可测试、状态应可解释。

## 文档地图

| 主题                   | 文档                                                                             |
| ---------------------- | -------------------------------------------------------------------------------- |
| 总体架构、分层和数据流 | [architecture.md](architecture.md)                                               |
| 产品原则               | [product/product-principles.md](product/product-principles.md)                   |
| 工作台状态语义         | [product/configuration-workbench.md](product/configuration-workbench.md)         |
| 信息架构               | [product/information-architecture.md](product/information-architecture.md)       |
| 核心模型               | [design/core-model.md](design/core-model.md)                                     |
| 配置结构               | [design/config-schema.md](design/config-schema.md)                               |
| 应用服务               | [design/application-services.md](design/application-services.md)                 |
| 管道阶段               | [design/pipeline.md](design/pipeline.md)                                         |
| 运行时快照             | [design/runtime-snapshot.md](design/runtime-snapshot.md)                         |
| 能力注册表             | [design/capability-registry.md](design/capability-registry.md)                   |
| 诊断模型               | [design/diagnostics.md](design/diagnostics.md)                                   |
| 目标格式投影           | [design/target-projection.md](design/target-projection.md)                       |
| 渲染                   | [design/rendering.md](design/rendering.md)                                       |
| 配置与资源 I/O         | [design/config-io.md](design/config-io.md)                                       |
| 远程资源读取           | [design/resource-adapter.md](design/resource-adapter.md)                         |
| 校验                   | [design/validation.md](design/validation.md)                                     |
| API                    | [design/api.md](design/api.md)                                                   |
| Web UI                 | [design/web-ui.md](design/web-ui.md)                                             |
| 目标包结构             | [engineering/project-structure.md](engineering/project-structure.md)             |
| 实施顺序               | [engineering/implementation-sequence.md](engineering/implementation-sequence.md) |
| 测试策略               | [engineering/testing-strategy.md](engineering/testing-strategy.md)               |
| 部署原则               | [deployment.md](deployment.md)                                                   |

## 设计边界

- 根部 `docs/` 是 v3.0 的唯一目标契约。
- 归档目录只作为资料留存，不参与 v3.0 架构决策。
- API、文档和包结构按 v3.0 重新定义；旧实现只能作为可复用素材，不能成为结构约束。
