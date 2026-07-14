# Admin UX 信息架构与精简规范

## 1. 目标

本规范约束 `apps/admin` 的日常内容管理体验。组件库继续使用 Arco Design，但页面组织以真实 CMS 工作流为准，不以组件演示页或大屏模板为准。

核心目标：

- 日常主任务在首屏内完成，不纵向堆叠互不相关的工作流。
- 列表与完整编辑器分路由，支持刷新、前进后退和地址直达。
- 高频动作直接可见，低频维护和危险动作进入二级入口并保留确认。
- 完整业务能力继续保留；“精简”表示降低视觉优先级，不等于删除后端能力。
- 路由只加载当前模块所需数据，避免切换页面时请求整个后台。

## 2. 参考样例

| 产品 | 官方资料 | 采用的规律 |
|---|---|---|
| Ghost Admin | <https://ghost.org/help/posts/>、<https://ghost.org/help/post-settings/> | 文章列表和编辑器分离；编辑页只突出返回、保存、预览、发布；元数据集中到侧栏 |
| WordPress | <https://wordpress.org/documentation/article/posts-screen/>、<https://wordpress.org/documentation/article/wordpress-editor/> | 列表搜索、状态筛选、分页；删除/回收站和批量操作分层 |
| Strapi | <https://docs.strapi.io/cms/features/content-manager> | Collection List View 与 Edit View 分离；筛选、排序、列配置属于列表工作区 |
| Directus | <https://directus.io/docs/guides/content/explore>、<https://directus.io/docs/guides/content/editor> | 集合与单条内容分路由；复制、归档等放入高级动作并要求确认 |
| Payload CMS | <https://payloadcms.com/docs/custom-components/list-view> | 搜索、筛选、分页、批量动作围绕集合列表组织 |
| Sanity Studio | <https://www.sanity.io/docs/studio/structure-introduction>、<https://www.sanity.io/docs/studio/document-actions> | 文档动作分层；删除、复制、发布不与正文编辑混排 |
| Arco Design Pro | <https://github.com/arco-design/arco-design-pro> | 只参考 B 端壳层、表格和分页实现，不直接复制其业务信息架构 |

## 3. 路由与工作流

| 领域 | 路由 | 页面职责 |
|---|---|---|
| 文章列表 | `/posts` | 搜索、状态筛选、分页、进入编辑、归档 |
| 新建文章 | `/posts/new` | 独立编辑工作区 |
| 编辑文章 | `/posts/:postID/edit` | 独立编辑工作区，可直达和刷新 |
| 页面列表 | `/pages` | 搜索、状态筛选、分页、进入编辑、归档 |
| 新建页面 | `/pages/new` | 独立编辑工作区 |
| 编辑页面 | `/pages/:pageID/edit` | 独立编辑工作区，可直达和刷新 |
| 内容组织 | `/taxonomy` | 分类/标签 Tabs；创建表单使用 Modal |
| 媒体 | `/media` | 搜索、分页、上传；清理进入“维护”入口 |
| 评论 | `/comments` | 按审核状态切换和分页 |
| 发布 | `/publishing` | 发布任务/正式版本/预览 Tabs；任务日志进入 Drawer |
| 账号权限 | `/users` | 账号/角色 Tabs；创建和权限编辑进入 Modal |
| 设置 | `/settings` | 两列紧凑配置；保存、预览、发布位于页头 |
| 审计 | `/audit` | 紧凑分页表格 |

## 4. 页面编排规则

