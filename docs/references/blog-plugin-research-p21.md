# P21 博客插件与特色能力调研

调研日期：2026-07-13。目标是为 Hugo Theme Stack + Go/Gin 全栈博客寻找有价值的扩展，不以“插件越多越好”为目标。

## 1. 真实博客观察

| 站点 | 特色 | 对当前项目的启发 |
|---|---|---|
| [Simon Willison](https://simonwillison.net/) | Entries、Links、Quotes、Notes、Guides 多内容流，提供订阅入口 | 可在内容量增长后增加轻量内容类型，不应现在扩张数据库模型 |
| [Derek Sivers](https://sive.rs/now) | `/now` 页面表达作者当前状态，另有 `/uses` | 最适合低成本增强作者辨识度，复用现有 Hugo Page 和后台页面管理 |
| [Julia Evans](https://jvns.ca/projects/) | Projects、TIL、Zines、Favorites 等内容发现入口 | 现有 GitHub 实时项目页可增加人工维护的精选项目页 |
| [Drew DeVault](https://drewdevault.com/) | 文章、开源项目、演讲、联系方式并列展示 | 博客可以自然成为作者主页，但项目和 Talks 应先使用静态页面 |
| [Maggie Appleton](https://maggieappleton.com/garden) | Essays、Notes、Patterns、Library、Now 等数字花园分层 | 内容类型和成熟度要有真实字段，不能仅靠发布日期猜测 |
| [Gwern](https://gwern.net/design) | sidenotes、反向链接、链接预览、折叠、transclusion | 研究型增强价值高，但远程抓取涉及 SSRF、缓存和清洗，必须后置 |
| [overreacted](https://overreacted.io/) | 极简索引、日期、描述、RSS | 内容标题和订阅本身比复杂动效更重要 |

## 2. 插件/能力矩阵

| 能力 | 当前状态 | 成熟度 | 价值 | 决策 |
|---|---|---:|---:|---|
| Mermaid | 已接入，按需加载 | 高 | 技术文章图表 | 保留，固定完整 CDN 版本或后续自托管 |
| KaTeX | 已接入，带 SRI | 高 | 数学公式 | 保留，不与 MathJax 并用 |
| PhotoSwipe | 已接入，图片页按需加载 | 高 | 图片放大与画廊 | 保留，后续减少 CDN 依赖 |
| RSS | 已接入，全文输出 | 高 | 订阅和联邦桥接基础 | 保留，可评估 Atom/JSON Feed |
| Pagefind | 未接入 | 高 | 大规模静态搜索、分块索引 | P1/P2，正文总量达数 MB 或移动端搜索变慢再接入 |
| Plausible | 未接入 | 高 | 隐私优先访问统计 | 有明确统计目标时 P1；不发送 PII |
| Umami | 未接入 | 高 | 自托管统计和 dashboard | P2；与 Plausible 二选一，不同时运行 |
| Giscus | Stack provider 存在但未启用 | 高 | GitHub 用户社区评论 | P3 可选；不能与当前 Go 评论系统同时展示 |
| Webmention | 未接入 | 中 | 开放网络引用和回复 | P3；需要独立表、SSRF 防护、抓取队列、审核 |
| ActivityPub | 未接入 | 高但集成复杂 | 联邦发布和互动 | P4 专题；采用独立 bridge，不混入核心 API |

## 3. 与当前系统的边界

当前已经具备阅读进度/续读、目录、代码复制、评论审核、系列文章、GitHub 项目、RSS、Mermaid、KaTeX、PhotoSwipe、发布审计和中文搜索。新插件必须先回答“当前能力缺什么”，不能重复引入第二套实现。

- **纯静态能力**：`Now`、`Uses`、Favorites、TIL、Talks、精选项目适合先作为 Hugo Page。
- **后台管理能力**：人工项目、Talks、精选文章、内容类型稳定后再进入 PostgreSQL 和 Admin。
- **公共异步能力**：评论继续使用 Go Public Comments API；Giscus 只作为技术读者导向的替代 provider。
- **高风险能力**：Webmention 的 source 抓取、链接预览、transclusion、统计和 ActivityPub 都需要单独的隐私、安全和运维设计。

## 4. 分阶段建议

### P1：低成本作者资产

1. 新增 `/now` 页面，复用独立页面编辑和发布。
2. 新增 `/uses` 页面，展示工具、设备、软件和外链。
3. 为 GitHub 实时仓库旁增加人工维护的精选项目说明。
4. 继续完善 RSS、代码复制、图片灯箱和 Mermaid/KaTeX 的资源安全。

### P2：内容增长后

1. 文章正文总量达到数 MB 或搜索下载成为瓶颈时接入 Pagefind。
2. 需要统计时只选 Plausible 或 Umami，并定义 cookie、IP、User-Agent、事件和数据保留策略。
3. TIL、Notes、Favorites 等先用 Hugo section/taxonomy，数量稳定后再建模型。
4. 浏览器本地收藏/稍后读可使用 localStorage，提供导出和清除，不创建匿名设备 ID。

### P3/P4：暂缓能力

Webmention 必须独立于普通评论，具备 URL 校验、SSRF 防护、幂等、抓取超时、HTML 清洗和审核；ActivityPub 必须通过独立 bridge 处理 Actor、WebFinger、签名、投递、重试、撤回和反滥用。两者都不应直接作为普通 Hugo 插件安装。

## 5. 官方资料

- [Pagefind](https://pagefind.app/docs/) / [GitHub](https://github.com/Pagefind/pagefind)
- [Giscus](https://giscus.app/) / [GitHub](https://github.com/giscus/giscus)
- [Mermaid](https://mermaid.js.org/intro/)
- [KaTeX](https://katex.org/docs/)
- [Plausible](https://plausible.io/docs) / [Umami](https://umami.is/docs)
- [Webmention 标准](https://www.w3.org/TR/webmention/) / [webmention.io](https://webmention.io/)
- [ActivityPub 标准](https://www.w3.org/TR/activitypub/) / [Hatsu bridge](https://github.com/importantimport/hatsu)
- [PhotoSwipe](https://photoswipe.com/)
