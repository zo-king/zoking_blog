# P34 打印与订阅发现

日期：2026-07-15

## 打印契约

- `print.scss` 只在独立文章页以 `media="print"` 加载，首页和普通页面不下载。
- 使用浏览器原生打印或保存 PDF，不增加站内按钮、脚本或服务端 PDF。
- A4 页边距为上/左右/下 `16/15/18mm`。
- 打印版面使用白底、深色正文、无阴影、无圆角、无动画；深色主题不会进入纸面。
- 标题和正文标题避免孤立在页尾；段落使用 3 行 widow/orphan 约束。
- 图片、引用、表格和代码块尽量避免跨页截断。
- 代码工具栏隐藏，Chroma 行号和代码正文保留；长代码可换行，不通过裁切消除溢出。
- 文章分类退化为普通文字，标签隐藏，许可与更新时间保留。

## 打印作者

屏幕端作者身份已由站点品牌和关于页表达，不新增常驻 byline。文章模板包含一个默认隐藏的 `article-print-byline`，打印时显示公开作者 `Zoking`，避免移除侧栏后纸面失去作者归属。

## RSS Discovery

- 首页继续使用 Hugo 自动生成的站点 RSS `<link rel="alternate">`。
- taxonomy 页面继续指向自己的 taxonomy feed。
- 文章页新增且仅新增一个站点主 feed discovery，绝对 URL 由 Hugo OutputFormat 生成。
- 页面上的可见 RSS 图标继续保留，两者职责不同：一个供读者点击，一个供阅读器自动发现。

## 结构化作者

`params.toml` 使用：

```toml
[author]
    name = "Zoking"
    url  = "https://github.com/zo-king"
```

BlogPosting 的 `author` 与 `publisher` 都使用 `Person` 和该公开身份。RSS 不填写虚假邮箱，因此不会输出 `managingEditor`、`webMaster` 或 item author 邮箱字段。

## 非目标

- 不生成 PDF 文件或引入 PDF 服务。
- 不新增打印统计、收藏、Markdown 输出、PWA 或 Service Worker。
- 不改变现有 `date/lastmod` 数据来源和 Admin 发布语义。
