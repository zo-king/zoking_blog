import { useEffect, useState } from "react";
import { Button, Form, Grid, Input, Modal, Popconfirm, Select, Space, Table, Tabs, Tag, Typography, type FormInstance, type TableColumnProps } from "@arco-design/web-react";
import { IconDelete, IconEdit, IconLock, IconPlus, IconUserAdd } from "@arco-design/web-react/icon";
import { ContentPanel, PageHeader } from "../components/AdminPage";
import { useListQuery } from "../hooks/useListQuery";
import { permissionLabel, roleLabel } from "../labels";
import type { AdminUser, PaginationMeta, SystemRole } from "../types/admin";

const { Row, Col } = Grid;
const { Text } = Typography;
const { TabPane } = Tabs;

type UserValues = { email: string; username: string; display_name: string; password: string; role_codes: string[] };
type RoleValues = { code: string; name: string; description: string; permission_codes: string[] };

type Props = {
  users: AdminUser[];
  userPagination: PaginationMeta;
  roles: SystemRole[];
  permissions: string[];
  userForm: FormInstance<UserValues>;
  roleForm: FormInstance<RoleValues>;
  userBusy: boolean;
  roleBusy: boolean;
  canManageUsers: boolean;
  canReadRoles: boolean;
  canManageRoles: boolean;
  onCreateUser: (values: UserValues) => void;
  onUpdateUserStatus: (userID: string, status: string) => void;
  onUpdateUserRoles: (userID: string, roleCodes: string[]) => void;
  onResetPassword: (user: AdminUser) => void;
  onCreateRole: (values: RoleValues) => void;
  onEditRole: (role: SystemRole) => void;
  onUpdateRolePermissions: (roleID: string, permissionCodes: string[]) => void;
  onDeleteRole: (roleID: string) => void;
};

