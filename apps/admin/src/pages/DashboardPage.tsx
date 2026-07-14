import { Button, Statistic, Tag, Typography } from "@arco-design/web-react";
import { IconArrowRight, IconCheckCircleFill, IconExclamationCircleFill, IconFile, IconStorage } from "@arco-design/web-react/icon";
import { useNavigate } from "react-router-dom";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import type { ApiStatus } from "../types/admin";

const { Text } = Typography;

function StatusTag({ status }: { status: ApiStatus }) {
  if (status === "checking") return <Tag>检查中</Tag>;
  if (status === "ok") return <Tag color="green" icon={<IconCheckCircleFill />}>正常</Tag>;
  return <Tag color="red" icon={<IconExclamationCircleFill />}>异常</Tag>;
}

export function DashboardPage({ health, ready, postCount, pageCount }: { health: ApiStatus; ready: ApiStatus; postCount: number; pageCount: number }) {
  const navigate = useNavigate();

  return (
    <>
      <PageHeader title="工作台" description="查看内容规模、服务状态和常用管理入口。" eyebrow="今日概览" actions={<Button type="primary" icon={<IconFile />} onClick={() => navigate("/posts")}>管理文章</Button>} />
      <div className="metric-grid">
        <div className="metric-tile"><span className="metric-icon metric-icon-green"><IconFile /></span><Statistic title="文章总数" value={postCount} /><Button type="text" icon={<IconArrowRight />} aria-label="前往文章管理" onClick={() => navigate("/posts")} /></div>
        <div className="metric-tile"><span className="metric-icon metric-icon-coral"><IconStorage /></span><Statistic title="独立页面" value={pageCount} /><Button type="text" icon={<IconArrowRight />} aria-label="前往页面管理" onClick={() => navigate("/pages")} /></div>
      </div>
      <ContentPanel title="系统运行状态" description="后台 API 与数据库连接的实时探测结果。">
        <div className="health-list">
          <div><span><strong>应用接口</strong><Text type="secondary">负责内容、媒体和发布管理</Text></span><StatusTag status={health} /></div>
          <div><span><strong>PostgreSQL 数据库</strong><Text type="secondary">持久化内容与系统配置</Text></span><StatusTag status={ready} /></div>
        </div>
      </ContentPanel>
    </>
  );
}
