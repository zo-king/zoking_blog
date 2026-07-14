import { useEffect, useState } from "react";
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
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
  type FormInstance
} from "@arco-design/web-react";
import {
  IconArrowLeft,
  IconDelete,
  IconEdit,
  IconEye,
  IconPlus,
  IconSafe,
  IconSave,
  IconSend
} from "@arco-design/web-react/icon";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import { useListQuery } from "../hooks/useListQuery";
import type { Page, PageFormValues, PaginationMeta } from "../types/admin";

const { Row, Col } = Grid;
const { Text, Title } = Typography;

type Props = {
  pages: Page[];
  pagination: PaginationMeta;
  form: FormInstance<PageFormValues>;
  enabled: boolean;
  canCreate: boolean;
  canUpdate: boolean;
  canDelete: boolean;
  canSave: boolean;
  canPreview: boolean;
  canPublish: boolean;
  writeBlocked: boolean;
  busy: boolean;
  previewBusy: boolean;
  qualityBusy: boolean;
  mode: "list" | "editor";
  onNew: () => void;
  onSelect: (page: Page) => void;
  onBack: () => void;
  onDelete: (id: string) => void;
  onSave: (values: PageFormValues) => void;
  onPreview: () => void;
  onPublish: () => void;
  onQualityCheck: () => void;
  onFormChange: () => void;
};

