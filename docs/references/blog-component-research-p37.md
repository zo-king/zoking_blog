# 博客风格与组件调研 P37

日期：2026-07-16

## Grok 检索结果

本轮使用已配置的 Grok `grok-4.5` 做定向搜索。模型返回并可再次打开核验的候选包括：

- [Julia Evans](https://jvns.ca/)：高可读性的标题、日期和内容列表，少装饰，适合技术文章扫描。
- [Stefan Judis](https://www.stefanjudis.com/)：文章卡片、栏目导航和局部 hover/focus 层次。
- [Josh W. Comeau](https://www.joshwcomeau.com/)：教学型文章中的局部交互、代码演示和内容分层。
- [Gwern](https://gwern.net/)：高密度研究文章、内链、脚注和边注的组织方式。
- [Ahmad Shadeed](https://ishadeed.com/)：CSS、布局、容器和组件边界的教学式展示。
- [Kevin Powell](https://www.kevinpowell.co/)：原生 CSS 动效与布局教学，强调可解释的基础能力。
- [Una Kravets](https://una.im/)：现代 CSS 特性和渐进增强实验。
- [Modern CSS](https://moderncss.dev/)：原生 CSS 组件与可访问结构。
- [Piccalilli](https://piccalil.li/)：工程化 CSS、布局和可访问交互。

Grok 的部分长检索结果混入了生成式 HTML 片段，因此没有把未逐项复核的描述作为事实使用。本站只采纳站点 URL 和可在公开页面观察到的总体设计方向，不复制品牌、文案、图片或实现代码。

## 去重与筛选

当前博客已经有阅读进度、续读、目录、系列、文章导航、相关文章、快捷面板、页面过渡、打印、RSS/OPML 和友链发现，因此以下方向暂不新增：

- 第二套阅读侧轨、返回顶部、年份文章归档、随机文章、文章卡片大改版。
- 持续 Canvas、鼠标跟随、自动播放、热度榜、点赞和第三方数据墙。
- 依赖访客侧远程抓取的链接预览、友链最新文章和动态统计。

## P37 决策

采用 Theme Stack 已支持但本站正文尚未使用的 Markdown alert：`NOTE`、`TIP`、`IMPORTANT`、`WARNING`、`CAUTION`。它对 Go、Gin、GORM、PostgreSQL 等工程文章有真实语义，可以在不改变页面结构的情况下增加阅读层次。

实施原则：

- 内容必须是已有文章中的真实工程判断，不写装饰性文案。
- 输出保留原生 `blockquote`，无 JavaScript 仍可阅读，打印继续可见。
- 样式仅调整圆角、间距、轻阴影、标题和暗色主题，不引入依赖。
- 移动端不得产生横向溢出，动态效果不依赖 hover。
- 后续新增文章优先使用提示块表达边界、风险、建议和关键不变量。
