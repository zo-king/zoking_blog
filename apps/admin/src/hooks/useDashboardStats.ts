import { useCallback, useEffect, useState } from "react";
import { apiFetch } from "../api/client";

export type StatusCount = { status: string; count: number };
export type DayViews = { day: string; views: number };
export type TopPost = { id: string; title: string; views: number; likes: number };

export type DayCount = { day: string; count: number };

export type StatsOverview = {
  posts: StatusCount[];
  pages: StatusCount[];
  comments: StatusCount[];
  publish_jobs: StatusCount[];
  media_count: number;
  achievement_count: number;
  total_views: number;
  total_likes: number;
  views_by_day: DayViews[];
  likes_by_day: DayCount[];
  top_posts: TopPost[];
};

export const sumCounts = (rows: StatusCount[]): number => rows.reduce((acc, row) => acc + row.count, 0);

export const countOf = (rows: StatusCount[], status: string): number =>
  rows.find((row) => row.status === status)?.count ?? 0;

/** 拉取工作台概览统计;失败静默(仪表盘各卡片自行展示占位)。 */
export function useDashboardStats(enabled: boolean) {
  const [stats, setStats] = useState<StatsOverview | null>(null);
  const [loading, setLoading] = useState(false);

  const reload = useCallback(async () => {
    if (!enabled) return;
    setLoading(true);
    try {
      const payload = await apiFetch<{ data: StatsOverview }>("/api/v1/admin/stats/overview");
      setStats(payload.data);
    } catch {
      // 统计不可用不阻塞工作台其它内容
    } finally {
      setLoading(false);
    }
  }, [enabled]);

  useEffect(() => {
    void reload();
  }, [reload]);

  return { stats, loading, reload };
}
