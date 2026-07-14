import { useRef, useState, type Dispatch, type SetStateAction } from "react";
import { Message, type FormInstance } from "@arco-design/web-react";
import { ApiError, apiBase, apiFetch } from "../api/client";
import type { ApiEnvelope, ContentQualityReport, Page, PageFormValues, Post, PostFormValues, PreviewBuildResult, PublishJob, PublishRelease, SiteSettings, SiteSettingsResponse } from "../types/admin";

export function useEditorialCommands({ token, canUpdateSettings, refresh, setError, postForm, pageForm, settingsForm, setSiteSettings, setSiteSettingsHash }: {
  token: string; refresh: () => Promise<void>; setError: (value: string) => void;
  canUpdateSettings: boolean;
  postForm: FormInstance<PostFormValues>; pageForm: FormInstance<PageFormValues>; settingsForm: FormInstance<SiteSettings>;
  setSiteSettings: Dispatch<SetStateAction<SiteSettings | null>>; setSiteSettingsHash: Dispatch<SetStateAction<string>>;
}) {
  const [selectedPostID, setSelectedPostID] = useState("");
  const [selectedPageID, setSelectedPageID] = useState("");
  const [busy, setBusy] = useState(false);
  const [deletingPostID, setDeletingPostID] = useState("");
  const [pageBusy, setPageBusy] = useState(false);
  const [settingsBusy, setSettingsBusy] = useState(false);
  const [previewBusy, setPreviewBusy] = useState("");
  const [qualityBusy, setQualityBusy] = useState(false);
  const [qualityReport, setQualityReport] = useState<ContentQualityReport | null>(null);
  const [qualityVisible, setQualityVisible] = useState(false);
  const [qualityTarget, setQualityTarget] = useState<"post" | "page">("post");
  const qualityRunRef = useRef(0);
  const requireLogin = () => { if (token) return true; Message.warning("请先登录"); return false; };
  const requireSettingsUpdate = () => {
    if (!requireLogin()) return false;
    if (canUpdateSettings) return true;
    Message.warning("当前账号无权修改站点设置");
    return false;
  };
  const waitForPublishJob = async (jobID: string) => {
    const deadline = Date.now() + 120_000;
    while (Date.now() < deadline) {
      const result = await apiFetch<ApiEnvelope<PublishJob[]>>("/api/v1/admin/publish/jobs", {}, token);
      const job = result.data.find((item) => item.id === jobID);
      if (job?.status === "published") return job;
      if (job?.status === "failed" || job?.status === "canceled") throw new Error(job.error_message || "撤稿任务执行失败");
      await new Promise((resolve) => window.setTimeout(resolve, 600));
    }
    throw new Error("撤稿任务等待超时，请前往发布中心查看任务状态");
  };
  function openPreviewURL(value: string) {
    const base = new URL(apiBase, window.location.origin);
    let target: URL;
    try {
      target = new URL(value, base);
    } catch {
      Message.error("预览地址无效，已拒绝打开");
      return;
    }

    const allowedOrigins = new Set([base.origin]);
    for (const configured of [import.meta.env.VITE_SITE_URL, import.meta.env.VITE_PREVIEW_URL]) {
      if (!configured) continue;
      try { allowedOrigins.add(new URL(configured, window.location.origin).origin); } catch { /* Invalid build-time configuration is ignored. */ }
    }
    if (import.meta.env.DEV) allowedOrigins.add("http://localhost:1313");

    if ((target.protocol !== "http:" && target.protocol !== "https:") || !allowedOrigins.has(target.origin)) {
      Message.error("预览地址不属于已配置的站点或预览来源，已拒绝打开");
      return;
    }

    const url = target.toString();
    const opened = window.open(url, "_blank", "noopener,noreferrer");
    if (!opened) { void navigator.clipboard.writeText(url); Message.info("预览地址已复制"); }
  }

  function invalidateQualityReport() { qualityRunRef.current += 1; setQualityBusy(false); setQualityReport(null); setQualityVisible(false); }
  function showServerQualityReport(error: unknown, target: "post" | "page") {
    if (!(error instanceof ApiError) || error.code !== "CONTENT_QUALITY_BLOCKED" || !isContentQualityReport(error.details)) return false;
    setQualityTarget(target); setQualityReport(error.details); setQualityVisible(true); return true;
  }
  function editorialError(error: unknown) {
    if (error instanceof ApiError && error.status === 409 && error.code === "SERIES_ORDER_CONFLICT") return "文章保存失败：该系列序号已被占用，请更换序号。";
    return String(error);
  }
  function selectPost(post: Post) { invalidateQualityReport(); setSelectedPostID(post.id); postForm.setFieldsValue({ ...post, visibility: post.visibility || "public", allow_comment: post.allow_comment ?? true, cover_media_id: post.cover_media_id || undefined, category_ids: post.categories?.map((item) => item.id) || [], tag_ids: post.tags?.map((item) => item.id) || [], series_id: post.series_id || undefined, series_order: post.series_id ? post.series_order ?? null : null }); }
  function newPost() { invalidateQualityReport(); setSelectedPostID(""); postForm.setFieldsValue({ title: "新文章", slug: `new-post-${Date.now()}`, summary: "从管理后台创建的新文章。", content_md: "# 新文章\n\n在这里使用 Markdown 编写正文。", seo_title: "新文章", seo_description: "从管理后台创建的新文章。", status: "draft", visibility: "public", allow_comment: true, cover_media_id: undefined, category_ids: [], tag_ids: [], series_id: undefined, series_order: null }); }
  function postPayload(values: PostFormValues) { return { title: values.title, slug: values.slug, summary: values.summary || "", content_md: values.content_md || "", seo_title: values.seo_title || values.title, seo_description: values.seo_description || values.summary || "", status: values.status || "draft", visibility: values.visibility || "public", allow_comment: values.allow_comment ?? true, cover_media_id: values.cover_media_id || "", category_ids: values.category_ids || [], tag_ids: values.tag_ids || [], series_id: values.series_id || "", series_order: values.series_id ? values.series_order ?? null : null }; }
  function pagePayload(values: PageFormValues) { return { title: values.title, slug: values.slug, summary: values.summary || "", content_md: values.content_md || "", seo_title: values.seo_title || values.title, seo_description: values.seo_description || values.summary || "", status: values.status || "draft", visibility: values.visibility || "public", show_in_menu: values.show_in_menu ?? false, menu_weight: values.menu_weight ?? 0, menu_icon: values.menu_icon || "", allow_comment: values.allow_comment ?? false }; }
  async function requestQuality(target: "post" | "page", values: PostFormValues | PageFormValues, openOnSuccess: boolean) {
    if (!requireLogin()) return null;
    const runID = qualityRunRef.current + 1;
    qualityRunRef.current = runID;
    setQualityBusy(true); setQualityTarget(target);
    const selectedID = target === "post" ? selectedPostID : selectedPageID;
    const endpoint = selectedID ? `/api/v1/admin/${target}s/${selectedID}/quality-check` : `/api/v1/admin/${target}s/quality-check`;
    try {
      const payload = target === "post" ? postPayload(values as PostFormValues) : pagePayload(values as PageFormValues);
      const result = await apiFetch<ApiEnvelope<ContentQualityReport>>(endpoint, { method: "POST", body: JSON.stringify(payload) }, token);
      if (qualityRunRef.current !== runID) return null;
      setQualityReport(result.data);
      if (openOnSuccess || !result.data.ready) setQualityVisible(true);
      if (result.data.ready && !openOnSuccess) Message.success(result.data.status === "warning" ? "发布检查通过，存在优化建议" : "发布检查通过");
      return result.data;
    } catch (err) { if (qualityRunRef.current === runID) setError(String(err)); return null; } finally { if (qualityRunRef.current === runID) setQualityBusy(false); }
  }
  async function checkPostQuality() { try { return await requestQuality("post", await postForm.validate(), true); } catch { Message.warning("请先完善必填内容"); return null; } }
  async function checkPageQuality() { try { return await requestQuality("page", await pageForm.validate(), true); } catch { Message.warning("请先完善必填内容"); return null; } }
  async function retryQualityCheck() { return qualityTarget === "post" ? checkPostQuality() : checkPageQuality(); }
  async function saveDraft(values: PostFormValues): Promise<Post | null> {
    if (!requireLogin()) return null; setBusy(true);
    try {
      const payload = postPayload(values);
      const result = selectedPostID ? await apiFetch<ApiEnvelope<Post>>(`/api/v1/admin/posts/${selectedPostID}`, { method: "PATCH", body: JSON.stringify(payload) }, token) : await apiFetch<ApiEnvelope<Post>>("/api/v1/admin/posts", { method: "POST", body: JSON.stringify(payload) }, token);
      setSelectedPostID(result.data.id); postForm.setFieldsValue({ ...result.data, cover_media_id: result.data.cover_media_id || undefined, category_ids: result.data.categories?.map((item) => item.id) || [], tag_ids: result.data.tags?.map((item) => item.id) || [], series_id: result.data.series_id || undefined, series_order: result.data.series_id ? result.data.series_order ?? null : null }); await refresh(); Message.success("草稿已保存"); return result.data;
    } catch (err) { setError(editorialError(err)); return null; } finally { setBusy(false); }
  }
  async function publish(): Promise<Post | null> {
    if (!requireLogin()) return null;
    let values: PostFormValues;
    try { values = await postForm.validate(); } catch { Message.warning("请先完善必填内容"); return null; }
    const report = await requestQuality("post", values, false); if (!report?.ready) return null;
    const saved = await saveDraft(values); const id = saved?.id || selectedPostID; if (!id) { Message.warning("请先保存草稿"); return null; } setBusy(true);
    try { const result = await apiFetch<ApiEnvelope<{ post: Post; job: PublishJob; release?: PublishRelease }>>(`/api/v1/admin/posts/${id}/publish`, { method: "POST" }, token); postForm.setFieldsValue({ ...result.data.post, cover_media_id: result.data.post.cover_media_id || undefined, category_ids: result.data.post.categories?.map((item) => item.id) || [], tag_ids: result.data.post.tags?.map((item) => item.id) || [], series_id: result.data.post.series_id || undefined, series_order: result.data.post.series_id ? result.data.post.series_order ?? null : null }); await refresh(); Message.success("文章发布任务已进入队列"); return result.data.post; }
    catch (err) { if (!showServerQualityReport(err, "post")) setError(String(err)); return null; } finally { setBusy(false); }
  }
  async function previewPostDraft(): Promise<Post | null> { if (!requireLogin()) return null; const saved = await saveDraft(await postForm.validate()); const id = saved?.id || selectedPostID; if (!id) return null; setPreviewBusy("post"); try { const result = await apiFetch<ApiEnvelope<PreviewBuildResult>>(`/api/v1/admin/posts/${id}/preview`, { method: "POST" }, token); openPreviewURL(result.data.target_url); await refresh(); Message.success("文章预览已生成"); return saved; } catch (err) { setError(String(err)); return null; } finally { setPreviewBusy(""); } }
  async function deletePost(id: string) { if (!requireLogin() || deletingPostID) return; setDeletingPostID(id); try { const result = await apiFetch<ApiEnvelope<{ withdrawal_requested: boolean; job: PublishJob }>>(`/api/v1/admin/posts/${id}`, { method: "DELETE" }, token); Message.info("文章撤稿任务已进入队列"); await waitForPublishJob(result.data.job.id); if (selectedPostID === id) { setSelectedPostID(""); postForm.resetFields(); } await refresh(); Message.success("文章已撤稿"); } catch (err) { setError(String(err)); await refresh(); } finally { setDeletingPostID(""); } }

  function selectPage(page: Page) { invalidateQualityReport(); setSelectedPageID(page.id); pageForm.setFieldsValue({ ...page, visibility: page.visibility || "public", show_in_menu: page.show_in_menu ?? false, menu_weight: page.menu_weight ?? 0, menu_icon: page.menu_icon || "", allow_comment: page.allow_comment ?? false }); }
  function newPage() { invalidateQualityReport(); setSelectedPageID(""); pageForm.setFieldsValue({ title: "关于本站", slug: `about-${Date.now()}`, summary: "由管理后台维护的独立页面。", content_md: "# 关于本站\n\n在这里使用 Markdown 编写页面内容。", seo_title: "关于本站", seo_description: "由管理后台维护的独立页面。", status: "draft", visibility: "public", show_in_menu: true, menu_weight: 30, menu_icon: "user", allow_comment: false }); }
  async function savePage(values: PageFormValues): Promise<Page | null> {
    if (!requireLogin()) return null; setPageBusy(true);
    try { const payload = pagePayload(values); const result = selectedPageID ? await apiFetch<ApiEnvelope<Page>>(`/api/v1/admin/pages/${selectedPageID}`, { method: "PATCH", body: JSON.stringify(payload) }, token) : await apiFetch<ApiEnvelope<Page>>("/api/v1/admin/pages", { method: "POST", body: JSON.stringify(payload) }, token); setSelectedPageID(result.data.id); pageForm.setFieldsValue(result.data); await refresh(); Message.success("页面已保存"); return result.data; }
    catch (err) { setError(String(err)); return null; } finally { setPageBusy(false); }
  }
  async function publishPage(): Promise<Page | null> { if (!requireLogin()) return null; let values: PageFormValues; try { values = await pageForm.validate(); } catch { Message.warning("请先完善必填内容"); return null; } const report = await requestQuality("page", values, false); if (!report?.ready) return null; const saved = await savePage(values); const id = saved?.id || selectedPageID; if (!id) return null; setPageBusy(true); try { const result = await apiFetch<ApiEnvelope<{ page: Page; job: PublishJob }>>(`/api/v1/admin/pages/${id}/publish`, { method: "POST" }, token); pageForm.setFieldsValue(result.data.page); await refresh(); Message.success("页面发布任务已进入队列"); return result.data.page; } catch (err) { if (!showServerQualityReport(err, "page")) setError(String(err)); return null; } finally { setPageBusy(false); } }
  async function previewPageDraft(): Promise<Page | null> { if (!requireLogin()) return null; const saved = await savePage(await pageForm.validate()); const id = saved?.id || selectedPageID; if (!id) return null; setPreviewBusy("page"); try { const result = await apiFetch<ApiEnvelope<PreviewBuildResult>>(`/api/v1/admin/pages/${id}/preview`, { method: "POST" }, token); openPreviewURL(result.data.target_url); await refresh(); Message.success("页面预览已生成"); return saved; } catch (err) { setError(String(err)); return null; } finally { setPreviewBusy(""); } }
  async function deletePage(id: string) { if (!requireLogin()) return; setPageBusy(true); try { const result = await apiFetch<ApiEnvelope<{ withdrawal_requested: boolean; job: PublishJob }>>(`/api/v1/admin/pages/${id}`, { method: "DELETE" }, token); Message.info("页面撤稿任务已进入队列"); await waitForPublishJob(result.data.job.id); if (selectedPageID === id) { setSelectedPageID(""); pageForm.resetFields(); } await refresh(); Message.success("页面已撤稿"); } catch (err) { setError(String(err)); await refresh(); } finally { setPageBusy(false); } }

  async function saveSettings(values: SiteSettings) { if (!requireSettingsUpdate()) return; setSettingsBusy(true); try { const result = await apiFetch<ApiEnvelope<SiteSettingsResponse>>("/api/v1/admin/settings", { method: "PATCH", body: JSON.stringify(values) }, token); setSiteSettings(result.data.settings); setSiteSettingsHash(result.data.hash); settingsForm.setFieldsValue(result.data.settings); Message.success("站点设置已保存"); await refresh(); } catch (err) { setError(String(err)); } finally { setSettingsBusy(false); } }
  async function publishSettings() { if (!requireSettingsUpdate()) return; const values = await settingsForm.validate(); setSettingsBusy(true); try { const saved = await apiFetch<ApiEnvelope<SiteSettingsResponse>>("/api/v1/admin/settings", { method: "PATCH", body: JSON.stringify(values) }, token); setSiteSettings(saved.data.settings); setSiteSettingsHash(saved.data.hash); settingsForm.setFieldsValue(saved.data.settings); await apiFetch("/api/v1/admin/settings/publish", { method: "POST" }, token); Message.success("站点发布任务已进入队列"); await refresh(); } catch (err) { setError(String(err)); } finally { setSettingsBusy(false); } }
  async function previewSettings() { if (!requireSettingsUpdate()) return; setPreviewBusy("site"); try { const result = await apiFetch<ApiEnvelope<PreviewBuildResult>>("/api/v1/admin/settings/preview", { method: "POST", body: JSON.stringify(await settingsForm.validate()) }, token); openPreviewURL(result.data.target_url); await refresh(); Message.success("站点预览已生成"); } catch (err) { setError(String(err)); } finally { setPreviewBusy(""); } }
  function resetEditors() { invalidateQualityReport(); setSelectedPostID(""); setSelectedPageID(""); postForm.resetFields(); pageForm.resetFields(); }

  return { busy, deletingPostID, pageBusy, settingsBusy, previewBusy, qualityBusy, qualityReport, qualityVisible, qualityTarget, selectPost, newPost, saveDraft, publish, previewPostDraft, deletePost, selectPage, newPage, savePage, publishPage, previewPageDraft, deletePage, checkPostQuality, checkPageQuality, retryQualityCheck, invalidateQualityReport, closeQualityPanel: () => setQualityVisible(false), saveSettings, publishSettings, previewSettings, openPreviewURL, resetEditors };
}

function isContentQualityReport(value: unknown): value is ContentQualityReport {
  if (!value || typeof value !== "object") return false;
  const report = value as Partial<ContentQualityReport>;
  return typeof report.ready === "boolean" && typeof report.score === "number" && Array.isArray(report.issues);
}
