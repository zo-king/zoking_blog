import { useCallback, useEffect, useState } from "react";
import type { FormInstance } from "@arco-design/web-react";
import { useLocation } from "react-router-dom";
import { ApiError, apiFetch } from "../api/client";
import type { AdminSection, AdminUser, ApiEnvelope, ApiStatus, AuditLog, Comment, CurrentUser, MediaAsset, Page, PaginationMeta, Post, PublishJob, PublishPreview, PublishRelease, Series, SiteSettings, SiteSettingsResponse, SystemRole, Taxonomy } from "../types/admin";

const emptyPagination: PaginationMeta = { page: 1, page_size: 20, total: 0, total_pages: 0 };

function paginationFrom<T>(result: ApiEnvelope<T[]>, fallbackPageSize = 20): PaginationMeta {
  const total = result.data.length;
  return result.pagination || { page: 1, page_size: fallbackPageSize, total, total_pages: total === 0 ? 0 : Math.ceil(total / fallbackPageSize) };
}

function listSearch(search: string) {
  const source = new URLSearchParams(search);
  const target = new URLSearchParams();
  for (const key of ["page", "page_size", "q", "status", "sort"]) {
    const value = source.get(key);
    if (value) target.set(key, value);
  }
  const query = target.toString();
  return query ? `?${query}` : "";
}

