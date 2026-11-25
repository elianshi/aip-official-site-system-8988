# Sprint 1 API 与数据契约

此文档描述本分支已经实现的行为。机器可读契约见 [openapi.yaml](openapi.yaml)。所有接口为真实 PostgreSQL 读取；单元测试中使用的 fake/sqlmock 不进入服务启动路径。

## 内容结构

`GET /api/v1/site` 返回 `{ "data": Site }`，Site 包含：

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| release_id | UUID 字符串 | 当前发布版本 ID |
| version | 正整数 | 内容版本号 |
| published_at | RFC3339 时间 | 发布时间 |
| brand | Brand | 可替换的名称、标语、可选 Logo |
| sections | 固定 11 个键的对象 | 每个键的值均为 Content 数组，空时为 `[]` |

kind 值为 `banners`, `navigation`, `heroes`, `products`, `solutions`, `cases`, `services`, `security`, `pricing`, `resources`, `footer`。

Content 字段为 `id`, `kind`, `title`, `summary`, `position`, `links`, `features`，可选 `image`, `cta`, `price`。图片结构为 `{url, alt}`；链接结构为 `{label, url, children?}`，支持导航下拉树。价格为 `{amount_minor, currency, interval}`，金额使用整数分单位。数组字段始终返回数组；未设置的可选字段省略。

`GET /api/v1/contents/products` 示例：

```json
{
  "data": [
    {
      "id": "00000000-0000-0000-0000-000000000001",
      "kind": "products",
      "title": "团队文档",
      "summary": "共同编辑，让知识持续流动。",
      "position": 0,
      "links": [],
      "features": ["协同编辑"],
      "cta": {"label": "了解更多", "url": "/#resources"}
    }
  ],
  "release_id": "10000000-0000-0000-0000-000000000001",
  "version": 1
}
```

每类内容按 `position ASC, id ASC` 排序，相同位置按 UUID 确定稳定顺序。首页建议一次获取 `/site`，确保全部区块来自同一个版本；分次调用分类接口时前端可使用 `release_id` 检测期间发生的发布。

## 错误与跨域

错误格式统一为 `{"error":{"code":"...","message":"..."}}`。

| HTTP | code | 条件 |
| --- | --- | --- |
| 400 | invalid_kind | 不支持的内容分类 |
| 403 | origin_not_allowed | 预检 origin 不在允许列表 |
| 404 | not_found | 路由不存在（包括尚未提供的管理接口） |
| 404 | not_published | 没有发布版本 |
| 405 | method_not_allowed | 对只读接口执行写入或预检写入方法 |
| 503 | not_ready | 数据库不可用或未迁移 |
| 503 | content_unavailable | 读取失败或超时 |

错误响应不包含数据库错误或连接凭证。读取超时为 3 秒，请求取消会传入数据库查询。健康成功响应分别为 `{"status":"ok"}`、`{"status":"ready"}`。HEAD 与 GET 状态一致，无响应体。

`CORS_ALLOWED_ORIGINS` 为逗号分隔的 HTTP(S) origin，不允许通配符、路径和凭证。GET 是公开接口，未列出的 origin 仍可通过非浏览器客户端读取，但浏览器不能跨域读取响应。OPTIONS 允许 GET/HEAD，不支持带自定义请求头的预检；前端公开读取不应附带 Authorization 等头。无 cookie 或管理员凭证机制。响应使用 `Cache-Control: no-store`，便于后续发布刷新生效。

## 数据与后续发布流程

`site_releases` 存品牌和 draft/published/archived 状态，唯一部分索引保证最多一个 published 版本；published 状态必须有发布时间。`content_items` 存该版本完整内容、顺序和启用标记。公开查询使用一条 SQL 得到一致快照，避免发布期间混合版本。

后续编辑应复制发布版本创建草稿；发布事务先归档旧版本，再将完整草稿标记为 published。Sprint 1 不提供执行这些写操作的 HTTP 接口，也不声称已实现权限、乐观锁或发布审计。数据库持有者仍可直接修改表，发布版本不可变规则需要后续管理服务落实。URL 协议、链接长度、图片和价格细节的写入校验也须在该阶段补齐。

当前数据量按官网配置设计，整站返回，不提供分页。图片仅维护 URL/alt 契约，无上传或对象存储。开发种子未捏造飞书品牌或官方图片。
