# Task #4 · Sprint 1 后端交付记录

验证日期：2026-11-26（Asia/Shanghai）。工作分支：`aip/sprint-1-task-4-backend-go-dev-2a7eb4b6`。

## 范围与实现

开发前完整读取了指定项目上下文 JSON。原始需求为 Go REST API + PostgreSQL 与独立 React/TypeScript/Vite 前端；上下文没有 `charter.prd_markdown` 字段，`frontend` 和 `inputs` 为 null。已同时阅读仓库全部上游需求、分析与架构文档。以原始需求及本任务「领域模型、基础只读/健康 API 与单元测试」作为交付边界。

| 验收内容 | 实现位置与结果 |
| --- | --- |
| Go 服务可启动 | `cmd/api`，超时、连接池、信号优雅退出、只读 HTTP |
| PostgreSQL 可启动并持久化 | Compose PostgreSQL 17、命名卷、独立迁移容器 |
| 内容领域模型 | `internal/domain`，11 类内容、品牌、图片、CTA、嵌套链接、价格 |
| 草稿与公开内容隔离 | 完整版本快照；单条 SQL 只取 published + enabled；稳定排序 |
| 数据迁移与种子 | `migrations` 嵌入 SQL；事务、并发锁、校验和；已有内容不覆盖 |
| 基础 API | `/healthz`、`/readyz`、`/api/v1/site`、`/api/v1/contents/{kind}` |
| 强类型契约 | Go 类型、SQL 约束、`docs/openapi.yaml`、结构化错误 |
| 配置与部署 | `.env.example`、多阶段非 root Dockerfile、Compose、Makefile |
| 单元与集成测试 | 配置、领域、HTTP、存储、迁移及实际服务生命周期；CI 配置已加入 |

架构取舍：使用标准库 net/http + database/sql + pgx PostgreSQL 驱动；Sprint 1 不引入上游简略 ADR 提到的 Redis、Gateway 与乐观锁，因为当前只有只读内容服务。后续管理写入应补齐鉴权、输入校验、并发版本校验和发布审计。原占位 `records` 表未删除，业务内容使用追加迁移中的专用表。

## 已执行检查

本地 Go 工具链：`go1.27.3 darwin/arm64`；容器构建使用 `golang:1.25-alpine`。以下命令均在本任务工作目录执行：

| 检查 | 结果 |
| --- | --- |
| `go test ./...` | 通过；不配置测试数据库时仅跳过显式的真实数据库集成用例 |
| `TEST_DATABASE_URL=… go test -race -coverprofile=coverage.out ./...` | 通过；无数据竞争；含真实 PostgreSQL 用例 |
| `go vet ./...` | 通过 |
| `go build ./...` | 通过 |
| `git diff --check` | 通过 |
| OpenAPI 规范校验 + 真实响应 schema 校验 | 通过；14 个 GET 响应（健康、就绪、整站及 11 分类）匹配契约，含 UUID/时间格式 |
| `docker compose -p cms-sprint1-task4 up -d --build`（通过环境设置隔离端口） | 完成构建；迁移退出码 0；DB/API healthy |

含集成测试时，总语句覆盖率 **86.6%**；config/domain/httpapi **100%**，store **97.0%**，migrations **92.5%**，cmd/api **70.3%**。cmd/migrate CLI 的覆盖率未计入 Go 测试（0%），已由 Compose 实际运行验证。`coverage.out` 为本地生成并被 git 忽略，可用 README 中命令重现；GitHub CI 尚未远程运行，不冒充已通过。

测试覆盖包括：无发布数据、空内容、数据库错误与取消、未知路由/分类、禁止写入、HEAD、CORS、并发请求、草稿/归档/停用过滤、相同位置稳定排序、6 个并发迁移初始化、重复种子不覆盖编辑、版本事务切换、迁移失败回滚、校验和不一致、数据库约束及启动/退出。

## 真实运行证据

Compose 项目 `cms-sprint1-task4`，API `127.0.0.1:4101`，PostgreSQL `127.0.0.1:55434`（均仅绑定本机）。

```text
GET  /healthz                       200 {"status":"ok"}
GET  /readyz                        200 {"status":"ready"}
GET  /api/v1/site                    200 brand.name=协作云, version=1, sections=11
POST /api/v1/site                    405 method_not_allowed
migrate container                   Exited (0)
database and API containers          healthy
```

实际停止数据库后：`/healthz` 仍返回 200，`/readyz` 返回 503 `not_ready`，`/api/v1/site` 返回 503 `content_unavailable`。重新启动数据库和 API 后，整站 JSON 与重启前完全一致，验证命名卷与存储保留。数据库重新可用后服务自动恢复读取。

参考页面与浏览器工具限制见 [frontend-delivery.md](frontend-delivery.md)。本次没有浏览器截图或页面交互通过证据。

## 运行与后续工作

安装、环境配置、独立启动和完整测试命令见 [README](../README.md)。本次隔离验证服务保留运行，可访问 `http://127.0.0.1:4101/api/v1/site`。停止本次容器可用 `docker stop cms-sprint1-task4-api-1 cms-sprint1-task4-db-1`；命名卷保留。

剩余产品范围：React 官网与管理后台、管理员环境变量凭证和登录、内容 CRUD/排序/启停写入、图片维护、草稿预览与发布 API、端到端后台编辑后刷新官网验证、响应式和视觉核对。种子为明确标注的演示内容，不含真实客户背书或官方素材。Sprint 1 后端读取使用真实数据库，没有模拟的后端集成。
