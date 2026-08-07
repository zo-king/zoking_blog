# 生产审计报告：2026-08-07

## 1. 范围与方法

本次审计覆盖：

- 本地 Git 工作区、历史引用、`.gitignore`、CI 工作流、Dockerfile、Compose 和运维脚本。
- 物理服务器 `192.168.0.21 / 10.20.0.2` 的容器、监听端口、UFW 规则、文件权限、systemd timers、备份状态和仓库状态。
- Azure `104.41.165.102 / 10.20.0.1` 的 SSH、UFW、NSG、Caddy、WireGuard、监听端口、Edge timer、失败单元和备份账号。
- 五个公网 HTTPS 域名、重定向、响应安全头和证书检查。
- 依赖审计：官方 npm registry 的 `npm audit`；Go 依赖清单只做枚举，当前环境没有 `govulncheck`。

审计只读采集了生产状态；没有读取或输出生产 secret，没有修改生产数据或网络规则。

## 2. 总体结论

当前可用性和基础安全基线良好：公网入口、TLS、WireGuard、Caddy、Docker Compose、应用健康、SSH 密钥登录和备份 manifest 均通过。仓库扫描没有发现私钥、GitHub token、AWS key 或被跟踪的 `.env.prod`。

“灾备与供应链安全闭环”尚未完成。最高优先级是异机备份静态加密、恢复演练、限制备份 SSH key，以及处理 React Router high advisory。详细动作见 [post-audit-action-plan.md](../operations/post-audit-action-plan.md)。

## 3. 已验证证据

| 区域 | 证据 | 结论 |
|---|---|---|
| 公网 | `zoking.tech=200`、`api=/readyz=200`、`admin=200`、`preview/=404`、`stats=303`、HTTP 跳转 `308` | 通过 |
| TLS/Headers | Caddy、HSTS、CSP（Admin/Stats）、`nosniff`、`DENY`、Referrer-Policy | 通过；API/Preview 的 HSTS 需确认 Caddy 全局策略 |
| 物理机容器 | `api`、`worker`、`admin`、`site`、`postgres`、`goatcounter` running；Postgres/Site/GoatCounter healthy | 通过 |
| 物理机绑定 | 应用仅监听 `10.20.0.2`；无 PostgreSQL 发布端口 | 通过 |
| 物理机 UFW | 1313/18080/8081/8100 仅 `wg0` 来源 `10.20.0.1`；SSH 为 `192.168.0.0/24` | 应用通过，SSH 需收窄 |
| Azure | Caddy、WireGuard、SSH active/enabled；无 failed units；Caddy validate 通过 | 通过 |
| Azure NSG/UFW | 公网 22 仅 `218.64.59.174/32`；备份 22 仅 `10.20.0.2`；80/443/51820 正常 | 通过 |
| 备份 | 本地和 Azure 副本 manifest 均校验通过；`.env.prod` 未跟踪 | 传输通过，静态保护和恢复演练未闭环 |
| 仓库 secret | 未发现私钥、GitHub token、AWS key；命中项均为开发/CI 占位或测试参数 | 通过，但扫描不是密钥轮换证明 |
| npm | 官方 registry 报告 `react-router` 与 `react-router-dom@7.18.2` 两项 high | 需处理/批准例外 |

## 4. 风险发现

### AUD-001：异机备份未静态加密，且未完成恢复证明

**优先级：P1 高**  
**证据：** 备份归档包含 `config/env.prod`、数据库 dump、WireGuard 配置；当前仅 rsync over SSH，Azure 目录中仍是可读归档。最近备份虽然通过 SHA-256，但完整性不等于保密性或可恢复性。  
**影响：** Azure 磁盘、备份账号或 root 权限泄露会暴露数据库和生产配置；未做恢复演练则无法证明 RPO/RTO。  
**动作：** 见行动计划 P0-1、P0-3。

### AUD-002：备份 SSH key 没有 forced command/restrict

