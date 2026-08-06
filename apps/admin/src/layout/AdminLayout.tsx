import { Avatar, Breadcrumb, Button, Divider, Drawer, Dropdown, Layout, Menu, Tooltip } from "@arco-design/web-react";
import {
  IconBook,
  IconLaunch,
  IconMenu,
  IconMenuFold,
  IconMenuUnfold,
  IconPoweroff,
  IconRefresh,
} from "@arco-design/web-react/icon";
import { useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { roleLabel } from "../labels";
import type { AdminSection, CurrentUser } from "../types/admin";

const { Header, Sider, Content } = Layout;
const siteUrl =
  window.__ZOKING_ADMIN_CONFIG__?.siteBaseUrl?.trim() ||
  import.meta.env.VITE_SITE_URL ||
  (import.meta.env.DEV ? "http://localhost:1313" : "https://zoking.tech");

export type AdminNavItem = {
  key: string;
  icon: ReactNode;
  label: string;
  group: "工作区" | "管理";
};

type Props = {
  section: AdminSection;
  navItems: AdminNavItem[];
  currentUser: CurrentUser | null;
  loggedIn: boolean;
  children: ReactNode;
  onLogout: () => void;
  onRefresh: () => void;
};

export function AdminLayout(props: Props) {
  const navigate = useNavigate();
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);
  const selectedKey = `/${props.section}`;
  const navigateTo = (key: string) => {
    navigate(key);
    setMobileNavOpen(false);
  };
  const groups = ["工作区", "管理"] as const;
  const activeItem = props.navItems.find((item) => item.key === selectedKey);
  const userName = props.currentUser?.email || "管理员";
  const userInitial = userName.trim().charAt(0).toUpperCase() || "Z";
  const roleText = props.currentUser?.roles.map((role) => roleLabel(role)).join("、") || "未分配角色";

  const renderNavigation = (collapse: boolean) => (
    <nav className="admin-navigation" aria-label="后台主导航">
      {groups.map((group) => {
        const items = props.navItems.filter((item) => item.group === group);
        if (!items.length) return null;
        return (
          <div className="nav-section" key={group}>
            {!collapse && <div className="nav-section-label">{group}</div>}
            <Menu selectedKeys={[selectedKey]} onClickMenuItem={navigateTo} collapse={collapse} tooltipProps={{}}>
              {items.map((item) => (
                <Menu.Item key={item.key}>
                  {item.icon}
                  <span>{item.label}</span>
                </Menu.Item>
              ))}
            </Menu>
          </div>
        );
      })}
    </nav>
  );

  const userMenu = (
    <div className="user-menu">
      <div className="user-menu-summary">
        <strong>{userName}</strong>
        <span>{roleText}</span>
      </div>
      <Menu
        onClickMenuItem={(key) => {
          if (key === "logout") props.onLogout();
          if (key === "site") window.open(siteUrl, "_blank", "noopener,noreferrer");
        }}
      >
        <Menu.Item key="site">
          <IconLaunch /> 查看博客
        </Menu.Item>
        {props.loggedIn ? (
          <Menu.Item key="logout" className="user-menu-logout">
            <IconPoweroff /> 退出登录
          </Menu.Item>
        ) : null}
      </Menu>
    </div>
  );

  return (
    <Layout className="app-shell">
      <Sider
        width={220}
        collapsedWidth={64}
        collapsed={collapsed}
        className={`app-sider${collapsed ? " app-sider-collapsed" : ""}`}
      >
        <div className="brand">
          <span className="brand-mark">
            <IconBook />
          </span>
          {!collapsed && (
            <span>
              <strong>Zoking</strong>
              <small>内容管理平台</small>
            </span>
          )}
        </div>
        {renderNavigation(collapsed)}
        <button
          type="button"
          className="sider-collapse-trigger"
          onClick={() => setCollapsed((value) => !value)}
          aria-label={collapsed ? "展开导航" : "收起导航"}
        >
          {collapsed ? <IconMenuUnfold /> : <IconMenuFold />}
        </button>
      </Sider>
      <Drawer
        title="后台导航"
        placement="left"
        width={288}
        visible={mobileNavOpen}
        footer={null}
        onCancel={() => setMobileNavOpen(false)}
        className="mobile-nav-drawer"
      >
        {renderNavigation(false)}
      </Drawer>
      <Layout>
        <Header className="app-header">
          <div className="header-title-group">
            <Button
              className="mobile-nav-trigger"
              type="text"
              icon={<IconMenu />}
              onClick={() => setMobileNavOpen(true)}
              aria-label="打开导航"
            />
            <Breadcrumb className="header-breadcrumb">
              <Breadcrumb.Item>个人博客</Breadcrumb.Item>
              {activeItem ? <Breadcrumb.Item>{activeItem.group}</Breadcrumb.Item> : null}
              <Breadcrumb.Item>{activeItem?.label ?? "工作台"}</Breadcrumb.Item>
            </Breadcrumb>
          </div>
          <div className="header-actions">
            <Tooltip content="查看博客">
              <Button
                type="text"
                icon={<IconLaunch />}
                aria-label="查看博客"
                onClick={() => window.open(siteUrl, "_blank", "noopener,noreferrer")}
              />
            </Tooltip>
            <Tooltip content="刷新数据">
              <Button type="text" icon={<IconRefresh />} aria-label="刷新数据" onClick={props.onRefresh} />
            </Tooltip>
            <Divider type="vertical" />
            <Dropdown droplist={userMenu} position="br" trigger="click">
              <button type="button" className="header-user" aria-label="账号菜单">
                <Avatar size={30}>{userInitial}</Avatar>
                <span className="header-user-name">{userName}</span>
              </button>
            </Dropdown>
          </div>
        </Header>
        <Content className="app-content">
          <div className="content-frame">{props.children}</div>
        </Content>
      </Layout>
    </Layout>
  );
}
