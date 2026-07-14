import { useState, type Dispatch, type SetStateAction } from "react";
import { Message, type FormInstance } from "@arco-design/web-react";
import { ApiError, apiBase, apiFetch } from "../api/client";
import type { ApiEnvelope, CategoryFormValues, CleanupResult, Comment, MediaAsset, PostFormValues, Series, SeriesFormValues, TagFormValues, Taxonomy } from "../types/admin";

export function useContentAdminCommands({ token, canManageTaxonomy, canUploadMedia, canDeleteMedia, canInsertMediaMarkdown, canModerateComments, refresh, setError, setMedia, setComments, categoryForm, tagForm, seriesForm, postForm }: {
  token: string; refresh: () => Promise<void>; setError: (value: string) => void;
  canManageTaxonomy: boolean; canUploadMedia: boolean; canDeleteMedia: boolean; canInsertMediaMarkdown: boolean; canModerateComments: boolean;
  setMedia: Dispatch<SetStateAction<MediaAsset[]>>; setComments: Dispatch<SetStateAction<Comment[]>>;
  categoryForm: FormInstance<CategoryFormValues>; tagForm: FormInstance<TagFormValues>; seriesForm: FormInstance<SeriesFormValues>; postForm: FormInstance<PostFormValues>;
}) {
  const [taxonomyBusy, setTaxonomyBusy] = useState(false);
  const [mediaBusy, setMediaBusy] = useState(false);
  const [mediaCleanupBusy, setMediaCleanupBusy] = useState(false);
  const [commentBusy, setCommentBusy] = useState(false);
  const requireLogin = () => { if (token) return true; Message.warning("请先登录"); return false; };
  const requirePermission = (allowed: boolean) => {
    if (!requireLogin()) return false;
    if (allowed) return true;
    Message.warning("当前账号无权执行此操作");
    return false;
  };

  async function createTaxonomy(path: "categories" | "tags", values: CategoryFormValues | TagFormValues) {
    if (!requirePermission(canManageTaxonomy)) return;
    setTaxonomyBusy(true);
    try {
      await apiFetch<ApiEnvelope<Taxonomy>>(`/api/v1/admin/${path}`, { method: "POST", body: JSON.stringify(values) }, token);
      (path === "categories" ? categoryForm : tagForm).resetFields();
      Message.success(path === "categories" ? "分类已创建" : "标签已创建");
      await refresh();
    } catch (err) { setError(String(err)); } finally { setTaxonomyBusy(false); }
  }

  async function deleteTaxonomy(path: "categories" | "tags", id: string) {
    if (!requirePermission(canManageTaxonomy)) return;
    setTaxonomyBusy(true);
    try { await apiFetch(`/api/v1/admin/${path}/${id}`, { method: "DELETE" }, token); Message.success(path === "categories" ? "分类已删除" : "标签已删除"); await refresh(); }
    catch (err) { setError(String(err)); } finally { setTaxonomyBusy(false); }
  }

  function seriesError(error: unknown) {
    if (error instanceof ApiError && error.status === 409) {
      if (error.code === "SERIES_REFERENCED" || error.code === "SERIES_IN_USE") return "系列删除失败：该系列仍被文章引用，请先移除文章关联。";
      if (error.code === "SERIES_ORDER_CONFLICT") return "文章保存失败：该系列序号已被占用，请更换序号。";
      return `系列操作冲突：${error.message}`;
    }
    return String(error);
  }

  async function saveSeries(values: SeriesFormValues, id?: string): Promise<boolean> {
    if (!requirePermission(canManageTaxonomy)) return false;
    setTaxonomyBusy(true);
    try {
      const payload = {
        name: values.name,
        slug: values.slug,
        description: values.description || "",
        cover_media_id: values.cover_media_id || null,
        sort_order: values.sort_order ?? 0,
        enabled: values.enabled ?? true
      };
      const result = await apiFetch<ApiEnvelope<Series>>(id ? `/api/v1/admin/series/${id}` : "/api/v1/admin/series", { method: id ? "PATCH" : "POST", body: JSON.stringify(payload) }, token);
      seriesForm.setFieldsValue({ ...result.data, cover_media_id: result.data.cover_media_id || undefined });
      Message.success(id ? "系列已保存" : "系列已创建");
      await refresh();
      return true;
    } catch (err) {
      const message = seriesError(err);
      setError(message);
      Message.error(message);
      return false;
    } finally { setTaxonomyBusy(false); }
  }

  async function deleteSeries(id: string) {
    if (!requirePermission(canManageTaxonomy)) return;
    setTaxonomyBusy(true);
    try {
      await apiFetch(`/api/v1/admin/series/${id}`, { method: "DELETE" }, token);
      Message.success("系列已删除");
      await refresh();
    } catch (err) {
      const message = seriesError(err);
      setError(message);
      Message.error(message);
    } finally { setTaxonomyBusy(false); }
  }

  async function uploadMediaFile(options: { file: string | Blob | File; onSuccess?: (body: unknown) => void; onError?: (error: Error) => void }) {
    if (!requirePermission(canUploadMedia)) { options.onError?.(new Error("当前账号无权上传媒体")); return; }
    if (!(options.file instanceof File)) { options.onError?.(new Error("无效文件")); return; }
    setMediaBusy(true);
    try {
      const body = new FormData(); body.append("file", options.file);
      const result = await apiFetch<ApiEnvelope<MediaAsset>>("/api/v1/admin/media", { method: "POST", body }, token);
      setMedia((current) => [result.data, ...current]); options.onSuccess?.(result); Message.success("媒体已上传");
    } catch (err) { const error = err instanceof Error ? err : new Error(String(err)); options.onError?.(error); setError(String(err)); }
    finally { setMediaBusy(false); }
  }

  async function deleteMedia(id: string) {
    if (!requirePermission(canDeleteMedia)) return; setMediaBusy(true);
    try { await apiFetch(`/api/v1/admin/media/${id}`, { method: "DELETE" }, token); setMedia((current) => current.filter((item) => item.id !== id)); Message.success("媒体已删除"); }
    catch (err) { setError(String(err)); } finally { setMediaBusy(false); }
  }

  async function runMediaCleanup(dryRun: boolean) {
    if (!requirePermission(canDeleteMedia)) return; setMediaCleanupBusy(true);
    try {
      const result = await apiFetch<ApiEnvelope<CleanupResult>>("/api/v1/admin/media/cleanup", { method: "POST", body: JSON.stringify({ dry_run: dryRun }) }, token);
      if (dryRun) Message.info(`发现 ${result.data.candidate_count} 个孤立媒体候选`); else { Message.success(`已删除 ${result.data.deleted_count} 个孤立媒体`); await refresh(); }
    } catch (err) { setError(String(err)); } finally { setMediaCleanupBusy(false); }
  }

  const mediaURL = (asset: MediaAsset) => asset.public_url.startsWith("http") ? asset.public_url : `${apiBase}${asset.public_url}`;
  async function copyMediaURL(asset: MediaAsset) { await navigator.clipboard.writeText(mediaURL(asset)); Message.success("媒体地址已复制"); }
  function insertMediaMarkdown(asset: MediaAsset) { if (!requirePermission(canInsertMediaMarkdown)) return; const current = postForm.getFieldValue("content_md") || ""; const markdown = `![${asset.original_name}](${mediaURL(asset)})`; postForm.setFieldValue("content_md", current ? `${current}\n\n${markdown}` : markdown); Message.success("已插入 Markdown"); }

  async function moderateComment(id: string, status: string) {
    if (!requirePermission(canModerateComments)) return; setCommentBusy(true);
    try { const result = await apiFetch<ApiEnvelope<Comment>>(`/api/v1/admin/comments/${id}/moderation`, { method: "PATCH", body: JSON.stringify({ status }) }, token); setComments((current) => current.map((item) => item.id === id ? result.data : item)); Message.success("评论状态已更新"); }
    catch (err) { setError(String(err)); } finally { setCommentBusy(false); }
  }
  async function deleteComment(id: string) {
    if (!requirePermission(canModerateComments)) return; setCommentBusy(true);
    try { await apiFetch(`/api/v1/admin/comments/${id}`, { method: "DELETE" }, token); setComments((current) => current.filter((item) => item.id !== id)); Message.success("评论已删除"); }
    catch (err) { setError(String(err)); } finally { setCommentBusy(false); }
  }

  return {
    taxonomyBusy, mediaBusy, mediaCleanupBusy, commentBusy,
    createCategory: (values: CategoryFormValues) => createTaxonomy("categories", values), deleteCategory: (id: string) => deleteTaxonomy("categories", id),
    createTag: (values: TagFormValues) => createTaxonomy("tags", values), deleteTag: (id: string) => deleteTaxonomy("tags", id),
    saveSeries, deleteSeries,
    uploadMediaFile, deleteMedia, runMediaCleanup, mediaURL, copyMediaURL, insertMediaMarkdown, moderateComment, deleteComment
  };
}
