# 内容质量检查与发布门禁

日期：2026-07-13
任务：`CONTENT-QUALITY-P17-001`

## 1. 目标

内容质量检查是发布前服务端不变量，不是只由 Admin 展示的提示。系统允许草稿存在不完整内容，但任何文章、页面或全站重建进入正式发布流水线前，都必须通过同一版本的规则。

核心原则：

- PostgreSQL 仍是编辑源，检查不创建修订副本，也不修改数据库。
- 硬错误阻断发布；警告降低分数但不阻断。
- Admin 预检只改善反馈速度，最终授权和门禁由 API/worker 决定。
- 外链只做语法和协议检查，不发起网络请求，避免 SSRF、延迟和不确定性。
- Markdown 使用 Goldmark AST，渲染后 HTML 使用 `x/net/html` DOM，不以正文正则代替解析器。

## 2. API 契约

### 2.1 新内容预检

```http
POST /api/v1/admin/posts/quality-check
POST /api/v1/admin/pages/quality-check
```

请求体使用文章或页面表单字段。字段可以为空，API 会在报告中返回缺失项，不会创建内容。权限分别为 `post:create` 和 `page:create`。

### 2.2 已有内容预检

```http
POST /api/v1/admin/posts/:id/quality-check
POST /api/v1/admin/pages/:id/quality-check
```

API 先按 owner/global scope 加载已保存内容，再将请求体中的未保存字段覆盖到内存对象后检查。空请求体表示检查数据库当前版本。权限分别为 `post:update` 和 `page:update`；他人对象继续返回 404。

### 2.3 报告

```json
{
  "data": {
    "status": "blocked",
    "ready": false,
    "score": 40,
    "error_count": 2,
    "warning_count": 3,
    "content_hash": "sha256...",
    "policy_version": "2026-07-13.1",
    "issues": [
      {
        "code": "CONTENT_REQUIRED",
        "severity": "error",
        "field": "content_md",
        "message": "正文必须包含可见文本、代码或图片"
      }
    ]
  },
  "request_id": "..."
}
```

报告顺序稳定：错误优先，其次按字段与代码排序。`content_hash` 覆盖归一化后的全部检查输入，可用于识别旧报告；它不作为鉴权或并发控制凭据。

正式发布失败使用：

```http
HTTP/1.1 422 Unprocessable Entity
```

```json
{
  "error": {
    "code": "CONTENT_QUALITY_BLOCKED",
    "message": "内容未通过发布检查",
    "details": { "...": "同一报告结构" }
  },
  "request_id": "..."
}
```

## 3. 规则

硬错误：

- 标题、Slug 或可见正文缺失。
- Slug 格式错误，页面 Slug 使用保留根路径。
- 正式发布时可见性不是 `public`。
- Markdown 链接、图片或 HTML URL 使用 `javascript:`、`data:`、协议相对 URL等不受支持协议。
- 原始 HTML 包含 script、iframe、object、embed、form、表单控件、style、link、meta、base、事件属性、`srcdoc` 或内联 style。

警告：

- 摘要、SEO 标题或 SEO 描述缺失/长度不理想。
- 可见正文少于 200 个字符。
- 正文重复使用 H1。
- 图片 alt 缺失或为空。
- 文章缺封面、分类或标签。
- 菜单页面缺图标。

代码围栏中的危险 URL/HTML 示例不执行，也不误报为正文危险节点。纯 HTML 注释不算可见正文。

## 4. 发布不变量

文章与页面显式 publish endpoint 在单个事务中完成：

1. 按 owner/global scope 查询并对内容行执行 `FOR UPDATE`。
2. 检查是否已有 active publish job。
3. 对锁定后的最新正文执行质量检查。
4. 更新 `status=published` 和 `published_at`。
5. 同步 Markdown/封面媒体引用。
6. 创建 requested publish job。
7. 一次提交；任一步失败全部回滚。

发布端点不再把 private/unlisted 静默改为 public。用户必须先明确选择公开，否则返回质量错误。

普通 create/PATCH 不得从新建或草稿直接写入 `published`，即使调用者同时拥有 publish 权限；返回 `422 PUBLISH_ENDPOINT_REQUIRED`。已发布内容原有的 publish 权限约束保持不变。

文章或页面存在 requested、queued、snapshotting、building、verifying、promoting 任务时，PATCH 和撤稿均返回 `409 CONTENT_PUBLISH_IN_PROGRESS`。数据库部分唯一索引继续作为双发布的最终并发兜底。

## 5. 纵深防御

- Worker 在构建前重新加载文章/页面、taxonomy 和封面并复检；失败 job 写入 `CONTENT_QUALITY_BLOCKED`。
- Retry 在重新设为 requested 前复检；失败时 retry_count、状态和时间字段均不改变。
- 设置发布在创建 site job 前检查全部已发布文章和页面。
- Site worker 在构建前再次检查全部已发布内容。
- 撤稿任务不受内容质量阻断，确保坏内容仍可退出公开站点。
- 文章预览现在加载 `CoverMedia`，与正式发布的封面解析保持一致。

## 6. 规则演进

修改规则时必须：

1. 更新 `PolicyVersion`。
2. 增加正向、反向和代码块不误报测试。
3. 确认旧草稿只在发布时受新规则影响，不做后台批量写入。
4. 更新 Admin 文案和本文件。
5. 重新执行 PostgreSQL 零副作用、Worker/Retry 和 Playwright 顺序测试。
