import {
  Button,
  Image,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  Upload,
  type TableColumnProps
} from "@arco-design/web-react";
import { IconCode, IconCopy, IconDelete, IconEye, IconSettings, IconUpload } from "@arco-design/web-react/icon";
import { useEffect, useState } from "react";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import { useListQuery } from "../hooks/useListQuery";
import type { MediaAsset, PaginationMeta } from "../types/admin";

const { Text } = Typography;

type Props = {
  media: MediaAsset[];
  pagination: PaginationMeta;
  canUpload: boolean;
  canDelete: boolean;
  canInsertMarkdown: boolean;
  busy: boolean;
  cleanupBusy: boolean;
  mediaURL: (asset: MediaAsset) => string;
  onUpload: (options: {
    file: File;
    onSuccess?: (body?: unknown) => void;
    onError?: (event: Error) => void;
  }) => void;
  onCleanup: (dryRun: boolean) => void;
  onCopyURL: (asset: MediaAsset) => void;
  onInsertMarkdown: (asset: MediaAsset) => void;
  onDelete: (id: string) => void;
};

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${Math.max(1, Math.round(value / 1024))} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : date.toLocaleString("zh-CN", { hour12: false });
}

function mediaStatus(status: string) {
  if (status === "ready") return { color: "green", label: "可用" };
  if (status === "failed") return { color: "red", label: "失败" };
  if (status === "processing" || status === "queued") return { color: "orange", label: "处理中" };
  if (status === "deleted") return { color: "gray", label: "已删除" };
  return { color: "gray", label: status || "未知" };
}

