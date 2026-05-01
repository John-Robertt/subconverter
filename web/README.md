# subconverter-go Admin

## 快速开始

```bash
open "subconverter Admin.html"
# 或
python3 -m http.server 8000
```

无需任何构建工具，浏览器直接打开。

## Docker 预览

`web/Dockerfile` 用于 v2.0 的生产 Web 容器。当前目录还是设计原型时，镜像会把原型文件静态托管起来；后续接入正式 Vite 工程后，同一个 Dockerfile 会执行 `npm run build` 并用 nginx 托管 `dist/`。

```bash
docker build -t subconverter-web .
docker run --rm -p 8080:80 subconverter-web
```

## 看什么

- **设计画布** 列出 13 个核心页面（订阅来源 / 过滤器 / 分组 / 路由 / 规则集 / 内联规则 / 其他配置 / 校验 / 节点预览 / 分组预览 / 生成下载 / 系统状态）
- **Tweaks 面板**（右上角切换）
  - 演示状态：在 A1 订阅来源 / A8 校验上叠加 5 种交互模式（Modal / Toast 成功 / Toast 失败 / 校验中 / 保存中 / 删除确认 / 校验 Drawer）
  - 主题：浅色 / 深色
  - 强调色：6 个预设色板

## 文档

| 文件                                       | 说明                                             |
| ------------------------------------------ | ------------------------------------------------ |
| [docs/PRD.md](docs/PRD.md)                 | 产品需求：场景、信息架构、功能模块、交互模式锁定 |
| [docs/Design-Spec.md](docs/Design-Spec.md) | 设计 Tokens、组件库、交互规范                    |
| [docs/Dev-Guide.md](docs/Dev-Guide.md)     | 工程结构、运行调试、扩展指南、接入真实后端       |

## 文件结构

```
subconverter Admin.html       入口
├ app.jsx                     主入口（assemble）
├ direction-b.jsx             Shell + 通用组件 + Sources
├ screens-config.jsx          A2–A7 编辑页
├ screens-runtime.jsx         A8 / B 区 / C 区
├ interaction-layer.jsx       state-driven 交互覆盖层
├ demos-modal.jsx             Modal/Drawer/Confirm 组件
├ demos-feedback.jsx          Toast/Spinner 组件
├ mock-data.jsx               节点 / 分组 / 路由 mock
├ mock-data-extra.jsx         校验 / 热重载 mock
├ design-canvas.jsx           设计画布
└ tweaks-panel.jsx            Tweaks 面板
docs/                         三份文档
```

## 锁定的交互模式

| 场景                   | 模式                                     |
| ---------------------- | ---------------------------------------- |
| 添加订阅 / 复杂表单    | 居中 Modal                               |
| 保存成功               | 右下绿色 Toast（4s 自动消失）            |
| 保存失败               | 右下红色 Toast（不消失，必带"查看详情"） |
| 校验中 / 保存中        | 顶栏按钮 Spinner 替换 + 禁用             |
| 删除订阅等不可撤销操作 | 居中红色确认弹窗                         |
| 校验报错跳修           | 右侧 Drawer 直接编辑字段                 |

详见 PRD 第 5 节。
