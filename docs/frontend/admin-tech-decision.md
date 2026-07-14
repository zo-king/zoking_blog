# B 端后台技术决策

本文记录后台控制台的技术选型、迁移边界和验收要求。最后更新：2026-07-12。

## 1. 已采用方案

后台统一采用：

- React 18.3.1。
- TypeScript 5。
- Vite 6。
- React Router 7。
- Arco Design React 2.66.15。
- Gin REST API + PostgreSQL 作为数据源。

组件库必须保持单一。业务源码不得同时引入 Ant Design、Semi Design、TDesign 或其他完整组件库。

## 2. 选型记录

2026-07-12 对候选库进行了官方仓库、npm 元数据和 React/Vite 兼容性核验：

| 方案 | 版本快照 | React peer | npm 解压体积 | 结论 |
|---|---:|---:|---:|---|
| Arco Design React | 2.66.15 | `>=16` | 约 17.2 MB | 采用，Admin 固定 React 18.3.1 |
| Semi Design | 2.101.0 | `>=16`，React 19 有独立适配建议 | 约 60.9 MB | 内容组件丰富，但迁移与适配成本更高 |
| TDesign React | 1.18.0 | `>=16.13.1` | 约 53.0 MB | 维护活跃，当前项目收益不足 |
| Ant Design | 原项目 5.23.2 | 依赖 React 19 补丁 | 约 48.4 MB | 生态最强，但现有默认观感已被否决 |

选择 Arco 的工程原因：

- 字节跳动开源，MIT 许可证，符合厂商开源组件复用要求。
- 当前版本 peer dependency 覆盖 React 18，不需要保留 Ant React 19 补丁。
- 表格、表单、上传、抽屉、弹窗、通知、权限菜单等后台基础组件完整。
- API 心智与原 Ant 页面接近，当前直接依赖范围仅限 Admin 约 16 个文件，一次性迁移可控。
- 候选库中安装体积最小，便于控制依赖与构建成本。

已知取舍：

- Arco 2026 年近期发布频率低于 Semi 和 Ant，需要持续关注安全与兼容更新。
- Arco 2.66.15 在 React 19 开发模式下仍有 Trigger/ResizeObserver 读取废弃 `element.ref` 的 console error；当前代码未使用 React 19 专属能力，因此固定 React 18.3.1，避免维护不可复现的依赖补丁。
- 换库本身不能解决页面观感；必须同时重构信息层级、间距、导航、表格密度和交互反馈。
- 若未来 Arco 维护明显下降，替换评估优先考虑 Semi，但必须作为独立 ADR 和完整迁移项目实施。

## 3. 迁移边界

本次迁移必须一次完成：

- 删除 `antd`、`@ant-design/icons`、`@ant-design/v5-patch-for-react-19`。
- 组件统一从 `@arco-design/web-react` 引入。
- 图标统一从 `@arco-design/web-react/icon` 引入。
- 全局加载 Arco 中文语言包和基础样式。
- Form、Message、Modal、Drawer、Upload、Table 等行为必须做功能回归。
- 不使用兼容层长期模拟 Ant API。
- 不更改 API payload、权限码、路由路径或发布业务语义。

## 4. 后台信息架构

导航按任务分组：

- 概览：工作台。
- 内容：文章、页面、分类与标签、媒体库。
- 运营：评论审核、发布中心。
- 系统：用户与权限、站点设置、审计日志。

路由保持真实且可直达：

```text
/dashboard
/posts
/pages
/taxonomy
/media
/comments
/publishing
/users
/settings
/audit
```

菜单和直接 URL 访问都必须受实时权限码约束。

## 5. 工程边界

- `App.tsx` 负责表单实例、数据 Hook 和页面命令编排。
- `layout/AdminLayout.tsx` 负责桌面侧栏、移动抽屉、全局页头和账号操作。
- `pages/*` 负责领域页面，不直接实现 API 请求。
- `hooks/*` 负责认证、数据刷新和领域命令。
- `components/AdminPage.tsx` 提供页面标题和内容工具面板等轻量公共结构。
- `styles.css` 维护视觉 Token、Arco 主题覆盖和响应式布局。

不得把所有业务页重新合并进单页，不得在页面中绕过 Hook 直接硬编码权限或请求地址。

## 6. 页面通用模式

列表与队列页应提供：

- 明确的页面标题和任务描述。
- 扫描友好的表头、状态标签和行操作。
- 空状态、加载状态、错误状态和成功反馈。
- 数据量增长后可接入搜索、筛选、排序、分页和批量操作。
- 长表格横向滚动，不压缩到文字重叠。

编辑页应提供：

- 保存草稿、预览和发布主流程。
- 必填和格式校验。
- 内容字段、发布字段和 SEO 字段的清晰分组。
- 高危删除或切换操作的二次确认。

## 7. 验收门禁

必须通过：

1. `npm run build`。
2. 源码中无 `antd` 和 `@ant-design/icons` 引用。
3. 登录、退出、刷新、真实路由和 RBAC 回归。
4. 文章保存、预览、发布和归档回归。
5. 媒体上传、评论审核、版本切换、用户角色与站点设置回归。
6. 1440px 桌面和 390px 移动视口无横向页面溢出、文字遮挡或失效导航。
7. 浏览器 console 无 error，关键 API 无异常 4xx/5xx。

## 8. 官方来源

- Arco Design: <https://github.com/arco-design/arco-design>
- Arco React 文档: <https://arco.design/react/docs/overview>
- Semi Design: <https://github.com/DouyinFE/semi-design>
- TDesign React: <https://github.com/Tencent/tdesign-react>
- Ant Design: <https://github.com/ant-design/ant-design>
