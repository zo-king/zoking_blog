# SITE-SIDEBAR-WIDGETS-P28-001 QA

## 范围

- 首页右栏按固定顺序展示“大家抢着看”、Pixiv 每日排行、一言。
- 验证本地热门文章解析、第三方请求边界、一言成功/刷新/缓存/失败降级。
- 验证 1280px、1024px 与 390px 布局、运行时错误和上传前置条件。

## 功能契约

### 大家抢着看

- 数据源是 `params.toml` 中显式配置的文章 slug，不读取或伪造浏览量。
- 当前顺序为 `about-zoking`、`city-walk`、`thoughtful-workspace`。
- 缺失 slug 静默跳过；全部缺失时不输出空 widget。
- 链接、标题、日期和封面均来自本地 Hugo 内容，封面缺失时使用无文本占位。

### Pixiv 每日排行

- iframe 地址为 `https://pixiv.mokeyjay.com/?limit=10`。
- 保留 `loading="lazy"`、`referrerpolicy="no-referrer"` 和受限 sandbox。
- iframe 不向本站父页面注入脚本；第三方内容、可用性和更新频率不由本仓库保证。

### 一言

- API 为 `https://v1.hitokoto.cn/?encode=json&charset=utf-8&max_length=48`。
- 进入可视区后请求，5 秒超时，`credentials: omit`、`no-referrer`。
- 仅接受非空文本和来源，文本最大 80 字符、来源最大 40 字符；使用 `textContent` 渲染。
- 成功结果在 `sessionStorage` 缓存 30 分钟；刷新按钮强制请求新内容。
- 网络、CORS、超时、非法 JSON、非法字段或存储受限时保留“保持好奇，也保持耐心。/本地寄语”，不得阻塞阅读。

## 测试矩阵

验收日期：2026-07-14。Hugo 为 Extended `0.160.1`，浏览器为系统 Microsoft Edge headless。

| 检查 | 结果 | 说明 |
|---|---|---|
| Production/minify build | Pass | 注入 `HUGO_COMMENTS_API_BASE=https://api.zoking.tech` 后，内存构建 61 pages |
| 首页 HTTP/静态契约 | Pass | 首页 200；3 篇热门；Pixiv 安全属性；指纹化一言脚本存在 |
| 第三方端点探测 | Pass | Pixiv HTML 与一言 JSON 均返回 HTTP 200；仅代表验收时状态 |
| 1280px | Pass | 三组件顺序正确、右栏可见、无横向溢出、零 console/page error |
| 1024px 临界宽度 | Pass | 三组件仍可见、无横向溢出、零 console/page error |
| 390px 移动端 | Pass | 整个右栏隐藏、正文无横向溢出、零 console/page error |
| 大家抢着看 | Pass | 按配置顺序渲染 3 条站内文章，无外部数据请求 |
| Pixiv 隔离 | Pass | lazy、no-referrer、sandbox 与来源链接保持 |
| 一言真实请求 | Pass | 可见后替换本地寄语，页面无运行时错误 |
| 一言首次/刷新 | Pass | 请求拦截分别渲染第一句和第二句，刷新按钮触发第二次请求 |
| 一言会话缓存 | Pass | 刷新后 reload 复用第二句，请求总数保持 2 |
| 一言断网降级 | Pass | API abort 后保持本地寄语与来源，page error 为 0 |
| 刷新图标回归 | Pass | 验收中曾因缺少资源导致构建失败；补齐 `assets/icons/refresh.svg` 后最终构建和首页均恢复 |

未在本轮重复执行完整 API/Admin/PostgreSQL E2E；P28 只修改 C 端静态组件，仓库级回归仍由上传前 preflight 负责。

## 验证命令

```powershell
$env:HUGO_COMMENTS_API_BASE = 'https://api.zoking.tech'
.\.tools\hugo\hugo.exe --source apps/site --environment production --minify --renderToMemory

Invoke-WebRequest -UseBasicParsing 'https://pixiv.mokeyjay.com/?limit=10'
Invoke-WebRequest -UseBasicParsing 'https://v1.hitokoto.cn/?encode=json&charset=utf-8&max_length=48'
```

浏览器矩阵使用缓存的 Playwright Node 包和系统 Edge headless，检查 widget 顺序、显示状态、overflow、console/page error，并通过 route fulfill/abort 覆盖一言成功、刷新、缓存与失败路径。

## 上传准备

当前状态：**P28 组件级 Ready；当前代码以新仓库 `zo-king/zoking_blog` 的 `main` 分支为发布基线。**

上传前必须完成：

1. 保留 `popular-posts.html`、`pixiv-ranking.html`、`hitokoto.html`、`hitokoto.ts`、`custom.scss`、`params.toml` 和 `assets/icons/refresh.svg` 的配套变更。
2. 生产环境注入 `HUGO_COMMENTS_API_BASE=https://api.zoking.tech`；缺失时 production build 应继续失败，不能绕过既有评论配置门禁。
3. 执行 `pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1 -SkipE2E`；涉及数据库写链路发布时使用隔离 `_test` 数据库执行完整 preflight/E2E。
4. 不直接上传开发态 `apps/site/public`。应由生产 Hugo 构建、容器镜像或现有 release 流水线生成并发布静态产物。
5. 发布后从公网复核 `/` 的三个 widget、390px 隐藏行为、CSP/代理对两个第三方域名的访问，以及第三方失败时正文仍可阅读。

第三方在线探测通过不是发布门禁的永久保证。Pixiv 或一言服务故障时不得回滚本站内容；先确认本站布局与降级路径正常，再判断是否需要临时关闭对应 widget。
