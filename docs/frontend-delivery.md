# Sprint 1 后端任务的前端交接

本分支负责 Task #4 Go 后端骨架与单元测试，当前没有 React/Vite 前端工程或渲染页面。本文件保留需求到页面/API 的映射与验证边界；不将后端 JSON 响应描述为完成的官网。

| 原始需求 | 目标页面/组件 | 本次提供的契约 |
| --- | --- | --- |
| 品牌名称、Logo、文案可替换 | 官网 `/` 品牌区域 | `Site.brand` |
| 顶部活动横幅 | 官网 `/` 横幅 | `sections.banners` |
| 导航与下拉 | 官网导航 | `sections.navigation`，嵌套 Link.children |
| 首屏轮播与 CTA | 官网首屏 | `sections.heroes`，Image/CTA/position |
| 产品卡片排序 | 官网 `/#products` | `sections.products`，稳定排序 |
| 方案、客户案例 | 官网 `/#solutions`、案例区 | `sections.solutions`、`cases` |
| 服务与安全 | 官网服务、安全区 | `sections.services`、`security` |
| 价格、资源、页脚 | 官网对应区块 | `sections.pricing`、`resources`、`footer` |
| 加载/空数据/错误 | 官网数据加载层 | 异步请求；空数组；404 未发布；503 暂时不可用 |
| 管理员登录、编辑、预览、发布 | 后续 `/admin/*` | 本次未实现；仅提供草稿/发布版本数据结构 |
| 手机桌面适配、切换、跳转 | 后续 React 页面 | 需前端实现与浏览器验收 |

安装、构建和预览：后端运行命令见 [README](../README.md)，`go mod download`、`go build ./...`、`go run ./cmd/api`；Compose 可独立启动整套后端。当前无 `npm install`、Vite build 或 UI preview 命令，后续前端应在仓库根目录采用 React + TypeScript + Vite。开发时配置 `VITE_API_BASE_URL` 指向后端（该变量是前端建议，后端不读取），并在后端 CORS 允许实际前端 origin。

已执行 Go 单元/集成测试、构建、静态检查和真实 HTTP 验证，详见 [后端交付记录](backend-delivery.md)。本地验证端口为 4101，承载 API JSON。

浏览器证据：2026-11-26 尝试通过 Computer Use 打开 `https://www.feishu.cn/`，工具返回 `native pipe closed before response`，未获得可用浏览器会话、临时 profile 或截图。随后网页读取跳转至 [Lark 国际站](https://www.larksuite.com/en_sg/?from_site=feishu)，仅取得导航、产品、方案、客户与 CTA 等文本结构；未据此声称查看了飞书中国站的视觉与响应式效果。项目输入未提供截图。

因此本次没有完成渲染页面、交互状态或移动端浏览器验证。待浏览器工具恢复且前端完成后，应在隔离的 4101 端口与临时 profile 验证首页、移动布局、轮播/导航以及后台修改发布刷新流程；届时将 API 改至另一独立端口，避免冲突。
