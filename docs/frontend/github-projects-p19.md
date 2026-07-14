# P19 内容发现精简与 GitHub 项目页

日期：2026-07-13

## 1. 信息架构决策

分类和标签的数据语义并不重复：分类表达稳定主题，标签表达跨主题关键词。重复发生在导航层，而不是数据模型层。

本轮采用以下结构：

- 一级导航保留“归档”，移除“分类”“标签”。
- `/categories/`、`/tags/` 及文章元数据链接继续保留，SEO 和深链不变。
- `/archives/` 作为统一内容探索入口，依次提供分类、标签和按年份文章。
- 首页右栏在 P19 只保留搜索和标签云；P20 进一步移除显式搜索，仅保留标签云和隐藏搜索路由。
- 新增一级“项目”入口 `/projects/`，不与友链混合。

## 2. GitHub 数据契约

项目页由 front matter 配置：

```yaml
github:
  username: zo-king
  maxRepos: 6
```

浏览器仅在项目页请求 GitHub Public REST API。仓库按以下规则处理：

1. 排除 fork 和 archived。
2. 按 `pushed_at` 降序，缺失时使用 `updated_at`，最后按名称稳定排序。
3. 最多显示 6 个仓库。
4. 只保留名称、描述、语言、Star、Fork、推送时间、GitHub URL 和过滤字段。
5. 所有远端文本通过 `textContent` 写入，仓库 URL 仅接受 `https://github.com`。

## 3. 可靠性与隐私

- Hugo 构建不访问 GitHub，外部 API 故障不会阻断发布。
- 请求超时 8 秒，不自动循环重试；失败后只能由读者显式重试。
- `credentials: omit`、`referrerPolicy: no-referrer`，不使用 PAT、Cookie 或 Authorization。
- 使用 5 分钟 `sessionStorage` 白名单缓存，不创建长期设备标识。
- 空数据、403/429、超时和网络失败均保留 `zo-king` 主页链接。
- 无 JavaScript 时通过 `noscript` 保留个人主页入口。

浏览器直连 GitHub 时，GitHub 仍会看到访问者 IP、User-Agent 和访问时间；该功能不引入本站侧追踪。

## 4. 视觉与可访问性

- 桌面双列，700px 以下单列；卡片圆角不超过 8px。
- 唯一 H1，仓库名称使用 H2，集合使用 `ul/li/article`。
- 加载、成功、空态和失败状态使用 polite live region。
- 语言色点同时显示语言文字，不单独依赖颜色表达。
- 外链均设置 `noopener noreferrer`，可访问名称明确说明新窗口。
- 长仓库名、空描述、空语言在 390px 和 320px 下均不得产生页面级横向滚动。

## 5. 维护方式

更换 GitHub 账号或数量只修改 `apps/site/content/page/projects/index.md`。不要把 GitHub Token 写入 Hugo 配置、浏览器代码或 source map；需要私有仓库时必须另行设计服务端授权和缓存代理。
