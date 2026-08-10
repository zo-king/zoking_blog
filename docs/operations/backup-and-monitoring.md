# 生产备份、恢复与监控

本文固化 Zoking Blog 的自动备份、异机副本、恢复演练和健康告警。命令默认生产代码位于物理机 `/opt/zoking-blog`。

## 1. 目标

- 每天备份 PostgreSQL、媒体、发布产物、统计数据和生产配置。
- 保留 7 天日备份、4 周周备份和 3 个月月备份。
- 备份带 SHA-256 manifest，可独立校验。
- 至少保存一份异机副本，避免物理机磁盘故障同时损失线上数据和备份。
- 每 5 分钟检查应用或 Azure 入口；失败写入 journal，并可投递 Webhook。

## 2. 备份脚本行为

`scripts/ops/backup-production.sh` 会：

1. 使用 `flock` 防止任务重叠。
2. 通过生产 Compose 内的 `pg_dump -Fc` 生成逻辑数据库备份。
3. 解析 Compose project label，定位并归档 `media_data`、`site_releases`、`publisher_site`、`goatcounter_data`。
4. 复制 `.env.prod`、Compose 配置、WireGuard 配置（可读时）、Git commit 和容器清单。
5. 使用 `BACKUP_AGE_RECIPIENT` 将所有内容加密为 `.age` 文件；没有接收方公钥时直接失败。
6. 先校验明文内容 manifest，再删除 staging 明文，最后生成加密文件的 `SHA256SUMS`。
7. 先写 `.incomplete-*`，全部成功后才原子改名为正式日备份。
8. 周日用硬链接建立周备份，每月 1 日建立月备份。
9. 按日/周/月保留期清理旧目录。
10. 配置 `BACKUP_REMOTE` 时用 rsync over SSH 复制新备份；异机只收到加密文件。

脚本必须以 root 运行，因为 Docker volume mountpoint 和生产配置不是普通用户可读数据。

服务器 SSH 基线另见 `infra/ssh/00-zoking-hardening.conf`。安装前必须先验证目标账号的公钥登录；安装后依次执行 `sshd -t`、reload 和第二个新会话验证，不能直接关闭唯一管理会话。

## 3. 物理机安装

```bash
cd /opt/zoking-blog
sudo install -d -m 0750 /etc/zoking-blog /var/backups/zoking-blog
sudo install -m 0644 infra/systemd/zoking-backup.service /etc/systemd/system/
sudo install -m 0644 infra/systemd/zoking-backup.timer /etc/systemd/system/
sudo install -m 0644 infra/systemd/zoking-healthcheck.service /etc/systemd/system/
sudo install -m 0644 infra/systemd/zoking-healthcheck.timer /etc/systemd/system/
```

创建 `/etc/zoking-blog/ops.env`，权限必须为 `0600`：

```env
REPO_DIR=/opt/zoking-blog
ENV_FILE=/opt/zoking-blog/infra/docker/.env.prod
COMPOSE_FILE=/opt/zoking-blog/infra/docker/compose.prod.yml
BACKUP_ROOT=/var/backups/zoking-blog
DAILY_KEEP_DAYS=7
WEEKLY_KEEP_DAYS=35
MONTHLY_KEEP_DAYS=100
BACKUP_AGE_RECIPIENT=age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
DISK_WARN_PERCENT=80
BACKUP_MAX_AGE_HOURS=36
WG_MAX_AGE_SECONDS=300
# Azure remote-backup host only:
REMOTE_DAILY_KEEP_DAYS=7
REMOTE_WEEKLY_KEEP_DAYS=35
REMOTE_MONTHLY_KEEP_DAYS=100
REMOTE_DISK_WARN_PERCENT=80
# BACKUP_REMOTE=zoking-backup@10.20.0.1:/
# BACKUP_SSH_KEY=/etc/zoking-blog/backup_ed25519
# ALERT_WEBHOOK_URL=https://open.feishu.cn/open-apis/bot/v2/hook/REDACTED
```

启用并立即测试：

```bash
sudo systemctl daemon-reload
sudo systemctl start zoking-backup.service
sudo /opt/zoking-blog/scripts/ops/verify-backup.sh /var/backups/zoking-blog/daily/latest
sudo systemctl enable --now zoking-backup.timer zoking-healthcheck.timer
systemctl list-timers 'zoking-*' --all
```

日志：

```bash
sudo journalctl -u zoking-backup.service -n 100 --no-pager
sudo journalctl -u zoking-healthcheck.service -n 100 --no-pager
```

## 4. 异机副本

