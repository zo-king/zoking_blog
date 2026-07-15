# 博客动效与组件调研 P31

日期：2026-07-15

## 样本

本轮实际检查了以下个人技术博客与文章体验：

- [XAOXUU](https://xaoxuu.com/)
- [Sosuke Suzuki](https://sosukesuzuki.dev/)
- [Anthony Fu](https://antfu.me/posts)
- [keroway](https://keroway.com/)
- [Innei](https://innei.in/)
- [Josh W. Comeau](https://www.joshwcomeau.com/)
- [Emil Kowalski](https://emilkowal.ski/)
- [Rauno Freiberg](https://rauno.me/)
- [Maggie Appleton](https://maggieappleton.com/)
- [Gwern](https://gwern.net/)
- [Ahmad Shadeed](https://ishadeed.com/)
- [Lea Verou](https://lea.verou.me/)
- [Paco Coursey](https://paco.me/)
- [Shu Ding](https://shud.in/)

## 观察

- `shud.in`、Maggie Appleton 与 Paco Coursey 已使用 View Transition 或等价的页面连续性处理；适合本站的是短促淡入淡出，不是共享大图飞行或整页模糊。
- XAOXUU、Innei、Ahmad Shadeed 的活动目录、阅读进度与返回顶部很成熟，但本站已经具备顶部阅读进度、断点续读、Theme Stack TOC scrollspy、上一篇/下一篇，因此不再叠加第二套阅读侧轨。
- Josh Comeau、Lea Verou 和 Gwern 的脚注、链接预览与侧注适合研究型长文；当前本站文章没有脚注，站内互链也较少，现阶段实施会缺少真实内容场景。
- keroway 的字号、行距、链接下划线与减少动态设置有长期价值，可作为后续独立任务；不应连同其较重字体和 SVG 资源一起引入。
- Derek Sivers 与 Maggie Appleton 的 `/now` 模式不是履历或任务归档，而是可持续替换的当前状态页，适合补足本站“关于”和“成果时间线”之间的作者近况。

## 本轮采用

1. 原生 Cross-document View Transitions：同源页面使用 `140ms` 淡出、`180ms` 淡入与 `4px` 小位移；旧浏览器自然降级，reduced-motion 下关闭。
2. `/now/` 近况页：记录正在开发、学习和关注的事项，进入主菜单并自动进入 `Ctrl/Cmd+K` 快捷面板。

## 暂不采用

- 返回顶部圆环、第二套阅读进度或额外文章快捷键。
- 持续 Canvas、自定义鼠标、音乐、中控台和多层弹窗。
- 没有真实内容使用场景的脚注浮层。
- 需要 SSRF、缓存和清洗设计的远程链接预览。
- 伪造阅读量、热度、评分或点赞。

下一候选优先级：轻量阅读设置，其次是文章真实采用脚注后的渐进 Popover 增强。
