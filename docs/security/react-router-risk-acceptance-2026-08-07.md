# React Router 临时风险接受：2026-08-07

## 决策

- 状态：临时接受，CI 强制到期
- 负责人：项目所有者
- 适用版本：`react-router` 与 `react-router-dom@7.18.2`
- 公告：`GHSA-qwww-vcr4-c8h2`
- 到期时间：2026-08-14 23:59:59（Asia/Shanghai）
- 到期动作：升级到官方修复版本，或重新评估并由项目所有者明确续期；不得静默延长

## 依据

官方 registry 在 2026-08-07 报告该公告影响 `>=7.12.0 <8.3.0`，而 npm 上最新稳定版仍是 `7.18.2`。官方 audit 建议的 `7.11.0` 回退已在本地验证，但它会重新引入多项已修复的 high/moderate 公告，包括 XSS、开放重定向、DoS 和反序列化问题，因此已撤销，锁文件恢复到 `7.18.2`。

当前 Admin 是由 Nginx 提供的静态浏览器应用，只使用 `BrowserRouter`、`Routes`、`Route`、`Navigate`、`useNavigate`、`useParams`、`useLocation` 和 `useSearchParams`。生产环境没有 React Router Node 服务端、RSC mode、server action、route action/loader 或 `react-router/*` 服务端入口，因此公告所需的服务端 action 请求路径当前不存在。

## 补偿控制

- `scripts/qa/npm-security-audit.mjs` 通过官方 npm registry 执行生产依赖审计。
- CI 只临时允许上述两个包的唯一公告；新增 high/critical、公告集合变化或包版本变化都会失败。
- 例外到期后，只要公告仍存在，CI 就会失败。
- 启用 SSR、RSC、React Router action/loader 或 Node 服务端部署前，必须先撤销本例外并完成安全复核。

## 验证记录

- `npm run build`：通过。
- `npm run lint`：0 errors；15 个既有 warnings，与本次依赖评估无关。
- `npm run format:check`：通过。
- `npm audit --registry=https://registry.npmjs.org --omit=dev --audit-level=high`：仅剩本记录覆盖的两个依赖节点和一条公告。

本记录不声明依赖本身安全，只说明在当前静态客户端部署边界内接受有限时效的剩余风险。
