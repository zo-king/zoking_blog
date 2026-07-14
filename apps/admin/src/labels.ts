const labels: Record<string, string> = {
  active: "启用", disabled: "停用", draft: "草稿", published: "已发布", offline: "已下线", archived: "已归档",
  public: "公开", private: "私密", unlisted: "不公开列出", pending: "待审核", approved: "已通过",
  rejected: "已拒绝", spam: "垃圾评论", requested: "已请求", queued: "排队中", snapshotting: "生成快照",
  building: "构建中", verifying: "验证中", promoting: "切换版本", ready: "可预览", canceled: "已取消",
  failed: "失败", inactive: "非活动", post: "文章", page: "页面", site: "站点"
};

const actions: Record<string, string> = {
  read: "查看", create: "创建", update: "更新", write: "编辑", manage: "管理", publish: "发布",
  cleanup: "清理", moderate: "审核", delete: "删除", upload: "上传", rollback: "版本回退",
  read_all: "跨作者查看", manage_all: "跨作者管理"
};

const resources: Record<string, string> = {
  achievement: "成果", audit: "审计日志", comment: "评论", content: "内容范围", media: "媒体", page: "页面", post: "文章", publish: "发布",
  qa: "质量保障", role: "角色", setting: "站点设置", settings: "站点设置", system: "系统", taxonomy: "分类标签", user: "用户"
};

const systemRoles: Record<string, string> = {
  admin: "管理员", author: "作者", editor: "编辑", super_admin: "超级管理员", viewer: "访客"
};

export function displayLabel(value?: string | null) {
  if (!value) return "-";
  return labels[value] || value;
}

export function permissionLabel(code: string) {
  const [resource, action] = code.split(":");
  if (!resource || !action) return code;
  return `${resources[resource] || resource} - ${actions[action] || action}`;
}

export function roleLabel(code: string, fallback?: string) {
  return systemRoles[code] || fallback || code;
}
