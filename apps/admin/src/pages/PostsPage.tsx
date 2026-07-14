import {
  Avatar,
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
import { IconDelete, IconEdit, IconEye, IconLeft, IconPlus, IconSafe, IconSave, IconSend } from "@arco-design/web-react/icon";
import { useEffect, useState } from "react";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import { useListQuery } from "../hooks/useListQuery";
import type { MediaAsset, PaginationMeta, Post, PostFormValues } from "../types/admin";

const { Row, Col } = Grid;
const { Text } = Typography;

type Option = { label: string; value: string; disabled?: boolean };

type Props = {
  posts: Post[];
  pagination: PaginationMeta;
  form: FormInstance<PostFormValues>;
  enabled: boolean;
  canCreate: boolean;
  canUpdate: boolean;
  canDelete: boolean;
  canSave: boolean;
  canPreview: boolean;
  canPublish: boolean;
  writeBlocked: boolean;
  busy: boolean;
  deletingPostID: string;
  previewBusy: boolean;
  qualityBusy: boolean;
  media: MediaAsset[];
  mediaURL: (asset: MediaAsset) => string;
  categoryOptions: Option[];
  tagOptions: Option[];
  seriesOptions: Option[];
  mode: "list" | "editor";
  onNew: () => void;
  onSelect: (post: Post) => void;
  onBack: () => void;
  onDelete: (id: string) => Promise<void>;
  onSave: (values: PostFormValues) => void;
  onPreview: () => void;
  onPublish: () => void;
  onQualityCheck: () => void;
  onFormChange: () => void;
};

function formatDate(value?: string | null) {
  if (!value) return "尚未发布";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "-" : date.toLocaleString("zh-CN", { hour12: false });
}

export function PostsPage(props: Props) {
  const listQuery = useListQuery(20);
  const selectedSeriesID = Form.useWatch("series_id", props.form);
  const [keyword, setKeyword] = useState(listQuery.q);

  useEffect(() => {
    setKeyword(listQuery.q);
  }, [listQuery.q]);

  const coverOptions = props.media
    .filter((asset) => asset.status === "ready" && asset.mime_type.startsWith("image/"))
    .map((asset) => ({
      value: asset.id,
      label: (
        <Space size={8}>
          <Avatar shape="square" size={28}>
            <img src={props.mediaURL(asset)} alt="" />
          </Avatar>
          <Text ellipsis style={{ display: "block", maxWidth: 260 }}>
            {asset.original_name || asset.filename}
          </Text>
        </Space>
      )
    }));

  if (props.mode === "list") {
    return (
      <>
        <PageHeader
          title="文章"
          description="管理草稿、已发布内容和文章归类。"
          actions={
            props.canCreate ? <Button type="primary" icon={<IconPlus />} onClick={props.onNew}>
              新建文章
            </Button> : null
          }
        />

        <ContentPanel className="list-workbench">
          <div className="page-toolbar">
            <Input.Search
              allowClear
              value={keyword}
              placeholder="搜索标题、摘要或 Slug"
              onChange={(value) => {
                setKeyword(value);
                if (!value) listQuery.update({ q: "" });
              }}
              onSearch={(value) => listQuery.update({ q: value.trim() })}
            />
            <Select
              value={listQuery.status || "all"}
              onChange={(value) => listQuery.update({ status: value === "all" ? "" : value })}
              options={[
                { label: "全部状态", value: "all" },
                { label: "草稿", value: "draft" },
                { label: "已发布", value: "published" }
              ]}
            />
            <Text type="secondary">共 {props.pagination.total} 篇文章</Text>
          </div>

          <Table<Post>
            rowKey="id"
            data={props.posts}
            size="small"
            tableLayoutFixed
            pagination={{
              current: listQuery.page,
              pageSize: listQuery.pageSize,
              total: props.pagination.total,
              hideOnSinglePage: true,
              showTotal: true,
              size: "small",
              sizeCanChange: true,
              onChange: (page, pageSize) => listQuery.update({ page, pageSize })
            }}
            scroll={{ x: 1040 }}
            noDataElement={<Empty description="没有符合条件的文章。" />}
            onRow={(record) => props.canUpdate ? ({ onClick: () => props.onSelect(record), style: { cursor: "pointer" } }) : {}}
            columns={[
              {
                title: "文章",
                dataIndex: "title",
                width: 360,
                render: (_, record) => (
                  <div style={{ minWidth: 0 }}>
                    <Text ellipsis style={{ display: "block" }}><strong>{record.title || "未命名文章"}</strong></Text>
                    <Text type="secondary" ellipsis style={{ display: "block", marginTop: 2 }}>{record.summary || record.slug}</Text>
                  </div>
                )
              },
              {
                title: "状态",
                dataIndex: "status",
                width: 100,
                render: (value) => <Tag color={value === "published" ? "green" : "orange"}>{value === "published" ? "已发布" : "草稿"}</Tag>
              },
              {
                title: "内容归类",
                width: 300,
                render: (_, record) => {
                  const items = [...(record.categories || []), ...(record.tags || [])];
                  if (!items.length) return <Text type="secondary">未分类</Text>;
                  return (
                    <Space size={4}>
                      {items.slice(0, 3).map((item) => <Tag key={`${item.id}-${item.name}`}>{item.name}</Tag>)}
                      {items.length > 3 ? <Text type="secondary">+{items.length - 3}</Text> : null}
                    </Space>
                  );
                }
              },
              { title: "发布时间", dataIndex: "published_at", width: 190, render: (value) => formatDate(value) },
              {
                title: "操作",
                width: 90,
                fixed: "right",
                render: (_, record) => {
                  const deleting = props.deletingPostID === record.id;
                  return (
                    <div onClick={(event) => event.stopPropagation()}>
                      <Space size={2}>
                        {props.canUpdate && <Tooltip content="编辑"><Button type="text" size="mini" icon={<IconEdit />} aria-label="编辑文章" onClick={() => props.onSelect(record)} /></Tooltip>}
                        {props.canDelete && <Popconfirm
                          title="确认将此文章移出线上内容？"
                          okText="归档"
                          disabled={!props.enabled || Boolean(props.deletingPostID)}
                          okButtonProps={{ status: "danger", loading: deleting }}
                          onOk={() => props.onDelete(record.id)}
                        >
                          <Tooltip content="归档"><Button type="text" size="mini" status="danger" icon={<IconDelete />} aria-label="归档文章" loading={deleting} disabled={!props.enabled || Boolean(props.deletingPostID)} /></Tooltip>
                        </Popconfirm>}
                      </Space>
                    </div>
                  );
                }
              }
            ]}
          />
        </ContentPanel>
      </>
    );
  }

  return (
    <>
      <PageHeader
        title="文章编辑"
        description="专注正文创作，发布设置集中在右侧。"
        actions={
          <Space wrap size={4}>
            <Button icon={<IconLeft />} onClick={props.onBack}>返回列表</Button>
            <Button icon={<IconSave />} disabled={!props.canSave || props.writeBlocked} loading={props.busy} onClick={() => props.form.submit()}>保存</Button>
            <Button icon={<IconEye />} disabled={!props.canPreview || props.writeBlocked} loading={props.previewBusy} onClick={props.onPreview}>预览</Button>
            <Button icon={<IconSafe />} disabled={!props.canSave || props.writeBlocked} loading={props.qualityBusy} onClick={props.onQualityCheck}>发布检查</Button>
            {props.canPublish && <Button type="primary" icon={<IconSend />} loading={props.busy || props.qualityBusy} onClick={props.onPublish}>发布</Button>}
          </Space>
        }
      />

      <ContentPanel className="editor-workbench">
        <Form<PostFormValues> form={props.form} layout="vertical" requiredSymbol={false} onSubmit={props.onSave} onValuesChange={(changed) => {
          if (Object.prototype.hasOwnProperty.call(changed, "series_id") && !changed.series_id) props.form.setFieldValue("series_order", null);
          props.onFormChange();
        }}>
          <Row gutter={[22, 18]} align="stretch">
            <Col xs={24} xl={17} style={{ minWidth: 0 }}>
              <div className="editor-main">
                <Form.Item field="title" label="标题" rules={[{ required: true, message: "请输入文章标题" }]}>
                  <Input size="large" placeholder="输入文章标题" />
                </Form.Item>
                <Form.Item field="summary" label="摘要"><Input placeholder="用一句话概括文章" /></Form.Item>
                <Form.Item field="content_md" label="Markdown 正文">
                  <Input.TextArea autoSize={{ minRows: 17, maxRows: 32 }} placeholder="在这里编写 Markdown 正文。" />
                </Form.Item>
              </div>
            </Col>

            <Col xs={24} xl={7} style={{ minWidth: 0 }}>
              <aside className="editor-sidebar">
                <div className="editor-section-title">发布设置</div>
                <Form.Item field="status" label="状态">
                  <Select disabled={!props.canPublish} options={[{ label: "草稿", value: "draft" }, { label: "已发布", value: "published" }]} />
                </Form.Item>
                <Form.Item field="visibility" label="可见性">
                  <Select options={[{ label: "公开", value: "public" }, { label: "私密", value: "private" }, { label: "不公开列出", value: "unlisted" }]} />
                </Form.Item>
                <Form.Item field="allow_comment" label="允许评论" triggerPropName="checked"><Switch /></Form.Item>
                <Form.Item field="slug" label="Slug" rules={[{ required: true, message: "请输入文章 Slug" }]}><Input placeholder="article-slug" /></Form.Item>
                <Form.Item field="cover_media_id" label="封面图">
                  <Select allowClear placeholder="从媒体库选择" options={coverOptions} notFoundContent="媒体库中暂无图片" />
                </Form.Item>

                <div className="editor-section-title">内容组织</div>
                <Form.Item field="category_ids" label="分类"><Select mode="multiple" allowClear placeholder="选择分类" options={props.categoryOptions} /></Form.Item>
                <Form.Item field="tag_ids" label="标签"><Select mode="multiple" allowClear placeholder="选择标签" options={props.tagOptions} /></Form.Item>
                <Form.Item field="series_id" label="系列">
                  <Select allowClear placeholder="不加入系列" options={props.seriesOptions} onChange={(value) => {
                    if (!value) props.form.setFieldValue("series_order", null);
                  }} />
                </Form.Item>
                <Form.Item
                  field="series_order"
                  label="系列序号"
                  dependencies={["series_id"]}
                  rules={[{ validator: (value, callback) => {
                    if (!selectedSeriesID) { callback(); return; }
                    if (!Number.isInteger(value) || Number(value) < 1) callback("请选择大于等于 1 的正整数");
                    else callback();
                  } }]}
                >
                  <InputNumber min={1} precision={0} mode="button" disabled={!selectedSeriesID} style={{ width: "100%" }} placeholder="例如：1" />
                </Form.Item>
                <Form.Item field="seo_description" label="搜索摘要"><Input.TextArea rows={3} placeholder="用于搜索结果的页面描述" /></Form.Item>
              </aside>
            </Col>
          </Row>
        </Form>
      </ContentPanel>
    </>
  );
}