**优先级：P1 高**  
**证据：** Azure `zoking-backup` 无 sudo，但 `/var/lib/zoking-backup/.ssh/authorized_keys` 未包含 `restrict` 或 `command=` 限制。  
**影响：** 备份私钥泄露后可执行任意备份账号命令、探测 Azure 主机并读取该账号可读的全部归档。  
**动作：** 使用只允许 rsync 目标目录的 wrapper，并用负向 SSH 测试证明命令、PTY、转发均被拒绝。

### AUD-003：React Router 依赖存在官方 high advisory

**优先级：P1 高（当前利用面待确认）**  
**证据：** `npm audit --registry=https://registry.npmjs.org --omit=dev --audit-level=high` 报 `GHSA-qwww-vcr4-c8h2`，影响 `react-router-dom@7.18.2`。代码审阅未发现 RSC action/server route，只使用浏览器路由和 hooks。  
**影响：** 依赖升级或未来启用相关模式后，可能出现 CSRF action 执行风险；当前代码路径降低了直接利用可能性，但不能替代升级。  
**动作：** 分支升级/降级验证，或形成有到期日的风险接受记录。

### AUD-004：CI Action 与 Docker 下载源未固定到不可变版本

**优先级：P2 中**  
**证据：** GitHub Actions 使用 `actions/checkout@v7`、`setup-node@v7` 等浮动 major tag；Dockerfile 使用 `ghfast.top`、镜像站和远端 release URL，未校验下载 SHA256。  
**影响：** 上游 tag、镜像代理或下载链路被替换时，构建可引入未审查代码；也会造成不可重复构建。  
**动作：** 固定 action SHA、记录基础镜像 digest、为 Hugo/Pagefind/GoatCounter 下载增加 checksum，并减少第三方代理依赖。

### AUD-005：物理机 SSH 防火墙范围大于必要管理范围

**优先级：P2 中**  
**证据：** `/etc/ufw/user.rules` 允许 `192.168.0.0/24 -> tcp/22`。应用端口已正确限制到 `wg0` 的 `10.20.0.1`。  
**影响：** 局域网任意受感染主机都能到达 SSH 暴露面；密钥和 sshd 加固降低了风险，但未达到最小来源原则。  
**动作：** 收窄到固定管理终端 `/32`，或明确只通过 WireGuard/Bastion 管理。

### AUD-006：健康检查会把自身 failed 状态当作新的故障

**优先级：P2 中，已修复待部署验证**  
**证据：** 原 `check_failed_units` 统计所有 failed units；systemd oneshot 失败状态会保留，导致后续探针可能因为上一次自身失败继续失败。此次现场曾出现两个 LXD failed 单元并触发该行为。  
**修复：** 已在 `scripts/ops/check-production.sh` 中排除两个健康探针自身，保留应用、备份和主机单元失败；需随下一次生产同步验证。

### AUD-007：外部告警和恢复演练尚未产生可审计证据

**优先级：P2 中**  
**证据：** timers 和 journal 检查通过，但 `ALERT_WEBHOOK_URL` 未配置，也没有受控失败接收记录；备份验证通过但没有隔离恢复记录。  
**影响：** 线上故障可能只有本机 journal，没有人收到通知；备份可读不代表能恢复业务。  
**动作：** 见行动计划 P1-7 和 P0-3。

## 5. 限制与未完成检查

- 当前环境没有 `govulncheck`、Trivy 或 Syft，因此 Go 依赖和最终镜像漏洞没有独立扫描证明。
- GitHub Actions 权限、分支保护和管理员交付状态未通过 GitHub 管理 API 完整核验。
- 没有配置 Webhook，无法审计真实通知链路。
- 没有执行生产数据库/媒体恢复演练，不能声明 RTO 已达标。
- 生产 GitHub 直连曾发生 TLS 超时；本次同步使用 Git bundle，因此 `git fetch` 的网络恢复仍需单独确认。

## 6. 审计复核标准

下一次复核必须重新采集：公网探针、两台主机 failed units、timers、SSH/UFW/NSG、备份最新时间与远端 manifest、npm/go/container 扫描结果、恢复演练记录和外部告警接收证据。复核报告应引用具体 commit 和命令时间，不把旧文档状态当作实时证据。