export function useAdminData(token: string, setCurrentUser: (user: CurrentUser | null) => void, settingsForm: FormInstance<SiteSettings>, onUnauthorized: () => void, section: AdminSection, editorID: string | undefined, settingsFormConnected: boolean) {
  const location = useLocation();
  const [health, setHealth] = useState<ApiStatus>("checking");
  const [ready, setReady] = useState<ApiStatus>("checking");
  const [posts, setPosts] = useState<Post[]>([]);
  const [pages, setPages] = useState<Page[]>([]);
  const [categories, setCategories] = useState<Taxonomy[]>([]);
  const [tags, setTags] = useState<Taxonomy[]>([]);
  const [series, setSeries] = useState<Series[]>([]);
  const [media, setMedia] = useState<MediaAsset[]>([]);
  const [comments, setComments] = useState<Comment[]>([]);
  const [publishPreviews, setPublishPreviews] = useState<PublishPreview[]>([]);
  const [publishJobs, setPublishJobs] = useState<PublishJob[]>([]);
  const [publishReleases, setPublishReleases] = useState<PublishRelease[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [adminUsers, setAdminUsers] = useState<AdminUser[]>([]);
  const [systemRoles, setSystemRoles] = useState<SystemRole[]>([]);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [siteSettings, setSiteSettings] = useState<SiteSettings | null>(null);
  const [siteSettingsHash, setSiteSettingsHash] = useState("");
  const [postPagination, setPostPagination] = useState<PaginationMeta>(emptyPagination);
  const [pagePagination, setPagePagination] = useState<PaginationMeta>(emptyPagination);
  const [mediaPagination, setMediaPagination] = useState<PaginationMeta>(emptyPagination);
  const [commentPagination, setCommentPagination] = useState<PaginationMeta>(emptyPagination);
  const [previewPagination, setPreviewPagination] = useState<PaginationMeta>(emptyPagination);
  const [jobPagination, setJobPagination] = useState<PaginationMeta>(emptyPagination);
  const [releasePagination, setReleasePagination] = useState<PaginationMeta>(emptyPagination);
  const [userPagination, setUserPagination] = useState<PaginationMeta>(emptyPagination);
  const [auditPagination, setAuditPagination] = useState<PaginationMeta>(emptyPagination);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    setError("");
    if (section === "dashboard") {
      try { await apiFetch("/healthz"); setHealth("ok"); } catch (err) { setHealth("error"); setError(String(err)); }
      try { await apiFetch("/readyz"); setReady("ok"); } catch { setReady("error"); }
    }

    try {
      const currentUserResult = token ? await apiFetch<ApiEnvelope<CurrentUser>>("/api/v1/admin/auth/me", {}, token) : ({ data: null, request_id: "" } as unknown as ApiEnvelope<CurrentUser>);
      const granted = (permission: string) => Boolean(currentUserResult.data?.permissions.includes(permission));
      const [postPath, categoryPath, tagPath] = token && granted("post:read") ? ["/api/v1/admin/posts", "/api/v1/admin/categories", "/api/v1/admin/tags"] : ["/api/v1/public/posts", "/api/v1/public/categories", "/api/v1/public/tags"];
      const pagePath = token && granted("page:read") ? "/api/v1/admin/pages" : "/api/v1/public/pages";
      const settingsPath = token && granted("setting:read") ? "/api/v1/admin/settings" : "/api/v1/public/site/public-settings";
      const empty = <T,>() => Promise.resolve({ data: [] as T[], request_id: "" } as ApiEnvelope<T[]>);
      const search = listSearch(location.search);
      setCurrentUser(currentUserResult.data);

      const editorAllowed = !editorID || (
        section === "posts"
          ? editorID === "new" ? granted("post:create") : granted("post:update")
          : section === "pages"
            ? editorID === "new" ? granted("page:create") : granted("page:update")
            : true
      );
      if (!editorAllowed) {
        if (section === "posts") {
          setPosts([]); setCategories([]); setTags([]); setSeries([]); setMedia([]); setPostPagination(emptyPagination);
        }
        if (section === "pages") {
          setPages([]); setPagePagination(emptyPagination);
        }
        return;
      }

      if (section === "dashboard") {
        const [postResult, pageResult] = await Promise.all([
          apiFetch<ApiEnvelope<Post[]>>(`${postPath}?page=1&page_size=1`, {}, token || undefined),
          apiFetch<ApiEnvelope<Page[]>>(`${pagePath}?page=1&page_size=1`, {}, token || undefined)
        ]);
        setPosts(postResult.data); setPages(pageResult.data); setPostPagination(paginationFrom(postResult, 1)); setPagePagination(paginationFrom(pageResult, 1));
      } else if (section === "posts") {
        const mediaRequest = token && granted("media:read") ? apiFetch<ApiEnvelope<MediaAsset[]>>("/api/v1/admin/media?page=1&page_size=100", {}, token) : empty<MediaAsset>();
        const seriesPath = token && granted("taxonomy:read") ? "/api/v1/admin/series" : "/api/v1/public/series";
        const postRequest = editorID === "new"
          ? empty<Post>()
          : editorID
            ? apiFetch<ApiEnvelope<Post>>(`/api/v1/admin/posts/${editorID}`, {}, token).then((result) => ({ ...result, data: [result.data], pagination: { page: 1, page_size: 1, total: 1, total_pages: 1 } }))
            : apiFetch<ApiEnvelope<Post[]>>(`${postPath}${search}`, {}, token || undefined);
        const [postResult, categoryResult, tagResult, seriesResult, mediaResult] = await Promise.all([
          postRequest,
          apiFetch<ApiEnvelope<Taxonomy[]>>(categoryPath, {}, token || undefined),
          apiFetch<ApiEnvelope<Taxonomy[]>>(tagPath, {}, token || undefined),
          apiFetch<ApiEnvelope<Series[]>>(seriesPath, {}, token || undefined),
          mediaRequest
        ]);
        setPosts(postResult.data); setCategories(categoryResult.data); setTags(tagResult.data); setSeries(seriesResult.data); setMedia(mediaResult.data); setPostPagination(paginationFrom(postResult));
      } else if (section === "pages") {
        const pageResult = editorID === "new"
          ? await empty<Page>()
          : editorID
            ? await apiFetch<ApiEnvelope<Page>>(`/api/v1/admin/pages/${editorID}`, {}, token).then((result) => ({ ...result, data: [result.data], pagination: { page: 1, page_size: 1, total: 1, total_pages: 1 } }))
            : await apiFetch<ApiEnvelope<Page[]>>(`${pagePath}${search}`, {}, token || undefined);
        setPages(pageResult.data); setPagePagination(paginationFrom(pageResult));
      } else if (section === "taxonomy") {
        const adminTaxonomy = token && granted("taxonomy:read");
        const mediaRequest = token && granted("media:read") ? apiFetch<ApiEnvelope<MediaAsset[]>>("/api/v1/admin/media?page=1&page_size=100", {}, token) : empty<MediaAsset>();
        const [categoryResult, tagResult, seriesResult, mediaResult] = await Promise.all([
          apiFetch<ApiEnvelope<Taxonomy[]>>(adminTaxonomy ? "/api/v1/admin/categories" : "/api/v1/public/categories", {}, token || undefined),
          apiFetch<ApiEnvelope<Taxonomy[]>>(adminTaxonomy ? "/api/v1/admin/tags" : "/api/v1/public/tags", {}, token || undefined),
          apiFetch<ApiEnvelope<Series[]>>(adminTaxonomy ? "/api/v1/admin/series" : "/api/v1/public/series", {}, token || undefined),
          mediaRequest
        ]);
        setCategories(categoryResult.data); setTags(tagResult.data); setSeries(seriesResult.data); setMedia(mediaResult.data);
      } else if (section === "media") {
        const result = token && granted("media:read") ? await apiFetch<ApiEnvelope<MediaAsset[]>>(`/api/v1/admin/media${search}`, {}, token) : await empty<MediaAsset>();
        setMedia(result.data); setMediaPagination(paginationFrom(result));
      } else if (section === "comments") {
        const result = token && granted("comment:read") ? await apiFetch<ApiEnvelope<Comment[]>>(`/api/v1/admin/comments${search}`, {}, token) : await empty<Comment>();
        setComments(result.data); setCommentPagination(paginationFrom(result));
      } else if (section === "publishing") {
        const [previewResult, jobResult, releaseResult] = token && granted("publish:read") ? await Promise.all([
          apiFetch<ApiEnvelope<PublishPreview[]>>(`/api/v1/admin/publish/previews${search}`, {}, token),
          apiFetch<ApiEnvelope<PublishJob[]>>(`/api/v1/admin/publish/jobs${search}`, {}, token),
          apiFetch<ApiEnvelope<PublishRelease[]>>(`/api/v1/admin/publish/releases${search}`, {}, token)
        ]) : [await empty<PublishPreview>(), await empty<PublishJob>(), await empty<PublishRelease>()];
        setPublishPreviews(previewResult.data); setPublishJobs(jobResult.data); setPublishReleases(releaseResult.data);
        setPreviewPagination(paginationFrom(previewResult)); setJobPagination(paginationFrom(jobResult)); setReleasePagination(paginationFrom(releaseResult));
      } else if (section === "users") {
        const [userResult, roleResult, permissionResult] = await Promise.all([
          token && granted("user:read") ? apiFetch<ApiEnvelope<AdminUser[]>>(`/api/v1/admin/users${search}`, {}, token) : empty<AdminUser>(),
          token && granted("role:read") ? apiFetch<ApiEnvelope<SystemRole[]>>("/api/v1/admin/roles", {}, token) : empty<SystemRole>(),
          token && granted("role:read") ? apiFetch<ApiEnvelope<string[]>>("/api/v1/admin/permissions", {}, token) : empty<string>()
        ]);
        setAdminUsers(userResult.data); setSystemRoles(roleResult.data); setPermissions(permissionResult.data); setUserPagination(paginationFrom(userResult));
      } else if (section === "settings") {
        const settingsResult = await apiFetch<ApiEnvelope<SiteSettingsResponse>>(settingsPath, {}, token || undefined);
        setSiteSettings(settingsResult.data.settings); setSiteSettingsHash(settingsResult.data.hash);
        if (settingsFormConnected) settingsForm.setFieldsValue(settingsResult.data.settings);
      } else if (section === "audit") {
        const result = token && granted("audit:read") ? await apiFetch<ApiEnvelope<AuditLog[]>>(`/api/v1/admin/audit-logs${search}`, {}, token) : await empty<AuditLog>();
        setAuditLogs(result.data); setAuditPagination(paginationFrom(result));
      }
    } catch (err) {
      if (section === "dashboard") { setPosts([]); setPages([]); }
      if (section === "posts") { setPosts([]); setCategories([]); setTags([]); setSeries([]); setMedia([]); }
      if (section === "pages") setPages([]);
      if (section === "taxonomy") { setCategories([]); setTags([]); setSeries([]); setMedia([]); }
      if (section === "media") setMedia([]);
      if (section === "comments") setComments([]);
      if (section === "publishing") { setPublishPreviews([]); setPublishJobs([]); setPublishReleases([]); }
      if (section === "users") { setAdminUsers([]); setSystemRoles([]); setPermissions([]); }
      if (section === "settings") { setSiteSettings(null); setSiteSettingsHash(""); }
      if (section === "audit") setAuditLogs([]);
      setError(String(err));
      if (err instanceof ApiError && err.status === 401) {
        setCurrentUser(null);
        onUnauthorized();
      }
    }
  }, [editorID, location.search, onUnauthorized, section, settingsForm, settingsFormConnected, setCurrentUser, token]);

  useEffect(() => { void refresh(); }, [refresh]);

  return { health, ready, posts, pages, categories, tags, series, media, setMedia, comments, setComments, publishPreviews, publishJobs, publishReleases, auditLogs, adminUsers, systemRoles, permissions, siteSettings, setSiteSettings, siteSettingsHash, setSiteSettingsHash, postPagination, pagePagination, mediaPagination, commentPagination, previewPagination, jobPagination, releasePagination, userPagination, auditPagination, error, setError, refresh };
}
