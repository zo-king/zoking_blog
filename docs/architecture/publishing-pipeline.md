# Hugo 发布流水线设计

本文定义后台内容发布到 Hugo Theme Stack C 端站点的工程链路。

## 1. 发布目标

- 从 PostgreSQL 编辑源生成 Hugo 兼容内容快照和站点配置快照。
- 使用 Hugo Theme Stack 构建静态站点。
- 发布结果可追踪、可验证、可回滚。
- 发布失败不影响上一版线上站点。
- 发布范围支持 `post`、`page`、`site`。

## 2. 发布状态机

```text
requested
  -> snapshotting
  -> building
  -> verifying
  -> promoting
  -> published

任何阶段可进入 failed
人工取消进入 cancelled
```

状态含义：

| 状态 | 含义 |
|---|---|
| requested | 发布任务已创建 |
| snapshotting | 正在生成 Hugo 内容快照 |
| building | 正在执行 Hugo build |
| verifying | 正在校验构建产物 |
| promoting | 正在切换 release |
| published | 发布成功 |
| failed | 发布失败 |
| cancelled | 已取消 |

## 3. 快照目录

建议目录：

```text
storage/
  snapshots/
    20260618-120000-job_<id>/
      site/
        config/
        content/
        assets/
        static/
      manifest.json
  releases/
    rel_20260618_120030/
      public/
      manifest.json
  current -> releases/rel_...
```

规则：

- snapshot 不可变。
- release 不可变。
- current 只作为 active 指针。
- 回滚是切换 current，不是重写历史 release。

## 4. 发布流程

1. Admin 发起发布。
2. API 校验权限。
3. API 执行发布前检查。
4. API 创建 `publish_jobs`。
5. Worker 使用 `FOR UPDATE SKIP LOCKED` 获取任务。
6. Worker 生成快照目录。
7. Worker 按 job scope 写入 content 或 config，并生成 manifest。
8. Worker 执行 `hugo --minify --source <snapshot/site> --destination <build/public>`。
9. Worker 校验首页、文章页、页面页、RSS、Sitemap、搜索索引、资源引用和 settings 生效。
10. Worker 生成 release。
11. Worker 原子切换 active release。
12. Worker 写审计和发布日志。
13. 可选刷新 CDN。

## 5. 发布前检查

必须检查：

- slug 唯一。
- 文章/页面标题非空。
- 文章/页面正文非空。
- 状态允许发布。
- 分类和标签存在。
- 引用媒体存在且可访问。
- SEO 字段符合最低要求。
- 作者存在且未禁用。
- Hugo 配置可生成。
- 页面 slug 不得占用 Stack/Hugo 根路径、语言前缀、`p`、`search`、`categories`、`tags` 等保留路径。

建议检查：

- 封面图尺寸。
- 摘要长度。
- canonical 格式。
- 文章内部链接。
- 图片 alt。

## 6. 幂等与重试

幂等键：

- `publish_jobs.id`
- `snapshot.content_hash`
- `release.release_key`

规则：

- 同一任务重复执行不应产生多个 active release。
- 已 published 的 job 不重复发布。
- failed job 可重试，重试次数写入 `retry_count`。
- 重试应创建新的构建日志，旧日志保留。

## 7. 并发控制

默认策略：

- 同一时间只允许一个全站发布任务进入 build/promote。
- 多个内容编辑可并行，但发布串行。
- 定时发布任务进入同一队列。

PostgreSQL 锁：

```sql
select *
from publish_jobs
where status = 'requested'
  and run_at <= now()
order by run_at asc
for update skip locked
limit 1;
```

## 8. 构建校验

构建后必须校验：

- `public/index.html` 存在。
- `post` job 必须确认文章页 `/p/{slug}/index.html`、首页、RSS、taxonomy 页面和 sitemap 收录。
- `page` job 必须确认页面页 `/{slug}/index.html`、可选首页菜单和 sitemap 收录。
- `site` job 必须确认配置快照写入 manifest，并确认关键站点设置在最终 HTML 中生效。
- `sitemap.xml` 存在；多语言站点需要递归解析所有 sitemap XML 的 `<loc>`，确认新发布文章/页面 URL 收录。
- `index.xml` 或 RSS 存在。
- 静态资源引用无明显缺失。
- Hugo 命令退出码为 0。

可选校验：

- Playwright 打开首页和文章页。
- 检查 Stack 布局关键元素存在。
- 检查移动端无明显错位。

## 9. 回滚

回滚流程：

1. Admin 选择历史 release。
2. API 校验 `publish:rollback`。
3. API 检查 release 产物存在。
4. Worker 或 API 切换 active 指针。
5. 写审计日志。
6. 刷新缓存。

禁止：

- 删除当前 active release。
- 在无备份情况下覆盖 active release 目录。

## 10. 发布日志

每次发布记录：

- job id。
- 操作者。
- 触发来源。
- 内容范围：`post`、`page` 或 `site`。
- snapshot key。
- settings hash。
- release key。
- Hugo 版本。
- build 命令。
- build 耗时。
- 校验结果。
- 错误码和错误摘要。
