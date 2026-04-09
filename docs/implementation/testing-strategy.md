# 测试策略

## 目标

本文件定义系统在实现阶段需要覆盖的核心验证范围，确保管道、分组和渲染行为稳定。

---

## 单元测试

建议覆盖：

- 保序映射解析
- SS URI 解析
- SIP002 明文 `userinfo`
- SS plugin query 解析与转义处理
- 订阅过滤
- 地区节点组匹配
- `relay_through` 三种模式展开
- `@all` 不包含链式节点
- `@auto` 展开为节点组+@all 服务组+DIRECT，去重且排除自身
- `REJECT` 不在 `@auto` 中，需显式声明且位置保持不变
- 同一 entry 内重复 `@auto` 会被静态校验拒绝
- `@auto` 与 `@all` 在同一 entry 中互斥
- `Route(cfg, nil)` 按空 `GroupResult` 处理，不发生 panic
- `routing` 不允许显式引用原始代理名
- 代理名、节点组名、服务组名共享命名空间无冲突
- 服务组引用校验
- 循环引用校验

---

## 渲染测试

建议覆盖：

- Clash Meta 输出快照
- Surge 输出快照
- 链式节点渲染字段
- Clash Meta 的通用 SS plugin 透传
- Surge 对不支持 SS plugin 的错误路径
- ruleset 输出顺序
- fallback 输出位置
- Clash / Surge 的 `url-test` 默认参数一致

---

## 集成测试

建议覆盖：

- 从示例配置生成 Clash Meta
- 从示例配置生成 Surge
- 真实订阅样本的解析回归
- 订阅拉取失败场景
- 配置非法场景

---

## 验收重点

- 面板顺序与配置书写顺序一致
- 同一份配置在两种输出中语义一致
- 链式组出现在节点组层，而不是服务组层
- 所有节点组都显式指定策略
