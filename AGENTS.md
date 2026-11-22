# subconverter · Agent 工作指南

这份文件帮你在本仓库里少走弯路。它讲五件事：这个项目做什么、去哪里查事实、改一处要连带改哪些地方、怎样确认改对了、哪些事先问用户。

工程方法和沟通方式按全局规则执行；系统设计和接口细节写在 `docs/` 里。这份文件负责指路，具体内容去链接的文档里看。维护这份文件时也按这个分工来：某条内容在别处已有权威出处，这里就只留链接。

## 这个项目做什么

用户写一份 YAML，说明三件事：节点从哪里来、怎么分组、哪类流量走哪个出口。服务按这份 YAML 把节点拉下来，生成 Clash Meta、Surge 或 Loon 可以直接导入的配置。同一个程序还带一个网页管理后台，用来编辑配置、预览结果和热重载。

生成过程像一条流水线，每一站只做一件事：

1. `config.Prepare`：检查 YAML 是否写对，提前编译好正则。
2. `pipeline.Build`：拉取节点，搭出“节点 → 分组 → 路由”关系图。图里用编号来认节点，所以两个同名节点可以同时存在。
3. `target.ForClash` / `ForSurge` / `ForLoon`：按目标客户端的能力，去掉它不支持的节点，清理因此变空的分组，再在这个客户端里给每个节点分配一个不重复的名字。
4. `render.*`：把整理好的结果写成配置文本，并合并用户提供的底版模板。

网页后台和 `/generate` 都通过 `app.Service` 调用这条流水线。完整说明见 [架构设计](docs/architecture.md)。

## 去哪里查事实

