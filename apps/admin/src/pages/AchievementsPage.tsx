import {
  Button,
  Empty,
  Form,
  Grid,
  Input,
  InputNumber,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  type FormInstance,
  type TableColumnProps
} from "@arco-design/web-react";
import { IconDelete, IconEdit, IconLeft, IconPlus, IconSave, IconSend } from "@arco-design/web-react/icon";
import { useEffect } from "react";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import { AchievementMediaPicker } from "../components/AchievementMediaPicker";
import type { Achievement, AchievementFormValues, MediaAsset, PaginationMeta } from "../types/admin";
import "../styles/achievements.css";

const { Row, Col } = Grid;
const { Text } = Typography;

export type AchievementQuery = {
  page: number;
  pageSize: number;
  year: string;
};

export type AchievementsPageProps = {
  achievements: Achievement[];
  achievement?: Achievement | null;
  pagination: PaginationMeta;
  form: FormInstance<AchievementFormValues>;
  enabled?: boolean;
  canRead?: boolean;
  canCreate?: boolean;
  canUpdate?: boolean;
  canDelete?: boolean;
  canPublish?: boolean;
  canReadMedia?: boolean;
  mode: "list" | "editor";
  editorID?: string;
  loading?: boolean;
  busy?: boolean;
  deletingID?: string;
  publishing?: boolean;
  query: AchievementQuery;
  mediaURL: (asset: MediaAsset) => string;
  searchMedia: (query: string, page: number, pageSize?: number) => Promise<{ data: MediaAsset[]; pagination: PaginationMeta }>;
  onQueryChange: (patch: { page?: number; pageSize?: number; year?: string }) => void;
  onNew: () => void;
  onSelect: (achievement: Achievement) => void;
  onBack: () => void;
  onDelete: (id: string) => void | Promise<void>;
  onSave: (values: AchievementFormValues) => void | Promise<void>;
  onStatusChange?: (id: string, status: string) => void;
  onPublish: () => void | Promise<void>;
};

function formatDate(value?: string | null) {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString("zh-CN");
}

function statusMeta(status: string) {
  if (status === "published") return { label: "已发布", color: "green" as const };
  if (status === "archived") return { label: "已归档", color: "gray" as const };
  return { label: "草稿", color: "orange" as const };
}

function kindLabel(kind: string) {
  return ({ award: "奖项", certificate: "证书", project: "项目" } as Record<string, string>)[kind] || kind || "其他";
}

function defaultValues(): Partial<AchievementFormValues> {
  return { kind: "award", summary: "", external_url: "", credential_id: "", image_media_id: undefined, sort_order: 0, status: "draft" };
}

