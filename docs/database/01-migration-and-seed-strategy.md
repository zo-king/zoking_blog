# 数据库迁移与 Seed 策略

## 1. 迁移工具

默认建议：

- `golang-migrate` 或 `goose`。
- 迁移文件放在 `db/migrations`。
- API 代码不在启动时自动修改生产 schema。

命名：

```text
YYYYMMDDHHMMSS_create_users.sql
YYYYMMDDHHMMSS_create_posts.sql
```

每个迁移必须包含：

- Up。
- Down。
- 幂等注意事项。
- 必要索引。
- 约束。

## 2. 初始化扩展

第一批迁移：

```sql
create extension if not exists pgcrypto;
create extension if not exists citext;
create extension if not exists pg_trgm;
```

说明：

- `pgcrypto` 用于 `gen_random_uuid()`。
- `citext` 用于邮箱大小写不敏感。
- `pg_trgm` 可用于后续模糊搜索。

## 3. 迁移分层

建议顺序：

1. extensions。
2. users / roles / permissions。
3. content core。
4. taxonomy。
5. media。
6. comments。
7. site settings / menus。
8. publish jobs / snapshots / releases。
9. stats。
10. audit logs。

## 4. Seed 数据

必须 seed：

- permissions。
- system roles。
- role permissions。
- first super admin。
- default site settings。

可选 seed：

- demo post。
- default categories。
- default tags。
- default menu。

环境差异：

| 环境 | seed 策略 |
|---|---|
| development | 可创建 demo 内容 |
| test | 每次测试前重建最小数据 |
| staging | 接近生产，不使用大量 demo |
| production | 只创建必要系统数据和超级管理员 |

## 5. 生产规则

- 生产迁移只向前执行。
- 迁移前备份数据库。
- 大表改字段要评估锁表风险。
- 删除字段先废弃，再清理。
- 增加非空字段时先加 nullable，回填后再加约束。
- 索引大表时优先 `CREATE INDEX CONCURRENTLY`。

## 6. CI 验收

每次 PR 至少验证：

- 空库执行全部 migration 成功。
- seed 成功。
- GORM model 与关键表字段不明显偏离。
- Down 在开发环境可回滚最近迁移。

## 7. Migration DoD

- 文件名符合规则。
- Up/Down 成对。
- 索引和外键明确。
- 软删除唯一索引正确。
- 不含真实数据和密钥。
- 更新 `docs/database/00-data-model.md`。
- 工作日志记录迁移目的。
