import {
  Button,
  Input,
  Popconfirm,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  type TableColumnProps
} from "@arco-design/web-react";
import { IconCheck, IconClose, IconDelete, IconMessageBanned } from "@arco-design/web-react/icon";
import { useEffect, useState } from "react";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import { useListQuery } from "../hooks/useListQuery";
import { displayLabel } from "../labels";
import type { Comment, PaginationMeta } from "../types/admin";

const { Paragraph, Text } = Typography;
const TabPane = Tabs.TabPane;

type CommentStatusFilter = "all" | "pending" | "approved" | "rejected" | "spam";

type Props = {
  comments: Comment[];
  pagination: PaginationMeta;
  canModerate: boolean;
  busy: boolean;
  onModerate: (id: string, status: string) => void;
  onDelete: (id: string) => void;
};

function formatDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : date.toLocaleString("zh-CN", { hour12: false });
}

function commentStatusColor(status: string) {
  if (status === "approved") return "green";
  if (status === "pending") return "orange";
  if (status === "spam") return "red";
  return "gray";
}

export function CommentsPage({ comments, pagination, canModerate, busy, onModerate, onDelete }: Props) {
  const listQuery = useListQuery(20);
  const statusFilter = (listQuery.status || "all") as CommentStatusFilter;
  const [searchQuery, setSearchQuery] = useState(listQuery.q);

  useEffect(() => setSearchQuery(listQuery.q), [listQuery.q]);

  const columns: TableColumnProps<Comment>[] = [
    {
      title: "文章",
      width: 220,
      fixed: "left",
      render: (_, record) => (
        <div style={{ minWidth: 0 }}>
          <Text bold ellipsis={{ showTooltip: true }} style={{ display: "block" }}>
            {record.post?.title || record.post_id}
          </Text>
          <Text
            type="secondary"
            ellipsis={{ showTooltip: true }}
            style={{ display: "block", marginTop: 2, fontSize: 12 }}
          >
            {record.post?.slug ? `/${record.post.slug}` : record.post_id}
          </Text>
        </div>
      )
    },
    {
      title: "评论者",
      width: 180,
      render: (_, record) => (
        <div style={{ minWidth: 0 }}>
          <Text bold ellipsis={{ showTooltip: true }} style={{ display: "block" }}>
            {record.author_name}
          </Text>
          <Text
            type="secondary"
            ellipsis={{ showTooltip: true }}
            style={{ display: "block", marginTop: 2, fontSize: 12 }}
          >
            {record.author_website || "未提供站点"}
          </Text>
        </div>
      )
    },
    {
      title: "评论内容",
      dataIndex: "content",
      width: 390,
      render: (value, record) => (
        <div style={{ minWidth: 0 }}>
          <Paragraph
            ellipsis={{ rows: 3, showTooltip: true }}
            style={{ margin: 0, lineHeight: 1.6, whiteSpace: "pre-wrap" }}
          >
            {value}
          </Paragraph>
          {record.spam_reason ? (
            <Text type="error" style={{ display: "block", marginTop: 4, fontSize: 12 }}>
              垃圾判定：{record.spam_reason}
            </Text>
          ) : null}
        </div>
      )
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 100,
      render: (value) => <Tag color={commentStatusColor(value)}>{displayLabel(value)}</Tag>
    },
    {
      title: "提交时间",
      dataIndex: "created_at",
      width: 176,
      render: (value) => formatDate(value)
    },
    {
      title: "操作",
      width: 176,
      align: "center",
      fixed: "right",
      render: (_, record) => (
        <Space size={2}>
          <Tooltip content={record.status === "approved" ? "当前已通过" : "通过评论"}>
            <span style={{ display: "inline-flex" }}>
              <Button
                type="text"
                size="mini"
                status="success"
                icon={<IconCheck />}
                aria-label={`通过 ${record.author_name} 的评论`}
                disabled={busy || record.status === "approved"}
                onClick={() => onModerate(record.id, "approved")}
              />
            </span>
          </Tooltip>
          <Tooltip content={record.status === "rejected" ? "当前已拒绝" : "拒绝评论"}>
            <span style={{ display: "inline-flex" }}>
              <Button
                type="text"
                size="mini"
                status="warning"
                icon={<IconClose />}
                aria-label={`拒绝 ${record.author_name} 的评论`}
                disabled={busy || record.status === "rejected"}
                onClick={() => onModerate(record.id, "rejected")}
              />
            </span>
          </Tooltip>
          <Tooltip content={record.status === "spam" ? "当前已标记为垃圾评论" : "标记为垃圾评论"}>
            <span style={{ display: "inline-flex" }}>
              <Button
                type="text"
                size="mini"
                status="danger"
                icon={<IconMessageBanned />}
                aria-label={`将 ${record.author_name} 的评论标记为垃圾评论`}
                disabled={busy || record.status === "spam"}
                onClick={() => onModerate(record.id, "spam")}
              />
            </span>
          </Tooltip>
          <Popconfirm
            title="确认删除此评论？"
            content="删除后无法恢复，审核记录也会一并移除。"
            okText="删除"
            cancelText="取消"
            okButtonProps={{ status: "danger" }}
            onOk={() => onDelete(record.id)}
          >
            <Tooltip content="删除评论">
              <Button
                type="text"
                size="mini"
                status="danger"
                icon={<IconDelete />}
                aria-label={`删除 ${record.author_name} 的评论`}
                disabled={busy}
              />
            </Tooltip>
          </Popconfirm>
        </Space>
      )
    }
  ];

  return (
    <>
      <PageHeader
        title="评论审核"
        description="集中处理待审核评论、异常内容与历史审核结果。"
        eyebrow="审核队列"
      />

      <ContentPanel
        title="审核队列"
        description={`共 ${pagination.total} 条评论，可直接在操作列调整审核状态。`}
      >
        <Tabs
          activeTab={statusFilter}
          type="line"
          onChange={(key) => listQuery.update({ status: key === "all" ? "" : key })}
        >
          <TabPane key="all" title="全部" />
          <TabPane key="pending" title="待审核" />
          <TabPane key="approved" title="已通过" />
          <TabPane key="rejected" title="已拒绝" />
          <TabPane key="spam" title="垃圾" />
        </Tabs>
        <Input.Search
          allowClear
          value={searchQuery}
          placeholder="搜索作者、邮箱或正文"
          style={{ width: 280, maxWidth: "100%", marginBottom: 16 }}
          onChange={setSearchQuery}
          onSearch={(value) => listQuery.update({ q: value.trim() })}
        />
        <Table<Comment>
          rowKey="id"
          size="small"
          border={{ wrapper: true, headerCell: true }}
          tableLayoutFixed
          loading={busy}
          pagination={{
            current: pagination.page,
            pageSize: pagination.page_size,
            total: pagination.total,
            hideOnSinglePage: true,
            size: "small",
            sizeCanChange: true,
            sizeOptions: [20, 50],
            showTotal: true,
            onChange: (page, pageSize) => listQuery.update({ page, pageSize })
          }}
          data={comments}
          columns={canModerate ? columns : columns.filter((column) => column.title !== "操作")}
          noDataElement="当前状态下暂无评论"
          scroll={{ x: 1242 }}
        />
      </ContentPanel>
    </>
  );
}
