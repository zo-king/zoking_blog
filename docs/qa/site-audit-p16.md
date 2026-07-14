# SITE-AUDIT-P16 验收记录

日期：2026-07-13
范围：C 端阅读体验、搜索/评论可访问性、移动菜单、中文 section、生产 Nginx 404/gzip。

## 自动化入口

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
$env:SITE_P16_BASE='http://localhost:1313'
node .\scripts\qa\site-reader-ui.mjs
```

脚本只操作浏览器本地状态；评论 POST 和搜索失败均在浏览器侧拦截，不写数据库。

## 结果

| 验收项 | 结果 |
|---|---|
| 阅读进度保存、续读定位、完成清除、30 天过期 | PASS |
| 搜索 H1/H2/H3、live region、图片 alt | PASS |
| 搜索索引断网中文错误与原地重试 | PASS |
| 深色 secondary/tertiary 对卡片背景对比度 >= 4.5 | PASS |
| `/post/`、`/page/` 中文标题与唯一 aside 名称 | PASS |
| 390x844 菜单 Escape、外部关闭、焦点归还、无横向溢出 | PASS |
| 评论断网本地化 live-region 错误且无数据库写入 | PASS |
| Hugo production build | PASS，45 pages |
| Nginx 随机路径 | PASS，HTTP 404 + 中文 404 + `noindex` |
| Nginx CSS 压缩 | PASS，`Content-Encoding: gzip` |
| 仓库 `preflight.ps1 -SkipE2E` | PASS，Go tests、Admin build、Hugo build |

## 命令记录

```powershell
.\.tools\hugo\hugo.exe --source apps/site --destination D:\zoking\zoking-blog\dist\qa\site-p16 --cleanDestinationDir --gc --minify
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\qa\preflight.ps1 -SkipE2E
```

生产 Nginx 使用一次性 `nginx:1.27-alpine` 容器挂载 `dist/qa/site-p16` 和 `infra/docker/site.nginx.conf` 验证，结束后容器已自动停止。

## 证据

- `docs/process/evidence/site-p16-reading-progress-desktop.png`
- `docs/process/evidence/site-p16-mobile-home-390x844.png`
