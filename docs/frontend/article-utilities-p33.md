# P33 文章轻量工具

日期：2026-07-15

## 目标

让技术文章中的章节引用和代码复制更清楚、更可靠，同时保持 Hugo Theme Stack 的正文结构、无 JavaScript 降级和克制视觉密度。

## 章节链接契约

- Hugo render hook 负责输出标题 `id`、原生 hash 链接、章节名称和可访问名称。
- 普通主键点击复制 `new URL(hash, location.href).href`，但不阻止现有锚点监听更新历史与滚动。
- Ctrl、Meta、Shift、Alt 修饰点击不触发复制增强。
- 成功状态持续 1400ms，`#` 变为小号勾选；live region 宣告具体章节已复制。
- Clipboard API 不可用时使用本地 textarea + `execCommand('copy')` 兼容路径；两者都失败时仍执行原生章节定位。
- reduced-motion 下锚点滚动为 `auto`，其余环境保持 Theme Stack 的平滑滚动。

## 代码工具栏契约

- `apps/site/assets/ts/code-copy.ts` 以同路径覆盖主题资产，不直接修改上游文件。
- 工具栏只处理 `.article-content div.highlight code[data-lang]`。
- 语言标签来自 Hugo `data-lang`，已知语言映射为常用显示名，未知语言显示大写原值。
- 复制内容严格取 `code.textContent`，Chroma 独立行号列不会进入剪贴板。
- 每个代码块只创建一个工具栏和一个复制按钮，重复初始化由数据属性阻止。
- 失败反馈为中文并给出“手动选择代码”的恢复路径，不再弹出原始 Error 对话框。

## 视觉与可访问性

- 工具栏是代码块内部普通布局，不覆盖第一行代码。
- 动效仅使用 160ms opacity；reduced-motion 下取消过渡。
- 键盘聚焦显示 2px accent outline。
- 触屏标题链接保持可见，命中区为 44x44px。
- live region 视觉隐藏但保留读屏输出；语言标签是信息文本，不伪装成按钮。

## 边界

- 不执行代码、不加载远程 Playground、不追踪复制行为。
- 不把文件名或行号混入复制正文。
- 不新增折叠附录、脚注系统或文章写作语法。
