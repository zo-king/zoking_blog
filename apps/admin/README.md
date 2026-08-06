# Zoking Admin(内容管理后台)

React 18 + Vite + [Arco Design](https://arco.design)(字节跳动开源设计体系)+ ECharts。

## 开发

```powershell
npm install
npm run dev        # http://localhost:5173,/api 代理到 localhost:18080
npm run build      # tsc 类型检查 + 产物构建
npx vite preview --port 5173   # 预览生产构建(必须 5173,后端 admin 源白名单限制)
```

## 设计体系(可被其他项目复用)

设计目标:模块化、令牌化,换品牌/换项目只动配置不动组件。

### 1. 设计令牌(`src/styles.css` 顶部 `:root`)

所有颜色集中在一段 CSS 变量里,与 Arco 官方 arcoblue/gray 色板对齐:

- `--admin-primary` / `--admin-primary-soft` / `--admin-line` / `--admin-ink` … 供业务样式引用;
- `--primary-1..10`(RGB 三元组)覆写 Arco 组件内部色板,保证组件与自定义样式同源。

**换品牌只需替换这一段变量值**,无需改任何组件代码。

### 2. 应用壳层(`src/layout/AdminLayout.tsx`)

Arco Pro 风格:浅色可折叠侧栏(220px ↔ 64px)+ 面包屑顶栏 + 右上用户下拉。
完全配置驱动,与业务解耦:

```tsx
<AdminLayout
  section={section}          // 当前路由 key,用于选中态与面包屑
  navItems={navItems}        // { key, icon, label, group }[] —— 换项目传自己的导航
  currentUser={currentUser}
  loggedIn={loggedIn}
  onLogout={...}
  onRefresh={...}
>{page}</AdminLayout>
```

### 3. 图表(`src/components/charts/EChart.tsx`)

ECharts 按需注册(line/bar/pie + 基础组件,独立 chunk ~190KB gzip),
`ResizeObserver` 自适应,默认使用与令牌一致的 `CHART_COLORS` 色板:

```tsx
<EChart option={option} height={280} />
```

### 4. 页面骨架(`src/components/AdminPage.tsx`)

`PageHeader`(标题/描述/操作区)与 `ContentPanel`(卡片容器),
所有页面统一使用,保证间距与层级一致。

### 接入新项目的最小拷贝集

`styles.css` 的令牌段与基础组件样式 + `layout/AdminLayout.tsx` +
`components/AdminPage.tsx` + `components/charts/EChart.tsx`;
依赖:`@arco-design/web-react`、`echarts`、`react-router-dom`。

## 工作台数据

仪表盘图表由 `GET /api/v1/admin/stats/overview` 驱动(权限 `system:read`),
返回:文章/页面/评论/发布任务的状态分布、媒体与成果计数、
浏览与点赞总量、近 14 天浏览与点赞按日序列、热门文章 Top5。
前端 `src/hooks/useDashboardStats.ts` 负责拉取,失败静默不阻塞其余内容。

## 运行时配置

生产镜像通过 `public/runtime-config.js`(`window.__ZOKING_ADMIN_CONFIG__`)
注入 `apiBaseUrl` / `siteBaseUrl`,无需重新构建。
