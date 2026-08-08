# 加密备份迁移记录：2026-08-07

## 实施

- 工具：Ubuntu 24.04 官方 `age 1.1.1-1ubuntu0.24.04.3`。
- 仓库提交：`3343648`。
- 物理机配置：`/etc/zoking-blog/ops.env` 仅保存 `BACKUP_AGE_RECIPIENT` 公钥，权限 `root:root 0600`。
- 私钥：未上传 Azure；物理机只在一次隔离验证期间临时持有，验证结束已删除。

## 备份验证

- 备份：`20260807T150122Z`。
- 物理机本地：`SHA256SUMS` 通过，目录中除 manifest 外没有明文文件。
- Azure 异机副本：`SHA256SUMS` 通过，目录中除 manifest 外没有明文文件；目标目录权限保持 `zoking-backup:zoking-backup 0700`。
- 完整恢复校验：临时 identity 解密 `CONTENT-SHA256SUMS`、PostgreSQL dump、媒体和三类发布/统计归档均通过，随后删除临时 identity。
- 加密失败保护：首次运行发现 manifest 自引用并失败，staging 自动清理；修复后重跑成功，没有上传半成品。

## 迁移完成：2026-08-08

- 项目所有者确认恢复私钥已进入独立托管位置，本机 ACL 暂存文件随后删除。
- 物理机 6 份、Azure 4 份历史明文日备份已按精确目录删除。
- 两端只保留 `20260807T150122Z` 与自动生成的 `20260808T033549Z` 两份加密备份；两份最新 manifest 均通过，最终扫描无明文文件。
- 备份与健康检查 timers 保持 active + enabled，最新应用健康检查通过。

后续季度服务级演练仍需使用独立托管私钥验证 API `/readyz`、Admin 登录、站点读取和隔离发布；该事项不再阻塞静态加密迁移闭环。
