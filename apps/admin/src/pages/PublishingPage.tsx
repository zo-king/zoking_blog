import { Button, Drawer, Popconfirm, Space, Table, Tabs, Tag, Typography, type TableColumnProps } from "@arco-design/web-react";
import { IconDelete, IconLaunch, IconRefresh, IconStop, IconSwap } from "@arco-design/web-react/icon";
import { useState } from "react";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import { useListQuery } from "../hooks/useListQuery";
import { displayLabel } from "../labels";
import type { PaginationMeta, PublishJob, PublishPreview, PublishRelease } from "../types/admin";

const { Text } = Typography;

const statusColors: Record<string, string> = {
  requested: "arcoblue",
  queued: "purple",
  snapshotting: "cyan",
  building: "orange",
  verifying: "magenta",
  promoting: "gold",
  published: "green",
  ready: "green",
  canceled: "gray",
  failed: "red"
};

function targetLabel(record: PublishJob | PublishRelease | PublishPreview) {
  if (record.post?.title) return record.post.title;
  if (record.page?.title) return record.page.title;
  if (record.post_id) return record.post_id;
  if (record.page_id) return record.page_id;
  return "站点设置";
}

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : "-";
}

function ErrorCell({ record }: { record: { status: string; error_code?: string | null; error_message?: string | null } }) {
  if (record.status !== "failed") return <>-</>;

  return (
    <div className="publish-error-cell">
      <Text type="error">{record.error_code || "失败"}</Text>
      {record.error_message ? <Text type="secondary" ellipsis={{ rows: 2, showTooltip: true }}>{record.error_message}</Text> : null}
    </div>
  );
}

type Props = {
  jobs: PublishJob[];
  releases: PublishRelease[];
  previews: PublishPreview[];
  jobPagination: PaginationMeta;
  releasePagination: PaginationMeta;
  previewPagination: PaginationMeta;
  canManageJobs: boolean;
  canPromote: boolean;
  canCleanup: boolean;
  releaseBusy: string;
  releaseCleanupBusy: boolean;
  previewCleanupBusy: boolean;
  onRetryJob: (id: string) => void;
  onCancelJob: (id: string) => void;
  onPromoteRelease: (id: string) => void;
  onReleaseCleanup: (dryRun: boolean) => void;
  onPreviewCleanup: (dryRun: boolean) => void;
  onOpenPreview: (url: string) => void;
};