export function AchievementsPage(props: AchievementsPageProps) {
  const canCreate = props.canCreate === true;
  const canUpdate = props.canUpdate === true;
  const canDelete = props.canDelete === true;
  const canPublish = props.canPublish === true;
  const imageMediaID = Form.useWatch("image_media_id", props.form) as string | undefined;

  useEffect(() => {
    if (props.mode !== "editor") return;
    if (props.editorID === "new") {
      props.form.resetFields();
      props.form.setFieldsValue(defaultValues());
    } else if (props.achievement) {
      props.form.resetFields();
      props.form.setFieldsValue({ ...props.achievement, ended_at: props.achievement.ended_at || undefined, image_media_id: props.achievement.image_media_id || undefined });
    }
  }, [props.achievement, props.editorID, props.form, props.mode]);

  if (props.mode === "list") {
    const years = Array.from({ length: Math.max(3, new Date().getFullYear() - 2023) }, (_, index) => String(new Date().getFullYear() - index));
    const columns: TableColumnProps<Achievement>[] = [
      {
        title: "成果",
        width: 320,
        render: (_, record) => <div className="achievement-title-cell"><Text ellipsis={{ showTooltip: true }}><strong>{record.title || "未命名成果"}</strong></Text><Text type="secondary" ellipsis={{ showTooltip: true }}>{record.organization || "未填写组织"}</Text></div>
      },
      { title: "类型", dataIndex: "kind", width: 100, render: (value) => <Tag>{kindLabel(value)}</Tag> },
      { title: "发生日期", dataIndex: "occurred_at", width: 126, render: (value) => formatDate(value) },
      { title: "状态", dataIndex: "status", width: 98, render: (value) => { const meta = statusMeta(value); return <Tag color={meta.color}>{meta.label}</Tag>; } },
      { title: "排序", dataIndex: "sort_order", width: 76, align: "right" },
      {
        title: "操作",
        width: 150,
        fixed: "right",
        render: (_, record) => <div onClick={(event) => event.stopPropagation()}><Space size={2}>
          {canUpdate ? <Tooltip content="编辑成果"><Button type="text" size="mini" icon={<IconEdit />} aria-label="编辑成果" onClick={() => props.onSelect(record)} /></Tooltip> : null}
          {canDelete && record.status !== "published" ? <Popconfirm title="确认删除此成果？" content="删除后无法恢复。" okText="删除" cancelText="取消" okButtonProps={{ status: "danger" }} onOk={() => props.onDelete(record.id)}><Tooltip content="删除成果"><Button type="text" size="mini" status="danger" icon={<IconDelete />} aria-label="删除成果" loading={props.deletingID === record.id} /></Tooltip></Popconfirm> : null}
          {canPublish && props.onStatusChange ? <Tooltip content={record.status === "published" ? "归档成果" : "标记为已发布"}><Button type="text" size="mini" icon={<IconSend />} aria-label={record.status === "published" ? "归档成果" : "标记为已发布"} onClick={() => props.onStatusChange?.(record.id, record.status === "published" ? "archived" : "published")} /></Tooltip> : null}
        </Space></div>
      }
    ];
    return <div className="achievements-page achievements-list-page">
      <PageHeader eyebrow="内容管理" title="成果" description="维护奖项、项目与认证等可公开展示的成果记录。" actions={canCreate || canPublish ? <Space size={8} wrap>
        {canPublish ? <Button icon={<IconSend />} loading={props.publishing} onClick={() => void props.onPublish()}>发布成果</Button> : null}
        {canCreate ? <Button type="primary" icon={<IconPlus />} onClick={props.onNew}>新建成果</Button> : null}
      </Space> : null} />
      <ContentPanel className="achievements-list-panel">
        <div className="achievements-toolbar">
          <Select value={props.query.year || "all"} options={[{ label: "全部年份", value: "all" }, ...years.map((year) => ({ label: `${year} 年`, value: year }))]} onChange={(value) => props.onQueryChange({ year: value === "all" ? "" : value })} />
          <Text type="secondary">共 {props.pagination.total} 项成果</Text>
        </div>
        <Table<Achievement>
          rowKey="id"
          data={props.achievements}
          loading={props.loading}
          size="small"
          tableLayoutFixed
          scroll={{ x: 900 }}
          pagination={{ current: props.pagination.page, pageSize: props.pagination.page_size, total: props.pagination.total, hideOnSinglePage: true, size: "small", showTotal: true, onChange: (page, pageSize) => props.onQueryChange({ page, pageSize }) }}
          noDataElement={<Empty description={!props.enabled ? "请登录后管理成果。" : props.canRead === false ? "当前账号无权读取成果。" : props.query.year ? "该年份暂无成果。" : "暂无成果记录。"} />}
          onRow={(record) => canUpdate ? { onClick: () => props.onSelect(record), style: { cursor: "pointer" } } : {}}
          columns={columns}
        />
      </ContentPanel>
    </div>;
  }

  const selectedMedia = props.achievement?.image_media || null;
  return <div className="achievements-page achievements-editor-page">
    <PageHeader eyebrow="内容管理" title={props.editorID === "new" ? "新建成果" : "编辑成果"} description="完善成果信息、配图与公开状态。" actions={<Space wrap size={4}>
      <Button icon={<IconLeft />} onClick={props.onBack}>返回列表</Button>
      <Button icon={<IconSave />} disabled={!props.enabled || !(props.editorID === "new" ? canCreate : canUpdate)} loading={props.busy} onClick={() => props.form.submit()}>保存</Button>
      {canPublish ? <Button type="primary" icon={<IconSend />} disabled={props.editorID === "new"} loading={props.publishing} onClick={() => void props.onPublish()}>发布成果</Button> : null}
    </Space>} />
    <ContentPanel className="achievements-editor-panel">
      <Form<AchievementFormValues> form={props.form} layout="vertical" requiredSymbol={false} onSubmit={props.onSave}>
        <Row gutter={[24, 18]} align="stretch">
          <Col xs={24} xl={16} style={{ minWidth: 0 }}>
            <div className="achievement-editor-main">
              <Form.Item field="title" label="成果名称" rules={[{ required: true, message: "请输入成果名称" }]}><Input size="large" placeholder="例如：年度优秀开源项目" /></Form.Item>
              <Row gutter={[12, 0]}>
                <Col xs={24} md={12}><Form.Item field="organization" label="组织 / 颁发方" rules={[{ required: true, message: "请输入组织或颁发方" }]}><Input placeholder="组织名称" /></Form.Item></Col>
                <Col xs={24} md={12}><Form.Item field="kind" label="成果类型" rules={[{ required: true, message: "请选择成果类型" }]}><Select options={[{ label: "奖项", value: "award" }, { label: "证书", value: "certificate" }, { label: "项目", value: "project" }]} /></Form.Item></Col>
              </Row>
              <Form.Item field="summary" label="简介"><Input.TextArea autoSize={{ minRows: 5, maxRows: 10 }} placeholder="用一两句话说明成果背景与价值。" /></Form.Item>
              <Row gutter={[12, 0]}>
                <Col xs={24} md={12}><Form.Item field="occurred_at" label="发生日期" rules={[{ required: true, message: "请输入发生日期" }]}><Input placeholder="YYYY-MM-DD" /></Form.Item></Col>
                <Col xs={24} md={12}><Form.Item field="ended_at" label="结束日期"><Input placeholder="可选，YYYY-MM-DD" /></Form.Item></Col>
              </Row>
              <Form.Item field="external_url" label="外部链接"><Input placeholder="https://..." /></Form.Item>
            </div>
          </Col>
          <Col xs={24} xl={8} style={{ minWidth: 0 }}>
            <aside className="achievement-editor-sidebar">
              <div className="achievement-section-title">公开设置</div>
              <Form.Item field="status" label="状态"><Select disabled={!canPublish || props.editorID === "new"} onChange={(status) => props.editorID && props.onStatusChange?.(props.editorID, status)} options={[{ label: "草稿", value: "draft" }, { label: "已发布", value: "published" }, { label: "已归档", value: "archived" }]} /></Form.Item>
              <Form.Item field="credential_id" label="凭证编号"><Input placeholder="可选" /></Form.Item>
              <Form.Item field="sort_order" label="排序权重"><InputNumber min={0} precision={0} mode="button" style={{ width: "100%" }} /></Form.Item>
              <div className="achievement-section-title">成果配图</div>
              <AchievementMediaPicker value={imageMediaID} selected={selectedMedia} canRead={props.canReadMedia} mediaURL={props.mediaURL} search={props.searchMedia} onChange={(value) => props.form.setFieldValue("image_media_id", value)} />
              {props.canDelete && props.editorID && props.editorID !== "new" && props.achievement?.status !== "published" ? <Popconfirm title="确认删除此成果？" okText="删除" cancelText="取消" okButtonProps={{ status: "danger" }} onOk={() => props.onDelete(props.editorID || "")}><Button long status="danger" icon={<IconDelete />} loading={Boolean(props.deletingID)}>删除成果</Button></Popconfirm> : null}
            </aside>
          </Col>
        </Row>
      </Form>
    </ContentPanel>
  </div>;
}
