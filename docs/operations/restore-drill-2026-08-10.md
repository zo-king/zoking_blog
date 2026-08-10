# 服务级恢复演练记录：2026-08-10

## 范围

- 备份：`20260810T063709Z`
- 备份提交：`c30ad845bd97`
- 环境：物理服务器上的内部 Docker 网络、专用临时容器和专用临时 volumes
- 生产影响：无；未连接生产数据库或 volumes，未映射主机端口，未停止或重建生产容器
- 目标：验证加密备份能够恢复 PostgreSQL、媒体和发布数据，并启动 API、Worker 和 Site 完成管理员登录与隔离发布

## 结果

| 检查 | 结果 |
|---|---|
| 加密与明文内容 manifest | 通过 |
| PostgreSQL 恢复与迁移 | 通过，版本 `20260806000100` |
| API `/readyz` | 通过 |
| 公开 API、站点和媒体读取 | 通过 |
| 恢复后的管理员登录 | 通过 |
| 隔离发布 | `requested -> building -> published` |
| `users/posts/media_assets/publish_jobs` | `1/0/0/2` |
| 媒体文件 | 0 |
| RTO | 183 秒 |
| RPO | 501 秒 |
| 临时资源清理 | 通过；容器、网络、volumes、解密数据和 age identity 均已删除 |
| 生产复核 | 容器未重建；Site/API/Admin/Preview/Stats 为 `200/200/200/404/303` |

## 演练中发现并修复的问题

1. 隔离环境最初使用 `.invalid` 保留域名，触发生产公开 URL 安全策略。提交 `c30ad845` 改为在无公网出口的内部网络中保留正式 canonical 元数据，并让探针从同一 Admin Origin 配置取值。
2. 备份 `20260810T055931Z` 暴露历史配置漂移：`site.base_url` 和 `comments.api_base` 仍为 localhost。后台已改为 `https://zoking.tech/` 和 `https://api.zoking.tech`，成功发布后创建本次最终备份。
3. 管理员密码已通过受控脚本重置；重置前后均创建加密备份，旧 refresh tokens 已撤销，新密码登录在生产和恢复环境均通过。
4. 后台多标签页出现 CSRF cookie 与标签页 session token 不一致。后续提交 `4d285996` 已使刷新和会话恢复复用合法现有 token；CI、生产部署和双标签页人工验收均通过。

## 结论

本次已证明加密备份能够在不影响生产的前提下完成服务级恢复、认证和发布。观测 RTO 为 183 秒，RPO 为 501 秒，满足当前季度演练闭环要求。私钥、管理员密码和解密内容均未写入日志、仓库或本记录。