export function PublishingPage(props: Props) {
  const [selectedJob, setSelectedJob] = useState<PublishJob | null>(null);
  const [activeTab, setActiveTab] = useState("jobs");
  const listQuery = useListQuery(20);

  const paginationFor = (pagination: PaginationMeta) => ({
    current: listQuery.page,
    pageSize: listQuery.pageSize,
    total: pagination.total,
    hideOnSinglePage: true,
    size: "small" as const,
    showTotal: true,
    onChange: (page: number, pageSize: number) => listQuery.update({ page, pageSize })
  });

  const jobColumns: TableColumnProps<PublishJob>[] = [
    { title: "目标", width: 220, render: (_, record) => targetLabel(record) },
    { title: "类型", dataIndex: "job_type", width: 100, render: (value) => <Tag>{displayLabel(value || "post")}</Tag> },
    { title: "状态", dataIndex: "status", width: 110, render: (value) => <Tag color={statusColors[value] || "gold"}>{displayLabel(value)}</Tag> },
    { title: "版本", dataIndex: "release_key", width: 180, ellipsis: true, render: (value) => value || "-" },
    { title: "错误", width: 160, render: (_, record) => record.status === "failed" ? <Text type="error">{record.error_code || "失败"}</Text> : "-" },
    { title: "创建时间", dataIndex: "created_at", width: 180, render: (value) => formatDate(value) },
    {
      title: "操作",
      width: 240,
      fixed: "right",
      render: (_, record) => (
        <Space size="mini">
          <Button size="mini" onClick={() => setSelectedJob(record)}>详情</Button>
          {props.canManageJobs && <Button
            size="mini"
            icon={<IconRefresh />}
            disabled={record.status !== "failed" && record.status !== "canceled"}
            onClick={() => props.onRetryJob(record.id)}
          >
            重试
          </Button>}
          {props.canManageJobs && <Button
            size="mini"
            status="danger"
            icon={<IconStop />}
            disabled={record.status !== "requested" && record.status !== "queued"}
            onClick={() => props.onCancelJob(record.id)}
          >
            取消
          </Button>}
        </Space>
      )
    }
  ];

  const releaseColumns: TableColumnProps<PublishRelease>[] = [
    { title: "版本", dataIndex: "release_key", width: 180, ellipsis: true },
    { title: "状态", dataIndex: "status", width: 100, render: (value) => <Tag color={value === "active" ? "green" : "gray"}>{displayLabel(value || "inactive")}</Tag> },
    { title: "当前版本", dataIndex: "is_active", width: 100, render: (value) => <Tag color={value ? "green" : "gray"}>{value ? "是" : "否"}</Tag> },
    { title: "目标", width: 220, render: (_, record) => targetLabel(record) },
    { title: "创建时间", dataIndex: "created_at", width: 180, render: (value) => formatDate(value) },
    { title: "启用时间", dataIndex: "promoted_at", width: 180, render: (value) => formatDate(value) },
    {
      title: "操作",
      width: 120,
      fixed: "right",
      render: (_, record) => record.is_active ? (
        <Text type="secondary">当前版本</Text>
      ) : !props.canPromote ? (
        <Text type="secondary">只读</Text>
      ) : (
        <Popconfirm title="确认切换到此版本？" okText="切换" onOk={() => props.onPromoteRelease(record.id)}>
          <Button size="mini" icon={<IconSwap />} loading={props.releaseBusy === record.id}>切换</Button>
        </Popconfirm>
      )
    }
  ];

  const previewColumns: TableColumnProps<PublishPreview>[] = [
    { title: "目标", width: 220, render: (_, record) => targetLabel(record) },
    { title: "类型", dataIndex: "scope", width: 100, render: (value) => <Tag>{displayLabel(value)}</Tag> },
    { title: "状态", dataIndex: "status", width: 110, render: (value) => <Tag color={statusColors[value] || "gold"}>{displayLabel(value)}</Tag> },
    { title: "预览标识", dataIndex: "preview_key", width: 220, ellipsis: true },
    { title: "错误", width: 260, render: (_, record) => <ErrorCell record={record} /> },
    { title: "过期时间", dataIndex: "expires_at", width: 180, render: (value) => formatDate(value) },
    {
      title: "操作",
      width: 120,
      fixed: "right",
      render: (_, record) => (
        <Button
          size="mini"
          icon={<IconLaunch />}
          disabled={record.status !== "ready" || !record.target_url}
          onClick={() => props.onOpenPreview(record.target_url)}
        >
          打开
        </Button>
      )
    }
  ];

  return (
    <>
      <PageHeader
        title="发布中心"
        description="查看任务、正式版本和临时预览。"
      />

      <ContentPanel className="tabbed-workbench">
        <Tabs
          activeTab={activeTab}
          onChange={(tab) => {
            setActiveTab(tab);
            if (listQuery.page !== 1) listQuery.update({ page: 1 }, true);
          }}
        >
          <Tabs.TabPane key="jobs" title={`发布任务 (${props.jobPagination.total})`}>
            <Table
              rowKey="id"
              data={props.jobs}
              columns={jobColumns}
              pagination={paginationFor(props.jobPagination)}
              size="small"
              scroll={{ x: 1190 }}
            />
          </Tabs.TabPane>

          <Tabs.TabPane key="releases" title={`正式版本 (${props.releasePagination.total})`}>
            {props.canCleanup && <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 12 }}>
              <Space wrap size="small">
                <Button
                  size="small"
                  icon={<IconRefresh />}
                  disabled={!props.canCleanup}
                  loading={props.releaseCleanupBusy}
                  onClick={() => props.onReleaseCleanup(true)}
                >
                  清理预演
                </Button>
                <Popconfirm
                  title="确认清理旧的非活动版本？"
                  content="当前活动版本不会被删除。"
                  okText="清理"
                  okButtonProps={{ status: "danger" }}
                  disabled={!props.canCleanup}
                  onOk={() => props.onReleaseCleanup(false)}
                >
                  <Button
                    size="small"
                    status="danger"
                    icon={<IconDelete />}
                    disabled={!props.canCleanup}
                    loading={props.releaseCleanupBusy}
                  >
                    清理旧版本
                  </Button>
                </Popconfirm>
              </Space>
            </div>}
            <Table
              rowKey="id"
              data={props.releases}
              columns={releaseColumns}
              pagination={paginationFor(props.releasePagination)}
              size="small"
              scroll={{ x: 1080 }}
            />
          </Tabs.TabPane>

          <Tabs.TabPane key="previews" title={`预览构建 (${props.previewPagination.total})`}>
            {props.canCleanup && <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 12 }}>
              <Space wrap size="small">
                <Button
                  size="small"
                  icon={<IconRefresh />}
                  disabled={!props.canCleanup}
                  loading={props.previewCleanupBusy}
                  onClick={() => props.onPreviewCleanup(true)}
                >
                  清理预演
                </Button>
                <Popconfirm
                  title="确认删除所有过期预览目录？"
                  okText="执行清理"
                  okButtonProps={{ status: "danger" }}
                  disabled={!props.canCleanup}
                  onOk={() => props.onPreviewCleanup(false)}
                >
                  <Button
                    size="small"
                    status="danger"
                    icon={<IconDelete />}
                    disabled={!props.canCleanup}
                    loading={props.previewCleanupBusy}
                  >
                    执行清理
                  </Button>
                </Popconfirm>
              </Space>
            </div>}
            <Table
              rowKey="id"
              data={props.previews}
              columns={previewColumns}
              pagination={paginationFor(props.previewPagination)}
              size="small"
              scroll={{ x: 1210 }}
            />
          </Tabs.TabPane>
        </Tabs>
      </ContentPanel>

      <Drawer
        title="发布任务详情"
        width={560}
        visible={Boolean(selectedJob)}
        footer={null}
        onCancel={() => setSelectedJob(null)}
      >
        {selectedJob ? (
          <div style={{ display: "grid", gap: 20 }}>
            <div style={{ display: "grid", gridTemplateColumns: "96px minmax(0, 1fr)", gap: "10px 16px" }}>
              <Text type="secondary">目标</Text><Text>{targetLabel(selectedJob)}</Text>
              <Text type="secondary">任务类型</Text><Text>{displayLabel(selectedJob.job_type || "post")}</Text>
              <Text type="secondary">状态</Text><Tag color={statusColors[selectedJob.status] || "gold"}>{displayLabel(selectedJob.status)}</Tag>
              <Text type="secondary">版本</Text><Text copyable={Boolean(selectedJob.release_key)}>{selectedJob.release_key || "-"}</Text>
              <Text type="secondary">重试次数</Text><Text>{selectedJob.retry_count || 0}</Text>
              <Text type="secondary">创建时间</Text><Text>{formatDate(selectedJob.created_at)}</Text>
              <Text type="secondary">完成时间</Text><Text>{formatDate(selectedJob.finished_at)}</Text>
              <Text type="secondary">取消时间</Text><Text>{formatDate(selectedJob.canceled_at)}</Text>
              <Text type="secondary">内容路径</Text><Text copyable={Boolean(selectedJob.content_path)}>{selectedJob.content_path || "-"}</Text>
              <Text type="secondary">输出路径</Text><Text copyable={Boolean(selectedJob.output_path)}>{selectedJob.output_path || "-"}</Text>
            </div>

            <div>
              <Typography.Title heading={6}>错误详情</Typography.Title>
              {selectedJob.error_code || selectedJob.error_message ? (
                <div style={{ display: "grid", gap: 8 }}>
                  <Text type="error">{selectedJob.error_code || "失败"}</Text>
                  <Text style={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>{selectedJob.error_message || "-"}</Text>
                </div>
              ) : <Text type="secondary">无错误信息</Text>}
            </div>

            <div>
              <Typography.Title heading={6}>最近日志</Typography.Title>
              {Array.isArray(selectedJob.log_json) && selectedJob.log_json.length > 0 ? (
                <div style={{ display: "grid", gap: 10 }}>
                  {selectedJob.log_json.slice(-10).reverse().map((entry, index) => (
                    <div key={`${selectedJob.id}-${index}`} style={{ borderBottom: "1px solid var(--color-neutral-3)", paddingBottom: 10 }}>
                      <Space size="mini" wrap>
                        <Tag size="small">{entry.stage || "阶段"}</Tag>
                        <Text type="secondary">{entry.level || "信息"}</Text>
                        <Text type="secondary">{formatDate(entry.at)}</Text>
                      </Space>
                      <div style={{ marginTop: 6, whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>{entry.message || "-"}</div>
                      {entry.fields && Object.keys(entry.fields).length > 0 ? (
                        <Text type="secondary" style={{ display: "block", marginTop: 6, whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>
                          {JSON.stringify(entry.fields, null, 2)}
                        </Text>
                      ) : null}
                    </div>
                  ))}
                </div>
              ) : <Text type="secondary">暂无日志</Text>}
            </div>
          </div>
        ) : null}
      </Drawer>
    </>
  );
}
