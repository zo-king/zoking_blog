# SITE-GITHUB-SNAPSHOT-P30-001 QA

## 决策

项目页从访客侧实时 GitHub REST 请求改为仓库内静态快照。定时工作流在每月 1 日和 16 日北京时间 10:00 同步，失败时保留最后一次成功数据。

## 发布边界

- 工作流权限仅为 `contents: write`。
- 同步使用 Actions 自动提供的 `GITHUB_TOKEN`，不要求个人 PAT。
- 写入前完成响应状态、字段、日期和 URL 校验。
- 数据通过临时文件原子替换；失败请求不会先清空或改写正式 JSON。
- 自动提交只包含 `apps/site/data/github_projects.json`。
- 提交前执行 Hugo production/minify 构建。

## 验收

- 同步脚本 fixture 黑盒：Pass。
- 真实公开仓库同步：Pass，当前 4 项。
- Hugo 生产构建：Pass，61 pages。
- Playwright：桌面/移动、零访客 API 请求、零运行时错误全部 Pass。
