# 博客阅读生命周期调研 P34

日期：2026-07-15

## 样本

本轮以文章发布后的发现、更新、打印和离线阅读为重点，实测以下个人技术博客：

| 站点 | 已确认能力 | 本站判断 |
|---|---|---|
| [Simon Willison](https://simonwillison.net/2026/Jul/9/gpt-5-6/) | Atom 自动发现、可见订阅入口、无 JS 完整阅读 | RSS/Atom 发现值得保留，页面保持极简 |
| [Julia Evans](https://jvns.ca/blog/2026/05/15/moving-away-from-tailwind--and-learning-to-structure-my-css-/) | RSS、邮件订阅，打印时明显清理导航 | 采用打印清理原则，不引入邮件订阅 PII |
| [Jeremy Keith](https://adactio.com/journal/22647) | RSS/JSON Feed、Webmention、PWA 离线 | PWA 有价值但会影响缓存失效与发布回滚，暂缓 |
| [Lea Verou](https://lea.verou.me/blog/2026/polyfills/) | 报告页面问题、GitHub 编辑、Atom、无 JS 可用 | 本站已有评论反馈；生产内容不保证同步回 GitHub，暂不提供编辑链接 |
| [Josh W. Comeau](https://www.joshwcomeau.com/css/anchor-positioning/) | 发布日期、Last updated、RSS、外链提示 | 本站已有真实 `lastmod` 和可见更新时间，不重复实现 |
| [Dave Rupert](https://daverupert.com/2026/06/curated-public-domain-images/) | Atom、Service Worker，访问后可离线重载 | 离线缓存需要独立版本与回滚设计 |
| [Tom MacWright](https://macwright.com/2026/07/01/recently) | RSS/Atom 自动发现、极简静态文章 | 作为低脚本基线 |
| [Dan Abramov](https://overreacted.io/there-are-no-instances-in-atproto/) | 极简静态文章，无收藏、打印按钮或复制整文 | 不为“功能数量”增加常驻控件 |
| [Chris Coyier](https://chriscoyier.net/2026/07/13/choosing-movies/) | 打印隐藏交互控件、评论、RSS | 采用打印隔离；其 Copy 并非复制文章 Markdown，不作为依据 |
| [Max Böck](https://mxb.dev/blog/faster-horses/) | Edit this Post、RSS、Webmention、Manifest | 发布快照不是 GitHub 源文件，编辑入口不适合本站 |

## 仓库事实

- 文章 front matter 已包含 `date` 与 `lastmod`，发布器分别来自 `published_at` 和数据库 `updated_at`。
- 文章页 JSON-LD 已输出 `datePublished/dateModified`，修改日期不同的文章页会显示“最后更新于”。
- 首页和 taxonomy 页通过 Hugo AlternativeOutputFormats 自动发现 RSS；普通文章页此前没有 feed discovery。
- 顶部有可见 RSS 图标，RSS 全文输出并进入发布 E2E。
- 仓库此前没有 `@media print`。打印会带入顶栏、左右侧栏、评论、分享、进度条和屏幕背景。

## 本轮采用

1. 文章专用打印样式，只由文章页加载，不增加打印按钮。
2. 打印保留标题、作者、发布日期、条件更新时间、正文、图片、代码和许可。
3. 隐藏开屏、顶栏、侧栏、目录、阅读设置、阅读进度、系列、翻页、分享、评论、相关文章和站点页脚。
4. A4 与窄视口均取消负边距，代码使用 `pre-wrap`，表格和媒体限制在版心内。
5. 深色主题打印强制 light color-scheme、白底深字，并关闭所有动画与过渡。
6. 文章页增加站点主 RSS discovery；首页和 taxonomy 继续使用各自已有 feed，不生成重复标签。
7. 配置公开作者 `Zoking` 与 GitHub URL，BlogPosting 的 author/publisher 不再错误使用站点标题。

## 暂缓

- 本地收藏/稍后读：10 个样本均未提供站内本地收藏，当前仅 12 篇文章且已有 30 天断点续读。文章规模明显增长后再评估列表、导出和清空的完整闭环。
- 整篇 Markdown 复制：样本中没有真实采用；发布内容还涉及 shortcode、图片相对路径和 CMS 快照来源。
- PWA/Service Worker：必须先设计缓存版本、发布回滚、评论/API 网络策略和离线失效。
- GitHub Edit/纠错：评论已经承担反馈，生产快照也不保证同步回源码仓库。
- Newsletter/Webmention：分别涉及邮箱 PII、退订合规，以及 SSRF、去重和内容审核。