生产已在 Azure VPS 创建专用 `zoking-backup` 用户和 `/var/backups/zoking-blog`，只允许物理机专用 SSH key 写入该目录。不要复用管理员私钥，也不要允许该账号 sudo。启用加密后，Azure 只保存 `.age` 文件和加密文件 manifest。

物理机 `/etc/zoking-blog/ops.env`：

```env
BACKUP_REMOTE=zoking-backup@10.20.0.1:/
BACKUP_SSH_KEY=/etc/zoking-blog/backup_ed25519
```

Azure 的 `authorized_keys` 应使用 `restrict,command="/usr/bin/rrsync -wo -no-del /var/backups/zoking-blog"`。`rrsync` 会把客户端的 `/daily/...` 映射到该受限根目录，并拒绝任意 shell 命令、删除选项、PTY 和端口转发。

要求：

- 只经 WireGuard 地址传输。
- Azure 目标目录仅 `zoking-backup` 与 root 可读。
- `.env.prod` 与数据库包含敏感数据；推荐使用 age/对象存储服务端加密再形成长期异机归档。
- 远端必须有独立保留清理策略，不能让 rsync 使用未经检查的 `--delete`。

Azure 上以 root 安装独立维护任务。它先校验最新加密 manifest，按 UTC 日/周/月建立硬链接保留层级，再按目录名中的 UTC 备份时间清理过期目录；不会使用 `rsync --delete`，也不会删除最新目录。默认保留期与物理机一致：7 天日备份、35 天周备份、100 天月备份。

```bash
sudo install -m 0755 maintain-remote-backups.sh /opt/zoking-ops/maintain-remote-backups.sh
sudo install -m 0644 zoking-remote-backup-maintenance.service /etc/systemd/system/
sudo install -m 0644 zoking-remote-backup-maintenance.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now zoking-remote-backup-maintenance.timer
sudo /opt/zoking-ops/maintain-remote-backups.sh --check
```

`zoking-edge-healthcheck.service` 同时校验 Azure 最新备份的加密布局、`SHA256SUMS`、备份年龄和 `/var/backups/zoking-blog` 所在文件系统。维护 timer 默认在每日 `04:15 UTC` 后运行，避开物理机 `03:30 UTC` 备份窗口。

## 5. 手工验证备份

```bash
sudo scripts/ops/verify-backup.sh /var/backups/zoking-blog/daily/latest
sudo du -sh /var/backups/zoking-blog/daily/latest
sudo sha256sum -c /var/backups/zoking-blog/daily/latest/SHA256SUMS
```

`verify-backup.sh` 默认只检查 `.age` 文件、加密 manifest 和文件完整性；它不会修改生产数据，也不需要在服务器保存私钥。恢复演练时，在隔离环境临时提供密码管理器导出的 `BACKUP_AGE_IDENTITY`：

```bash
BACKUP_AGE_IDENTITY=/run/user/1000/zoking-blog-recovery-key.txt \
  sudo -E scripts/ops/verify-backup.sh /var/backups/zoking-blog/daily/latest
```

验证完成后立即删除临时 identity 文件；物理机和 Azure 不保存长期私钥。

## 6. 数据库恢复演练

恢复演练应优先在隔离 PostgreSQL 容器中完成，不能直接覆盖生产：

```bash
BACKUP=/var/backups/zoking-blog/daily/latest
export BACKUP_AGE_IDENTITY=/secure/recovery/zoking-blog-age-key.txt
docker run --rm -d --name zoking-restore-test \
  -e POSTGRES_PASSWORD=restore-test-only \
  -e POSTGRES_DB=zoking_blog_restore \
  postgres:16-alpine

until docker exec zoking-restore-test pg_isready -U postgres; do sleep 1; done
docker exec -i zoking-restore-test pg_restore \
  -U postgres -d zoking_blog_restore --clean --if-exists < <(
    age --decrypt --identity "$BACKUP_AGE_IDENTITY" "$BACKUP/postgres.dump.age"
  )
docker exec zoking-restore-test psql -U postgres -d zoking_blog_restore \
  -c 'select count(*) as users from users; select count(*) as posts from posts;'
docker rm -f zoking-restore-test
unset BACKUP_AGE_IDENTITY
rm -f /secure/recovery/zoking-blog-age-key.txt
```

演练记录至少包含备份时间、恢复时间、数据行抽查、错误、RPO 和 RTO。

### 服务级恢复演练

