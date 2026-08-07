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
5. 为所有文件生成 `SHA256SUMS` 并立即校验。
6. 先写 `.incomplete-*`，全部成功后才原子改名为正式日备份。
7. 周日用硬链接建立周备份，每月 1 日建立月备份。
8. 按日/周/月保留期清理旧目录。
9. 配置 `BACKUP_REMOTE` 时用 rsync over SSH 复制新备份。

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
DISK_WARN_PERCENT=80
BACKUP_MAX_AGE_HOURS=36
WG_MAX_AGE_SECONDS=300
# BACKUP_REMOTE=backup@10.20.0.1:/var/backups/zoking-blog
# ALERT_WEBHOOK_URL=https://example.invalid/secret-webhook
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

推荐在 Azure VPS 创建专用 `backup` 用户和 `/var/backups/zoking-blog`，只允许物理机专用 SSH key 写入该目录。不要复用管理员私钥，也不要允许该账号 sudo。

物理机 `/etc/zoking-blog/ops.env`：

```env
BACKUP_REMOTE=backup@10.20.0.1:/var/backups/zoking-blog
```

要求：

- 只经 WireGuard 地址传输。
- Azure 目标目录仅 `backup` 与 root 可读。
- `.env.prod` 与数据库包含敏感数据；推荐使用 age/对象存储服务端加密再形成长期异机归档。
- 远端必须有独立保留清理策略，不能让 rsync 使用未经检查的 `--delete`。

## 5. 手工验证备份

```bash
sudo scripts/ops/verify-backup.sh /var/backups/zoking-blog/daily/latest
sudo du -sh /var/backups/zoking-blog/daily/latest
sudo cat /var/backups/zoking-blog/daily/latest/git-commit.txt
```

`verify-backup.sh` 会检查必需文件、SHA-256 和四个 tar 归档是否可读取，但不会修改生产数据。

## 6. 数据库恢复演练

恢复演练应优先在隔离 PostgreSQL 容器中完成，不能直接覆盖生产：

```bash
BACKUP=/var/backups/zoking-blog/daily/latest
docker run --rm -d --name zoking-restore-test \
  -e POSTGRES_PASSWORD=restore-test-only \
  -e POSTGRES_DB=zoking_blog_restore \
  postgres:16-alpine

until docker exec zoking-restore-test pg_isready -U postgres; do sleep 1; done
docker exec -i zoking-restore-test pg_restore \
  -U postgres -d zoking_blog_restore --clean --if-exists < "$BACKUP/postgres.dump"
docker exec zoking-restore-test psql -U postgres -d zoking_blog_restore \
  -c 'select count(*) as users from users; select count(*) as posts from posts;'
docker rm -f zoking-restore-test
```

演练记录至少包含备份时间、恢复时间、数据行抽查、错误、RPO 和 RTO。

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

将受信 Webhook 写入 `/etc/zoking-blog/ops.env` 的 `ALERT_WEBHOOK_URL`。URL 是 secret，不可提交。脚本发送最小 JSON：

```json
{"text":"Zoking edge health check failed on host: ..."}
```

不同平台的 Webhook schema 可能不同；上线前必须触发一次受控失败，确认接收人确实收到告警。没有配置 Webhook 时，只能通过 journal 和失败单元发现问题。

## 11. TLS 说明

Caddy 自动续期证书。Edge 健康检查读取实际外部证书，当剩余时间少于 14 天时失败。证书监控用于发现 DNS、80/443、ACME 或系统时间异常，不替代 Caddy 自动续期。
