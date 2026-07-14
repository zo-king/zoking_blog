# 常见博客功能调研

本文件基于官方文档、常见个人/技术博客实践和子 agent 调研结论整理。目的不是追求功能越多越好，而是明确“正常博客通常需要什么、第一版应该做什么、什么可以后置”。

## 结论摘要

个人/技术博客第一版必须优先保证：

- 可读：文章页、移动端、排版、图片、代码块。
- 可找：搜索、归档、分类、标签。
- 可订阅：RSS。
- 可理解：SEO、描述、Open Graph。
- 可维护：Markdown、front matter、page bundle、构建和部署流程。
- 可接力：工作日志、需求文档、上下文接力规则。

评论、统计、Newsletter、CMS、多语言、PWA 等是后续增强，不应阻塞第一版上线。

## Must：第一版必须具备

| 功能 | 目的 | 用户价值 | 实现注意点 |
|---|---|---|---|
| 首页文章列表 | 展示最近文章和站点定位 | 读者快速判断博客主题和更新状态 | Stack 默认支持首页文章列表；配置 `mainSections = ["post"]` |
| 文章详情页 | 承载正文阅读 | 提供专注阅读体验 | 标题、日期、更新时间、阅读时间、目录、代码块、图片 alt 要完整 |
| 关于页 | 建立作者身份和信任 | 读者知道作者是谁、站点写什么 | `content/page/about/index.md`，通常关闭评论 |
| 归档页 | 按时间发现旧文章 | 老文章不会沉底丢失 | Stack 有 `layout: archives` 模板，需要创建页面 |
| 分类与标签 | 长期组织内容 | 读者可按主题浏览 | 分类少而稳，标签细而受控；避免近义重复 |
| 站内搜索 | 快速找内容 | 技术博客读者可按关键词回溯 | Stack 内置搜索；搜索页必须输出 JSON |
| RSS | 支持订阅 | 不依赖平台算法和社交平台 | Hugo 原生支持 RSS；确认 RSS 输出策略 |
| 基础 SEO | 让搜索引擎理解内容 | 搜索流量和分享预览更稳定 | `title`、`description`、canonical、sitemap、语义结构、图片 alt |
| 移动端体验 | 覆盖主要阅读场景 | 手机阅读不崩、不累 | 检查菜单、目录、代码块横向滚动、图片宽度 |
| 无障碍基础 | 让更多读者可用 | 键盘、读屏、弱视用户能阅读 | 标题层级、对比度、焦点态、链接语义 |
| 性能基础 | 快速打开和稳定阅读 | 降低跳出率 | 控制图片、减少第三方脚本、关注 LCP/INP/CLS |
| Frontmatter 规范 | 统一内容元数据 | 搜索、归档、RSS、SEO 都有数据 | 必填 `title/date/slug/description/categories/tags/draft` |
| Page bundle | 文章和资源绑定 | 图片管理、迁移、删除更可靠 | 每篇文章一个目录，正文和图片放一起 |
| 本地预览与生产构建 | 可持续发布 | 作者能稳定写、看、发 | `hugo server -D --gc --disableFastRender` 和 `hugo --gc --minify` |
| 工作日志 | 支持多 agent 和跨窗口接力 | 新窗口不丢上下文 | 中心窗口维护 `docs/process/worklog.md` |

## Should：第一版可做或第二阶段优先做

| 功能 | 目的 | 用户价值 | 实现注意点 |
|---|---|---|---|
| 评论系统 | 建立反馈回路 | 读者可提问、纠错、补充 | Stack 支持多 provider；需考虑审核、通知、隐私和性能 |
| 统计分析 | 理解内容表现 | 作者知道哪些文章有用 | 优先轻量和隐私友好；涉及 cookie 时必须进入隐私策略 |
| Open Graph / Twitter Card | 优化社交分享预览 | 分享链接更可信 | Stack 已有 OG 支持；每篇重要文章应有描述和封面 |
| 社交分享 | 降低传播成本 | 读者方便复制或分享链接 | 技术博客第一版可只做“复制链接”，少接第三方 SDK |
| 文章增强 | 提升技术内容表达 | 长文、公式、图表更易读 | TOC、heading anchor、代码高亮、KaTeX、Mermaid、图片灯箱按需启用 |
| 版权许可 | 明确转载规则 | 减少误用和沟通成本 | Stack 支持文章 license |
| 404 与别名 | 减少断链影响 | 旧链接仍可访问 | Hugo front matter 支持 `aliases` |
| CI 检查 | 降低发布事故 | 避免构建失败或草稿误发 | 构建、链接检查、图片体积检查逐步加入 |
| 隐私页 | 说明数据收集 | 读者知道是否有统计、评论和 cookie | 如果接入第三方脚本，隐私页变成必须 |

## Could：后续增强

| 功能 | 目的 | 用户价值 | 实现注意点 |
|---|---|---|---|
| CMS 后台 | 非技术化写作 | 不想碰 Git 也能发文 | 技术博客 MVP 不需要；可后续考虑 Decap CMS 等 |
| Newsletter | 主动触达读者 | 新文章可推送邮箱 | 需要退订、隐私和邮件服务成本 |
| Webmention / 反应按钮 | 轻互动 | 比评论更低门槛 | 需要额外服务或自建端点 |
| 多语言 | 扩展受众 | 中英双语内容分发 | 会增加 URL、SEO、翻译维护复杂度 |
| 相关文章 / 系列 | 增强内容网络 | 读者连续阅读 | 可通过 `related.toml`、tags、series taxonomy 实现 |
| PWA / 离线阅读 | 强化移动体验 | 弱网仍可访问 | 缓存策略复杂，后置 |
| 运行监控 | 发现线上问题 | 域名、证书、构建失败可提醒 | Uptime、Actions 通知、Search Console 后续加入 |

## 第一版推荐范围

第一版建议完成：

- 首页文章列表。
- 文章页。
- 关于页。
- 归档页。
- 搜索页。
- 分类和标签页。
- RSS。
- Sitemap。
- 基础 SEO / Open Graph。
- 移动端和暗色模式。
- 图片规范。
- Frontmatter 规范。
- 本地预览和生产构建。
- 工作日志和上下文接力机制。

第一版建议暂缓：

- 评论系统，除非用户明确选择 provider。
- 统计分析，除非用户明确接受隐私和 cookie 成本。
- 多语言，除非用户明确有中英双语计划。
- CMS、Newsletter、PWA。

## 参考来源

- [Stack 中文 Guide](https://stack.cai.im/zh/guide/)
- [Stack Widgets 文档](https://stack.cai.im/zh/config/widgets/)
- [Stack 评论系统文档](https://stack.cai.im/zh/config/comments/)
- [Stack Cookies 文档](https://stack.cai.im/zh/config/cookies/)
- [Hugo Front matter](https://gohugo.io/content-management/front-matter/)
- [Hugo Page bundles](https://gohugo.io/content-management/page-bundles/)
- [Hugo Taxonomies](https://gohugo.io/content-management/taxonomies/)
- [Hugo RSS templates](https://gohugo.io/templates/rss/)
- [Hugo Sitemap templates](https://gohugo.io/templates/sitemap/)
- [Google SEO Starter Guide](https://developers.google.com/search/docs/fundamentals/seo-starter-guide)
- [web.dev Web Vitals](https://web.dev/articles/vitals)
- [W3C WAI Accessibility Principles](https://www.w3.org/WAI/fundamentals/accessibility-principles/)
