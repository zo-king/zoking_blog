import {
  Button,
  Form,
  Grid,
  Input,
  InputNumber,
  Space,
  Switch,
  type FormInstance
} from "@arco-design/web-react";
import { IconEye, IconSave, IconSend } from "@arco-design/web-react/icon";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import type { SiteSettings } from "../types/admin";

const { Row, Col } = Grid;

const defaultSettings: SiteSettings = {
  site: { title: "Zoking Blog", base_url: "http://localhost:1313/" },
  sidebar: { emoji: "✏️", subtitle: "使用 Stack 设计的全栈博客。" },
  comments: { enabled: true, api_base: "http://localhost:18080" },
  footer: { since: 2026 },
  pagination: { pager_size: 3 }
};

type Props = {
  form: FormInstance<SiteSettings>;
  settings: SiteSettings | null;
  settingsHash: string;
  canUpdate: boolean;
  busy: boolean;
  previewBusy: boolean;
  onSave: (values: SiteSettings) => void;
  onPreview: () => void;
  onPublish: () => void;
};

export function SettingsPage(props: Props) {
  return (
    <>
      <PageHeader
        title="站点设置"
        description="维护公开信息、评论服务和内容展示规则。"
        actions={props.canUpdate ? (
          <Space wrap size={8}>
            <Button icon={<IconSave />} loading={props.busy} onClick={() => props.form.submit()}>保存</Button>
            <Button icon={<IconEye />} loading={props.previewBusy} onClick={props.onPreview}>预览</Button>
            <Button type="primary" icon={<IconSend />} loading={props.busy} onClick={props.onPublish}>发布站点</Button>
          </Space>
        ) : undefined}
      />

      <ContentPanel className="settings-workbench">
        <Form<SiteSettings>
          form={props.form}
          layout="vertical"
          requiredSymbol={false}
          disabled={!props.canUpdate}
          initialValues={props.settings || defaultSettings}
          onSubmit={props.onSave}
        >
          <Row gutter={[28, 18]} align="stretch">
            <Col xs={24} xl={12} style={{ minWidth: 0 }}>
              <section className="settings-section">
                <div className="editor-section-title">站点与侧栏</div>
                <Row gutter={[14, 0]}>
                  <Col xs={24} md={12}>
                    <Form.Item field="site.title" label="站点标题" rules={[{ required: true, message: "请输入站点标题" }]}>
                      <Input placeholder="Zoking Blog" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item field="sidebar.emoji" label="侧栏图标" rules={[{ required: true, message: "请输入侧栏图标" }]}>
                      <Input placeholder="✏️" />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item
                  field="site.base_url"
                  label="站点地址"
                  rules={[{ required: true, message: "请输入站点地址" }, { type: "url", message: "请输入有效的站点地址" }]}
                >
                  <Input placeholder="http://localhost:1313/" />
                </Form.Item>
                <Form.Item field="sidebar.subtitle" label="侧栏副标题" rules={[{ required: true, message: "请输入侧栏副标题" }]}>
                  <Input placeholder="使用 Stack 设计的全栈博客。" />
                </Form.Item>
              </section>
            </Col>

            <Col xs={24} xl={12} style={{ minWidth: 0 }}>
              <section className="settings-section settings-section-secondary">
                <div className="editor-section-title">交互与展示</div>
                <Row gutter={[14, 0]}>
                  <Col xs={24} md={8}>
                    <Form.Item field="comments.enabled" label="启用评论" triggerPropName="checked"><Switch /></Form.Item>
                  </Col>
                  <Col xs={24} md={16}>
                    <Form.Item
                      field="comments.api_base"
                      label="评论 API 地址"
                      rules={[{ required: true, message: "请输入评论 API 地址" }, { type: "url", message: "请输入有效的评论 API 地址" }]}
                    >
                      <Input placeholder="http://localhost:18080" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item field="footer.since" label="页脚起始年份" rules={[{ required: true, message: "请输入页脚起始年份" }]}>
                      <InputNumber min={1900} max={2100} style={{ width: "100%" }} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item field="pagination.pager_size" label="每页文章数" rules={[{ required: true, message: "请输入每页文章数" }]}>
                      <InputNumber min={1} max={50} style={{ width: "100%" }} />
                    </Form.Item>
                  </Col>
                </Row>
                {props.settingsHash ? <div className="settings-version">当前配置版本 {props.settingsHash.slice(0, 12)}</div> : null}
              </section>
            </Col>
          </Row>
        </Form>
      </ContentPanel>
    </>
  );
}
