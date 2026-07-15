# 博客组件与渐进增强调研 P36

日期：2026-07-15

## 调研目标

继续寻找适合 Zoking Blog 的轻量组件与动效，并先排除本站已经具备的阅读进度、断点续读、活动目录、系列导航、快捷面板、随机文章、页面过渡、打印和 Feed 发现。候选必须支持中文长标题、移动端、无 JavaScript 与隐私友好边界。

## 东亚样本

| 站点 | 可借鉴点 | 边界 |
|---|---|---|
| [XAOXUU 归档](https://xaoxuu.com/blog/archives/) | 年份、月日与标题组成紧凑文章年表 | 文章页存在宽内容溢出，不能复制其媒体边界 |
| [木木木木木归档](https://immmmm.com/archives/) | 归档总数、标签计数和按月排列 | 全量页面很长，年份分组更适合移动扫描 |
| [DIYgod](https://diygod.cc/beancount) | 原生 `details` 目录，无 JS 可用 | 代码块会扩张移动宽度，候选组件必须限制根节点溢出 |
| [blog.jxck.io](https://blog.jxck.io/entries/2026-06-29/tc39-llm-wiki.html) | 明确展示创建、更新日期，正文静态优先 | 仅吸收信息表达，不增加统计脚本 |
| [Web Scratch](https://efcl.info/2026/06/23/hardening-npm-publishing/) | 评论点击后加载，初始正文保持静态 | 适合作为第三方内容保护壳参考，不立即改变现有组件 |
| [Sosuke Suzuki](https://sosukesuzuki.dev/posts/jsc-using-bytecode-generation/) | 首页按年份分组全部文章，脚注有往返链接 | 代码仍需独立处理移动横向滚动 |
| [azukiazusa](https://azukiazusa.dev/blog/nextjs-instant-navigations) | 原生折叠补充说明、Markdown 复制 | 第三方字体、分析与媒体较多，不复制其网络边界 |
| [r7kamura](https://r7kamura.com/articles) | 日期和标题组成纯静态文章索引 | 不分年时页面过长，证明必须建立年份导航 |

共同结论：可靠模式不是持续动画，而是服务端生成的静态 HTML 加少量增强。年份分组比无分组全量列表更易扫描，且不需要 API、Cookie、统计或第三方资源。

## 欧美样本

| 站点 | 可借鉴点 | 当前判断 |
|---|---|---|
| [Eric Meyer](https://meyerweb.com/eric/thoughts/2025/10/29/custom-asidenotes/) | 宽屏能力检测后把注释放到页边，窄屏回正文 | 适合作为后续标准 Markdown 脚注增强参考 |
| [Gwern](https://gwern.net/sidenote) | 完整边注、回链、链接预览和键盘入口 | 交互上限较高，整体实现与第三方请求过重 |
| [Jim Nielsen](https://blog.jim-nielsen.com/2026/notes-shuffle/) | 把随机逻辑集中到单一 Shuffle 页面 | 当前仅 12 篇文章，优化收益暂时有限 |
| [fasterthanlime](https://fasterthanli.me/series/making-our-own-executable-packer/part-1) | 明确系列章节带与服务器目录 | 本站 P18 系列导航已覆盖，不重复实现 |
| [Chris Burnell](https://chrisburnell.com/article/unusual-rotations-2026-03/) | Container Query 让目录在宽屏侧置、窄屏回流 | 本站 Theme Stack TOC 已具备等价布局 |
| [Adactio](https://adactio.com/journal/22647) | h-entry、Feed、Webmention 等 IndieWeb 语义 | 可继续改善机器语义，Webmention 服务仍需审核与反滥用 |
| [Rach Smith](https://rachsmith.com/) | 静态数字花园归档与可选装饰效果 | 静态内容可借鉴，光标轨迹等纯装饰排除 |
| [Simon Willison](https://simonwillison.net/2026/Jun/18/datasette-apps/) | 服务器 HTML、Atom 和最小 Webmention 发现 | 保持无 JS 阅读，避免嵌入内容造成移动溢出 |

## 候选比较

| 候选 | 新增价值 | 成本 | 决策 |
|---|---:|---:|---|
| `/post/` 文章年份归档 | 高 | 低 | 本轮采用；填补文章总目录缺口 |
| 标准脚注的宽屏边注增强 | 高 | 中 | 下一候选；先让真实技术文章采用脚注 |
| 第三方组件点击加载保护壳 | 中高 | 中 | 后续隐私切片；需要兼顾用户已要求的右栏可见性 |
| 独立随机页面 | 中 | 低 | 文章规模增长后替换每页内嵌 URL |
| Webmention | 中 | 高 | 暂缓；涉及访客 URL、去重、审核和数据保留 |

## P36 结果：未采用

曾在 `/post/` 实现 Hugo `RegularPages.GroupByDate` 文章年表，但用户在视觉审核后明确否决，相关模板、样式、测试与证据已删除，页面恢复 Theme Stack 默认列表。后续不得重新实现同一版式。

本轮保留的设计约束：

- `/archives/` 继续专用于后台可管理的获奖成果时间线，不混入文章。
- 新文章目录方案必须先提供视觉草图或独立实验页面，不直接替换 `/post/`。
- 不采集浏览量、不称“热门”、不发起不必要的第三方请求。
- 无 JavaScript 时仍需保留完整文章入口。

## 明确排除

持续 Canvas、鼠标跟随、粒子、自动播放音频、外站头像墙、虚假阅读量、评分、点赞排行、无限长无分组列表，以及依赖 JavaScript 才能显示正文的页面结构。
