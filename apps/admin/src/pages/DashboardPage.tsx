import { Button, Empty, Skeleton, Statistic, Tag, Typography } from "@arco-design/web-react";
import {
  IconCheckCircleFill,
  IconEye,
  IconExclamationCircleFill,
  IconFile,
  IconHeart,
  IconMessage,
  IconRight,
} from "@arco-design/web-react/icon";
import { useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import { CHART_LINE, CHART_TEXT, EChart, type EChartOption } from "../components/charts/EChart";
import { countOf, sumCounts, useDashboardStats, type StatsOverview } from "../hooks/useDashboardStats";
import { displayLabel } from "../labels";
import type { ApiStatus } from "../types/admin";

const { Text } = Typography;

/** 状态计数渲染为一串「文案 数量」标签,给评论/发布任务用。 */
function StatusBreakdown({ rows, highlight }: { rows: { status: string; count: number }[]; highlight?: string }) {
  if (!rows.length) return <Text type="secondary">暂无数据</Text>;
  return (
    <span className="status-breakdown">
      {rows.map((row) => (
        <span
          key={row.status}
          className={`status-chip${row.status === highlight && row.count > 0 ? " is-highlight" : ""}`}
        >
          {displayLabel(row.status)} <strong>{row.count}</strong>
        </span>
      ))}
    </span>
  );
}

function StatusTag({ status }: { status: ApiStatus }) {
  if (status === "checking") return <Tag>检查中</Tag>;
  if (status === "ok")
    return (
      <Tag color="green" icon={<IconCheckCircleFill />}>
        正常
      </Tag>
    );
  return (
    <Tag color="red" icon={<IconExclamationCircleFill />}>
      异常
    </Tag>
  );
}

/** 近 14 天浏览/点赞:后端只返回有数据的日期,这里按天补零成连续序列。 */
function buildTrend(stats: StatsOverview | null): { days: string[]; views: number[]; likes: number[] } {
  const viewsByDay = new Map((stats?.views_by_day ?? []).map((row) => [row.day, row.views]));
  const likesByDay = new Map((stats?.likes_by_day ?? []).map((row) => [row.day, row.count]));
  const days: string[] = [];
  const views: number[] = [];
  const likes: number[] = [];
  const now = new Date();
  for (let offset = 13; offset >= 0; offset--) {
    const date = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() - offset));
    const key = date.toISOString().slice(0, 10);
    days.push(key.slice(5).replace("-", "/"));
    views.push(viewsByDay.get(key) ?? 0);
    likes.push(likesByDay.get(key) ?? 0);
  }
  return { days, views, likes };
}

