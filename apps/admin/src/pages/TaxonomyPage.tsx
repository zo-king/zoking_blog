import {
  Button,
  Avatar,
  ColorPicker,
  Form,
  Grid,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tabs,
  Tooltip,
  Typography,
  type FormInstance,
  type TableColumnProps
} from "@arco-design/web-react";
import { IconDelete, IconEdit, IconPlus } from "@arco-design/web-react/icon";
import { useState } from "react";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import type { CategoryFormValues, MediaAsset, Series, SeriesFormValues, TagFormValues, Taxonomy } from "../types/admin";

const { Row, Col } = Grid;
const { Text } = Typography;

const tagPresetColors = ["#165DFF", "#00B42A", "#FF7D00", "#F53F3F", "#722ED1", "#0FC6C2"];

type Props = {
  categories: Taxonomy[];
  tags: Taxonomy[];
  series: Series[];
  media: MediaAsset[];
  mediaURL: (asset: MediaAsset) => string;
  categoryForm: FormInstance<CategoryFormValues>;
  tagForm: FormInstance<TagFormValues>;
  seriesForm: FormInstance<SeriesFormValues>;
  canManage: boolean;
  busy: boolean;
  onCreateCategory: (values: CategoryFormValues) => void;
  onDeleteCategory: (id: string) => void;
  onCreateTag: (values: TagFormValues) => void;
  onDeleteTag: (id: string) => void;
  onSaveSeries: (values: SeriesFormValues, id?: string) => Promise<boolean>;
  onDeleteSeries: (id: string) => void;
};

function TaxonomyIdentity({ item }: { item: Taxonomy }) {
  return (
    <div style={{ minWidth: 0 }}>
      <Text bold ellipsis={{ showTooltip: true }} style={{ display: "block" }}>
        {item.name}
      </Text>
      <Text
        type="secondary"
        ellipsis={{ showTooltip: true }}
        style={{ display: "block", marginTop: 2, fontSize: 12 }}
      >
        {item.description || "暂无说明"}
      </Text>
    </div>
  );
}

