import { Message } from "@arco-design/web-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { ApiError, apiBase, apiFetch } from "../api/client";
import type { Achievement, AchievementFormValues, ApiEnvelope, MediaAsset, PaginationMeta } from "../types/admin";

const emptyPagination: PaginationMeta = { page: 1, page_size: 20, total: 0, total_pages: 0 };

export type AchievementAdminPermissions = {
  read?: boolean;
  create?: boolean;
  update?: boolean;
  delete?: boolean;
  publish?: boolean;
  mediaRead?: boolean;
};

type MediaSearchResult = {
  data: MediaAsset[];
  pagination: PaginationMeta;
};

export type UseAchievementAdminOptions = AchievementAdminPermissions & {
  token: string;
  editorID?: string;
  onUnauthorized?: () => void;
};

function listParams(searchParams: URLSearchParams) {
  const params = new URLSearchParams();
  for (const key of ["page", "page_size", "year"]) {
    const value = searchParams.get(key);
    if (value) params.set(key, value);
  }
  return params;
}

function paginationFrom<T>(result: ApiEnvelope<T[]>, pageSize = 20): PaginationMeta {
  if (result.pagination) return result.pagination;
  const total = result.data.length;
  return { page: 1, page_size: pageSize, total, total_pages: total ? Math.ceil(total / pageSize) : 0 };
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export function useAchievementAdmin(options: UseAchievementAdminOptions) {
  const { token, editorID, onUnauthorized } = options;
  const canRead = options.read === true;
  const canCreate = options.create === true;
  const canUpdate = options.update === true;
  const canDelete = options.delete === true;
  const canPublish = options.publish === true;
  const canReadMedia = options.mediaRead === true;
  const [searchParams, setSearchParams] = useSearchParams();
  const query = useMemo(() => {
    const page = Math.max(1, Number(searchParams.get("page")) || 1);
    const pageSize = Math.min(100, Math.max(1, Number(searchParams.get("page_size")) || 20));
    return { page, pageSize, year: searchParams.get("year") || "" };
  }, [searchParams]);
  const [achievements, setAchievements] = useState<Achievement[]>([]);
  const [achievement, setAchievement] = useState<Achievement | null>(null);
  const [pagination, setPagination] = useState<PaginationMeta>(emptyPagination);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [deletingID, setDeletingID] = useState("");
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState("");

  const reportError = useCallback((error: unknown) => {
    const message = errorMessage(error);
    setError(message);
    Message.error(message);
    if (error instanceof ApiError && error.status === 401) onUnauthorized?.();
  }, [onUnauthorized]);

  const refresh = useCallback(async () => {
    if (!token || !canRead) {
      setAchievements([]);
      setAchievement(null);
      setPagination(emptyPagination);
      return;
    }
    setLoading(true);
    setError("");
    try {
      if (editorID && editorID !== "new") {
        const result = await apiFetch<ApiEnvelope<Achievement>>(`/api/v1/admin/achievements/${editorID}`, {}, token);
        setAchievement(result.data);
        setAchievements([result.data]);
        setPagination({ page: 1, page_size: 1, total: 1, total_pages: 1 });
      } else if (!editorID) {
        const params = listParams(searchParams);
        const queryString = params.toString();
        const result = await apiFetch<ApiEnvelope<Achievement[]>>(`/api/v1/admin/achievements${queryString ? `?${queryString}` : ""}`, {}, token);
        setAchievements(result.data);
        setAchievement(null);
        setPagination(paginationFrom(result, query.pageSize));
      } else {
        setAchievements([]);
        setAchievement(null);
        setPagination(emptyPagination);
      }
    } catch (error) {
      setAchievements([]);
      setAchievement(null);
      setPagination(emptyPagination);
      reportError(error);
    } finally {
      setLoading(false);
    }
  }, [canRead, editorID, query.pageSize, reportError, searchParams, token]);

  useEffect(() => { void refresh(); }, [refresh]);

  const updateQuery = useCallback((patch: { page?: number; pageSize?: number; year?: string }) => {
    const next = new URLSearchParams(searchParams);
    const page = patch.page ?? (patch.year !== undefined || patch.pageSize !== undefined ? 1 : query.page);
    if (page <= 1) next.delete("page"); else next.set("page", String(page));
    if (patch.pageSize !== undefined) {
      if (patch.pageSize === 20) next.delete("page_size"); else next.set("page_size", String(patch.pageSize));
    }
    if (patch.year !== undefined) {
      if (patch.year) next.set("year", patch.year); else next.delete("year");
    }
    setSearchParams(next);
  }, [query.page, searchParams, setSearchParams]);

  const save = useCallback(async (values: AchievementFormValues, id?: string) => {
    const targetID = id && id !== "new" ? id : (editorID && editorID !== "new" ? editorID : undefined);
    const allowed = targetID ? canUpdate : canCreate;
    if (!token || !allowed) {
      Message.warning("当前账号无权保存成果");
      return null;
    }
    setBusy(true);
    try {
      const payload = {
        kind: values.kind,
        title: values.title,
        organization: values.organization,
        summary: values.summary || "",
        occurred_at: values.occurred_at,
        ended_at: values.ended_at || null,
        external_url: values.external_url || "",
        credential_id: values.credential_id || "",
        image_media_id: values.image_media_id || null,
        sort_order: values.sort_order ?? 0,
        status: values.status || "draft"
      };
      const result = await apiFetch<ApiEnvelope<Achievement>>(
        targetID ? `/api/v1/admin/achievements/${targetID}` : "/api/v1/admin/achievements",
        { method: targetID ? "PATCH" : "POST", body: JSON.stringify(payload) },
        token
      );
      setAchievement(result.data);
      Message.success(targetID ? "成果已保存" : "成果已创建");
      await refresh();
      return result.data;
    } catch (error) {
      reportError(error);
      return null;
    } finally {
      setBusy(false);
    }
  }, [canCreate, canUpdate, editorID, refresh, reportError, token]);

  const updateStatus = useCallback(async (id: string, status: string) => {
    if (!token || !canPublish) { Message.warning("当前账号无权修改成果状态"); return false; }
    setBusy(true);
    try {
      await apiFetch<ApiEnvelope<Achievement>>(`/api/v1/admin/achievements/${id}/status`, { method: "PATCH", body: JSON.stringify({ status }) }, token);
      Message.success("成果状态已更新");
      await refresh();
      return true;
    } catch (error) { reportError(error); return false; } finally { setBusy(false); }
  }, [canPublish, refresh, reportError, token]);

  const remove = useCallback(async (id: string) => {
    if (!token || !canDelete) { Message.warning("当前账号无权删除成果"); return false; }
    setDeletingID(id);
    try {
      await apiFetch(`/api/v1/admin/achievements/${id}`, { method: "DELETE" }, token);
      Message.success("成果已删除");
      await refresh();
      return true;
    } catch (error) { reportError(error); return false; } finally { setDeletingID(""); }
  }, [canDelete, refresh, reportError, token]);

  const publish = useCallback(async () => {
    if (!token || !canPublish) { Message.warning("当前账号无权发布成果"); return false; }
    setPublishing(true);
    try {
      await apiFetch("/api/v1/admin/achievements/publish", { method: "POST" }, token);
      Message.success("成果发布任务已提交");
      await refresh();
      return true;
    } catch (error) { reportError(error); return false; } finally { setPublishing(false); }
  }, [canPublish, refresh, reportError, token]);

  const searchMedia = useCallback(async (q: string, page: number, pageSize = 12): Promise<MediaSearchResult> => {
    if (!token || !canReadMedia) return { data: [], pagination: emptyPagination };
    const params = new URLSearchParams({ page: String(page), page_size: String(pageSize), status: "ready" });
    if (q.trim()) params.set("q", q.trim());
    try {
      const result = await apiFetch<ApiEnvelope<MediaAsset[]>>(`/api/v1/admin/media?${params.toString()}`, {}, token);
      return {
        data: result.data.filter((asset) => asset.mime_type?.startsWith("image/")),
        pagination: paginationFrom(result, pageSize)
      };
    } catch (error) { reportError(error); return { data: [], pagination: emptyPagination }; }
  }, [canReadMedia, reportError, token]);

  const mediaURL = useCallback((asset: MediaAsset) => asset.public_url?.startsWith("http") ? asset.public_url : `${apiBase}${asset.public_url || ""}`, []);

  return {
    achievements,
    achievement,
    pagination,
    loading,
    busy,
    deletingID,
    publishing,
    error,
    query,
    refresh,
    updateQuery,
    save,
    updateStatus,
    remove,
    publish,
    searchMedia,
    mediaURL
  };
}