| 想知道 | 看这里 |
| --- | --- |
| 模块职责、关键决策、已知局限 | [docs/architecture.md](docs/architecture.md) |
| 包之间谁可以引用谁 | [project-structure.md §依赖方向](docs/implementation/project-structure.md#依赖方向) |
| 关系图必须一直成立的规则（节点编号、组名、`@all`、链式上游） | [domain-model.md §关键不变量](docs/design/domain-model.md#关键不变量) |
| 流水线每一站的输入和输出 | [docs/design/pipeline.md](docs/design/pipeline.md) |
| YAML 字段与校验规则 | [config-schema.md](docs/design/config-schema.md)、[validation.md](docs/design/validation.md) |
| 错误分类与 HTTP 状态码 | [validation.md §错误分层](docs/design/validation.md#错误分层)、[api.md §错误语义](docs/design/api.md#错误语义) |
| HTTP 接口与 `app.Service` 的数据结构 | [api.md](docs/design/api.md)、[app-service.md](docs/design/app-service.md) |
| 三种客户端格式的输出规则 | [rendering.md](docs/design/rendering.md) |
| 哪些功能已经可用 | [docs/README.md](docs/README.md) 状态表、[progress.md](docs/implementation/progress.md) |
| 测试编号与覆盖范围 | [testing-strategy.md](docs/implementation/testing-strategy.md) |
| 发布时实际检查哪些项 | [.github/workflows/release.yml](.github/workflows/release.yml)、[deployment.md](docs/deployment.md) |
| 网页后台的页面与交互 | [web/docs/README.md](web/docs/README.md)，接口和数据结构以 `docs/design/` 为准 |

## 改一处，连带改哪些

下表左列是改动，右列是同一次提交里要一起更新的内容。这样文档、测试和代码会一起变，读者看到的始终是同一套事实。

| 改动 | 一起更新 |
| --- | --- |
| 包之间新增或删除引用 | 更新 `project-structure.md §依赖方向`。`admin` 只调用 `app` 和 `auth`，这条边界由 `internal/admin/architecture_test.go` 自动检查 |
| 新增或修改 `ProxyKind` | `ProxyKind` 标记节点的来源，很多代码按它决定节点怎么处理：是否参与过滤、区域匹配、`@all`，能否做链式上游，在哪一轮分配名字。先用 `grep -rnE '\bKind[A-Z][A-Za-z]*\b' internal --include='*.go' --exclude='*_test.go'` 找出所有用到它的地方，逐个确认新值该走哪条路，再更新 `internal/model/proxy_invariant.go` 和 `domain-model.md` |
| 新增代理协议（`Proxy.Type`） | 三个渲染器各自的协议分支，`internal/target/filter_cascade.go` 里判断格式是否支持的逻辑，`rendering.md` 映射表，以及快照文件 |
| 新增来源类型或配置字段 | `config` 的校验、`configs/base_config.yaml` 里的注释示例、`config-schema.md`、`pipeline.md`、`validation.md`、前端 `web/src/api/types.ts` |
| 新增接口或修改响应字段 | `api.md`、`app-service.md`、`web/src/api/`，以及对应的 admin 处理器测试 |
| 新增错误码 | `internal/errtype/errors.go` 和 `errors_test.go`；状态码映射写进 `validation.md §错误分层` 和 `api.md §错误语义` |
| 渲染结果变化 | `testdata/render/` 里的快照文件存放每种格式的期望输出。Clash 和 Surge 用 `UPDATE_GOLDEN=1 go test ./internal/render/` 重新生成；`loon_golden.conf` 需要手工修改。提交前逐行看一遍差异，确认每处变化都是有意为之 |
| 修改 `web/src` | 运行 `pnpm web:embed`，把 `internal/webui/dist` 和源码一起提交。发布构建直接打包这份已提交的产物 |
| 修改核心术语的定义（原始节点、拉取类节点、`@all` 等） | 在 `docs/`、`configs/`、`web/docs/` 全文搜索这个词，把每处引用改成一致，概要文档 `architecture.md` 也包括在内 |
| 功能上线或下线 | `docs/README.md` 状态表和 `progress.md` |

## 用中文写作

所有给人读的文字都用中文，包括代码注释、日志、错误信息、测试失败信息、文档和提交信息。机器要读的内容保持原样，包括标识符、配置键名、协议字段名，以及引用的外部报错原文。

## 怎样确认改对了

CI 只在推送 `v*` tag 时运行，所以日常改动靠本地检查把关。下面的后端命令与 `release.yml` 中的检查一一对应；workflow 变了，这里跟着改。

```bash
# 后端与发布检查
go mod verify
test -z "$(gofmt -l .)"
golangci-lint run          # 版本与 CI 保持一致（v2.11.4），规则见 .golangci.yml
go test ./...
go test -tags webui ./...
go vet ./...
test -s internal/webui/dist/index.html

# 改了 web/ 时追加
pnpm web:test && pnpm web:typecheck && pnpm web:embed

# 改了 Dockerfile 或发布产物，且本机有 Docker 时
docker build -f Dockerfile .
```

检查范围跟着风险走：只改一个包的内部逻辑，跑这个包的测试，再跑 `gofmt` 和 `go vet`；改动跨包、改变输出格式或准备发布时，跑完整的一套。

## 几个容易出错的地方

- **删代码时，把只为它服务的测试函数和变量一起删掉。** golangci-lint 会把测试文件里没被用到的代码当作错误。
- **把 URL 写进错误信息之前，先用 `fetch.SanitizeURL` 处理。** 订阅 URL 里常带访问令牌，经过处理后令牌才不会出现在日志和页面上。
- **一连串删除导致某个结果变空时，错误信息要写出完整的原因链**，例如“fallback 为空 ← 服务组 X 为空 ← 节点组 Y 的节点全部被过滤”。用户顺着这条链，就能找到要改的那一行配置。
- **断言 YAML 文本的测试里，组名用 ASCII 字符。** go-yaml 会把国旗、🔗 这类字符写成 `\UXXXX`，ASCII 组名可以让期望值和输出逐字对上。直接比较 Go 结构体的测试可以随意使用 emoji。
- **测试只引入被测包本身依赖的包。** 例如测 `render` 时直接手写 `model.Proxy` 作为输入，每一层的测试就只验证这一层的行为。
- **函数的行为变了，就换一个能说清新行为的名字。** 调用方靠名字理解函数在做什么。
- **验证网页后台时，用 `-tags webui` 构建。** 默认构建只包含后端，这样体积更小；加上这个标签才会把前端打包进去。
- **本地编译产物、日志和工具放在 `bin/`。** 这个目录已被 git 忽略，清理时删掉它即可。

## 先问用户的事

下面这些改动会改变产品方向。动手前先说明为什么要改、能得到什么、有什么风险，等用户同意后再做：

- 修改 [关键不变量](docs/design/domain-model.md#关键不变量)，或调整 `@all`、`@auto` 的含义
- 增加或减少支持的客户端格式、节点来源格式
- 调整安全模型：后台登录方式、会话策略、`/generate` 访问令牌的作用（见 [architecture.md §安全与信任模型](docs/architecture.md#安全与信任模型)）
- 越过 [已知局限](docs/architecture.md#已知架构局限) 划定的边界
- 新增运行时依赖。Go 这边保持精简，当前只依赖 `gopkg.in/yaml.v3`；前端同理
- 推送 tag，或做任何会触发发布的操作
