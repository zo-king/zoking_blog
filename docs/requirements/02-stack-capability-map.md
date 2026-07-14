# Stack 能力映射

本文件把常见博客需求映射到 Hugo Theme Stack 的实际能力，帮助后续实施时判断“通过配置即可完成、通过内容约定完成、需要站点层覆盖、还是需要新代码”。

## 总体结论

Stack 对个人博客常见功能支持完整。第一阶段应优先通过：

1. `config/_default/*` 配置；
2. 内容 frontmatter 约定；
3. 必备页面创建；
4. `assets/scss/custom.scss` 和 `assets/ts/custom.ts`；
5. 少量站点层 partial 覆盖；

来完成需求。除非确有必要，不要直接修改主题核心模板和主题核心脚本。

## 能力矩阵

| 功能 | Stack 支持情况 | 主要位置 | 实现难度 | 实施方式 |
|---|---|---|---|---|
| 首页文章列表 | 内置 | `layouts/home.html`、`params.mainSections` | 低 | 配置 |
| 分页 | 内置 | `layouts/_partials/pagination.html`、`[pagination]` | 低 | 配置 |
| 侧边栏头像/简介 | 内置 | `params.sidebar`、`sidebar/left.html` | 低 | 配置 + 资源 |
| 社交菜单 | 内置 | `menu.toml`、`assets/icons` | 低 | 配置；缺图标时加站点图标 |
| 主导航 | 内置 | Hugo menu、`sidebar/left.html` | 低 | 配置或页面 frontmatter |
| 深色/浅色/自动模式 | 内置 | `params.colorScheme`、`assets/ts/colorScheme.ts` | 低 | 配置 |
| 右侧 widgets | 内置 | `params.widgets`、`widget/*` | 低 | 配置 |
| 文章 TOC | 内置 | `params.article.toc`、`widgets.page` | 低 | 配置 + frontmatter |
| 分类/标签 | 内置 | frontmatter、taxonomy 页面 | 低 | 内容约定 |
| 归档页 | 内置模板，需建页面 | `layouts/archives.html` | 低 | 创建页面 |
| 搜索页 | 内置模板和脚本，需建页面 | `layouts/page/search.*`、`assets/ts/search.tsx` | 中低 | 创建页面 + JSON output |
| 404 搜索兜底 | 内置 | `layouts/404.html` | 低 | 搜索页存在时更完整 |
| 封面图/缩略图 | 内置 | `image` frontmatter、image helper | 低 | 内容约定 |
| 响应式图片 | 内置 | `params.imageProcessing`、responsive image helper | 中低 | 配置 + Hugo Extended |
| 图片灯箱 | 内置 | render hook、PhotoSwipe、`gallery.ts` | 低 | 内容约定触发 |
| Mermaid | 内置 | render hook、`params.article.mermaid`、`mermaid.ts` | 中低 | 代码块触发 |
| KaTeX 数学公式 | 内置但需启用 | `math.html`、`data/external.toml` | 中低 | 配置或 frontmatter |
| 相关文章 | 内置 | `related-content.html`、`related.toml` | 低 | 配置 |
| 文章许可证 | 内置 | `params.article.license` | 低 | 配置 |
| 评论系统 | 多 provider 内置 | `params.comments`、`comments/provider/*` | 中 | 第三方配置 |
| 单篇关闭评论 | 内置 | `comments: false` | 低 | frontmatter |
| Cookie consent | 内置 | `params.cookies`、`cookies.ts` | 中 | 配置 + 验证 |
| Google Analytics | Hugo service + 主题门控 | head/cookies partial | 中低 | 配置 |
| SEO / Open Graph | 内置 | `head/*`、opengraph partial | 低 | 配置 + frontmatter |
| RSS | 内置 | `layouts/rss.xml`、`rssFullContent` | 低 | 配置 |
| Sitemap | Hugo 原生 | Hugo sitemap templates | 低 | 默认可用，必要时配置 |
| 多语言 | 内置适配 | `i18n/*`、`languages.toml` | 中 | 内容和配置策略 |
| 短代码 | 内置 | `layouts/_shortcodes/*` | 低 | 内容使用 |
| 外链处理 | 内置 render hook | `render-link.html` | 低 | 自动 |
| Alert blockquote | 内置 render hook | `render-blockquote.html`、`alertIcon` | 低 | Markdown 约定 |
| 标题锚点 | 内置开关 | `headingAnchor`、`render-heading.html` | 低 | 配置 |
| 友情链接 | 内容约定支持 | `links.html` | 低 | 页面 frontmatter |
| 自定义样式 | 预留 | `assets/scss/custom.scss` | 中 | 站点层覆盖 |
| 自定义脚本 | 预留 | `assets/ts/custom.ts` | 中 | 站点层覆盖 |
| 自定义头尾 | 预留 | `head/custom.html`、`footer/custom.html` | 中 | 站点层 partial |
| 新 widget | 可扩展 | `layouts/_partials/widget/*` | 中 | 新 partial |
| 新评论 provider | 可扩展 | `comments/provider/*` | 中高 | 新模板和脚本 |

## 第一阶段低风险配置清单

第一阶段可以直接依靠 Stack 完成：

- 首页文章列表。
- 分页。
- 侧边栏头像、简介、社交链接。
- 主菜单。
- 深色模式。
- 首页 widgets。
- 文章 TOC。
- 分类、标签。
- 归档页。
- 搜索页。
- 封面图、缩略图。
- RSS。
- Open Graph。
- 文章许可证。

## 第二阶段增强清单

第二阶段再考虑：

- 评论系统。
- 统计分析。
- Cookie consent。
- Mermaid 深度配置。
- KaTeX SRI 校验。
- 友情链接页面。
- 相关文章策略。
- 自定义 SCSS。
- 自定义 TS。

## 风险提示

- `demo` 中仍有 v3 import 和本地 replace，正式站点应使用 `github.com/CaiJimmy/hugo-theme-stack/v4`。
- 搜索页必须创建并配置 JSON output。
- 归档 widget 需要 `layout: archives` 页面。
- Mermaid 和 PhotoSwipe 使用 CDN 资源，部署环境和隐私策略要考虑第三方请求。
- `data/external.toml` 中 KaTeX SRI 值需要正式实施时核对。
- Cookie consent 启用后必须确认 analytics 和 comments 是否真的被门控。
