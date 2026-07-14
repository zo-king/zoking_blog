# P24 安全与可靠性验收

## 范围

- Admin HttpOnly Cookie、CSRF、Origin/CORS 和会话恢复。
- 媒体上传私有暂存、删除隔离、引用锁和孤立媒体候选。
- 发布任务恢复、取消/重试竞争、active release 与 `current` 对账。
- 生产域名、Preview 独立 Origin、Compose、migration 和运维文档。

## 已完成修复

- 登录与 `/auth/session` 不返回 JWT；会话恢复轮换 CSRF，退出后 Cookie 会话失效。
- post/page/series/achievement/release 的单媒体引用统一走 `mediaref`，创建引用前持有 PostgreSQL `FOR KEY SHARE`。
- 发布失败写入不覆盖 `requested/canceled/published`；Worker 每轮持锁时恢复 stale job，并按数据库唯一 active release 修复 `current`。
- active release 文件缺失时只记录对账错误，不阻塞新发布自愈。
- 孤立媒体候选在 SQL 层排除已有 usage，事务内保留二次检查；清理内部错误不再泄露到底层路径或数据库文本。
- 生产 Preview Origin 强制独立于站点、API 和 Admin；migration Down 在活跃发布任务存在时拒绝。
- E2E 评论断言遵循公开 DTO，不泄漏 moderation status，真实 pending 状态由 Admin 列表验证。

## 验收结果

- `go test ./... -count=1`：通过，使用真实 `zoking_blog_test` PostgreSQL。
- `go vet ./...`：通过。
- Admin `npm run build`：通过；官方 npm registry 审计为 0 漏洞。
- Hugo Extended 0.160.1 production/minify：54 pages，通过。
- development/production Compose config：通过。
- migration `20260714000200`：真实 `Down -> Up -> status` 通过。
- HTTP 黑盒：26/26，通过 Cookie 属性、CSRF 轮换、错误 Origin、退出 Cookie 和退出后 401。
- `preflight.ps1 -SkipE2E`：通过。
- 完整隔离 preflight/E2E：通过，涵盖预览、发布、媒体、评论、回滚和清理；`18081` 与隔离目录已清理。

## 剩余风险

`MEDIA-RECOVERY-P25-001` 负责跨数据库/文件系统崩溃一致性。需要持久化 media operation 或 manifest，覆盖 rename 前后、COMMIT 成功但 ACK 丢失、进程终止后重启 reconciliation，并使用 Toxiproxy/故障注入验证。现有正常路径、引用并发和生产卷拓扑已通过，不应把 P25 当作当前功能缺失，但生产高可用上线前应完成。
