# 恢复演练记录：2026-08-07

## 范围

- 备份：`20260807T134911Z`
- 环境：物理服务器上的隔离临时 PostgreSQL 容器和临时 Docker volumes
- 生产影响：无；未连接、覆盖或停止任何生产容器和生产 volume
- 目标：证明 PostgreSQL dump、媒体、release、publisher 和 GoatCounter 归档可以恢复

## 结果

| 检查 | 结果 |
|---|---|
| PostgreSQL `pg_restore --no-owner --no-privileges` | 通过 |
| `users` | 1 |
| `posts` | 0（当前生产无文章数据） |
| `media_assets` | 0（当前生产无媒体记录） |
| `publish_jobs` | 0（当前生产无发布任务） |
| media 归档 | 可解包，0 个文件 |
| site release 归档 | 可解包，164 个文件 |
| publisher site 归档 | 可解包，155 个文件 |
| GoatCounter 归档 | 可解包，3 个文件 |
| 临时资源清理 | 通过，无残留容器或 volume |

首次启动遇到 PostgreSQL 官方镜像初始化阶段的短暂 ready/restart 窗口。演练脚本改为连续两次 `pg_isready` 成功后再执行 `pg_restore`，第二次执行成功。

数据库、media 和 release 主演练约 22 秒；publisher、GoatCounter 和最终残留检查约 6 秒，总数据级 RTO 观测值约 28 秒。备份创建后立即演练，RPO 小于 1 小时。

## 后续

本次证明“数据可恢复”，没有启动 API/Worker 连接恢复数据库。季度服务级演练还需：使用恢复库启动隔离 API、验证 `/readyz`、管理员登录、站点读取、媒体读取和一次隔离发布，然后记录完整 RTO。

演练命令没有打印或记录生产密码；临时数据库密码仅用于隔离容器，演练结束后随容器删除。