export function UserManagementPage(props: Props) {
  const listQuery = useListQuery(20);
  const [userQuery, setUserQuery] = useState(listQuery.q);
  const [userModalVisible, setUserModalVisible] = useState(false);
  const [roleModalVisible, setRoleModalVisible] = useState(false);
  const [permissionRole, setPermissionRole] = useState<SystemRole | null>(null);
  const [permissionCodes, setPermissionCodes] = useState<string[]>([]);
  const roleOptions = props.roles.map((role) => ({ label: roleLabel(role.code, role.name), value: role.code }));
  const permissionOptions = props.permissions.map((code) => ({ label: permissionLabel(code), value: code }));

  useEffect(() => setUserQuery(listQuery.q), [listQuery.q]);

  const openUserModal = () => {
    props.userForm.resetFields();
    props.userForm.setFieldValue("role_codes", ["viewer"]);
    setUserModalVisible(true);
  };

  const openRoleModal = () => {
    props.roleForm.resetFields();
    props.roleForm.setFieldValue("permission_codes", []);
    setRoleModalVisible(true);
  };

  const openPermissionModal = (role: SystemRole) => {
    setPermissionRole(role);
    setPermissionCodes(role.permissions);
  };

  const canEditPermissionRole = Boolean(permissionRole && !permissionRole.is_system && props.canManageRoles);

  const userColumns: TableColumnProps<AdminUser>[] = [
    {
      title: "用户",
      width: 250,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text bold>{record.display_name || record.username}</Text>
          <Text type="secondary">{record.email}</Text>
        </Space>
      )
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 90,
      render: (value) => <Tag color={value === "active" ? "green" : "gray"}>{value === "active" ? "启用" : "停用"}</Tag>
    },
    {
      title: "角色",
      width: 360,
      render: (_, record) => props.canManageUsers ? (
        <Select
          mode="multiple"
          style={{ width: 320 }}
          value={record.roles}
          disabled={props.userBusy}
          options={roleOptions}
          onChange={(values: string[]) => props.onUpdateUserRoles(record.id, values)}
        />
      ) : (
        <Space wrap size="mini">
          {record.roles.map((role) => <Tag key={role}>{role}</Tag>)}
        </Space>
      )
    },
    { title: "创建时间", dataIndex: "created_at", width: 180, render: (value) => new Date(value).toLocaleString() },
    {
      title: "操作",
      width: 220,
      fixed: "right",
      render: (_, record) => props.canManageUsers ? (
        <Space size="mini">
          <Popconfirm
            title={`确认${record.status === "active" ? "停用" : "启用"}此用户？`}
            okText={record.status === "active" ? "停用" : "启用"}
            okButtonProps={{ status: record.status === "active" ? "danger" : "default" }}
            onOk={() => props.onUpdateUserStatus(record.id, record.status === "active" ? "disabled" : "active")}
          >
            <Button size="mini" status={record.status === "active" ? "danger" : "default"} loading={props.userBusy}>
              {record.status === "active" ? "停用" : "启用"}
            </Button>
          </Popconfirm>
          <Button size="mini" icon={<IconLock />} disabled={props.userBusy} onClick={() => props.onResetPassword(record)}>
            重置密码
          </Button>
        </Space>
      ) : null
    }
  ];

  const roleColumns: TableColumnProps<SystemRole>[] = [
    {
      title: "角色",
      width: 220,
      render: (_, record) => (
        <Space size="mini">
          <Text bold>{roleLabel(record.code, record.name)}</Text>
          <Tag>{record.code}</Tag>
        </Space>
      )
    },
    {
      title: "类型",
      width: 120,
      render: (_, record) => record.is_system ? <Tag color="arcoblue">系统角色</Tag> : <Tag>自定义角色</Tag>
    },
    { title: "说明", dataIndex: "description", width: 240, ellipsis: true, render: (value) => value || "-" },
    {
      title: "权限",
      width: 190,
      render: (_, record) => (
        <Space size="small">
          <Text type="secondary">{record.permissions.length} 项</Text>
          <Button
            size="mini"
            icon={!record.is_system && props.canManageRoles ? <IconEdit /> : undefined}
            disabled={props.roleBusy}
            onClick={() => openPermissionModal(record)}
          >
            {!record.is_system && props.canManageRoles ? "编辑权限" : "查看权限"}
          </Button>
        </Space>
      )
    },
    {
      title: "操作",
      width: 170,
      fixed: "right",
      render: (_, record) => !record.is_system && props.canManageRoles ? (
        <Space size="mini">
          <Button size="mini" icon={<IconEdit />} disabled={props.roleBusy} onClick={() => props.onEditRole(record)}>编辑</Button>
          <Popconfirm
            title="确认删除此角色？"
            content="已分配给用户的角色不能删除。"
            okText="删除"
            okButtonProps={{ status: "danger" }}
            onOk={() => props.onDeleteRole(record.id)}
          >
            <Button size="mini" status="danger" icon={<IconDelete />} loading={props.roleBusy}>删除</Button>
          </Popconfirm>
        </Space>
      ) : (
        <Text type="secondary">只读</Text>
      )
    }
  ];

  return (
    <>
      <PageHeader
        title="用户与权限"
        description="管理后台账号、角色与访问边界。"
      />

      <Tabs defaultActiveTab="accounts">
        <TabPane key="accounts" title={`账号 (${props.userPagination.total})`}>
          <ContentPanel
            title="账号管理"
            description="调整后台账号的启停状态、角色与登录密码。"
            actions={props.canManageUsers ? (
              <Button type="primary" size="small" icon={<IconUserAdd />} onClick={openUserModal}>创建账号</Button>
            ) : undefined}
          >
            <Space wrap size="small" style={{ marginBottom: 16 }}>
              <Input.Search
                allowClear
                value={userQuery}
                placeholder="搜索账号"
                style={{ width: 240 }}
                onChange={(value) => {
                  setUserQuery(value);
                  if (!value) listQuery.update({ q: "" });
                }}
                onSearch={(value) => listQuery.update({ q: value.trim() })}
              />
              <Select
                allowClear
                value={listQuery.status || undefined}
                placeholder="账号状态"
                style={{ width: 140 }}
                options={[
                  { label: "启用", value: "active" },
                  { label: "停用", value: "disabled" }
                ]}
                onChange={(value) => listQuery.update({ status: value || "" })}
              />
            </Space>
            <Table
              rowKey="id"
              data={props.users}
              columns={userColumns}
              pagination={{
                current: listQuery.page,
                pageSize: listQuery.pageSize,
                total: props.userPagination.total,
                hideOnSinglePage: true,
                size: "small",
                showTotal: true,
                onChange: (page, pageSize) => listQuery.update({ page, pageSize })
              }}
              size="small"
              scroll={{ x: 1100 }}
            />
          </ContentPanel>
        </TabPane>

        {props.canReadRoles ? (
          <TabPane key="roles" title={`角色 (${props.roles.length})`}>
            <ContentPanel
              title="角色与权限"
              description="维护角色定义及其权限集合，系统角色保持只读。"
              actions={props.canManageRoles ? (
                <Button type="primary" size="small" icon={<IconPlus />} onClick={openRoleModal}>创建角色</Button>
              ) : undefined}
            >
              <Table
                rowKey="id"
                data={props.roles}
                columns={roleColumns}
                pagination={{ pageSize: 8, hideOnSinglePage: true, size: "small" }}
                size="small"
                scroll={{ x: 940 }}
              />
            </ContentPanel>
          </TabPane>
        ) : null}
      </Tabs>

      <Modal
        title="创建账号"
        visible={userModalVisible}
        onCancel={() => setUserModalVisible(false)}
        onOk={() => props.userForm.submit()}
        confirmLoading={props.userBusy}
        okText="创建"
        cancelText="取消"
        unmountOnExit
        style={{ width: 720 }}
      >
        <Form
          form={props.userForm}
          layout="vertical"
          initialValues={{ role_codes: ["viewer"] }}
          scrollToFirstError
          onSubmit={(values) => {
            props.onCreateUser(values);
            setUserModalVisible(false);
          }}
        >
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item field="email" label="邮箱" rules={[{ required: true, type: "email", message: "请输入有效邮箱" }]}>
                <Input placeholder="name@example.com" autoComplete="email" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item field="username" label="用户名" rules={[{ required: true, minLength: 3, message: "用户名至少 3 个字符" }]}>
                <Input placeholder="zhangsan" autoComplete="username" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item field="display_name" label="显示名称">
                <Input placeholder="张三" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item field="password" label="初始密码" rules={[{ required: true, minLength: 10, maxLength: 72, message: "密码长度需为 10 至 72 个字符" }]}>
                <Input.Password autoComplete="new-password" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item field="role_codes" label="角色" rules={[{ required: true, message: "请至少选择一个角色" }]}>
            <Select mode="multiple" options={roleOptions} placeholder="选择角色" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="创建角色"
        visible={roleModalVisible}
        onCancel={() => setRoleModalVisible(false)}
        onOk={() => props.roleForm.submit()}
        confirmLoading={props.roleBusy}
        okText="创建"
        cancelText="取消"
        unmountOnExit
        style={{ width: 680 }}
      >
        <Form
          form={props.roleForm}
          layout="vertical"
          initialValues={{ permission_codes: [] }}
          scrollToFirstError
          onSubmit={(values) => {
            props.onCreateRole(values);
            setRoleModalVisible(false);
          }}
        >
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item
                field="code"
                label="角色编码"
                rules={[{ required: true, match: /^[a-z][a-z0-9_]{2,63}$/, message: "请输入 3 至 64 位小写字母、数字或下划线" }]}
              >
                <Input placeholder="content_editor" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item field="name" label="角色名称" rules={[{ required: true, message: "请输入角色名称" }]}>
                <Input placeholder="内容编辑" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item field="description" label="说明">
            <Input placeholder="负责文章与页面内容" />
          </Form.Item>
          <Form.Item field="permission_codes" label="权限">
            <Select mode="multiple" allowClear options={permissionOptions} placeholder="选择权限" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`${canEditPermissionRole ? "编辑" : "查看"}权限：${permissionRole ? roleLabel(permissionRole.code, permissionRole.name) : ""}`}
        visible={Boolean(permissionRole)}
        onCancel={() => setPermissionRole(null)}
        onOk={() => {
          if (permissionRole && canEditPermissionRole) {
            props.onUpdateRolePermissions(permissionRole.id, permissionCodes);
          }
          setPermissionRole(null);
        }}
        confirmLoading={props.roleBusy}
        okText={canEditPermissionRole ? "保存" : "关闭"}
        cancelText="取消"
        unmountOnExit
        style={{ width: 680 }}
      >
        <Select
          mode="multiple"
          allowClear={canEditPermissionRole}
          disabled={!canEditPermissionRole || props.roleBusy}
          options={permissionOptions}
          value={permissionCodes}
          onChange={(values: string[]) => setPermissionCodes(values)}
          placeholder="暂无权限"
          style={{ width: "100%" }}
        />
      </Modal>
    </>
  );
}
