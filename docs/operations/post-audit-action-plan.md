# 生产审计与后续行动计划

审计日期：2026-08-07  
审计对象：Zoking Blog 仓库、物理服务器、Azure Edge、WireGuard、Caddy、Docker Compose、备份与监控  
对应报告：[production-audit-2026-08-07.md](../security/production-audit-2026-08-07.md)

## 当前结论

线上服务、TLS、WireGuard、容器、应用备份和 SSH 密钥登录均正常。生产 secret 未被 Git 跟踪，`.env.prod` 仅存在物理机并为 `root:docker 0640`。以下事项不会阻断当前服务，但在宣布“灾备和安全闭环完成”前必须处理。

审计期间已完成：健康检查自失败循环修复并同步到物理机和 Azure；两端脚本 SHA-256 一致，最新两端探针均成功。

## P0：24 小时内

### 1. 保护异机备份静态数据（代码已完成，待密钥托管与迁移）

现状：备份通过 WireGuard + SSH 加密传输，但 Azure `/var/backups/zoking-blog` 上的归档本身未加密；归档包含数据库、`.env.prod` 和 `wg0.conf`。

实施：备份脚本已改为强制使用 `age` 接收方公钥；staging 明文只在单次任务内存在，正式目录和 Azure 副本只保留 `.age` 文件及加密文件 manifest。推荐用密码管理器和离线恢复介质托管唯一 identity 私钥，物理机只配置 `BACKUP_AGE_RECIPIENT`，不配置私钥。

待执行：将恢复私钥导入独立托管位置，向物理机 `/etc/zoking-blog/ops.env` 写入对应公钥，完成一次加密备份和旧明文副本清理。

验收：在临时目录中使用独立恢复环境解密一份加密备份，能读取 PostgreSQL dump 和媒体归档；物理机和 Azure 均不保存明文长期副本；恢复密钥由第二名管理员独立取得。

### 2. 限制备份 SSH key 的能力（已完成：2026-08-07）

现状：`zoking-backup` 无 sudo，但 `authorized_keys` 没有 `restrict` 或 forced command；密钥泄露后可在 Azure 账号权限内执行任意命令。

实施：Azure key 已配置 `restrict,command="/usr/bin/rrsync -wo -no-del /var/backups/zoking-blog"`；物理机 `BACKUP_REMOTE` 使用受限根 `zoking-backup@10.20.0.1:/`。

验收结果：完整备份 `20260807T134911Z` 传输并通过远端 manifest；任意 `id` 命令和 `--delete` 均被拒绝；测试目录已清理。端口转发和 PTY 由 `restrict` 禁用。

### 3. 完成一次隔离恢复演练（数据级已完成：2026-08-07）

实施：使用备份 `20260807T134911Z` 在临时 PostgreSQL 容器和临时 volumes 中恢复，未连接生产卷；记录见 [restore-drill-2026-08-07.md](restore-drill-2026-08-07.md)。

验收结果：数据库和四类文件归档均可恢复；总演练约 28 秒；所有临时容器和 volumes 已清理。下一次季度演练应增加“使用恢复库启动 API”的服务级验证。

## P1：7 天内

### 4. 补齐全站 HSTS（已完成：2026-08-07）

审计证据：公网响应中 Admin 带 `Strict-Transport-Security`，Reader、API、Preview 和 Stats 当前未稳定返回 HSTS。所有域名均已强制 HTTPS，但缺少浏览器侧的长期 HTTPS 记忆。

实施：生产 Caddy 配置已纳入 `infra/caddy/Caddyfile`，使用 deferred header 覆盖上游策略，避免重复响应头。

验收结果：五个 HTTPS 域名均只返回一条 HSTS；HTTP 仍为 308；Caddy validate、reload 和公网黑盒全部通过。

### 5. 处理 React Router 安全公告（临时风险接受至 2026-08-14）

审计证据：官方 npm registry 的 `npm audit --omit=dev --audit-level=high` 报告 `react-router` / `react-router-dom@7.18.2` 两项 high，GHSA-qwww-vcr4-c8h2，范围为 `>=7.12.0 <8.3.0`。当前代码只使用 BrowserRouter、Routes、Route 和导航 hooks，未发现 RSC action/server route；因此可利用面尚未证实，但不能把“未使用 RSC”当作永久豁免。

实施：官方建议的 `7.11.0` 回退会重新引入多项已修复公告，故保持 `7.18.2`。当前生产只部署静态浏览器 bundle，不存在 RSC/Node 服务端 action 入口。临时风险接受、补偿控制和退出条件见 [react-router-risk-acceptance-2026-08-07.md](../security/react-router-risk-acceptance-2026-08-07.md)。

验收结果：CI 使用官方 registry，仅允许精确版本上的该公告；新增 high/critical、依赖版本或公告集合变化、例外到期都会失败。最迟 2026-08-14 升级到官方修复版本或重新书面审批。

### 6. 增加依赖与镜像供应链门禁

现状：GitHub Actions 使用浮动 major tags；Dockerfile 通过 `ghfast.top`、镜像站和远端 release URL 下载构建依赖；仓库没有 Dependabot、govulncheck 或镜像 digest 校验。

动作：将 Actions 固定到 commit SHA；增加 Go `govulncheck`、npm audit 和容器镜像扫描；为 Hugo、Pagefind、GoatCounter 和 Debian/Node 基础镜像记录版本与 SHA256，优先移除不必要的第三方下载代理。

验收：PR 门禁在依赖漏洞、失效 checksum、未批准的浮动 action 或高危镜像时失败；每月有一次依赖升级记录。

### 7. 收窄物理机 SSH 来源

现状：物理机 UFW 当前允许 `192.168.0.0/24 -> 22/tcp`。这不是公网暴露，但超过单一管理终端的最小权限边界。

动作：确认管理终端地址后收窄到固定 `/32`；若需要 WireGuard 运维，则单独允许 `10.20.0.1/32`，并保留至少一个已验证密钥会话再 reload。

验收：局域网非管理地址和未授权 WireGuard 地址无法连接 22；管理密钥新会话、`sshd -t`、UFW 状态和回滚步骤均记录。

### 8. 配置外部告警并做受控失败测试

动作：在 `/etc/zoking-blog/ops.env` 写入受信 Webhook，权限保持 `0600`；人为停止一个非关键探针依赖，确认物理机和 Azure 均发送告警，再恢复服务并确认告警恢复。

验收：记录发送时间、接收人、恢复时间和升级路径；Webhook URL 不进入 Git、journal、截图或审计报告。

## P2：30 天内

- 为 Azure 异机目录增加独立保留清理策略和磁盘阈值；清理前先验证最新副本，禁止 `rsync --delete` 直接覆盖。
- 将健康检查、备份、证书和 WireGuard 指标接入长期监控；保留至少 30 天执行结果。
- 建立管理员、数据库、JWT、隐私哈希和备份密钥的季度轮换演练。
- 将生产 IP、服务端口、提交号和 SSH 管理来源从长文档中的散落常量改为一份生成的状态快照，减少文档漂移。
- 恢复 GitHub 直连后验证 `git fetch --prune origin`；在此之前使用可校验 Git bundle，并记录 bundle 的来源 commit。

## 责任与证据模板

每项动作完成后记录：负责人、完成时间、执行命令摘要、前置备份、验证结果、回滚方式和相关 commit。密码、私钥、OTP、Webhook URL 和完整数据库 URL 永远不写入此文件。