export function DashboardPage({ health, ready }: { health: ApiStatus; ready: ApiStatus }) {
  const navigate = useNavigate();
  const { stats, loading } = useDashboardStats(true);

  const trend = useMemo(() => buildTrend(stats), [stats]);

  const trendOption = useMemo<EChartOption>(
    () => ({
      grid: { top: 34, right: 16, bottom: 28, left: 40 },
      tooltip: { trigger: "axis" },
      legend: { top: 0, right: 0, icon: "circle", textStyle: { color: CHART_TEXT } },
      xAxis: {
        type: "category",
        data: trend.days,
        boundaryGap: false,
        axisLine: { lineStyle: { color: CHART_LINE } },
        axisLabel: { color: CHART_TEXT },
      },
      yAxis: {
        type: "value",
        minInterval: 1,
        splitLine: { lineStyle: { color: CHART_LINE, type: "dashed" } },
        axisLabel: { color: CHART_TEXT },
      },
      series: [
        {
          name: "浏览量",
          type: "line",
          smooth: true,
          symbolSize: 5,
          data: trend.views,
          lineStyle: { width: 2.5 },
          areaStyle: { opacity: 0.12 },
        },
        {
          name: "点赞",
          type: "line",
          smooth: true,
          symbolSize: 5,
          data: trend.likes,
          lineStyle: { width: 2.5 },
          areaStyle: { opacity: 0.08 },
        },
      ],
    }),
    [trend],
  );

  const postDonutOption = useMemo<EChartOption>(
    () => ({
      tooltip: { trigger: "item" },
      legend: { bottom: 0, icon: "circle", textStyle: { color: CHART_TEXT } },
      series: [
        {
          type: "pie",
          radius: ["52%", "74%"],
          center: ["50%", "44%"],
          avoidLabelOverlap: true,
          itemStyle: { borderColor: "#ffffff", borderWidth: 2 },
          label: { show: false },
          data: (stats?.posts ?? []).map((row) => ({
            name: displayLabel(row.status),
            value: row.count,
          })),
        },
      ],
    }),
    [stats],
  );

  const topPostsOption = useMemo<EChartOption>(() => {
    const rows = [...(stats?.top_posts ?? [])].reverse();
    return {
      grid: { top: 8, right: 40, bottom: 8, left: 8, containLabel: true },
      tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
      xAxis: {
        type: "value",
        minInterval: 1,
        splitLine: { lineStyle: { color: CHART_LINE, type: "dashed" } },
        axisLabel: { color: CHART_TEXT },
      },
      yAxis: {
        type: "category",
        data: rows.map((row) => (row.title.length > 14 ? `${row.title.slice(0, 14)}…` : row.title)),
        axisLine: { lineStyle: { color: CHART_LINE } },
        axisLabel: { color: CHART_TEXT },
      },
      series: [
        {
          name: "浏览量",
          type: "bar",
          barWidth: 14,
          itemStyle: { borderRadius: [0, 7, 7, 0] },
          data: rows.map((row) => row.views),
        },
      ],
    };
  }, [stats]);

  const postTotal = sumCounts(stats?.posts ?? []);
  const publishedPosts = countOf(stats?.posts ?? [], "published");
  const draftPosts = countOf(stats?.posts ?? [], "draft");
  const pendingComments = countOf(stats?.comments ?? [], "pending");
  const commentTotal = sumCounts(stats?.comments ?? []);
  const hasTopPosts = (stats?.top_posts ?? []).some((row) => row.views > 0 || row.likes > 0);

  return (
    <>
      <PageHeader
        title="工作台"
        description="站点数据总览、内容规模与服务状态。"
        actions={
          <Button type="primary" icon={<IconFile />} onClick={() => navigate("/posts")}>
            管理文章
          </Button>
        }
      />

      <div className="stat-card-grid">
        <div className="stat-card">
          <span className="stat-card-icon stat-icon-blue">
            <IconEye />
          </span>
          <div className="stat-card-body">
            <Statistic title="总浏览量" value={stats?.total_views ?? 0} groupSeparator loading={loading} />
            <Text type="secondary">全站文章累计</Text>
          </div>
        </div>
        <div className="stat-card">
          <span className="stat-card-icon stat-icon-red">
            <IconHeart />
          </span>
          <div className="stat-card-body">
            <Statistic title="总点赞" value={stats?.total_likes ?? 0} groupSeparator loading={loading} />
            <Text type="secondary">读者互动</Text>
          </div>
        </div>
        <div className="stat-card stat-card-link" onClick={() => navigate("/posts")} role="button" tabIndex={0}>
          <span className="stat-card-icon stat-icon-cyan">
            <IconFile />
          </span>
          <div className="stat-card-body">
            <Statistic title="文章" value={postTotal} loading={loading} />
            <Text type="secondary">
              已发布 {publishedPosts} · 草稿 {draftPosts}
            </Text>
          </div>
          <IconRight className="stat-card-arrow" />
        </div>
        <div className="stat-card stat-card-link" onClick={() => navigate("/comments")} role="button" tabIndex={0}>
          <span className="stat-card-icon stat-icon-orange">
            <IconMessage />
          </span>
          <div className="stat-card-body">
            <Statistic title="待审评论" value={pendingComments} loading={loading} />
            <Text type="secondary">共 {commentTotal} 条评论</Text>
          </div>
          <IconRight className="stat-card-arrow" />
        </div>
      </div>

      <div className="dashboard-grid">
        <ContentPanel
          className="dashboard-main"
          title="浏览趋势"
          description="近 14 天全站文章浏览量(按访客每日去重)。"
        >
          {loading && !stats ? <Skeleton animation text={{ rows: 5 }} /> : <EChart option={trendOption} height={280} />}
        </ContentPanel>
        <ContentPanel className="dashboard-side" title="文章状态" description="按发布状态分布。">
          {loading && !stats ? (
            <Skeleton animation text={{ rows: 5 }} />
          ) : postTotal > 0 ? (
            <EChart option={postDonutOption} height={280} />
          ) : (
            <Empty description="暂无文章" />
          )}
        </ContentPanel>
        <ContentPanel className="dashboard-main" title="热门文章 Top 5" description="按累计浏览量排序。">
          {loading && !stats ? (
            <Skeleton animation text={{ rows: 5 }} />
          ) : hasTopPosts ? (
            <EChart option={topPostsOption} height={240} />
          ) : (
            <Empty description="暂无浏览数据" />
          )}
        </ContentPanel>
        <ContentPanel className="dashboard-side" title="系统与资产" description="服务探测与内容资产规模。">
          <div className="health-list">
            <div>
              <span>
                <strong>应用接口</strong>
                <Text type="secondary">内容、媒体与发布管理</Text>
              </span>
              <StatusTag status={health} />
            </div>
            <div>
              <span>
                <strong>PostgreSQL 数据库</strong>
                <Text type="secondary">持久化内容与系统配置</Text>
              </span>
              <StatusTag status={ready} />
            </div>
            <div>
              <span>
                <strong>独立页面</strong>
                <Text type="secondary">关于、友链等站点页面</Text>
              </span>
              <strong className="asset-count">{sumCounts(stats?.pages ?? [])}</strong>
            </div>
            <div>
              <span>
                <strong>成果</strong>
                <Text type="secondary">时间线与项目成果</Text>
              </span>
              <strong className="asset-count">{stats?.achievement_count ?? 0}</strong>
            </div>
            <div>
              <span>
                <strong>媒体资产</strong>
                <Text type="secondary">图片与附件</Text>
              </span>
              <strong className="asset-count">{stats?.media_count ?? 0}</strong>
            </div>
            <div>
              <span>
                <strong>评论</strong>
                <Text type="secondary">按审核状态</Text>
              </span>
              <StatusBreakdown rows={stats?.comments ?? []} highlight="pending" />
            </div>
            <div>
              <span>
                <strong>发布任务</strong>
                <Text type="secondary">全部历史任务</Text>
              </span>
              <StatusBreakdown rows={stats?.publish_jobs ?? []} highlight="failed" />
            </div>
          </div>
        </ContentPanel>
      </div>
    </>
  );
}