export function MediaPage(props: Props) {
  const listQuery = useListQuery(20);
  const [maintenanceVisible, setMaintenanceVisible] = useState(false);
  const [filenameQuery, setFilenameQuery] = useState(listQuery.q);
  const referencedCount = props.media.filter((asset) => (asset.usage_count ?? 0) > 0).length;

  useEffect(() => setFilenameQuery(listQuery.q), [listQuery.q]);

  const columns: TableColumnProps<MediaAsset>[] = [
    {
      title: "预览",
      width: 96,
      fixed: "left",
      render: (_, record) => (
        <Image
          width={72}
          height={48}
          src={props.mediaURL(record)}
          alt={record.original_name || record.filename}
          style={{ objectFit: "cover", borderRadius: 4, background: "#f2f5f3" }}
        />
      )
    },
    {
      title: "资产",
      width: 260,
      render: (_, record) => (
        <div style={{ minWidth: 0 }}>
          <Text bold ellipsis={{ showTooltip: true }} style={{ display: "block" }}>
            {record.original_name || record.filename}
          </Text>
          <Text
            type="secondary"
            ellipsis={{ showTooltip: true }}
            style={{ display: "block", marginTop: 2, fontSize: 12 }}
          >
            {record.mime_type || record.filename}
          </Text>
        </div>
      )
    },
    {
      title: "规格",
      width: 148,
      render: (_, record) => (
        <div>
          <Text style={{ display: "block" }}>{formatBytes(record.size_bytes)}</Text>
          <Text type="secondary" style={{ display: "block", marginTop: 2, fontSize: 12 }}>
            {record.width && record.height ? `${record.width} x ${record.height}` : "尺寸未知"}
          </Text>
        </div>
      )
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 90,
      render: (value) => {
        const status = mediaStatus(value);
        return <Tag color={status.color}>{status.label}</Tag>;
      }
    },
    {
      title: "引用",
      dataIndex: "usage_count",
      width: 84,
      align: "right",
      render: (value) => {
        const count = value ?? 0;
        return <Tag color={count > 0 ? "orange" : "gray"}>{count}</Tag>;
      }
    },
    {
      title: "上传时间",
      dataIndex: "created_at",
      width: 176,
      render: (value) => formatDate(value)
    },
    {
      title: "操作",
      width: 132,
      align: "center",
      fixed: "right",
      render: (_, record) => {
        const usageCount = record.usage_count ?? 0;

        return (
          <Space size={2}>
            <Tooltip content="复制媒体地址">
              <Button
                type="text"
                size="mini"
                icon={<IconCopy />}
                aria-label={`复制 ${record.original_name || record.filename} 的媒体地址`}
                disabled={props.busy}
                onClick={() => props.onCopyURL(record)}
              />
            </Tooltip>
            {props.canInsertMarkdown ? (
              <Tooltip content="插入 Markdown">
                <Button
                  type="text"
                  size="mini"
                  icon={<IconCode />}
                  aria-label={`将 ${record.original_name || record.filename} 插入 Markdown`}
                  disabled={props.busy}
                  onClick={() => props.onInsertMarkdown(record)}
                />
              </Tooltip>
            ) : null}
            {props.canDelete && (usageCount > 0 ? (
              <Tooltip content={`仍被 ${usageCount} 处内容引用，不能删除`}>
                <span style={{ display: "inline-flex" }}>
                  <Button
                    type="text"
                    size="mini"
                    status="danger"
                    icon={<IconDelete />}
                    aria-label={`删除 ${record.original_name || record.filename}`}
                    disabled
                  />
                </span>
              </Tooltip>
            ) : (
              <Popconfirm
                title="确认删除此媒体？"
                content="该操作会永久移除文件，且无法恢复。"
                okText="删除"
                cancelText="取消"
                okButtonProps={{ status: "danger" }}
                onOk={() => props.onDelete(record.id)}
              >
                <Tooltip content="删除媒体">
                  <Button
                    type="text"
                    size="mini"
                    status="danger"
                    icon={<IconDelete />}
                    aria-label={`删除 ${record.original_name || record.filename}`}
                    disabled={props.busy}
                  />
                </Tooltip>
              </Popconfirm>
            ))}
          </Space>
        );
      }
    }
  ];

  return (
    <>
      <PageHeader
        title="媒体资产"
        description="统一管理内容图片、引用关系与孤立文件清理。"
        eyebrow="内容资源"
        actions={props.canUpload || props.canDelete ? (
          <Space size={8} wrap>
            {props.canDelete ? (
              <Button
                icon={<IconSettings />}
                onClick={() => setMaintenanceVisible(true)}
              >
                维护
              </Button>
            ) : null}
            {props.canUpload ? (
              <Upload
                accept=".png,.jpg,.jpeg,.gif,.webp,image/png,image/jpeg,image/gif,image/webp"
                disabled={props.busy}
                showUploadList={false}
                customRequest={({ file, onSuccess, onError }) => {
                  props.onUpload({
                    file,
                    onSuccess: (body) => onSuccess((body ?? {}) as object),
                    onError: (error) => onError(error)
                  });
                }}
              >
                <Button
                  type="primary"
                  icon={<IconUpload />}
                  loading={props.busy}
                >
                  上传图片
                </Button>
              </Upload>
            ) : null}
          </Space>
        ) : undefined}
      />

      {props.canDelete ? <Modal
        title="媒体维护"
        visible={maintenanceVisible}
        footer={null}
        unmountOnExit
        onCancel={() => setMaintenanceVisible(false)}
      >
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <div>
            <Text bold style={{ display: "block" }}>清理预演</Text>
            <Text type="secondary">检查可清理的孤立媒体，不会删除文件。</Text>
          </div>
          <Button
            long
            icon={<IconEye />}
            loading={props.cleanupBusy}
            onClick={() => props.onCleanup(true)}
          >
            运行清理预演
          </Button>
          <div>
            <Text bold style={{ display: "block" }}>执行清理</Text>
            <Text type="secondary">永久删除未被内容引用的媒体文件。</Text>
          </div>
          <Popconfirm
            title="确认清理孤立媒体？"
            content="仅删除未被内容引用的媒体文件，建议先执行清理预演。"
            okText="开始清理"
            cancelText="取消"
            okButtonProps={{ status: "danger" }}
            onOk={() => props.onCleanup(false)}
          >
            <Button
              long
              status="danger"
              icon={<IconDelete />}
              loading={props.cleanupBusy}
            >
              清理孤立媒体
            </Button>
          </Popconfirm>
        </Space>
      </Modal> : null}

      <ContentPanel
        title="资产列表"
        description={`共 ${props.pagination.total} 个媒体文件，当前页 ${referencedCount} 个正在被内容引用。`}
      >
        <Space wrap size={8} style={{ marginBottom: 16 }}>
          <Input.Search
            allowClear
            value={filenameQuery}
            placeholder="搜索文件名"
            style={{ width: 240, maxWidth: "100%" }}
            onChange={setFilenameQuery}
            onSearch={(value) => listQuery.update({ q: value.trim() })}
          />
          <Select
            value={listQuery.status || "all"}
            style={{ width: 132 }}
            options={[
              { label: "全部状态", value: "all" },
              { label: "可用", value: "ready" },
              { label: "处理中", value: "processing" },
              { label: "失败", value: "failed" },
              { label: "已删除", value: "deleted" }
            ]}
            onChange={(value) => listQuery.update({ status: value === "all" ? "" : value })}
          />
        </Space>
        <Table<MediaAsset>
          rowKey="id"
          size="small"
          border={{ wrapper: true, headerCell: true }}
          tableLayoutFixed
          loading={props.busy}
          pagination={{
            current: props.pagination.page,
            pageSize: props.pagination.page_size,
            total: props.pagination.total,
            hideOnSinglePage: true,
            size: "small",
            sizeCanChange: true,
            sizeOptions: [20, 50],
            showTotal: true,
            onChange: (page, pageSize) => listQuery.update({ page, pageSize })
          }}
          data={props.media}
          columns={columns}
          noDataElement={listQuery.q || listQuery.status ? "未找到匹配的媒体文件" : "暂无媒体文件"}
          scroll={{ x: 986 }}
        />
      </ContentPanel>
    </>
  );
}
