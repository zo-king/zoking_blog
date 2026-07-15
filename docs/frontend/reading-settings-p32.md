# P32 轻量阅读设置

日期：2026-07-15

## 目标

为长篇 Go、Gin、GORM、PostgreSQL 和数据结构文章提供个人化阅读选项，同时不修改站点全局排版、不依赖账号体系，也不增加常驻侧栏。

## 交互

- 入口只在文章页顶部操作区显示，使用熟悉的 `Aa` 字体符号。
- 原生 `<dialog>` 承载设置，支持 Escape、焦点约束和关闭后焦点恢复。
- 字号使用“标准、较大、特大”三段式控件。
- 宽松行距、正文链接下划线使用原生复选框。
- “恢复默认”同时清除当前样式和本地存储。

## 数据契约

浏览器只保存一个本地键：

```text
zoking:reading-preferences:v1
```

```json
{
  "font": "large",
  "relaxedSpacing": true,
  "underlineLinks": true
}
```

`font` 仅接受 `default`、`large`、`xlarge`。其他值回退为默认；两个布尔项只有严格等于 `true` 时启用。全部恢复默认后删除存储键，不创建用户 ID、统计事件或跨设备同步。

## 渲染策略

- 文章页 `<head>` 中的同步脚本在正文绘制前读取白名单设置，并写入 `<html>` 数据属性，避免刷新后字体闪动。
- `articleActions.ts` 负责对话框状态、控件同步、保存和重置。
- SCSS 继续复用 Theme Stack 的 `--article-font-size` 与 `--article-line-height`，不重写正文排版系统。
- 设置只影响 `.article-content`，不会放大导航、后台、项目页、友链或评论表单。

## 边界

- 不提供任意数字字号、字体下载、主题市场或云端同步。
- 不覆盖系统 `prefers-reduced-motion`；P31 已按系统偏好关闭页面动画。
- 不改变代码块横向滚动、图片尺寸和移动端布局约束。
