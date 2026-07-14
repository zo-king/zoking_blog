# Admin 发布检查面板

日期：2026-07-13
任务：`CONTENT-QUALITY-P17-001`

## 1. 交互位置

文章和页面编辑器工具栏增加“发布检查”，不增加独立路由，也不把检查结果铺成长页面。结果使用 Arco Drawer：

- 桌面右侧 380px。
- `<=480px` 时全视口宽度。
- 标题、分数、状态、错误/警告计数、问题列表、规则版本和内容指纹在抽屉内部滚动。
- 页面主体不产生横向溢出，抽屉打开时由遮罩明确隔离编辑区。

## 2. 发布顺序

文章和页面必须遵循同一顺序：

```text
表单校验 -> 质量检查 -> ready=false 时打开 Drawer 并停止
                         -> ready=true 时保存 -> 调用 publish endpoint
```

不能为了复用旧保存逻辑而先写数据库再检查。这样才能让阻断项在不产生草稿写入的情况下反馈给用户。

手工检查：

- 新内容调用无 ID endpoint。
- 已有内容调用带 ID endpoint，同时发送当前未保存表单。
- 警告报告显示“可以发布”，阻断报告显示“暂不可发布”。
- “重新检查”始终读取当前表单，不复用旧请求体。

## 3. 状态失效

`qualityRunRef` 为每次请求分配递增编号。以下事件立即使旧报告和在途请求失效：

- 用户修改任一表单值。
- 切换文章或页面。
- 新建内容。
- 离开/重置编辑器。

旧请求即使稍后返回，也不能覆盖新状态。表单变更会关闭 Drawer，防止用户把旧报告误认为当前内容已通过。

## 4. 服务端错误回显

`ApiError` 解析标准 API error envelope，保留 `status/code/details/request_id`。如果最终 publish endpoint 因并发变化返回 `CONTENT_QUALITY_BLOCKED`，Admin 直接把 `details` 恢复成质量报告并打开 Drawer；其他错误继续进入全局错误提示。

前端不信任 `ready` 来绕过服务端 publish gate，也不提交 `content_hash` 作为授权凭据。

## 5. 权限

- 新内容检查随 create 权限显示。
- 已有内容检查随 update/save 权限显示。
- Publish 按钮仍要求 publish 权限。
- UI 隐藏/禁用只负责体验；API authorization middleware 是授权真相。

## 6. 可访问性与响应式

- Drawer 使用 Arco 自带 focus lock、Escape 和关闭按钮。
- 图标按钮具有可见文字或 `aria-label`。
- 状态不能只依赖颜色，始终显示“已就绪/需修复”和错误/提示计数。
- 问题字段与代码允许任意断行，不撑宽抽屉。
- 390x844 验收要求抽屉 `x=0`、宽度等于 viewport，footer 按钮可见，文档宽度不超过 viewport。

## 7. 维护边界

检查规则和中文 message 由 API 返回，Admin 不复制一套规则。前端只负责分组、状态呈现、流程控制和过期状态管理。新增 issue code 不需要修改组件；新增 severity 时必须先扩展契约与视觉规范。
