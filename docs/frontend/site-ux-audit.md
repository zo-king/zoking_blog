# C 端 Theme Stack 体验审计

## 1. 范围与结论

`SITE-UX-P9-001` 对首页、搜索、归档、分类、标签、友链、关于、文章、404、RSS、robots 和 sitemap 进行了桌面/移动审计。实现继续复用 Hugo Theme Stack，通过 `apps/site` 本地配置、partial、render hook 和资源覆盖修复，不修改上游主题核心。

当前本地 C 端已达到可持续阅读和内容发现基线。`PUBLISH-URL-P10-001` 已完成：正式站点为 `https://zoking.tech/`，公开 API 为 `https://api.zoking.tech`，生产 release 与历史 release 提升均执行公开 URL 安全校验。

## 2. 已完成修复

### 阅读与互动

- 文章详情标题改为唯一语义化 `h1`，列表标题保持 `h2`；侧栏站点名仅首页使用 `h1`。
- 详情封面使用中文 alt，首屏文章图 eager + high priority，其余列表图保持 lazy。
- 去除两篇展示文章正文中与封面重复的首图；展示媒体改用站点同源 `/img/showcase/*`。
- 增加上一篇/下一篇、系统分享/复制链接和相关文章无匹配回退。
- 开启章节锚点，并为锚点补描述性 `aria-label`。
- 评论标题、字段、加载、空态、异常、提交和审核反馈全部中文化；补充邮箱不公开和审核说明。

### 导航与发现

- 分类、标签加入主导航，RSS 加入图标入口；删除指向 GitHub 平台首页的伪个人主页链接。
- 默认分页由演示用 3 条调整为 10 条，避免 taxonomy 总览产生低密度分页。
- 搜索索引同时包含 taxonomy slug 和中文显示名；“效率”“工程实践”等中文词可命中文章。
- 搜索和归档补独立页面标题与描述；404 浏览器标题改为“404 页面不存在”。
- RSS 标题、描述和封面 alt 中文化；robots 输出 sitemap。

### SEO

- 首页输出 `WebSite` + `SearchAction` JSON-LD。
- 文章输出 `BlogPosting` JSON-LD，包含发布时间、更新时间、描述、图片和规范 URL。
- taxonomy/list 输出 `CollectionPage` JSON-LD；所有 JSON-LD 已通过 JSON 解析。
- 补 `og:locale=zh_CN`；搜索页输出 `noindex, nofollow` 并从 sitemap 排除；404 保持 noindex。
- 保留主题已有 canonical、description、Open Graph、Twitter Card、RSS alternate 和 sitemap。

### 可访问性与性能

- 增加“跳到正文”链接和可聚焦 `main`。
- 暗色模式改为原生 button，支持 Tab、Enter/Space 和 `aria-pressed`。
- 移动菜单同步 `aria-controls/aria-expanded`，展开后焦点进入首个导航项。
- 搜索页、右栏搜索和 404 搜索的 label 与 input 正确关联。
- 代码复制按钮在键盘焦点和无 hover 设备上保持可发现；补焦点样式。
- 暗色辅助文本和行内代码对比度提高；移除 Google Fonts 阻塞请求，改用系统中文字体栈。
- 侧栏头像从 649,926 B 降为 13,986 B，favicon 为 12,670 B。

## 3. 验收结果

- 1280x720 与 390x844：8 个核心路由均返回 200，无横向溢出。
- 核心内容页拥有一个 `h1`；图片缺失 alt 0、空名称链接 0、无名称表单控件 0。
- 暗色模式键盘切换 `aria-pressed=false -> true`；移动菜单 `aria-expanded=false -> true`，焦点进入“首页”。
- 中文 taxonomy 搜索返回正确结果；文章显示上一篇/下一篇、分享、2 条相关文章和中文评论区。
- 404 返回 404 且标题中文；搜索不在 sitemap；robots 包含 sitemap。
- Hugo production build、Go tests、Admin build 和 `preflight -SkipE2E` 全部通过。

证据：

- `docs/process/evidence/site-p9-home-desktop-1280x720.png`
- `docs/process/evidence/site-p9-article-mobile-390x844.png`

## 4. 上线 URL 基线

本地开发配置允许 `http://localhost:1313` 和 `http://localhost:18080`。正式发布不得沿用这些地址，否则 canonical、Open Graph、JSON-LD、RSS、robots、sitemap 和评论接口会指向访问者本机。

`PUBLISH-URL-P10-001` 已实现：

1. `SITE_BASE_URL=https://zoking.tech/`，`PUBLIC_API_BASE_URL=https://api.zoking.tech`。
2. 正式发布校验数据库设置、部署声明和媒体公开 URL，拒绝 localhost、127.0.0.1、`::1`、userinfo、query/fragment 与非 HTTPS 绝对 URL。
3. 预览构建不调用生产策略，继续允许 loopback/临时地址。
4. Hugo 构建后扫描文本产物；回滚或手动提升历史 release 前执行相同检查。
5. 生产实构建已确认 canonical、OG、JSON-LD、RSS、robots、sitemap 使用 `zoking.tech`，评论使用 `api.zoking.tech`，回环地址扫描为空。

## 5. 后续增强

- 内容超过 10 条重新出现分页后，补充分页 aria 中文与 page 2+ 自引用 canonical 回归。
- 为搜索 JSON 请求增加 HTTP/JSON 失败中文空态。
- 移动折叠目录的 scrollspy 可作为长文专项增强；当前短文目录不构成发布阻断。
- 获得品牌视觉后可生成 1200x630 默认社交分享图。
