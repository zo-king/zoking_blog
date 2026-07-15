# Blogroll 与内容纠错调研 P35

日期：2026-07-15

## 样本

| 站点 | 已确认能力 | 本站判断 |
|---|---|---|
| [Chris Burnell](https://chrisburnell.com/blogroll/) | Blogroll、独立 OPML、每站 RSS、Webring、Edit this page | OPML 与纠错值得采用；不加载统计或头像服务 |
| [Kev Quirk](https://kevquirk.com/blogroll/) | 字母排序 Blogroll、每站 Feed | 结构化 Feed URL 比浏览器运行时猜测可靠 |
| [Manuel Moreale](https://manuelmoreale.com/blogroll) | 大型纯链接 Blogroll | 无 JS 静态列表是可靠基线 |
| [Jamie Tanna](https://www.jvt.me/blogroll/) | 大型 Blogroll、IndieWeb Webring、订阅入口 | Webring 需要成员协议，本站已有随机拜访，不重复 |
| [Chris Aldrich](https://boffosocko.com/about/following/) | Following 页面和大量 Feed | 超长订阅列表不适合当前 9 个站点规模 |
| [Lea Verou](https://lea.verou.me/blog/2026/polyfills/) | 报告页面问题、GitHub 编辑、Atom | 采用报告问题，不提供直接编辑源文件 |
| [Max Böck](https://mxb.dev/blog/faster-horses/) | Edit this Post、RSS、Webmention | 后台数据库才是本站内容真源，GitHub Edit 不适用 |
| [Jeremy Keith](https://adactio.com/journal/22647) | RSS、JSON Feed、Webmention | Webmention 继续因 SSRF 和审核成本后置 |
| [Simon Willison](https://simonwillison.net/2026/Jul/9/gpt-5-6/) | Atom 自动发现、订阅入口 | 订阅能力保持静态、低脚本 |

## 采用率

- Blogroll/Following：5/9。
- 独立 OPML：1/9。
- Webring：2/9。
- GitHub 编辑或纠错：3/9。
- 友链最近文章快照：0/9。
- 主要内容无 JavaScript 可读：9/9。

## 仓库审计

友链原先位于 `content/page/links/index.md` 的自定义 `links:` front matter。后台 `Page` 模型和 `buildPageMarkdown` 不认识该字段，若在 Admin 重新发布 `/links/`，发布器会用白名单 front matter 覆盖文件并删除全部友链。

因此 P35 先把结构化友链迁移到 `apps/site/data/blogroll.json`：

- 页面正文、标题、菜单继续属于后台页面发布域。
- 站点名称、主页、Feed 和分类属于 Git 管理的结构化 blogroll 数据。
- 页面模板和 OPML 使用同一个数据源，不维护双份列表。

## 本轮采用

1. 同源 `/blogroll.opml`，由 Hugo 构建生成，不在浏览器中拼 XML。
2. 仅导出人工验证为 HTTPS 且返回 Feed 的 4 个站点：lvy-neko、Anthony Fu、Innei、Josh W. Comeau。
3. OPML 包含名称、主页、Feed、分类、公开作者和固定更新时间；不包含头像、简介、热度或抓取状态。
4. 友链页面提供原生下载链接，无 JavaScript 可用。
5. 文章底部增加 GitHub Issue 内容纠错链接，预填文章标题、当前发布 URL、页面更新时间和问题描述骨架。
6. 点击纠错前不加载 GitHub API、徽章、头像或脚本，不附带正文、评论、邮箱、Cookie、Token 或浏览器信息。
7. Web Share 成功提示改为“分享操作已完成”，剪贴板路径复用 P33 的兼容复制工具。

## 暂缓

- 友链最近文章：样本采用率为 0，且需结构化 RSS/Atom 解析、SSRF 防护、重定向复检、大小/超时限制、清洗和失败保留旧快照。
- Webring：需要多个站点共同维护成员顺序和失效治理；本站已有随机拜访。
- Webmention：仍涉及 source URL 抓取、SSRF、去重、恶意 HTML 和审核。
- 直接 GitHub Edit：后台数据库和发布快照才是内容真源，源码仓库不能保证同步。
