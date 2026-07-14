import { Input, Select, Space, Table, Tag, Tooltip, Typography, type TableColumnProps } from "@arco-design/web-react";
import { useEffect, useState } from "react";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import { useListQuery } from "../hooks/useListQuery";
import type { AuditLog, PaginationMeta } from "../types/admin";

const { Text } = Typography;

function TruncatedValue({ value, code = false }: { value: string; code?: boolean }) {
  return (
    <Tooltip content={value}>
      <Text code={code} ellipsis style={{ display: "block", maxWidth: "100%" }}>{value}</Text>
    </Tooltip>
  );
}

type Props = {
  logs: AuditLog[];
  pagination: PaginationMeta;
};

export function AuditPage({ logs, pagination }: Props) {
  const listQuery = useListQuery(20);
  const [searchQuery, setSearchQuery] = useState(listQuery.q);

  useEffect(() => setSearchQuery(listQuery.q), [listQuery.q]);

  const columns: TableColumnProps<AuditLog>[] = [
    { title: "时间", dataIndex: "created_at", width: 180, render: (value) => new Date(value).toLocaleString() },
    { title: "操作者", dataIndex: "actor_email", width: 200, render: (value) => value || "系统" },
    { title: "操作", dataIndex: "action", width: 190, ellipsis: true },
    {
      title: "资源",
      width: 240,
      render: (_, record) => {
        const resource = record.resource_id ? `${record.resource_type}:${record.resource_id}` : record.resource_type;
        return <TruncatedValue value={resource} />;
      }
    },
    { title: "路由", dataIndex: "route", width: 240, render: (value) => <TruncatedValue value={value} code /> },
    {
      title: "结果",
      dataIndex: "result",
      width: 100,
      render: (value) => <Tag color={value === "success" ? "green" : "red"}>{value === "success" ? "成功" : "失败"}</Tag>
    },
    { title: "HTTP", width: 110, render: (_, record) => `${record.method} ${record.status_code}` },
    { title: "请求 ID", dataIndex: "request_id", width: 220, render: (value) => <TruncatedValue value={value} code /> }
  ];

  return (
    <>
      <PageHeader
        title="审计日志"
        description="按时间追踪后台操作、资源、响应结果和请求链路。"
        eyebrow="系统记录"
      />
      <ContentPanel title="操作记录" description={`共 ${pagination.total} 条审计记录。`}>
        <Space wrap size={8} style={{ marginBottom: 16 }}>
          <Input.Search
            allowClear
            value={searchQuery}
            placeholder="搜索操作者、操作、资源或请求 ID"
            style={{ width: 320, maxWidth: "100%" }}
            onChange={setSearchQuery}
            onSearch={(value) => listQuery.update({ q: value.trim() })}
          />
          <Select
            value={listQuery.status || "all"}
            style={{ width: 132 }}
            options={[
              { label: "全部结果", value: "all" },
              { label: "成功", value: "success" },
              { label: "失败", value: "failure" }
            ]}
            onChange={(value) => listQuery.update({ status: value === "all" ? "" : value })}
          />
        </Space>
        <Table
          rowKey="id"
          data={logs}
          columns={columns}
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
          size="small"
          scroll={{ x: 1480 }}
        />
      </ContentPanel>
    </>
  );
}