季度演练使用 `drill-service-restore.sh`。脚本从当前生产 API 容器读取不可变镜像 ID，但只创建带 `zoking.restore-drill=true` 标签的临时容器、内部网络和专用 volumes；PostgreSQL、API 与 Site 均不映射主机端口，验证请求由内部网络中的一次性探针发起。它不会调用生产 Compose 的启动、停止或重建命令。

先执行无密钥、只读预检：

```bash
cd /opt/zoking-blog
sudo scripts/ops/drill-service-restore.sh --preflight
```

从密码管理器把 age identity 直接导出到 `/run/user/<uid>/` 或 root 专用的 `/run/zoking-recovery/`，权限必须为 `0600`。不要把私钥粘贴到聊天、命令行参数、journal、仓库或普通磁盘临时目录。完整演练会在终端静默读取博客后台管理员密码（不是 SSH/sudo 密码，最多尝试三次），依次验证内容 manifest、数据库恢复、迁移、API `/readyz`、公开数据、站点/媒体、管理员登录及一次隔离发布：

```bash
sudo install -d -m 0700 /run/zoking-recovery
sudo scripts/ops/drill-service-restore.sh \
  --identity /run/zoking-recovery/zoking-blog-age-identity.txt
```

脚本无论成功或失败都会删除 identity、解密文件、临时容器、网络和 volumes，并核对生产容器 ID 未变化。只有明确记录 `service-level restore passed`、RTO/RPO、数据计数和清理结果后，才能关闭服务级恢复演练事项。

## 7. 生产恢复

生产恢复是高风险操作，必须先记录现状并保留故障数据：

1. 确认目标备份 manifest 通过。
2. 停止 `worker`，避免继续发布。
3. 停止或切换 API 到维护状态，阻止写入。
4. 再备份一次当前数据库和卷。
5. 对数据库执行 `pg_restore --clean --if-exists`。
6. 仅在确需恢复文件时清空目标卷后解压媒体/release；先在临时目录检查归档。
7. 启动 PostgreSQL、执行 `/readyz`，再启动 API、Worker、Admin 和 Site。
8. 验证登录、文章、媒体、评论、发布和公网入口。

恢复卷会覆盖数据，本文不提供可直接粘贴的递归删除命令。操作者必须通过 `docker volume inspect` 明确实际卷，并在变更记录中写明目标和回滚副本。

## 8. 应用健康检查

`check-production.sh --role app` 检查：

- `postgres`、`api`、`worker`、`admin`、`site`、`goatcounter` 容器运行状态。
- 容器 health 非 unhealthy。
- Reader、API ready、Admin 和 Stats 的 WireGuard 地址。
- WireGuard 最近握手。
- 根分区使用率。
- 最近备份不超过 36 小时。

手工执行：

```bash
sudo /opt/zoking-blog/scripts/ops/check-production.sh --role app
```

## 9. Azure 入口健康检查

把脚本复制到 Azure：

```bash
sudo install -d -m 0755 /opt/zoking-ops
sudo install -m 0755 check-production.sh /opt/zoking-ops/check-production.sh
sudo install -m 0644 zoking-edge-healthcheck.service /etc/systemd/system/
sudo install -m 0644 zoking-edge-healthcheck.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now zoking-edge-healthcheck.timer
```

Edge 模式检查 Caddy、WireGuard、四个内网服务、五个公网域名、证书剩余时间和磁盘。

## 10. 外部告警

当前使用飞书群机器人接收告警。创建机器人后，将 Webhook URL 仅写入服务器 `/etc/zoking-blog/ops.env` 的 `ALERT_WEBHOOK_URL`。URL 是 secret，不可粘贴到聊天、写入 Git、命令行历史、journal、截图或审计报告。

推荐用编辑器在服务器上写入，避免 URL 出现在 shell 历史：

```bash
sudoedit /etc/zoking-blog/ops.env
sudo chown root:root /etc/zoking-blog/ops.env
sudo chmod 0600 /etc/zoking-blog/ops.env
```

脚本发送飞书文本消息：

```json
{"msg_type":"text","content":{"text":"Zoking edge health check failed on host: ..."}}
```

飞书返回非零业务码或 HTTP 错误时，脚本会在 journal 记录投递失败，但不会记录 URL 或响应正文。配置完成后必须触发一次受控失败，确认接收群确实收到告警；恢复服务后再确认下一轮健康检查通过。没有配置 Webhook 时，只能通过 journal 和失败单元发现问题。

## 11. TLS 说明

Caddy 自动续期证书。Edge 健康检查读取实际外部证书，当剩余时间少于 14 天时失败。证书监控用于发现 DNS、80/443、ACME 或系统时间异常，不替代 Caddy 自动续期。
