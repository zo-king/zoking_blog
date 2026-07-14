# Front Matter 与快照映射

本文定义 PostgreSQL 内容字段如何映射为 Hugo/Stack 可识别的 Markdown 和配置。

## 1. 基本原则

- 数据库字段是编辑源。
- Markdown 文件是发布快照。
- Front matter 字段必须可重复生成。
- 不把后台私密字段写入 front matter。
- 不让 C 端直接读取数据库。

## 2. 文章文件路径

推荐使用 page bundle：

```text
content/
  post/
    my-first-post/
      index.md
      cover.jpg
```

路径规则：

- `content/post/{slug}/index.md`
- slug 由后台生成和校验。
- 附属图片可复制到 bundle，也可使用对象存储 URL。

## 3. 文章 Front Matter

建议 YAML：

```yaml
---
title: "文章标题"
description: "SEO 描述或摘要"
slug: "my-first-post"
date: 2026-06-18T12:00:00+08:00
lastmod: 2026-06-18T12:00:00+08:00
draft: false
categories:
  - "工程"
tags:
  - "Go"
  - "Hugo"
image: "cover.jpg"
comments: true
toc: true
readingTime: true
---
```

数据库映射：

| DB 字段 | Front matter | 说明 |
|---|---|---|
| `posts.title` | `title` | 文章标题 |
| `posts.seo_description` or `summary` | `description` | SEO 描述 |
| `posts.slug` | `slug` | URL slug |
| `posts.published_at` | `date` | 发布时间 |
| `posts.updated_at` | `lastmod` | 更新时间 |
| `posts.status` | `draft` | 非发布状态为 true |
| `categories.name` | `categories` | 分类 |
| `tags.name` | `tags` | 标签 |
| `cover_media.public_url` or copied file | `image` | 封面 |
| `posts.allow_comment` | `comments` | 评论开关 |

## 4. 页面 Front Matter

```yaml
---
title: "关于"
description: "关于本站"
slug: "about"
date: 2026-06-18T12:00:00+08:00
lastmod: 2026-06-18T12:00:00+08:00
draft: false
comments: false
toc: true
menu:
  main:
    weight: 20
    params:
      icon: "user"
---
```

后台页面字段映射：

| DB 字段 | Front matter | 说明 |
|---|---|---|
| `pages.title` | `title` | 页面标题 |
| `pages.seo_description` or `summary` | `description` | SEO 描述 |
| `pages.slug` | `slug` | 根路径 slug，发布为 `/{slug}/` |
| `pages.published_at` | `date` | 发布时间 |
| `pages.updated_at` | `lastmod` | 更新时间 |
| `pages.status` | `draft` | 发布时固定生成 `draft: false` |
| `pages.allow_comment` | `comments` | 页面评论开关 |
| `pages.show_in_menu` | `menu.main` | 是否生成主菜单 |
| `pages.menu_weight` | `menu.main.weight` | 菜单排序 |
| `pages.menu_icon` | `menu.main.params.icon` | Stack 菜单图标 |

页面 slug 占用 Hugo 根路径，因此后台必须校验 reserved slug，至少包括 `p`、`post`、`page`、`search`、`categories`、`tags`、`archives`、`api`、`admin`、语言前缀和静态资源目录。

## 5. 分类标签

Hugo taxonomy 由文章 front matter 自动聚合。Front matter 必须写数据库 `slug` 作为稳定 URL key，中文名称和描述写入 taxonomy term 的 `_index.md`，避免后台 slug 与 C 端路由分裂。

后台仍需生成：

- 分类列表数据。
- 标签列表数据。
- taxonomy `_index.md`，用于中文展示名称、描述和 SEO，由发布器生成。

示例：

```text
content/
  categories/
    engineering/
      _index.md
  tags/
    go/
      _index.md
```

## 6. 站点配置映射

后台 `site_settings` 可生成 Hugo 配置片段：

```text
config/_default/
  hugo.toml
  params.toml
  languages.toml
```

当前已实现白名单映射：

| Setting key | Hugo 配置 | 说明 |
|---|---|---|
| `site.title` | `hugo.toml:title`、默认语言 `languages.toml:[lang].title` | 站点标题 |
| `site.base_url` | `hugo.toml:baseURL` | canonical base URL |
| `sidebar.subtitle` | `params.toml:[sidebar].subtitle`、默认语言 sidebar subtitle | Stack 侧栏副标题 |
| `sidebar.emoji` | `params.toml:[sidebar].emoji` | Stack 侧栏 emoji |
| `comments.enabled` | `params.toml:[comments].enabled` | C 端评论总开关 |
| `comments.api_base` | `params.toml:[comments.public].apiBase` | Public Comments API 地址 |
| `footer.since` | `params.toml:[footer].since` | 页脚起始年份 |
| `pagination.pager_size` | `hugo.toml:[pagination].pagerSize` | 列表分页大小 |

未实现但可扩展：

- 语言独立标题/副标题。
- 菜单与社交链接。
- widgets。
- Open Graph。
- 图片处理。

私密配置不写入：

- JWT secret。
- 数据库连接。
- S3 secret。
- 内部 token。

## 7. 媒体映射

本地存储模式：

- 发布时复制被引用媒体到 snapshot `static/uploads` 或 page bundle。
- front matter 与 Markdown 中写相对路径或站点绝对路径。

对象存储模式：

- Markdown 中写 CDN URL。
- release manifest 记录媒体 checksum 和 URL。

删除规则：

- 文章创建、编辑和发布时解析 Markdown 中的媒体引用，写入 `media_usages(resource_type=post, usage_type=markdown)`。
- 发布成功后同步 release 级引用，写入 `media_usages(resource_type=release, usage_type=markdown)`。
- 被当前文章或已保留 release 引用的媒体不能直接物理删除。
- 媒体清理任务必须检查 active release、保留期内 release 和孤立媒体策略。
- 当前清理实现默认 dry-run；真实清理只处理无引用且超过 `MEDIA_ORPHAN_GRACE_PERIOD` 的媒体，以及超过 `PUBLISH_RELEASE_KEEP_LATEST` / `PUBLISH_RELEASE_KEEP_DAYS` 的 inactive release。

## 8. Manifest

每次快照写 `manifest.json`：

```json
{
  "job_id": "job_...",
  "scope": "post",
  "post_id": "uuid",
  "page_id": "",
  "slug": "my-first-post",
  "created_at": "2026-06-18T12:00:00+08:00",
  "content_path": "content/post/my-first-post/index.md",
  "content_hash": "sha256...",
  "release_key": "rel_...",
  "output_path": "dist/releases/rel_...",
  "current_path": "dist/site",
  "hugo_command": "hugo --source ...",
  "settings_hash": "sha256..."
}
```

用途：

- 发布审计。
- 回滚确认。
- 构建复现。
- 媒体引用保护。

## 9. 预览快照

文章、页面和站点设置预览会先复制 `apps/site` 到临时工作目录，再在副本中应用设置与内容快照，最终构建到 `PUBLISH_PREVIEW_ROOT/{preview_key}`。

- 文章目标：`/p/{slug}/`
- 页面目标：`/{slug}/`
- 设置目标：`/`
- 预览 manifest 使用 `preview_key` 作为构建标识，但不写入 `publish_releases`。
- 设置预览只对内存快照应用白名单 patch，不更新 `site_settings`。
- 预览 URL 使用独立 baseURL，正式 release sitemap 与 active/current 语义不受影响。
