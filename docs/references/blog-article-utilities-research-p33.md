# 博客文章工具调研 P33

日期：2026-07-15

## 样本

本轮以 1280px 桌面和 390px 移动视口检查了以下个人技术博客：

| 站点 | 可借鉴能力 | 不直接照搬的部分 |
|---|---|---|
| [Josh W. Comeau](https://www.joshwcomeau.com/react/server-components/) | 标题永久链接、代码复制后的勾选反馈、较大的复制按钮 | 标题链接移动端命中区不稳定，成功反馈缺少明确 live region |
| [Lea Verou](https://lea.verou.me/blog/2026/polyfills/) | 标题永久链接、原生折叠脚注、无 JavaScript 可用 | 本站目前没有脚注内容，不先建设浮层或折叠脚注系统 |
| [Piccalilli](https://piccalil.li/blog/navigating-the-age-old-problem-of-checkmarks-in-ui-with-progressive-enhancement/) | 完整文字代码复制按钮、可聚焦并横向滚动的代码区 | 不增加第二套复制入口 |
| [CSS Wizardry](https://csswizardry.com/2026/05/better-browser-caching-with-no-vary-search/) | 原生 details/summary 渐进披露 | 部分 summary 触控高度偏小，不能原样复用 |
| [Gwern](https://gwern.net/about) | 明确的“复制章节链接”语义 | 章节链接 `tabindex=-1`，键盘不可达，不采用 |
| [Ahmad Shadeed](https://ishadeed.com/article/css-container-query-guide/) | 针对教程定制的折叠段落和交互演示 | 全局引入成本和视觉重量过高 |
| [Maggie Appleton](https://maggieappleton.com/garden-history) | 长列表渐进展开 | 实测移动端存在横向溢出，只借鉴披露原则 |
| [Anthony Fu](https://antfu.me/posts/categorize-deps) | 标题 `#` 永久链接、代码语言元数据 | 标题链接对读屏和键盘隐藏，不采用其可访问性处理 |
| [Max Böck](https://mxb.dev/blog/live-cms-previews-with-sanity-and-eleventy/) | 代码保留 JS、JSON、Bash 等语言类型 | 文件名写进代码首行会污染复制内容 |
| [Simon Willison](https://simonwillison.net/2025/Mar/11/using-llms-for-code/) | 极简静态文章基线 | 不因“有趣”而增加非必要运行时组件 |

## 本轮采用

1. 章节链接继续使用原生 `href="#id"`，无 JavaScript 时仍可定位。
2. 普通点击同时复制完整章节 URL，保留原生 hash、历史记录和目录滚动行为。
3. `#` 在键盘聚焦时可见，触屏端保持可见并提供 44x44px 命中区。
4. 成功时短暂显示勾选，通过 `aria-live="polite"` 宣告；失败不阻止章节定位。
5. 代码块从 Hugo `data-lang` 生成 Go、C++、SQL、Shell 等纯文本语言标签。
6. 语言标签与现有复制按钮合并为一个紧凑工具栏，复制正文继续排除行号。
7. 锚点滚动遵循 `prefers-reduced-motion`，减少动态时使用即时滚动。

## 暂缓

- 原生折叠附录：技术上成熟，但当前文章没有运行日志、完整 SQL 或扩展推导等真实附录；有内容场景时再制定写作规范。
- 脚注 Popover、远程链接预览、代码运行器、多文件标签页和速度控制。
- 第二套复制按钮、返回顶部、阅读进度或活动目录。
- 点赞、热度、评分等需要后端真实性、限流和去重的能力。

后续候选应先由内容场景驱动。若新增长篇教程，优先研究“可选文件名但不污染复制正文”和原生折叠附录，不引入远程 Playground。
