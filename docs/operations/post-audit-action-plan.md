# 生产审计与后续行动计划

审计日期：2026-08-07  
审计对象：Zoking Blog 仓库、物理服务器、Azure Edge、WireGuard、Caddy、Docker Compose、备份与监控  
对应报告：[production-audit-2026-08-07.md](../security/production-audit-2026-08-07.md)

## 当前结论

线上服务、TLS、WireGuard、容器、应用备份和 SSH 密钥登录均正常。生产 secret 未被 Git 跟踪，`.env.prod` 仅存在物理机并为 `root:docker 0640`。以下事项不会阻断当前服务，但在宣布“灾备和安全闭环完成”前必须处理。

## P0：24 小时内

### 1. 保护异机备份静态数据

现状：备份通过 WireGuard + SSH 加密传输，但 Azure `/var/backups/zoking-blog` 上的归档本身未加密；归档包含数据库、`.env.prod` 和 `wg0.conf`。

动作：选择并记录独立的 at-rest 加密方案（优先 age + 离线/密码管理器托管恢复私钥，或 Azure Storage 服务端加密），不要把解密私钥与备份放在同一主机。保留现有 SHA-256 manifest，明确加密前后校验顺序。

验收：在临时目录中使用独立恢复环境解密一份备份，能读取 PostgreSQL dump 和媒体归档；物理机和 Azure 均不保存明文长期副本；恢复密钥由第二名管理员独立取得。

### 2. 限制备份 SSH key 的能力

现状：`zoking-backup` 无 sudo，但 `authorized_keys` 没有 `restrict` 或 forced command；密钥泄露后可在 Azure 账号权限内执行任意命令。

动作：增加只允许 rsync 写入 `/var/backups/zoking-blog` 的 forced-command wrapper，同时启用 `restrict`、禁用 PTY、转发和 agent forwarding。wrapper 必须拒绝非预期 `SSH_ORIGINAL_COMMAND`，并记录拒绝事件但不得记录密钥或归档内容。

验收：正常备份仍能传输；使用同一 key 执行 `id`、端口转发、PTY 请求均被拒绝；目标目录之外不可写。

### 3. 完成一次隔离恢复演练

动作：按 `docs/operations/backup-and-monitoring.md` 第 6 节，在临时 PostgreSQL 和临时卷恢复最近备份，不接触生产卷；抽查用户、文章、媒体、发布产物和统计数据。

验收：记录备份时间、恢复开始/结束时间、RPO、RTO、抽查行数、失败项和清理结果；演练产物不得包含明文密码并在结束后删除。

## P1：7 天内

### 4. 补齐全站 HSTS

审计证据：公网响应中 Admin 带 `Strict-Transport-Security`，Reader、API、Preview 和 Stats 当前未稳定返回 HSTS。所有域名均已强制 HTTPS，但缺少浏览器侧的长期 HTTPS 记忆。

动作：在 Caddy 的公共站点块统一添加 `Strict-Transport-Security: max-age=31536000; includeSubDomains`；先确认所有现有和计划中的子域名均只提供 HTTPS，再 reload 并检查五个域名。

验收：五个 HTTPS 域名均返回 HSTS；HTTP 仍为 308；Caddy validate、证书续期和公网黑盒全部通过。

### 5. 处理 React Router 安全公告

审计证据：官方 npm registry 的 `npm audit --omit=dev --audit-level=high` 报告 `react-router` / `react-router-dom@7.18.2` 两项 high，GHSA-qwww-vcr4-c8h2，范围为 `>=7.12.0 <8.3.0`。当前代码只使用 BrowserRouter、Routes、Route 和导航 hooks，未发现 RSC action/server route；因此可利用面尚未证实，但不能把“未使用 RSC”当作永久豁免。

动作：先在分支验证官方建议版本或替代路由方案，执行 `npm ci`、`npm run lint`、`npm run build`、路由黑盒和后台登录回归；若暂时无法升级，建立带负责人和到期日的临时风险接受记录。

验收：CI 中 `npm audit --registry=https://registry.npmjs.org --omit=dev --audit-level=high` 不再出现 high，或有明确批准的例外、监控和到期日期。

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
