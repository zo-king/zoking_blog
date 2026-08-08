# React Router 风险接受闭环记录：2026-08-07 至 2026-08-08

## 决策

- 状态：已关闭，不再需要风险例外
- 负责人：项目所有者
- 适用版本：`react-router` 与 `react-router-dom@7.18.2`
- 公告：`GHSA-qwww-vcr4-c8h2`
- 关闭时间：2026-08-08（Asia/Shanghai）
- 关闭依据：GitHub Advisory 于 2026-08-07 18:16:54 UTC 更新受影响范围，`7.18.2` 被列为 7.x 首个修复版本

## 依据

官方 registry 在 2026-08-07 初次报告该公告影响 `>=7.12.0 <8.3.0`，当时 npm 上最新稳定版为 `7.18.2`，因此项目建立了到 2026-08-14 的临时例外。GitHub Advisory 随后把 7.x 受影响范围修正为 `>=7.12.0 <7.18.2`，并把 `7.18.2` 列为首个修复版本；8.x 的修复版本为 `8.3.0`。

当前 Admin 是由 Nginx 提供的静态浏览器应用，只使用 `BrowserRouter`、`Routes`、`Route`、`Navigate`、`useNavigate`、`useParams`、`useLocation` 和 `useSearchParams`。生产环境没有 React Router Node 服务端、RSC mode、server action、route action/loader 或 `react-router/*` 服务端入口，因此公告所需的服务端 action 请求路径当前不存在。

## 关闭动作

- 保持 `react-router` 与 `react-router-dom@7.18.2`，不做会重新引入旧公告的版本回退。
- 删除带到期时间和公告白名单的例外脚本。
- `npm run audit:security` 改为直接调用官方 npm registry；任何 high/critical 生产依赖漏洞都会使 CI 失败。
- Dependabot 每周检查 npm 依赖，后续升级仍须通过构建、lint 和无豁免安全审计。

## 验证记录

- `npm run build`：通过。
- `npm run lint`：0 errors；15 个既有 warnings，与本次依赖评估无关。
- `npm run format:check`：通过。
- GitHub Advisory API：`GHSA-qwww-vcr4-c8h2` 未撤销，7.x 范围为 `>=7.12.0 <7.18.2`，`first_patched_version=7.18.2`。
- `npm audit --registry=https://registry.npmjs.org --omit=dev --audit-level=high`：0 个 high/critical，退出码 0。

本记录保留初始判断和后续公告修正的审计链；当前不再存在该公告对应的临时风险接受。
