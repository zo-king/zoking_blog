# 生产审计与后续行动计划

审计日期：2026-08-07  
审计对象：Zoking Blog 仓库、物理服务器、Azure Edge、WireGuard、Caddy、Docker Compose、备份与监控  
对应报告：[production-audit-2026-08-07.md](../security/production-audit-2026-08-07.md)

## 当前结论

线上服务、TLS、WireGuard、容器、应用备份和 SSH 密钥登录均正常。生产 secret 未被 Git 跟踪，`.env.prod` 仅存在物理机并为 `root:docker 0640`。以下事项不会阻断当前服务，但在宣布“灾备和安全闭环完成”前必须处理。

审计期间已完成：健康检查自失败循环修复并同步到物理机和 Azure；两端脚本 SHA-256 一致，最新两端探针均成功。

## P0：24 小时内

### 1. 保护异机备份静态数据（已完成：2026-08-08）

现状：备份通过 WireGuard + SSH 加密传输，但 Azure `/var/backups/zoking-blog` 上的归档本身未加密；归档包含数据库、`.env.prod` 和 `wg0.conf`。

实施：备份脚本已改为强制使用 `age` 接收方公钥；staging 明文只在单次任务内存在，正式目录和 Azure 副本只保留 `.age` 文件及加密文件 manifest。推荐用密码管理器和离线恢复介质托管唯一 identity 私钥，物理机只配置 `BACKUP_AGE_RECIPIENT`，不配置私钥。

已完成：公钥已配置，`20260807T150122Z` 本地和 Azure 副本均为 `.age` 并通过 manifest；记录见 [encrypted-backup-migration-2026-08-07.md](encrypted-backup-migration-2026-08-07.md)。

验收结果：项目所有者已确认恢复私钥进入独立托管位置；本机暂存私钥已删除。物理机 6 份、Azure 4 份历史明文目录已精确删除，两端只保留 `20260807T150122Z` 和 `20260808T033549Z` 两份加密备份，最终扫描无明文残留。

恢复验证：隔离环境已使用托管前的临时 identity 完整解密 PostgreSQL dump、媒体和发布/统计归档；identity 随后从物理机删除。服务级恢复演练仍按 P0-3 的季度后续执行。

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

### 5. 处理 React Router 安全公告（已完成：2026-08-08）

初始证据：官方 npm registry 曾报告 `react-router` / `react-router-dom@7.18.2` 命中 GHSA-qwww-vcr4-c8h2，项目因此建立限期例外。GitHub Advisory 于 2026-08-07 18:16:54 UTC 将 7.x 受影响范围修正为 `>=7.12.0 <7.18.2`，并把 `7.18.2` 列为首个修复版本。

实施：保持已修复的 `7.18.2`，删除公告白名单和到期逻辑，CI 直接执行无豁免的官方 registry 生产依赖审计。闭环记录见 [react-router-risk-acceptance-2026-08-07.md](../security/react-router-risk-acceptance-2026-08-07.md)。

验收结果：`npm audit --omit=dev --audit-level=high` 返回 0 个 high/critical，Admin build、lint 和格式检查通过；不再存在 2026-08-14 到期事项。

### 6. 增加依赖与镜像供应链门禁（已完成：2026-08-08）

初始现状：GitHub Actions 使用浮动 major tags；Dockerfile 通过 `ghfast.top`、镜像站和远端 release URL 下载构建依赖；仓库没有 Dependabot、govulncheck 或镜像 digest 校验。

实施：Actions 已固定 commit SHA；新增 Dependabot、Go 1.26.5 `govulncheck` 和 API/Admin/GoatCounter 最终镜像 Trivy 门禁；基础镜像固定 digest；Pagefind 官方 release 增加 SHA256；Hugo `v0.164.0` 和 GoatCounter `v2.7.0` 固定模块源码并由 Go checksum database 校验；移除 `ghfast.top`、清华 Debian 镜像和 `goproxy.cn`。

当前验收：Go 全量测试、vet、govulncheck、Admin build/lint/format/npm audit、actionlint、YAML 和 Compose config 均通过；CI run `31246299311` 已通过 API、Admin、GoatCounter 最终镜像 Trivy 门禁，健康探针修复后的 CI run `31248514854` 亦完整成功。生产已部署提交 `ee8989c`，API/worker 使用镜像 `e37df740...`，Admin 使用 `b805e4a2...`，GoatCounter 使用 `73ceee16...`；内部 site/api/admin/stats 返回 `200/200/200/303`，GoatCounter health 为 healthy，公网五域名和 HTTP→HTTPS `308` 验证通过。

### 7. 收窄物理机 SSH 来源（已实施：2026-08-08，待只读复核）

现状：物理机 UFW 当前允许 `192.168.0.0/24 -> 22/tcp`。这不是公网暴露，但超过单一管理终端的最小权限边界。

实施：确认管理终端 WLAN 地址为 `192.168.0.223/24`。先保留旧规则并新增 `192.168.0.223/32 -> 22/tcp`，通过 UFW 状态确认新规则存在和第二个独立密钥会话后，再删除 `192.168.0.0/24 -> 22/tcp`。删除命令成功，且删除后的全新 SSH 会话可正常登录。

剩余复核：需要在物理机交互执行一次 `sudo sshd -t`，并重新保存 `sudo ufw status numbered` 输出，确认只剩管理 `/32` 且没有新增 WireGuard SSH 来源。本次两次只读弹窗均因未输入 sudo 密码超时，未把该项误记为完全闭环。

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
