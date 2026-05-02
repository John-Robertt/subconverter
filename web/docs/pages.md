# 页面契约

## 路由

正式 SPA 使用 React Router，路由结构与 `docs/design/web-ui.md` 保持一致。本文件只维护路由总览；逐页字段、动作、状态和测试矩阵见 [`page-specs.md`](page-specs.md)。

| 路由 | 页面 | 数据来源 |
|------|------|----------|
| `/sources` | A1 订阅来源 | `GET /api/config` 的 `sources` |
| `/filters` | A2 过滤器 | 草稿 config，预览用 `POST /api/preview/nodes` |
| `/groups` | A3 节点分组 | 草稿 config，预览用 `POST /api/preview/groups` |
| `/routing` | A4 路由策略 | `routing` + 可引用成员集合 |
| `/rulesets` | A5 规则集 | `rulesets` |
| `/rules` | A6 内联规则 | `rules` |
| `/settings` | A7 其他配置 | `fallback` / `base_url` / `templates` |
| `/validate` | A8 静态配置校验 | `POST /api/config/validate` |
| `/nodes` | B1 节点预览 | `GET /api/preview/nodes` |
| `/preview/groups` | B2 分组预览 | `GET /api/preview/groups` |
| `/download` | B3 生成下载 | `GET/POST /api/generate/preview` + `/generate` |
| `/status` | C 系统状态 | `GET /api/status` + `GET /healthz` |

`/generate` 只表示后端生成接口和订阅链接路径，不作为 SPA 页面路由。

## 页面状态

每个页面必须覆盖：

- `loading`：显示骨架或明确加载态。
- `empty`：字段或列表为空时给出可执行入口。
- `error`：展示 HTTP 状态、错误码和可重试动作。
- `readonly`：配置源不可写时禁用保存、删除、排序和新增。
- `dirty`：已保存配置与运行时配置不一致时提示用户 reload。

逐页状态落点由 [`page-specs.md`](page-specs.md) 定义。实现页面时不得只满足本节通用描述，必须同时满足对应页面矩阵中的字段、API、只读行为、dirty 行为和测试映射。

## A 区编辑页

- 编辑页操作前端草稿，不直接修改运行时配置。
- A2/A3 草稿预览会联网拉取来源，必须显示明确加载文案。
- A4 成员选择器必须支持节点组、服务组、`DIRECT`、`REJECT`、`@all`、`@auto`。
- A8 诊断跳转以 `locator.json_pointer` 为定位依据，不能依赖展示路径反查字段。

## B 区预览页

- GET 预览只读取当前 `RuntimeConfig`。
- POST 预览只读取前端草稿，不写文件、不替换运行时配置。
- B2 若 ValidateGraph 返回图级错误，不展示部分成功结果。
- B3 必须区分“当前运行时生成预览”和“草稿生成预览”。

## C 区状态页

- 展示版本、配置源、可写性、revision、dirty 和最近 reload 结果。
- 不把 `GET /api/status` 解释为远程配置源实时探测。HTTP(S) 配置源的 revision 基于最近观测结果。
