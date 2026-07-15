# SITE-DISCOVERY-P29-001 QA

## 范围

- 友链搜索、分类、结果计数和随机拜访。
- 全站快捷面板的按钮、`Ctrl/Cmd+K`、过滤、键盘导航、页面跳转和主题切换。
- 首页文章与友链卡片入场动画、开屏协调及 reduced-motion 降级。
- 1280x900 与 390x844 响应式、横向溢出和运行时错误。

## 自动化

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH = '<playwright-package-directory>'
node .\scripts\qa\site-discovery-p29.mjs
node .\scripts\qa\about-links-p21.mjs
```

P29 使用本地 favicon 和外站页面 fixture，不依赖第三方站点在线状态。随机拜访先筛选到“前端动效”，断言目标只能是当前可见的 Josh Comeau 或 Cassie Evans。

## 结果

| 检查 | 结果 |
|---|---|
| Hugo production/minify 61 pages | Pass |
| 友链 9 项与头像三级回退 | Pass |
| 搜索 `Josh` 返回唯一结果 | Pass |
| “前端动效”返回 2 项 | Pass |
| 随机拜访限制在可见结果 | Pass |
| 顶部按钮与 Ctrl+K 打开并聚焦输入框 | Pass |
| 过滤“项目”并 Enter 跳转 | Pass |
| 主题命令切换并关闭面板 | Pass |
| 1280/390 无横向溢出 | Pass |
| 移动端单列与面板边界 | Pass |
| reduced-motion 无入场动画 | Pass |
| pageerror/console error | 0 |
| P21 关于/友链回归 | Pass |

## 证据

- `docs/process/evidence/site-discovery-links-p29-1280x900.png`
- `docs/process/evidence/site-command-palette-p29-1280x900.png`
- `docs/process/evidence/site-command-palette-p29-390x844.png`

快捷面板没有可见的操作说明或快捷键教程；按钮通过 `aria-label` 命名，模态框提供焦点恢复与循环。无 JavaScript 时友链、主导航和搜索页面仍可直接使用。