1. 全局侧栏只分“工作区”和“管理”两组，标签使用短名称。
2. 顶栏不重复面包屑和页面标题，只保留移动导航、博客入口、刷新和退出。
3. 页面头只出现一次标题、一句必要说明和主要命令，不显示装饰性 eyebrow。
4. 列表页常驻工具栏最多包含搜索、一个主要筛选和结果数。
5. 文章列表默认只显示文章摘要、状态、内容归类、发布时间和操作。
6. 完整编辑器不得放在列表下方，也不得放进 Modal/Drawer。
7. 桌面编辑器使用主内容区和右侧设置区；页面本身固定在视口内，两区独立滚动。
8. 发布、角色、分类等互斥子工作流使用 Tabs，禁止继续纵向堆叠多个完整表格。
9. 创建分类、创建标签、创建账号、创建角色使用 Modal，不常驻占用页面高度。
10. 发布日志和完整错误进入 Drawer；表格只显示能帮助判断状态的摘要。
11. 清理、删除、归档、版本切换保留 Popconfirm 或 Modal，不因精简而绕过安全确认。
12. 1280x720 桌面下，除有意设计的内部滚动区外，业务页面不得产生页面级横向或纵向滚动。

## 5. 数据加载规则

`useAdminData` 按当前 `AdminSection` 加载：

- 工作台：health、ready、文章和页面计数。
- 文章：文章、分类、标签、可用媒体。
- 页面：独立页面。
- 内容组织：分类和标签。
- 媒体、评论、发布、账号权限、设置、审计：只加载各自模块数据。
- 每个路由仍加载 `/api/v1/admin/auth/me`，用于实时权限与账号状态校验。

单个模块失败只清理当前模块状态，不再清空整个后台的数据。

## 6. 服务端分页契约

P8 已将文章、页面、媒体、评论、发布预览、发布任务、发布版本、账号和审计升级为 API 服务端分页：

- 请求参数统一为 `page`、`page_size`、`q`、`status`、`sort`；审计兼容旧 `limit`，媒体 `checksum/public_url` 精确查询保持旧数组响应。
- 默认 `page=1`、`page_size=20`，最大 `page_size=100`，非法页码、页容量或排序返回 `422 VALIDATION_FAILED`。
- 响应统一返回 `pagination.page/page_size/total/total_pages`；Admin 结果数和工作台计数使用数据库 `total`，不再使用当前数组长度冒充总数。
- Admin 的页码、每页数量、搜索、状态和排序写入 URL query；筛选变化自动回到第一页，刷新和浏览器前进后退可恢复状态。
- 文章和页面编辑路由按 ID 请求单条详情，不依赖目标记录恰好位于当前分页结果中。
- 所有 OFFSET 排序追加唯一 ID，避免同值记录跨页重复或漏项；超出总数的页码在 Count 后直接返回空数组，避免执行巨大 OFFSET。
- 发布和评论列表只预加载目标内容的 `id/title/slug`，不通过列表权限返回完整 Markdown 正文。

分类和标签当前数据量小，仍作为编辑器引用数据和内容组织页的客户端列表；若进入多租户或万级 taxonomy，再单独升级其服务端分页。

## 7. 当前验收结果

- 1280x720：12 个路由无页面级横向溢出；所有路由均为单屏高度。
- 390x844：12 个路由无页面级横向溢出；普通管理页为单屏，文章/页面编辑器约 2 屏，设置约 1.2 屏。
- 路由交互：文章进入编辑、直达加载和返回列表通过。
- 二级入口：发布详情 Drawer、分类创建 Modal、媒体维护 Modal、账号创建 Modal 均通过。
- 网络：按路由加载矩阵无异常 HTTP 响应。
- 构建：Admin build、Go tests、Hugo build 和仓库 preflight 通过。
- 依赖：官方 npm registry audit 为 0 漏洞。
- P8：`/posts?page=2&page_size=1` 返回 1 行、真实总数 3 且活动页为 2；搜索和状态 query 刷新后恢复，编辑后浏览器返回保留原 query。
- P8：九个 Admin 列表端点均返回分页元数据；非法排序返回 422，超大越界页返回空数组和真实总数。

## 8. 证据

- `docs/process/evidence/admin-ux-posts-list-desktop-1280x720.png`
- `docs/process/evidence/admin-ux-post-editor-desktop-1280x720.png`
- `docs/process/evidence/admin-ux-post-editor-mobile-390x844.png`
