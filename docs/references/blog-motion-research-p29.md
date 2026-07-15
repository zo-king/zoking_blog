# 博客动效与内容发现调研 P29

## 样本

- `https://lvyovo-wiki.tech/bloggers`
- `https://antfu.me/`
- `https://innei.in/`
- `https://www.joshwcomeau.com/`
- `https://rauno.me/`
- `https://www.cassie.codes/`

## 观察

`lvyovo` 使用 Motion 让卡片从 `opacity: 0 / scale: 0.6` 进入视口，悬停与按下分别使用 `1.05 / 0.95` 缩放；导航选中背景使用 spring，弹窗使用淡入、缩放和位移，页面底层运行模糊 Canvas。搜索、分类和博客目录本身有明确的信息价值，但持续 Canvas、整卡缩放、外站头像预加载和五星评分不适合本站的长期阅读定位。

Anthony Fu 的优势是点阵背景、行内项目徽标和克制导航；Innei 使用编号近期文章、短句、统计和探索入口建立个人叙事；Josh Comeau 把动画放进文章中的可操作示例；Rauno 强调复制、悬停和状态反馈等微交互；Cassie Evans 更适合作为 SVG、GSAP、Canvas 和 WebGL 专题实验参考。

## 采用

- 友链搜索、分类和随机探索。
- 全站键盘快捷面板。
- 小位移、短时长、只执行一次的内容入场。
- 后续优先把复杂动画用于数据结构、Gin 生命周期、GORM 事务等文章内演示。

## 不采用

- 全站持续 Canvas、WebGL 或鼠标拖尾。
- 自定义鼠标和自动播放音频。
- 整张内容卡片明显缩放。
- 友链星级排名和不透明推荐算法。
- 为装饰而预加载大量第三方资源。

所有站点仅作为交互模式和信息架构研究样本，不复制品牌、文案、图像或实现代码。
