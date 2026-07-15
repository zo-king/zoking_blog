# P19/P30 GitHub 项目页

更新时间：2026-07-15

## 1. 信息架构

- 一级导航保留“项目”入口 `/projects/`，不与友链混合。
- `/categories/`、`/tags/` 和归档深链继续保留，但分类与标签不占用一级导航。
- 项目页只展示账号自己的公开、未归档、非 Fork 仓库。

## 2. 静态数据契约

项目页由 front matter 控制账号和展示上限：

```yaml
github:
  username: zo-king
  maxRepos: 6
```

`scripts/sync-github-projects.mjs` 请求 GitHub REST API，并原子写入 `apps/site/data/github_projects.json`。快照只保留：

- 仓库名称、描述、主要语言。
- Star、Fork。
- `updated_at`、`pushed_at`。
- 经过 `https://github.com` 主机白名单校验的仓库 URL。

仓库按 `pushed_at` 降序排列，缺失时使用 `updated_at`，最后按名称稳定排序。同步阶段排除 Fork 和已归档仓库；Hugo 构建时按 `maxRepos` 截取展示数量。

## 3. 更新策略

`.github/workflows/sync-github-projects.yml` 在每月 1 日、16 日的 UTC 02:00，即北京时间 10:00 自动执行，也支持 `workflow_dispatch` 手动触发。

```text
GitHub REST API
  -> 字段验证、过滤、排序和 URL 白名单
  -> 临时文件
  -> 原子替换 github_projects.json
  -> Hugo production build
  -> github-actions[bot] 提交快照
```

同步失败时脚本在写文件前退出，不覆盖上一次成功快照。定时任务使用仓库自带的 `GITHUB_TOKEN`，不需要配置个人 PAT，也不会把 Token 写入数据、Hugo 页面或浏览器脚本。

手动同步：

```powershell
node .\scripts\sync-github-projects.mjs --username zo-king
```

## 4. 可靠性与隐私

- 访客浏览器不会请求 `api.github.com`，因此不会出现截图中的公开 API 限流错误。
- GitHub 不再因项目页访问获知读者 IP、User-Agent 或访问时间。
- Hugo 构建只读取仓库内快照，GitHub 临时不可用不会影响线上阅读。
- 空快照显示固定 GitHub 主页入口，不显示加载骨架、重试按钮或错误卡片。
- 所有远端文本由 Hugo 自动 HTML 转义，外链固定使用 `noopener noreferrer`。

## 5. 视觉与可访问性

- 桌面双列，700px 以下单列；卡片圆角不超过 8px。
- 唯一 H1，仓库名称使用 H2，集合使用 `ul/li/article`。
- 页面明确显示快照日期，让读者知道数据不是实时统计。
- 语言色点同时显示语言文字，不只依赖颜色表达。
- 长仓库名、空描述、空语言在 390px 和 320px 下不得产生页面级横向滚动。