export function PagesPage({
  pages,
  pagination,
  form,
  enabled,
  canCreate,
  canUpdate,
  canDelete,
  canSave,
  canPreview,
  canPublish,
  writeBlocked,
  busy,
  previewBusy,
  qualityBusy,
  mode,
  onNew,
  onSelect,
  onBack,
  onDelete,
  onSave,
  onPreview,
  onPublish,
  onQualityCheck,
  onFormChange
}: Props) {
  const listQuery = useListQuery(20);
  const [query, setQuery] = useState(listQuery.q);

  useEffect(() => {
    setQuery(listQuery.q);
  }, [listQuery.q]);

  if (mode === "list") {
    return (
      <div className="pages-list-workspace">
        <PageHeader
          eyebrow="内容管理"
          title="独立页面"
          description="管理固定页面及其菜单展示属性。"
          actions={
            canCreate ? <Button type="primary" icon={<IconPlus />} onClick={onNew}>
              新建页面
            </Button> : null
          }
        />

        <ContentPanel className="pages-list-panel">
          <div className="pages-list-toolbar" style={{ marginBottom: 14 }}>
            <Space wrap size={8} style={{ width: "100%" }}>
              <Input.Search
                allowClear
                placeholder="搜索标题或 Slug"
                value={query}
                onChange={(value) => {
                  setQuery(value);
                  if (!value) listQuery.update({ q: "" });
                }}
                onSearch={(value) => listQuery.update({ q: value.trim() })}
                style={{ width: 280, maxWidth: "100%" }}
              />
              <Select
                value={listQuery.status || "all"}
                onChange={(value) => listQuery.update({ status: value === "all" ? "" : value })}
                style={{ width: 136 }}
                options={[
                  { label: "全部状态", value: "all" },
                  { label: "草稿", value: "draft" },
                  { label: "已发布", value: "published" },
                  { label: "已下线", value: "offline" }
                ]}
              />
              <Text type="secondary">共 {pagination.total} 个页面</Text>
            </Space>
          </div>

          <div style={{ minWidth: 0, width: "100%" }}>
            <Table<Page>
              rowKey="id"
              data={pages}
              pagination={{
                current: listQuery.page,
                pageSize: listQuery.pageSize,
                total: pagination.total,
                hideOnSinglePage: true,
                showTotal: true,
                size: "small",
                sizeCanChange: true,
                onChange: (page, pageSize) => listQuery.update({ page, pageSize })
              }}
              size="small"
              tableLayoutFixed
              scroll={{ x: 860 }}
              noDataElement={(
                <Empty
                  description={
                    !enabled
                      ? "请登录后管理页面。"
                      : listQuery.q || listQuery.status
                        ? "没有符合筛选条件的页面。"
                        : "暂无独立页面。"
                  }
                />
              )}
              onRow={(record) => canUpdate ? ({
                onClick: () => onSelect(record),
                style: { cursor: "pointer" }
              }) : {}}
              columns={[
                {
                  title: "页面",
                  dataIndex: "title",
                  width: 300,
                  render: (_, record) => (
                    <div style={{ minWidth: 0 }}>
                      <Text ellipsis style={{ display: "block" }}>
                        <strong>{record.title || "未命名页面"}</strong>
                      </Text>
                      <Text type="secondary" ellipsis style={{ display: "block", marginTop: 2 }}>
                        {record.summary || "暂无摘要"}
                      </Text>
                    </div>
                  )
                },
                { title: "Slug", dataIndex: "slug", width: 190, ellipsis: true },
                {
                  title: "状态",
                  dataIndex: "status",
                  width: 110,
                  render: (value) => (
                    <Tag color={value === "published" ? "green" : value === "offline" ? "gray" : "orange"}>
                      {value === "published" ? "已发布" : value === "offline" ? "已下线" : "草稿"}
                    </Tag>
                  )
                },
                {
                  title: "菜单",
                  dataIndex: "show_in_menu",
                  width: 100,
                  render: (value) => <Tag color={value ? "green" : undefined}>{value ? "显示" : "隐藏"}</Tag>
                },
                {
                  title: "操作",
                  width: 170,
                  fixed: "right",
                  render: (_, record) => (
                    <div onClick={(event) => event.stopPropagation()}>
                      <Space size={4}>
                        {canUpdate && <Tooltip content="编辑页面">
                          <Button type="text" size="mini" icon={<IconEdit />} onClick={() => onSelect(record)}>
                            编辑
                          </Button>
                        </Tooltip>}
                        {canDelete && <Popconfirm
                          title="确认归档此页面？"
                          okText="归档"
                          okButtonProps={{ status: "danger" }}
                          onOk={() => onDelete(record.id)}
                        >
                          <Tooltip content="归档页面">
                            <Button type="text" size="mini" status="danger" icon={<IconDelete />}>
                              归档
                            </Button>
                          </Tooltip>
                        </Popconfirm>}
                      </Space>
                    </div>
                  )
                }
              ]}
            />
          </div>
        </ContentPanel>
      </div>
    );
  }

  return (
    <div className="page-editor-workspace">
      <PageHeader
        eyebrow="内容管理"
        title="页面编辑器"
        description="维护页面正文、发布状态与菜单入口。"
        actions={
          <Space wrap size={4}>
            <Button icon={<IconArrowLeft />} onClick={onBack}>
              返回列表
            </Button>
            <Button icon={<IconSave />} disabled={!canSave || writeBlocked} loading={busy} onClick={() => form.submit()}>
              保存
            </Button>
            <Button icon={<IconEye />} disabled={!canPreview || writeBlocked} loading={previewBusy} onClick={onPreview}>
              预览
            </Button>
            <Button icon={<IconSafe />} disabled={!canSave || writeBlocked} loading={qualityBusy} onClick={onQualityCheck}>
              发布检查
            </Button>
            {canPublish && <Button type="primary" icon={<IconSend />} loading={busy || qualityBusy} onClick={onPublish}>
              发布
            </Button>}
          </Space>
        }
      />

      <Form<PageFormValues>
        form={form}
        layout="vertical"
        requiredSymbol={false}
        onSubmit={onSave}
        onValuesChange={onFormChange}
      >
        <Row className="page-editor-grid" gutter={[18, 18]} align="start">
          <Col xs={24} xl={16} style={{ minWidth: 0 }}>
            <ContentPanel title="页面内容" description="标题、检索摘要与 Markdown 正文。">
              <Row gutter={[12, 0]}>
                <Col xs={24} md={16}>
                  <Form.Item field="title" label="标题" rules={[{ required: true, message: "请输入页面标题" }]}>
                    <Input placeholder="关于" />
                  </Form.Item>
                </Col>
                <Col xs={24} md={8}>
                  <Form.Item field="slug" label="Slug" rules={[{ required: true, message: "请输入页面 Slug" }]}>
                    <Input placeholder="about" />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item field="summary" label="摘要">
                <Input placeholder="独立页面摘要" />
              </Form.Item>
              <Form.Item field="seo_description" label="SEO 描述">
                <Input placeholder="用于搜索结果的页面描述" />
              </Form.Item>
              <Form.Item field="content_md" label="Markdown 正文">
                <Input.TextArea
                  rows={14}
                  autoSize={{ minRows: 12, maxRows: 24 }}
                  placeholder="在这里编写独立页面正文。"
                />
              </Form.Item>
            </ContentPanel>
          </Col>

          <Col xs={24} xl={8} style={{ minWidth: 0 }}>
            <ContentPanel className="page-editor-sidebar" title="发布与菜单" description="控制访问范围和导航展示。">
              <Title heading={6} style={{ marginTop: 0 }}>发布设置</Title>
              <Row gutter={[12, 0]}>
                <Col xs={24} sm={12} xl={24} xxl={12}>
                  <Form.Item field="status" label="状态">
                    <Select disabled={!canPublish} options={[
                      { label: "草稿", value: "draft" },
                      { label: "已发布", value: "published" },
                      { label: "已下线", value: "offline" }
                    ]} />
                  </Form.Item>
                </Col>
                <Col xs={24} sm={12} xl={24} xxl={12}>
                  <Form.Item field="visibility" label="可见性">
                    <Select options={[
                      { label: "公开", value: "public" },
                      { label: "私密", value: "private" },
                      { label: "不公开列出", value: "unlisted" }
                    ]} />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item field="allow_comment" label="允许评论" triggerPropName="checked">
                <Switch />
              </Form.Item>

              <Title heading={6}>菜单设置</Title>
              <Form.Item field="show_in_menu" label="显示在菜单" triggerPropName="checked">
                <Switch />
              </Form.Item>
              <Row gutter={[12, 0]}>
                <Col xs={24} sm={12} xl={24} xxl={12}>
                  <Form.Item field="menu_weight" label="菜单排序">
                    <InputNumber min={0} style={{ width: "100%" }} />
                  </Form.Item>
                </Col>
                <Col xs={24} sm={12} xl={24} xxl={12}>
                  <Form.Item field="menu_icon" label="菜单图标">
                    <Input placeholder="user" />
                  </Form.Item>
                </Col>
              </Row>
            </ContentPanel>
          </Col>
        </Row>
      </Form>
    </div>
  );
}
