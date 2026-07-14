# C 端 Hugo Theme Stack 集成规范

本文定义如何保留 Hugo Theme Stack 的页面效果，同时把后台发布内容接入 C 端。

## 1. 目标

- 保留 Stack 的三栏布局、卡片式列表、文章页、归档、分类、标签、搜索、明暗色模式和移动端体验。
- 通过 Hugo content/config 快照接入后台内容。
- 不把 C 端改造成依赖 API 的动态渲染站点。

## 2. Stack 能力边界

Stack/Hugo 可承担：

- 首页文章列表。
- 文章详情页。
- 分类、标签、归档。
- RSS、Sitemap。
- Open Graph。
- 搜索索引。
- 评论 provider 插槽。
- widgets。
- 图片处理。

全栈系统另做：

- 后台内容编辑。
- 权限和审计。
- 媒体库。
- 评论审核与反垃圾。
- 发布任务和回滚。
- 数据统计和看板。

## 3. 目录边界

推荐：

```text
apps/site/
  config/_default/
  content/
  assets/
  static/
  layouts/
```

规则：

- 默认不直接改 vendored theme 核心。
- 站点层通过配置、custom scss/ts、局部 partial 覆盖扩展。
- 如必须改 Stack 核心模板，先写 ADR。

## 4. 主题保护清单

默认禁止直接修改：

- `layouts/baseof.html`
- `layouts/home.html`
- `layouts/single.html`
- `assets/scss/style.scss`
- `assets/scss/variables.scss`
- `assets/ts/main.ts`

优先使用：

- `config/_default/*`
- `assets/scss/custom.scss`
- `assets/ts/custom.ts`
- `layouts/_partials/head/custom.html`
- `layouts/_partials/footer/custom.html`
- 站点层同路径覆盖。

## 5. C 端动态增强

可异步增强：

- 评论。
- 浏览量。
- 点赞。
- 分享计数。
- 订阅入口。

规则：

- API 失败时文章仍可读。
- 动态组件必须有空状态和失败状态。
- 不阻塞首屏渲染。
- 不把未审核评论直接展示。

## 6. 搜索

MVP：

- 使用 Hugo 生成本地搜索索引。
- 索引内容包括标题、摘要、正文摘要、分类、标签。

增强：

- PostgreSQL 全文搜索用于后台。
- 未来可切 Meilisearch/Algolia，但不作为 MVP 前置。

## 7. SEO 验收

每篇文章必须输出：

- title。
- description。
- canonical。
- Open Graph 标题和图片。
- 发布时间和更新时间。
- 分类标签。

站点必须输出：

- RSS。
- Sitemap。
- robots 配置。
- 404 页面。

## 8. 视觉验收

桌面：

- 左侧站点栏存在。
- 中间文章列表或正文布局稳定。
- 右侧 widgets 正常。
- 文章卡片密度接近 Stack 原主题。

移动端：

- 导航可用。
- 正文不横向溢出。
- 代码块可滚动。
- 图片不撑破容器。

## 9. Hugo 构建验收

命令：

```powershell
hugo --minify
```

通过标准：

- 命令退出码为 0。
- `public/index.html` 生成。
- 至少一个文章页生成。
- 分类、标签、归档、RSS、Sitemap 可访问。
