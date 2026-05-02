# v1.0 文档归档

本目录是 subconverter v1.0 文档的冻结快照，保留纯 API 后端阶段的完整设计记录。

**v1.0 范围**：Go 单二进制 HTTP 服务，读取 YAML 配置 → 拉取订阅 → 生成 Clash Meta / Surge 配置文件。无 Web 前端、无配置热重载、无运行时管理 API。

**v2.0 变更**：新增 Web 管理后台（React SPA）、配置 CRUD API、运行时热重载、节点/分组预览。当前权威文档见 `docs/`（上级目录）。

**快照说明**：本文档树是 v1.0 设计在 TargetError/RenderError 拆分完成后的冻结快照，不会随 v2.0 开发继续更新。v2.0 完整设计见上级 `docs/` 目录。

---

## 文件索引

| 文件 | 内容 |
|------|------|
| `product-spec.md` | 产品规格：输入素材、面板结构、输出格式、配置草案 |
| `architecture.md` | 系统架构：运行模型、管道模型、模块边界、关键决策 |
| `deployment.md` | 构建部署：CI/CD、Docker、二进制分发、健康检查 |
| `design/api.md` | HTTP API：`/generate` + `/healthz`、错误语义、运行参数 |
| `design/caching.md` | 缓存策略：订阅/模板 TTL 缓存 |
| `design/config-schema.md` | 配置模式：YAML 字段规约与校验规则 |
| `design/domain-model.md` | 领域模型：Proxy、ProxyGroup、Ruleset、Rule、Pipeline |
| `design/pipeline.md` | 管道设计：9 阶段职责与数据流 |
| `design/rendering.md` | 渲染映射：Clash Meta / Surge 输出格式 |
| `design/validation.md` | 校验规则：静态 / 构建期 / 图级三层校验 |
| `implementation/implementation-plan.md` | 开发计划：M0-M5 里程碑（全部已完成） |
| `implementation/project-structure.md` | 项目结构：包布局与依赖约束 |
| `implementation/testing-strategy.md` | 测试策略：编号体系与覆盖策略 |