export function TaxonomyPage(props: Props) {
  const [categoryModalVisible, setCategoryModalVisible] = useState(false);
  const [tagModalVisible, setTagModalVisible] = useState(false);
  const [seriesModalVisible, setSeriesModalVisible] = useState(false);
  const [editingSeries, setEditingSeries] = useState<Series | null>(null);

  function openSeriesModal(series?: Series) {
    setEditingSeries(series || null);
    props.seriesForm.resetFields();
    props.seriesForm.setFieldsValue(series ? {
      name: series.name,
      slug: series.slug,
      description: series.description,
      cover_media_id: series.cover_media_id || undefined,
      sort_order: series.sort_order,
      enabled: series.enabled
    } : { sort_order: 0, enabled: true });
    setSeriesModalVisible(true);
  }

  const coverOptions = props.media
    .filter((asset) => asset.status === "ready" && asset.mime_type.startsWith("image/"))
    .map((asset) => ({
      value: asset.id,
      label: <Space size={8}><Avatar shape="square" size={24}><img src={props.mediaURL(asset)} alt="" /></Avatar><Text ellipsis>{asset.original_name || asset.filename}</Text></Space>
    }));

  const categoryColumns: TableColumnProps<Taxonomy>[] = [
    {
      title: "分类",
      width: 210,
      render: (_, record) => <TaxonomyIdentity item={record} />
    },
    {
      title: "Slug",
      dataIndex: "slug",
      width: 150,
      render: (value) => <Text code>{value}</Text>
    },
    {
      title: "排序",
      dataIndex: "sort_order",
      width: 72,
      align: "right",
      render: (value) => value ?? 0
    },
    {
      title: "状态",
      dataIndex: "enabled",
      width: 82,
      render: (value) => <Tag color={value ? "green" : "gray"}>{value ? "启用" : "停用"}</Tag>
    },
    {
      title: "操作",
      width: 64,
      align: "center",
      fixed: "right",
      render: (_, record) => (
        <Popconfirm
          title="确认删除此分类？"
          content="删除后无法恢复，请先确认没有文章依赖该分类。"
          okText="删除"
          cancelText="取消"
          okButtonProps={{ status: "danger" }}
          onOk={() => props.onDeleteCategory(record.id)}
        >
          <Tooltip content="删除分类">
            <Button
              type="text"
              size="mini"
              status="danger"
              icon={<IconDelete />}
              aria-label={`删除分类 ${record.name}`}
              disabled={props.busy}
            />
          </Tooltip>
        </Popconfirm>
      )
    }
  ];

  const tagColumns: TableColumnProps<Taxonomy>[] = [
    {
      title: "标签",
      width: 210,
      render: (_, record) => <TaxonomyIdentity item={record} />
    },
    {
      title: "Slug",
      dataIndex: "slug",
      width: 150,
      render: (value) => <Text code>{value}</Text>
    },
    {
      title: "颜色",
      dataIndex: "color",
      width: 140,
      render: (value) => value ? (
        <Space size={6}>
          <span
            aria-hidden="true"
            style={{
              width: 14,
              height: 14,
              flex: "0 0 14px",
              background: value,
              border: "1px solid rgba(0, 0, 0, 0.14)",
              borderRadius: 3
            }}
          />
          <Text code>{value}</Text>
        </Space>
      ) : <Text type="secondary">未设置</Text>
    },
    {
      title: "操作",
      width: 64,
      align: "center",
      fixed: "right",
      render: (_, record) => (
        <Popconfirm
          title="确认删除此标签？"
          content="删除后无法恢复，文章上的标签关联也会失效。"
          okText="删除"
          cancelText="取消"
          okButtonProps={{ status: "danger" }}
          onOk={() => props.onDeleteTag(record.id)}
        >
          <Tooltip content="删除标签">
            <Button
              type="text"
              size="mini"
              status="danger"
              icon={<IconDelete />}
              aria-label={`删除标签 ${record.name}`}
              disabled={props.busy}
            />
          </Tooltip>
        </Popconfirm>
      )
    }
  ];

  const seriesColumns: TableColumnProps<Series>[] = [
    {
      title: "系列",
      width: 230,
      render: (_, record) => (
        <Space size={8} style={{ minWidth: 0 }}>
          {record.cover_media ? <Avatar shape="square" size={28}><img src={props.mediaURL(record.cover_media)} alt="" /></Avatar> : null}
          <div style={{ minWidth: 0 }}>
            <Text bold ellipsis={{ showTooltip: true }} style={{ display: "block" }}>{record.name}</Text>
            <Text type="secondary" ellipsis={{ showTooltip: true }} style={{ display: "block", marginTop: 2, fontSize: 12 }}>{record.description || "暂无说明"}</Text>
          </div>
        </Space>
      )
    },
    { title: "Slug", dataIndex: "slug", width: 150, render: (value) => <Text code>{value}</Text> },
    { title: "文章", dataIndex: "post_count", width: 72, align: "right", render: (value) => value ?? 0 },
    { title: "排序", dataIndex: "sort_order", width: 72, align: "right" },
    { title: "状态", dataIndex: "enabled", width: 82, render: (value) => <Tag color={value ? "green" : "gray"}>{value ? "启用" : "停用"}</Tag> },
    {
      title: "操作",
      width: 102,
      align: "center",
      fixed: "right",
      render: (_, record) => (
        <Space size={2}>
          <Tooltip content="编辑系列">
            <Button type="text" size="mini" icon={<IconEdit />} aria-label={`编辑系列 ${record.name}`} disabled={props.busy} onClick={() => openSeriesModal(record)} />
          </Tooltip>
          <Popconfirm
            title="确认删除此系列？"
            content="删除前请先移除文章引用；被引用时 API 会拒绝删除。"
            okText="删除"
            cancelText="取消"
            okButtonProps={{ status: "danger" }}
            onOk={() => props.onDeleteSeries(record.id)}
          >
            <Tooltip content="删除系列">
              <Button type="text" size="mini" status="danger" icon={<IconDelete />} aria-label={`删除系列 ${record.name}`} disabled={props.busy} />
            </Tooltip>
          </Popconfirm>
        </Space>
      )
    }
  ];

  return (
    <>
      <PageHeader
        title="内容组织"
        description="维护文章分类、标签与系列结构。"
        eyebrow="内容结构"
      />

      <ContentPanel className="tabbed-workbench taxonomy-workbench" title="内容组织管理" description="统一维护分类、标签和系列。">
        <Tabs defaultActiveTab="categories">
          <Tabs.TabPane key="categories" title="分类">
            <Space direction="vertical" size={16} style={{ width: "100%" }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <Tag color="arcoblue">共 {props.categories.length} 个分类</Tag>
                {props.canManage ? (
                  <Button
                    type="primary"
                    icon={<IconPlus />}
                    disabled={props.busy}
                    onClick={() => setCategoryModalVisible(true)}
                  >
                    创建分类
                  </Button>
                ) : null}
              </div>
              <Table<Taxonomy>
                rowKey="id"
                size="small"
                border={{ wrapper: true, headerCell: true }}
                tableLayoutFixed
                pagination={{ pageSize: 7, hideOnSinglePage: true, size: "small" }}
                data={props.categories}
                columns={props.canManage ? categoryColumns : categoryColumns.filter((column) => column.title !== "操作")}
                noDataElement="暂无分类"
                scroll={{ x: 620, y: 252 }}
              />
            </Space>
          </Tabs.TabPane>
          <Tabs.TabPane key="tags" title="标签">
            <Space direction="vertical" size={16} style={{ width: "100%" }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <Tag color="green">共 {props.tags.length} 个标签</Tag>
                {props.canManage ? (
                  <Button
                    type="primary"
                    icon={<IconPlus />}
                    disabled={props.busy}
                    onClick={() => setTagModalVisible(true)}
                  >
                    创建标签
                  </Button>
                ) : null}
              </div>
              <Table<Taxonomy>
                rowKey="id"
                size="small"
                border={{ wrapper: true, headerCell: true }}
                tableLayoutFixed
                pagination={{ pageSize: 7, hideOnSinglePage: true, size: "small" }}
                data={props.tags}
                columns={props.canManage ? tagColumns : tagColumns.filter((column) => column.title !== "操作")}
                noDataElement="暂无标签"
                scroll={{ x: 580, y: 252 }}
              />
            </Space>
          </Tabs.TabPane>
          <Tabs.TabPane key="series" title="系列">
            <Space direction="vertical" size={12} style={{ width: "100%" }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <Tag color="purple">共 {props.series.length} 个系列</Tag>
                {props.canManage ? <Button type="primary" icon={<IconPlus />} disabled={props.busy} onClick={() => openSeriesModal()}>创建系列</Button> : null}
              </div>
              <Table<Series>
                rowKey="id"
                size="small"
                border={{ wrapper: true, headerCell: true }}
                tableLayoutFixed
                pagination={{ pageSize: 7, hideOnSinglePage: true, size: "small" }}
                data={props.series}
                columns={props.canManage ? seriesColumns : seriesColumns.filter((column) => column.title !== "操作")}
                noDataElement="暂无系列"
                scroll={{ x: 760, y: 252 }}
              />
            </Space>
          </Tabs.TabPane>
        </Tabs>
      </ContentPanel>

      {props.canManage ? <Modal
        title={editingSeries ? "编辑系列" : "创建系列"}
        visible={seriesModalVisible}
        onCancel={() => setSeriesModalVisible(false)}
        onOk={() => props.seriesForm.submit()}
        confirmLoading={props.busy}
        okText={editingSeries ? "保存" : "创建"}
        cancelText="取消"
        unmountOnExit
      >
        <Form<SeriesFormValues>
          form={props.seriesForm}
          layout="vertical"
          requiredSymbol={false}
          disabled={props.busy}
          onSubmit={async (values) => {
            if (await props.onSaveSeries(values, editingSeries?.id)) setSeriesModalVisible(false);
          }}
        >
          <Row gutter={12}>
            <Col xs={24} md={12}>
              <Form.Item field="name" label="名称" rules={[{ required: true, message: "请输入系列名称" }]}><Input placeholder="例如：Go 工程实践" /></Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item field="slug" label="Slug" rules={[{ required: true, message: "请输入系列 Slug" }]}><Input placeholder="例如：go-engineering" /></Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item field="description" label="说明"><Input.TextArea autoSize={{ minRows: 2, maxRows: 3 }} placeholder="说明该系列覆盖的文章范围" /></Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item field="cover_media_id" label="封面媒体"><Select allowClear placeholder="从媒体库选择图片" options={coverOptions} notFoundContent="媒体库中暂无图片" /></Form.Item>
            </Col>
            <Col xs={12} md={8}>
              <Form.Item field="sort_order" label="展示顺序"><InputNumber min={0} precision={0} mode="button" style={{ width: "100%" }} /></Form.Item>
            </Col>
            <Col xs={12} md={8}>
              <Form.Item field="enabled" label="状态" triggerPropName="checked"><Switch checkedText="启用" uncheckedText="停用" /></Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal> : null}

      {props.canManage ? <Modal
        title="创建分类"
        visible={categoryModalVisible}
        onCancel={() => setCategoryModalVisible(false)}
        onOk={() => props.categoryForm.submit()}
        confirmLoading={props.busy}
        okText="创建"
        cancelText="取消"
        unmountOnExit
      >
        <Form<CategoryFormValues>
          form={props.categoryForm}
          layout="vertical"
          requiredSymbol={false}
          disabled={props.busy}
          initialValues={{ enabled: true, sort_order: 0 }}
          onSubmit={props.onCreateCategory}
        >
          <Row gutter={12}>
            <Col xs={24} md={12}>
              <Form.Item field="name" label="名称" rules={[{ required: true, message: "请输入分类名称" }]}>
                <Input placeholder="例如：开发实践" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item field="slug" label="Slug" rules={[{ required: true, message: "请输入 Slug" }]}>
                <Input placeholder="例如：development" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item field="description" label="说明">
                <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} placeholder="说明该分类收录的内容范围" />
              </Form.Item>
            </Col>
            <Col xs={12} md={8}>
              <Form.Item field="sort_order" label="排序">
                <InputNumber min={0} mode="button" style={{ width: "100%" }} />
              </Form.Item>
            </Col>
            <Col xs={12} md={8}>
              <Form.Item field="enabled" label="状态" triggerPropName="checked">
                <Switch checkedText="启用" uncheckedText="停用" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal> : null}

      {props.canManage ? <Modal
        title="创建标签"
        visible={tagModalVisible}
        onCancel={() => setTagModalVisible(false)}
        onOk={() => props.tagForm.submit()}
        confirmLoading={props.busy}
        okText="创建"
        cancelText="取消"
        unmountOnExit
      >
        <Form<TagFormValues>
          form={props.tagForm}
          layout="vertical"
          requiredSymbol={false}
          disabled={props.busy}
          onSubmit={props.onCreateTag}
        >
          <Row gutter={12}>
            <Col xs={24} md={12}>
              <Form.Item field="name" label="名称" rules={[{ required: true, message: "请输入标签名称" }]}>
                <Input placeholder="例如：Go" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item field="slug" label="Slug" rules={[{ required: true, message: "请输入 Slug" }]}>
                <Input placeholder="例如：go" />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item field="description" label="说明">
                <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} placeholder="说明该标签适用的内容主题" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item field="color" label="标签颜色">
                <ColorPicker
                  mode="single"
                  format="hex"
                  showText
                  disabledAlpha
                  showPreset
                  presetColors={tagPresetColors}
                  style={{ width: "100%" }}
                />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal> : null}
    </>
  );
}
