# SITE-NAVIGATION-NOW-P31-001 QA

## 范围

- 原生跨文档 View Transition 的同源导航与浏览器前进/后退。
- `prefers-reduced-motion: reduce` 下页面动画关闭。
- `/now/` 唯一 H1、三个内容区块、canonical、主菜单和快捷面板入口。
- 1280x900 与 390x844 响应式、横向溢出和运行时错误。

## 自动化

```powershell
$env:PLAYWRIGHT_PACKAGE_PATH='C:\Users\zhaoxi\.codex\cache\npm\_npx\9833c18b2d85bc59\node_modules\playwright'
node .\scripts\qa\site-navigation-now-p31.mjs
```

支持 `PageRevealEvent` 的 Edge 中，测试同时监听 `pageswap` 与 `pagereveal`，断言同源导航实际获得 `ViewTransition` 对象，而不只检查 CSS 文本。

## 边界

- 不拦截链接，不改 History API，不影响外链、页内锚点和浏览器返回键。
- 不引入 Motion、GSAP、Canvas 或额外 JavaScript 动画运行时。
- 首次开屏仍只在会话首次访问出现；后续页面过渡不激活开屏层。
- `/now/` 使用现有 Hugo Page 与菜单契约，后续可由现有后台页面发布链路维护。

## 结果

| 检查 | 结果 |
|---|---|
| Edge `pageswap/pagereveal` ViewTransition | Pass |
| Reduced Motion 动画关闭 | Pass |
| 主菜单与快捷面板进入近况 | Pass |
| 浏览器前进/后退 | Pass |
| 1280/390 无横向溢出 | Pass |
| P29 快捷面板回归 | Pass |
| Go tests、Admin build、Hugo 62 pages | Pass |
| console/page error | 0 |

证据：`docs/process/evidence/site-p31-now-desktop-1280x900.png`、`site-p31-now-mobile-390x844.png`。
