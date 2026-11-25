# 官网内容管理系统 · Go 后端 Sprint 1

独立演示品牌「协作云」的内容服务。当前实现 PostgreSQL 持久化、版本化内容模型、只读公开 API、健康检查、迁移与测试。官网和管理后台由后续 React + TypeScript + Vite 工程接入；此分支尚未实现管理员登录、CRUD、预览和发布接口。

## 启动

需要 Docker Compose；本地运行 Go 命令需要 Go 1.23+（容器与 CI 使用 Go 1.25）。在仓库根目录执行：

```sh
cp .env.example .env
docker compose up -d --build
curl -f http://127.0.0.1:8080/readyz
curl -f http://127.0.0.1:8080/api/v1/site
```

Compose 按数据库健康 → 迁移和开发种子完成 → API 的顺序启动，使用命名卷保存数据。`.env` 中示例密码只用于本地开发，正式环境应替换；含 URL 特殊字符的密码需在连接 URL 中编码。停止使用 `docker compose down`，保留数据卷。

单独启动数据库与本地 Go 服务：

```sh
docker compose up -d db
set -a
. ./.env
set +a
go mod download
go run ./cmd/migrate -seed
go run ./cmd/api
```

迁移默认不插入内容；`-seed` 只在数据库没有任何内容版本时插入演示数据，已有内容不被覆盖。迁移包含事务、并发锁和 SQL 校验和。已应用的迁移不可修改，应追加编号文件。原有 `001_init.sql` 占位表保留兼容，内容服务使用 `002_site_content.sql`。

## API

| 方法与路径 | 行为 |
| --- | --- |
| `GET /healthz` | 进程存活，不依赖数据库 |
| `GET /readyz` | 数据库及内容表可用；失败返回 503 |
| `GET /api/v1/site` | 当前发布版本、可替换品牌与全部 11 类内容 |
| `GET /api/v1/contents/{kind}` | 指定内容列表及版本号 |

所有读取接口支持 HEAD；CORS 仅允许环境变量中列出的精确 origin。公开接口不包含草稿、归档和停用内容。未发布返回 404，已发布但无条目返回空数组，数据库错误返回 503。未提供任何管理写入接口。

完整契约见 [OpenAPI](docs/openapi.yaml) 与 [API 说明](docs/backend-api.md)。

## 检查

```sh
go test ./...
go test -race -coverprofile=coverage.out ./...
go vet ./...
go build ./...
```

真实数据库集成测试默认跳过；显式设置测试数据库后执行：

```sh
TEST_DATABASE_URL="$DATABASE_URL" go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

集成测试创建独立临时 schema 并在完成后删除，不清空既有业务表；测试账号需要 `CREATE SCHEMA` 权限。GitHub Actions 配置会启动 PostgreSQL 并执行集成测试。`make build` 输出 `bin/api` 与 `bin/migrate`。

本次验证服务地址为 `http://127.0.0.1:4101`（API JSON，非官网页面）。运行记录见 [后端交付](docs/backend-delivery.md)，前端交接和浏览器限制见 [前端交付范围](docs/frontend-delivery.md)。
