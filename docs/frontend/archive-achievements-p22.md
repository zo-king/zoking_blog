# P22 成果时间线

## 页面职责

`/archives/` 现在只展示正式发布的成果，不再读取文章集合。页面标题为“成果时间线”，数据入口是 `.Site.Data.achievements.items`；每项使用 `kind`、`title`、`organization`、`summary`、`occurred_at`、`ended_at`、`external_url`、`credential_id`、`image_url`、`image_alt` 和 `sort_order`。

年份轨道固定从当前年份倒推到 2024。即使 `achievements` data 文件缺失、没有 `items` 或 items 为空，也会渲染完整年份结构，并显示“暂无已发布成果”。模板保留发布 snapshot 的稳定顺序，并按 `occurred_at` 年份归组；发布器负责“日期倒序、同日 `sort_order` 升序”的规范化排序。

## 可访问性与降级

年份导航是普通 `nav` 内的锚点链接，目标 section 有稳定的 `id` 和 `aria-labelledby`。`archiveTimeline.ts` 只使用小型 `IntersectionObserver` 为当前年份链接切换 `active` class，不负责内容渲染；脚本失败时，原生锚点和完整 HTML 仍可阅读。

默认锚点滚动由 CSS 提供，`prefers-reduced-motion: reduce` 时明确恢复 `scroll-behavior: auto`。移动端 390px 与 320px 使用单列年份轨道，导航可横向查看但页面本身不产生横向溢出。

头像位于 `apps/site/assets/img/avatar-github.png`，继续由主题的 Hugo image helper 生成资源 URL；不再依赖 `static/img/avatar-github.png`。
