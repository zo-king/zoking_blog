# SITE-READING-SETTINGS-P32-001 QA

## 自动化

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\site-reading-settings-p32.mjs
```

## 覆盖

- 阅读设置入口只存在于文章页。
- `reading-settings.scss` 只由文章页加载，非文章页面不下载该组件样式。
- 原生 dialog 打开、关闭、Escape 和焦点恢复。
- 字号三档严格递增，宽松行距增加实际 line-height。
- 正文外链获得可见下划线。
- 白名单 JSON 写入、刷新前恢复、对话框控件同步。
- 恢复默认删除 localStorage 和 `<html>` 数据属性。
- 1280x900、390x844、320x568 对话框边界与页面横向溢出。
- 评论 API 使用空列表 fixture，不依赖本地后端状态。

## 结果

| 检查 | 结果 |
|---|---|
| Article-only 入口与 dialog | Pass |
| 三档字号与宽松行距 | Pass |
| 链接下划线 | Pass |
| DOMContentLoaded 前恢复 | Pass |
| 刷新持久化与恢复默认 | Pass |
| Escape 与焦点返回 | Pass |
| 1280/390/320 无横向溢出 | Pass |
| P16 阅读进度/搜索/评论回归 7 项 | Pass |
| console/page error | 0 |

证据：`docs/process/evidence/site-p32-reading-settings-desktop-1280x900.png`、`site-p32-reading-settings-mobile-390x844.png`。
