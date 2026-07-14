import { useState } from "react";
import { Message } from "@arco-design/web-react";
import { apiFetch } from "../api/client";
import type { ApiEnvelope, CleanupResult, PublishJob, PublishRelease } from "../types/admin";

export function usePublishingCommands({ token, refresh, setError }: { token: string; refresh: () => Promise<void>; setError: (value: string) => void }) {
  const [releaseBusy, setReleaseBusy] = useState("");
  const [releaseCleanupBusy, setReleaseCleanupBusy] = useState(false);
  const [previewCleanupBusy, setPreviewCleanupBusy] = useState(false);
  const requireLogin = () => { if (token) return true; Message.warning("请先登录"); return false; };

  async function promoteRelease(id: string) {
    if (!requireLogin()) return; setReleaseBusy(id);
    try { const result = await apiFetch<ApiEnvelope<PublishRelease>>(`/api/v1/admin/publish/releases/${id}/promote`, { method: "POST" }, token); Message.success(`已切换到版本 ${result.data.release_key}`); await refresh(); }
    catch (err) { setError(String(err)); } finally { setReleaseBusy(""); }
  }
  async function cleanup(path: "releases" | "previews", dryRun: boolean) {
    if (!requireLogin()) return;
    const setBusy = path === "releases" ? setReleaseCleanupBusy : setPreviewCleanupBusy; setBusy(true);
    try {
      const result = await apiFetch<ApiEnvelope<CleanupResult>>(`/api/v1/admin/publish/${path}/cleanup`, { method: "POST", body: JSON.stringify({ dry_run: dryRun }) }, token);
      if (dryRun) Message.info(`发现 ${result.data.candidate_count} 个可清理项目`); else { Message.success(`已清理 ${result.data.deleted_count} 个项目`); await refresh(); }
    } catch (err) { setError(String(err)); } finally { setBusy(false); }
  }
  async function updateJob(id: string, action: "retry" | "cancel") {
    if (!requireLogin()) return; setReleaseBusy(id);
    try { await apiFetch<ApiEnvelope<PublishJob>>(`/api/v1/admin/publish/jobs/${id}/${action}`, { method: "POST" }, token); Message.success(action === "retry" ? "发布任务已重新排队" : "发布任务已取消"); await refresh(); }
    catch (err) { setError(String(err)); } finally { setReleaseBusy(""); }
  }

  return { releaseBusy, releaseCleanupBusy, previewCleanupBusy, promoteRelease, runReleaseCleanup: (dryRun: boolean) => cleanup("releases", dryRun), runPreviewCleanup: (dryRun: boolean) => cleanup("previews", dryRun), retryPublishJob: (id: string) => updateJob(id, "retry"), cancelPublishJob: (id: string) => updateJob(id, "cancel") };
}
