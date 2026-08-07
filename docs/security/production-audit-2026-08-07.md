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

“灾备与供应链安全闭环”尚未完成。最高优先级是异机备份静态加密、服务级恢复、外部告警和供应链固定；React Router high advisory 已建立限期风险接受和 CI 到期门禁。详细动作见 [post-audit-action-plan.md](../operations/post-audit-action-plan.md)。

## 3. 已验证证据

| 区域 | 证据 | 结论 |
|---|---|---|
| 公网 | `zoking.tech=200`、`api=/readyz=200`、`admin=200`、`preview/=404`、`stats=303`、HTTP 跳转 `308` | 通过 |
| TLS/Headers | Caddy、统一 HSTS、CSP（Admin/Stats）、`nosniff`、`DENY`、Referrer-Policy | 通过；五域名均只返回一条统一 HSTS |
| 物理机容器 | `api`、`worker`、`admin`、`site`、`postgres`、`goatcounter` running；Postgres/Site/GoatCounter healthy | 通过 |
| 物理机绑定 | 应用仅监听 `10.20.0.2`；无 PostgreSQL 发布端口 | 通过 |
| 物理机 UFW | 1313/18080/8081/8100 仅 `wg0` 来源 `10.20.0.1`；SSH 为 `192.168.0.0/24` | 应用通过，SSH 需收窄 |
| Azure | Caddy、WireGuard、SSH active/enabled；无 failed units；Caddy validate 通过 | 通过 |
| Azure NSG/UFW | 公网 22 仅 `218.64.59.174/32`；备份 22 仅 `10.20.0.2`；80/443/51820 正常 | 通过 |
| 备份 | 现有本地和 Azure 副本 manifest 均校验通过；加密脚本已部署到仓库但尚未配置接收方公钥；`.env.prod` 未跟踪 | 传输通过，静态加密待迁移 |
| 仓库 secret | 未发现私钥、GitHub token、AWS key；命中项均为开发/CI 占位或测试参数 | 通过，但扫描不是密钥轮换证明 |
| npm | 官方 registry 报告 `react-router` 与 `react-router-dom@7.18.2` 两项 high | 已建立至 2026-08-14 的 CI 强制到期例外 |

## 4. 风险发现

### AUD-001：异机备份尚未完成静态加密迁移，且服务级恢复尚未完成

**优先级：P1 高**  
**证据：** 现有历史归档仍包含可读的 `config/env.prod`、数据库 dump、WireGuard 配置；仓库脚本已改为 age 加密，但物理机尚未配置接收方公钥。数据级恢复演练已通过，但尚未使用恢复库启动 API 验证服务级 RTO。
**影响：** Azure 磁盘、备份账号或 root 权限泄露会暴露数据库和生产配置；服务级演练缺失则无法证明业务恢复 RTO。
**动作：** 见行动计划 P0-1、P0-3。

### AUD-002：备份 SSH key 没有 forced command/restrict（已修复）

**优先级：P1 高**  
**证据：** Azure `zoking-backup` 无 sudo，但 `/var/lib/zoking-backup/.ssh/authorized_keys` 未包含 `restrict` 或 `command=` 限制。  
**影响：** 备份私钥泄露后可执行任意备份账号命令、探测 Azure 主机并读取该账号可读的全部归档。  
**修复：** 已配置 `rrsync -wo -no-del` + `restrict`；完整备份和远端 manifest 通过，任意命令及删除选项负向测试通过。

### AUD-003：React Router 依赖存在官方 high advisory（限期风险接受）

**优先级：P1 高（当前利用面待确认）**  
**证据：** `npm audit --registry=https://registry.npmjs.org --omit=dev --audit-level=high` 报 `GHSA-qwww-vcr4-c8h2`，影响 `react-router-dom@7.18.2`。代码审阅未发现 RSC action/server route，只使用浏览器路由和 hooks。  
**影响：** 依赖升级或未来启用相关模式后，可能出现 CSRF action 执行风险；当前静态客户端部署没有公告要求的服务端 action 路径。
**处置：** `7.11.0` 回退验证会引入更多既有公告，故保持 `7.18.2`；CI 仅允许该精确公告并在 2026-08-14 强制到期。完整记录见 [react-router-risk-acceptance-2026-08-07.md](react-router-risk-acceptance-2026-08-07.md)。

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

**优先级：P2 中，已修复并验证**
**证据：** 原 `check_failed_units` 统计所有 failed units；systemd oneshot 失败状态会保留，导致后续探针可能因为上一次自身失败继续失败。此次现场曾出现两个 LXD failed 单元并触发该行为。  
**修复：** 已在 `scripts/ops/check-production.sh` 中排除两个健康探针自身，保留应用、备份和主机单元失败；物理机最新执行 `Result=success / ExecMainStatus=0`，Azure Edge 最新执行也成功。

### AUD-007：外部告警尚未产生可审计证据

**优先级：P2 中**  
**证据：** timers 和 journal 检查通过，但 `ALERT_WEBHOOK_URL` 未配置，也没有受控失败接收记录；数据级恢复记录已补充，服务级恢复仍在季度计划。
**影响：** 线上故障可能只有本机 journal，没有人收到通知；服务级恢复能力仍未完全证明。
**动作：** 见行动计划 P1-8 和 P0-3。

### AUD-008：HSTS 没有覆盖全部公网域名（已修复）

**优先级：P1 中**
**证据：** 初始公网 HEAD 检查中仅 Admin 返回 `Strict-Transport-Security`，Reader、API、Preview 和 Stats 未返回该头；整改后五个域名均返回统一 HSTS，所有 HTTP 请求仍能 308 跳转到 HTTPS。
**影响：** 首次访问或缓存清除后的浏览器仍可能先发出明文 HTTP 请求，增加降级和流量劫持窗口。
**修复：** Caddy 已统一添加 deferred HSTS；五域名均只有一条 `max-age=31536000; includeSubDomains`，HTTP 保持 308。

## 5. 限制与未完成检查

- 当前环境没有 `govulncheck`、Trivy 或 Syft，因此 Go 依赖和最终镜像漏洞没有独立扫描证明。
- GitHub Actions 权限、分支保护和管理员交付状态未通过 GitHub 管理 API 完整核验。
- 没有配置 Webhook，无法审计真实通知链路。
- 已完成数据级数据库/媒体恢复演练；尚未完成使用恢复库启动 API 的服务级演练。
- 生产 GitHub 直连曾发生 TLS 超时；本次同步使用 Git bundle，因此 `git fetch` 的网络恢复仍需单独确认。

## 6. 审计复核标准

下一次复核必须重新采集：公网探针、两台主机 failed units、timers、SSH/UFW/NSG、备份最新时间与远端 manifest、npm/go/container 扫描结果、恢复演练记录和外部告警接收证据。复核报告应引用具体 commit 和命令时间，不把旧文档状态当作实时证据。
